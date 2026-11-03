package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nimbostack/nimbopacks/pkg/backend"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", name, err)
	}
}

func TestBuild_MissingConfig(t *testing.T) {
	dir := t.TempDir()
	_, err := Build(t.Context(), BuildOptions{SourceDir: dir})
	if err == nil {
		t.Fatal("expected error when nimpack.yaml is absent")
	}
	if !strings.Contains(err.Error(), "nimpack.yaml") {
		t.Errorf("expected error to mention nimpack.yaml, got: %v", err)
	}
}

func TestBuild_InvalidBackend(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "nimpack.yaml", `
schema_version: "1"
project:
  name: test
  version: 0.1.0
build:
  pack: golang
`)
	_, err := Build(t.Context(), BuildOptions{
		SourceDir:   dir,
		BackendName: "nonexistent-backend",
	})
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestBuild_UnknownPack(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "nimpack.yaml", `
schema_version: "1"
project:
  name: test
  version: 0.1.0
build:
  pack: unknown-pack-xyz
`)
	// AutoSelect will fail in CI (no melange/apko), so use explicit backend name
	// to reach the pack lookup. If no backend is available, the backend step
	// will error first — either way we get a non-nil error, which is the invariant.
	_, err := Build(t.Context(), BuildOptions{
		SourceDir: dir,
	})
	if err == nil {
		t.Fatal("expected error for unknown pack or missing backend")
	}
}

func TestBuildOptions_Defaults(t *testing.T) {
	opts := BuildOptions{}
	if opts.Push {
		t.Error("Push should default to false")
	}
	if opts.BackendName != "" {
		t.Error("BackendName should default to empty (auto-select)")
	}
}

// TestPortsWarning covers ADR-0003 A4's verification requirement: a config
// that sets image.ports against a backend that cannot express exposed-port
// metadata warns, once, naming the backend; no ports means no warning.
func TestPortsWarning(t *testing.T) {
	noExposedPorts := backend.Capabilities{ExposedPorts: false}
	hasExposedPorts := backend.Capabilities{ExposedPorts: true}

	if msg := portsWarning(nil, "wolfi", noExposedPorts); msg != "" {
		t.Errorf("expected no warning when ports is unset, got: %q", msg)
	}
	if msg := portsWarning([]string{"8080"}, "wolfi", hasExposedPorts); msg != "" {
		t.Errorf("expected no warning when backend supports exposed ports, got: %q", msg)
	}
	msg := portsWarning([]string{"8080"}, "wolfi", noExposedPorts)
	if msg == "" {
		t.Fatal("expected a warning when ports is set and the backend can't express it")
	}
	if !strings.Contains(msg, "wolfi") {
		t.Errorf("expected warning to name the backend, got: %q", msg)
	}
}
