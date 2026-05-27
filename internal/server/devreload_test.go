package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWebDirMaxModTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.js")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	t1 := webDirMaxModTime(dir)
	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(path, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	t2 := webDirMaxModTime(dir)
	if t2 <= t1 {
		t.Fatalf("expected mod time to increase: %d -> %d", t1, t2)
	}
}
