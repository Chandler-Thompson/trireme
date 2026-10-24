package store

import (
	"path/filepath"
	"testing"
)

func TestOpen_EmptyPath_ReturnsError(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatal("expected an error for an empty path")
	}
}

func TestOpen_NewFile_CreatesAllTables(t *testing.T) {
	db, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, table := range []string{"events", "blockers", "inbox", "transcript", "sessions", "caps", "fleets"} {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Errorf("table %s missing: %v", table, err)
		}
	}
}

func TestOpen_ExistingFile_ReopensWithoutError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "t.db")
	first, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	first.Close()
	second, err := Open(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	second.Close()
}

func TestOpen_PathInMissingDirectory_ReturnsError(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "no", "such", "dir", "t.db")); err == nil {
		t.Fatal("expected an error for a path whose directory does not exist")
	}
}
