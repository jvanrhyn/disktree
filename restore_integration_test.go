package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMoveAndRestoreFile(t *testing.T) {
	// setup temp dir
	tmp, err := os.MkdirTemp("", "disktree-test-")
	if err != nil {
		t.Fatalf("mktemp: %v", err)
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmp)

	// create a file
	fpath := filepath.Join(tmp, "hello.txt")
	if err := os.WriteFile(fpath, []byte("hello"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// move to trash
	ti, err := moveToTrash(fpath)
	if err != nil {
		t.Fatalf("moveToTrash: %v", err)
	}
	if ti == nil {
		t.Fatalf("expected TrashItem, got nil")
	}
	// trashed file should exist
	if _, err := os.Stat(ti.TrashPath); err != nil {
		t.Fatalf("trashed file missing: %v", err)
	}

	// restore
	if err := restoreFromTrash(ti); err != nil {
		t.Fatalf("restoreFromTrash: %v", err)
	}
	// restored path should exist (may be original or with suffix)
	if _, err := os.Stat(ti.OrigPath); err == nil {
		// good: restored to original location
		return
	} else {
		// if not at original, check for a suffixed path in tmp
		// scan tmp for a file starting with the base name
		base := filepath.Base(ti.OrigPath)
		found := false
		ents, _ := os.ReadDir(tmp)
		for _, e := range ents {
			if e.Name() == base || strings.HasPrefix(e.Name(), base+"-") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("restored file not found in tmp")
		}
	}
}
