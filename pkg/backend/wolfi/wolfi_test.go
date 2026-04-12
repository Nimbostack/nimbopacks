package wolfi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackend_Name(t *testing.T) {
	b := &Backend{}
	if b.Name() != "wolfi" {
		t.Errorf("Name() = %q, want %q", b.Name(), "wolfi")
	}
}

func TestBackend_Available_ReturnsBool(t *testing.T) {
	b := &Backend{}
	ok, reason := b.Available(context.Background())
	// We can't assert a specific value since the test environment may or may not
	// have melange/apko. Verify the contract: if unavailable, reason must be set.
	if !ok && reason == "" {
		t.Error("Available() = false with empty reason; reason must explain why")
	}
}

func TestFindApkoSBOM_Found(t *testing.T) {
	dir := t.TempDir()
	// apko writes sbom-<arch>.spdx.json for the arch-specific SBOM.
	if err := os.WriteFile(filepath.Join(dir, "sbom-x86_64.spdx.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Also write the index file, which should be skipped.
	if err := os.WriteFile(filepath.Join(dir, "sbom-index.spdx.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := findApkoSBOM(dir)
	if got == "" {
		t.Fatal("findApkoSBOM returned empty, expected path to sbom-x86_64.spdx.json")
	}
	if strings.Contains(got, "sbom-index.spdx.json") {
		t.Errorf("findApkoSBOM returned index file %q; should return arch-specific SBOM", got)
	}
	if !strings.HasSuffix(got, "sbom-x86_64.spdx.json") {
		t.Errorf("findApkoSBOM = %q, want path ending in sbom-x86_64.spdx.json", got)
	}
}

func TestFindApkoSBOM_Empty(t *testing.T) {
	dir := t.TempDir()
	got := findApkoSBOM(dir)
	if got != "" {
		t.Errorf("findApkoSBOM on empty dir = %q, want empty string", got)
	}
}

func TestFindApkoSBOM_IndexOnly(t *testing.T) {
	dir := t.TempDir()
	// Only the index file — no arch-specific SBOM.
	if err := os.WriteFile(filepath.Join(dir, "sbom-index.spdx.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := findApkoSBOM(dir)
	if got != "" {
		t.Errorf("findApkoSBOM with index-only = %q, want empty string", got)
	}
}

func TestBuiltArch_Found(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "x86_64"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := builtArch(dir)
	if got != "x86_64" {
		t.Errorf("builtArch = %q, want %q", got, "x86_64")
	}
}

func TestBuiltArch_Empty(t *testing.T) {
	dir := t.TempDir()
	got := builtArch(dir)
	if got != "" {
		t.Errorf("builtArch on empty dir = %q, want empty string", got)
	}
}

func TestToolchainPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := toolchainPath("melange")
	if !strings.HasPrefix(got, home) {
		t.Errorf("toolchainPath(%q) = %q, expected path under home dir %q", "melange", got, home)
	}
	if !strings.HasSuffix(got, "melange") {
		t.Errorf("toolchainPath(%q) = %q, expected path to end with binary name", "melange", got)
	}
}

func TestDetectMelangeRunner_ValidValues(t *testing.T) {
	runner := detectMelangeRunner()
	// Must return "", "docker", or another valid runner name — never garbage.
	switch runner {
	case "", "docker":
		// expected
	default:
		// Other values (lima, kubernetes) are valid too; just make sure it's a
		// non-whitespace string if non-empty.
		if strings.TrimSpace(runner) == "" {
			t.Errorf("detectMelangeRunner returned whitespace-only string %q", runner)
		}
	}
}
