//go:build linux

package main

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// recorder stands in for chown: it only notes what it was asked to change.
type recorder struct {
	paths []string
	fail  map[string]bool
}

func (r *recorder) chown(path string, _, _ int) error {
	if r.fail[path] {
		return errors.New("operation not permitted")
	}
	r.paths = append(r.paths, path)
	return nil
}

// otherID is a user and group that differ from the test's own, so everything needs changing.
func otherID() identity { return identity{uid: os.Getuid() + 1, gid: os.Getgid() + 1} }

func makeTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, p := range []string{"cliparr.db", "thumbnails/a.jpg", "thumbnails/b.jpg"} {
		path := filepath.Join(dir, p)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func relative(dir string, paths []string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = strings.TrimPrefix(strings.TrimPrefix(p, dir), "/")
	}
	sort.Strings(out)
	return out
}

func TestChownTreeGivesEverythingToTheChosenUser(t *testing.T) {
	dir := makeTree(t)
	rec := &recorder{}

	changed, failed, err := chownTree(dir, otherID(), rec.chown)

	if failed != 0 || err != nil {
		t.Fatalf("failed = %d, err = %v", failed, err)
	}
	want := []string{"", "cliparr.db", "thumbnails", "thumbnails/a.jpg", "thumbnails/b.jpg"}
	if got := relative(dir, rec.paths); strings.Join(got, ",") != strings.Join(want, ",") || changed != len(want) {
		t.Errorf("changed %d: %q, want the folder and everything in it: %q", changed, got, want)
	}
}

func TestChownTreeLeavesAlreadyRightFilesAlone(t *testing.T) {
	dir := makeTree(t)
	rec := &recorder{}

	changed, _, _ := chownTree(dir, identity{uid: os.Getuid(), gid: os.Getgid()}, rec.chown)

	if changed != 0 || len(rec.paths) != 0 {
		t.Errorf("everything already belongs to the user, yet %d changes: %q", changed, rec.paths)
	}
}

func TestChownTreeNeverFollowsSymbolicLinks(t *testing.T) {
	dir := makeTree(t)
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret")
	if err := os.WriteFile(secret, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A link to a file and a link to a whole folder elsewhere.
	if err := os.Symlink(secret, filepath.Join(dir, "link-to-file")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link-to-folder")); err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}

	if _, failed, err := chownTree(dir, otherID(), rec.chown); failed != 0 || err != nil {
		t.Fatalf("failed = %d, err = %v", failed, err)
	}

	for _, p := range rec.paths {
		if strings.HasPrefix(p, outside) {
			t.Errorf("changed %s, which is outside the data folder", p)
		}
	}
	got := strings.Join(relative(dir, rec.paths), ",")
	if !strings.Contains(got, "link-to-file") || !strings.Contains(got, "link-to-folder") {
		t.Errorf("the links themselves should be handed over (chown without following): %s", got)
	}
}

func TestChownTreeKeepsGoingWhenSomethingRefuses(t *testing.T) {
	dir := makeTree(t)
	refused := filepath.Join(dir, "cliparr.db")
	rec := &recorder{fail: map[string]bool{refused: true}}

	changed, failed, err := chownTree(dir, otherID(), rec.chown)

	if failed != 1 || err == nil || !strings.Contains(err.Error(), "not permitted") {
		t.Errorf("failed = %d, err = %v", failed, err)
	}
	if changed != 4 {
		t.Errorf("the other four entries should still be handed over, changed %d", changed)
	}
}
