package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckWritableAcceptsAFolderItCanUse(t *testing.T) {
	dir := t.TempDir()
	if err := checkWritable(dir, "cliparr.db"); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("the check should leave nothing behind, found %d entries", len(entries))
	}

	if err := os.WriteFile(filepath.Join(dir, "cliparr.db"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkWritable(dir, "cliparr.db"); err != nil {
		t.Errorf("an existing database it can write to: %v", err)
	}
}

func TestCheckWritableExplainsWhatToDo(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nowhere")
	err := checkWritable(missing, "cliparr.db")
	if err == nil {
		t.Fatal("a folder that cannot be written to should be an error")
	}
	for _, want := range []string{missing, "uid", "gid", "user:", "Troubleshooting"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the message should mention %q: %v", want, err)
		}
	}
}

func TestCheckWritableNoticesAFolderOfAnotherUser(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere, so there is no other user's folder to find")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	if err := checkWritable(dir, "cliparr.db"); err == nil || !strings.Contains(err.Error(), "not writable") {
		t.Errorf("want a not-writable error, got %v", err)
	}
}

func TestCheckWritableNoticesADatabaseItCannotOpen(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can open any file")
	}
	dir := t.TempDir()
	db := filepath.Join(dir, "cliparr.db")
	if err := os.WriteFile(db, nil, 0o400); err != nil {
		t.Fatal(err)
	}

	err := checkWritable(dir, "cliparr.db")
	if err == nil || !strings.Contains(err.Error(), "cannot be opened for writing") {
		t.Errorf("want a database error, got %v", err)
	}
}
