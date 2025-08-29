package main

import (
	"context"
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
)

func TestExportCSVIntegration(t *testing.T) {
	// create temp dir with one file
	tmp, err := os.MkdirTemp("", "disktree-export-")
	if err != nil {
		t.Fatal(err)
	}
	defer func(path string) {
		_ = os.RemoveAll(path)
	}(tmp)

	if err := os.WriteFile(filepath.Join(tmp, "f1"), []byte{'x', 'y', 'z'}, 0644); err != nil {
		t.Fatal(err)
	}

	// prepare a model with a current node
	m := initialModel(tmp, 2, false)
	// force scan
	n := m.scanner.scanDir(context.Background(), tmp)
	m.current = n

	// run export command and get the message
	msg := m.exportCSV()()
	exMsg, ok := msg.(exportDoneMsg)
	if !ok {
		t.Fatalf("expected exportDoneMsg, got %T", msg)
	}
	if exMsg.err != nil {
		t.Fatalf("export error: %v", exMsg.err)
	}
	// open CSV and validate header
	f, err := os.Open(exMsg.path)
	if err != nil {
		t.Fatal(err)
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)
	r := csv.NewReader(f)
	rec, err := r.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(rec) < 1 || rec[0] != "Name" {
		t.Fatalf("unexpected csv header: %v", rec)
	}
}
