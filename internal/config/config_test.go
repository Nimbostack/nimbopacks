package config

import (
	"os"
	"reflect"
	"testing"

	"github.com/Nimbostack/nimbopacks/internal/types"
	golangpack "github.com/Nimbostack/nimbopacks/pkg/packs/golang"
)

func TestValidate_Valid(t *testing.T) {
	cfg := &types.NimpackConfig{}
	cfg.Project.Name = "myapp"
	cfg.Project.Version = "1.0.0"
	cfg.Build.Pack = "golang"
	if err := Validate(cfg); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidate_MissingProjectName(t *testing.T) {
	cfg := &types.NimpackConfig{}
	cfg.Project.Version = "1.0.0"
	cfg.Build.Pack = "golang"
	if err := Validate(cfg); err == nil {
		t.Error("expected error for missing project name")
	}
}

func TestValidate_MissingVersion(t *testing.T) {
	cfg := &types.NimpackConfig{}
	cfg.Project.Name = "myapp"
	cfg.Build.Pack = "golang"
	if err := Validate(cfg); err == nil {
		t.Error("expected error for missing version")
	}
}

func TestValidate_MissingPack(t *testing.T) {
	cfg := &types.NimpackConfig{}
	cfg.Project.Name = "myapp"
	cfg.Project.Version = "1.0.0"
	if err := Validate(cfg); err == nil {
		t.Error("expected error for missing build pack")
	}
}

func TestValidate_Nil(t *testing.T) {
	if err := Validate(nil); err == nil {
		t.Error("expected error for nil config")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load(t.TempDir(), "")
	if err != nil {
		t.Fatalf("expected no error for a missing config, got: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config for a missing file, got: %+v", cfg)
	}
}

func TestLoad_ExplicitPath(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/custom.yaml"
	writeFile(t, path, "project:\n  name: myapp\n  version: 1.0.0\nbuild:\n  pack: golang\n")

	cfg, err := Load(dir, path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Project.Name != "myapp" {
		t.Errorf("Project.Name: got %q", cfg.Project.Name)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestLoad_DeadFieldRegression verifies ADR-0003's leniency corollary: a
// config that still sets the three deleted fields (build.context,
// update.auto_check, update.repositories) loads without error and produces
// a byte-identical BuildPlan to the same config with those keys absent.
func TestLoad_DeadFieldRegression(t *testing.T) {
	dir := t.TempDir()

	withDeadFields := `
schema_version: "1"
project:
  name: myapp
  version: "1.0.0"
build:
  pack: golang
  context: ./subdir
image:
  packages: []
  entrypoint: ""
update:
  auto_check: true
  repositories:
    - https://example.invalid/os
`
	withoutDeadFields := `
schema_version: "1"
project:
  name: myapp
  version: "1.0.0"
build:
  pack: golang
image:
  packages: []
  entrypoint: ""
`

	pathWith := dir + "/with.yaml"
	pathWithout := dir + "/without.yaml"
	writeFile(t, pathWith, withDeadFields)
	writeFile(t, pathWithout, withoutDeadFields)

	cfgWith, err := Load(dir, pathWith)
	if err != nil {
		t.Fatalf("loading config with dead fields: %v", err)
	}
	cfgWithout, err := Load(dir, pathWithout)
	if err != nil {
		t.Fatalf("loading config without dead fields: %v", err)
	}

	p := &golangpack.Pack{}
	planWith, err := p.Plan(t.Context(), dir, cfgWith)
	if err != nil {
		t.Fatalf("planning config with dead fields: %v", err)
	}
	planWithout, err := p.Plan(t.Context(), dir, cfgWithout)
	if err != nil {
		t.Fatalf("planning config without dead fields: %v", err)
	}

	if !reflect.DeepEqual(planWith, planWithout) {
		t.Errorf("BuildPlan differs when deleted fields are set:\nwith:    %+v\nwithout: %+v", planWith, planWithout)
	}
}
