//go:build linux

package main

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
)

// handOverDataDir lets a container that starts as root give the data folder to the
// user the operator chose (PUID and PGID), and then gives up root for good, before
// Cliparr opens its database or a single connection. That way everything Cliparr
// writes belongs to the operator, who can back the folder up without sudo, even
// when Docker created the folder as root.
//
// A process that does not start as root (the container run with `user:`) has
// nothing to hand over and is left alone.
func handOverDataDir(log *slog.Logger, dir string, getenv func(string) string) error {
	if os.Geteuid() != 0 {
		return nil
	}
	id, err := identityFromEnv(getenv)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}
	// Resolve the folder once, here, so nothing below depends on a link changing.
	real, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return fmt.Errorf("resolve data dir: %w", err)
	}

	changed, failed, firstErr := chownTree(real, id, os.Lchown)
	if changed > 0 {
		log.Info("gave the data folder to the configured user", "dir", dir, "uid", id.uid, "gid", id.gid, "changed", changed)
	}
	if failed > 0 {
		// Some file systems (NFS with root squash, for one) refuse. Carry on: if Cliparr can still
		// write, nothing is wrong; if not, the check after this says so.
		log.Warn("could not change the owner of some files in the data folder", "dir", dir, "failed", failed, "err", firstErr)
	}
	return dropPrivileges(id)
}

// chownTree gives dir and everything below it to id, leaving alone what already belongs
// to it. It never follows a symbolic link: it runs as root, and a link inside the folder
// must not make it change the owner of a file somewhere else.
func chownTree(dir string, id identity, chown func(path string, uid, gid int) error) (changed, failed int, firstErr error) {
	_ = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			failed++
			firstErr = cmpFirst(firstErr, walkErr)
			return nil
		}
		info, err := entry.Info() // describes a link itself, not what it points to
		if err != nil {
			failed++
			firstErr = cmpFirst(firstErr, err)
			return nil
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) == id.uid && int(st.Gid) == id.gid {
			return nil
		}
		if err := chown(path, id.uid, id.gid); err != nil {
			failed++
			firstErr = cmpFirst(firstErr, err)
			return nil
		}
		changed++
		return nil
	})
	return changed, failed, firstErr
}

func cmpFirst(first, next error) error {
	if first != nil {
		return first
	}
	return next
}

// dropPrivileges makes the process the given user and group, for good, and checks
// that it worked: a process that could still become root would defeat the point.
func dropPrivileges(id identity) error {
	if err := syscall.Setgroups([]int{}); err != nil {
		return fmt.Errorf("drop supplementary groups: %w", err)
	}
	if err := syscall.Setgid(id.gid); err != nil {
		return fmt.Errorf("switch to group %d: %w", id.gid, err)
	}
	if err := syscall.Setuid(id.uid); err != nil {
		return fmt.Errorf("switch to user %d: %w", id.uid, err)
	}
	if os.Geteuid() != id.uid || os.Getegid() != id.gid {
		return fmt.Errorf("running as %d:%d after switching to %d:%d", os.Geteuid(), os.Getegid(), id.uid, id.gid)
	}
	if syscall.Setuid(0) == nil {
		return fmt.Errorf("the process can still become root after switching to user %d", id.uid)
	}
	return nil
}
