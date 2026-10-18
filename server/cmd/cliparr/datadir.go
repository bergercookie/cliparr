package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// checkWritable fails early, with advice, when Cliparr cannot write to its data
// folder or open its database there. Without it the first sign of the problem is
// SQLite's "unable to open database file (14)", which says nothing about the cause:
// the process runs as another user than the one who owns the folder.
func checkWritable(dir, databaseFile string) error {
	probe, err := os.CreateTemp(dir, ".cliparr-write-test-*")
	if err != nil {
		return notWritable(fmt.Sprintf("the data folder %s is not writable", dir), err)
	}
	_ = probe.Close()
	if err := os.Remove(probe.Name()); err != nil {
		return notWritable(fmt.Sprintf("the data folder %s is not writable", dir), err)
	}

	// A database made earlier by another user can be unwritable inside a writable folder.
	root, err := os.OpenRoot(dir)
	if err != nil {
		return notWritable(fmt.Sprintf("the data folder %s cannot be opened", dir), err)
	}
	defer root.Close()
	db, err := root.OpenFile(databaseFile, os.O_RDWR, 0)
	switch {
	case err == nil:
		_ = db.Close()
	case !errors.Is(err, fs.ErrNotExist):
		return notWritable(fmt.Sprintf("the database %s cannot be opened for writing", filepath.Join(dir, databaseFile)), err)
	}
	return nil
}

func notWritable(problem string, cause error) error {
	return fmt.Errorf("%s by this process (uid %d, gid %d): %w. If the container starts as root, set PUID and PGID "+
		"in the Compose file to your own user and group (`id -u`, `id -g`) and Cliparr takes care of the folder; "+
		`if you set user: "UID:GID" yourself, make the folder belong to that user. `+
		"See Troubleshooting in the documentation", problem, os.Getuid(), os.Getgid(), cause)
}
