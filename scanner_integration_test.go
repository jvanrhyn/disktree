package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestScannerIntegration(t *testing.T) {
	// reset in-memory cache between tests
	cache = sync.Map{}

	tmp, err := os.MkdirTemp("", "disktree-integ-")
	if err != nil {
		t.Fatal(err)
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmp)

	// Create tree:
	// tmp/
	//   a/
	//     file1 (100 bytes)
	//     b/
	//       file2 (200 bytes)
	//   file3 (300 bytes)
	if err := os.MkdirAll(filepath.Join(tmp, "a", "b"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmp, "a", "file1"), bytes.Repeat([]byte{'A'}, 100), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "a", "b", "file2"), bytes.Repeat([]byte{'B'}, 200), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "file3"), bytes.Repeat([]byte{'C'}, 300), 0644); err != nil {
		t.Fatal(err)
	}

	s := &Scanner{threads: 4, followSymlinks: false}

	res := s.sumDir(context.Background(), tmp)

	expFiles := int64(3)
	expDirs := int64(2) // total dirs in subtree (a and a/b)
	expSize := int64(100 + 200 + 300)

	if res.files != expFiles {
		t.Fatalf("sumDir files = %d; want %d", res.files, expFiles)
	}
	if res.dirs != expDirs {
		t.Fatalf("sumDir dirs = %d; want %d", res.dirs, expDirs)
	}
	if res.size != expSize {
		t.Fatalf("sumDir size = %d; want %d", res.size, expSize)
	}

	// scanDir should produce a Node with matching totals for sizes/files.
	// Note: scanDir stores nested dir counts in children (it does not count the immediate
	// child directory itself when aggregating into the parent node), so node.Dirs will be
	// one less than the total subtree dir count in this layout.
	node := s.scanDir(context.Background(), tmp)
	if node.Size != expSize {
		t.Fatalf("scanDir size = %d; want %d", node.Size, expSize)
	}
	if node.Files != expFiles {
		t.Fatalf("scanDir files = %d; want %d", node.Files, expFiles)
	}
	if node.Dirs != expDirs-1 {
		t.Fatalf("scanDir dirs = %d; want %d (one less than total subtree dirs)", node.Dirs, expDirs-1)
	}
	if !node.Scanned {
		t.Fatalf("scanDir: expected node.Scanned=true")
	}

	// immediate children should include 'a' and 'file3'
	names := map[string]bool{}
	for _, c := range node.Children {
		names[c.Name] = true
	}
	if !names["a"] || !names["file3"] {
		t.Fatalf("scanDir children missing expected entries: got %v", names)
	}
}
