package cache

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestList_empty(t *testing.T) {
	baseDirOverride = t.TempDir()
	t.Cleanup(func() { baseDirOverride = "" })

	entries, err := List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Size != 0 {
		t.Errorf("expected Size 0, got %d", entries[0].Size)
	}
	if entries[0].Count != 0 {
		t.Errorf("expected Count 0, got %d", entries[0].Count)
	}
}

func TestList_withBuilds(t *testing.T) {
	baseDirOverride = t.TempDir()
	t.Cleanup(func() { baseDirOverride = "" })

	// Create two workspace directories under BuildsDir, each with files inside.
	// Count should reflect the number of direct children (workspace dirs), not
	// the total number of files recursively.
	for _, ws := range []string{"20260411-120000-abcd", "20260411-130000-efgh"} {
		buildDir := filepath.Join(BuildsDir(), ws)
		if err := os.MkdirAll(buildDir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(buildDir, "melange.yaml"), "content: test")
		writeFile(t, filepath.Join(buildDir, "apko.yaml"), "content: test2")
	}

	entries, err := List()
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	// Two workspace directories = Count 2 (direct children of BuildsDir).
	if entries[0].Count != 2 {
		t.Errorf("expected Count 2 (two workspace dirs), got %d", entries[0].Count)
	}
	if entries[0].Size == 0 {
		t.Error("expected non-zero Size")
	}
}

func TestClean(t *testing.T) {
	baseDirOverride = t.TempDir()
	t.Cleanup(func() { baseDirOverride = "" })

	buildDir := filepath.Join(BuildsDir(), "20260411-120000-abcd")
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(buildDir, "melange.yaml"), "content: test")

	entries, err := List()
	if err != nil {
		t.Fatal(err)
	}

	if err := Clean(entries); err != nil {
		t.Fatalf("Clean() error: %v", err)
	}

	if _, err := os.Stat(BuildsDir()); os.IsNotExist(err) {
		t.Error("BuildsDir should still exist after Clean()")
	}

	after, err := List()
	if err != nil {
		t.Fatal(err)
	}
	if after[0].Count != 0 {
		t.Errorf("expected Count 0 after clean, got %d", after[0].Count)
	}
}

func TestNewBuildWorkDir(t *testing.T) {
	baseDirOverride = t.TempDir()
	t.Cleanup(func() { baseDirOverride = "" })

	dir, err := NewBuildWorkDir()
	if err != nil {
		t.Fatalf("NewBuildWorkDir() error: %v", err)
	}
	if !strings.HasPrefix(dir, BuildsDir()) {
		t.Errorf("work dir %q not under BuildsDir %q", dir, BuildsDir())
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Error("NewBuildWorkDir should create the directory")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
