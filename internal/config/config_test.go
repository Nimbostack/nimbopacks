package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Nimbostack/nimbopacks/internal/types"
	golangpack "github.com/Nimbostack/nimbopacks/pkg/packs/golang"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

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

func TestLoad_BuildOptions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "nimpack.yaml", `
schema_version: "1"
project:
  name: myapp
  version: "1.0.0"
build:
  pack: golang
  options:
    ENTRYPOINT: ./cmd/myapp
image:
  packages: []
  entrypoint: /usr/bin/myapp
`)
	cfg, err := Load(dir, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Build.Options["ENTRYPOINT"]; got != "./cmd/myapp" {
		t.Errorf("expected ENTRYPOINT=./cmd/myapp, got %q", got)
	}
}

func TestLoad_BuildEnvAlias(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "nimpack.yaml", `
schema_version: "1"
project:
  name: myapp
  version: "1.0.0"
build:
  pack: golang
  env:
    ENTRYPOINT: ./cmd/myapp
image:
  packages: []
  entrypoint: /usr/bin/myapp
`)
	cfg, err := Load(dir, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.Build.Options["ENTRYPOINT"]; got != "./cmd/myapp" {
		t.Errorf("expected build.env to be merged into Options, got %q", got)
	}
}

func TestResolveBuildOptions_OptionsWinsOverEnvAlias(t *testing.T) {
	cfg := &types.NimpackConfig{}
	cfg.Build.Env = map[string]string{"ENTRYPOINT": "./old"}
	cfg.Build.Options = map[string]string{"ENTRYPOINT": "./new"}

	warnings := resolveBuildOptions(cfg)

	if got := cfg.Build.Options["ENTRYPOINT"]; got != "./new" {
		t.Errorf("expected build.options to win over build.env alias, got %q", got)
	}
	if len(warnings) != 1 || warnings[0] != "build.env is deprecated; use build.options instead" {
		t.Errorf("expected single deprecation warning, got %v", warnings)
	}
}

func TestResolveBuildOptions_WarnsOnUnrecognizedKey(t *testing.T) {
	cfg := &types.NimpackConfig{}
	cfg.Build.Options = map[string]string{"NOT_A_REAL_KEY": "x"}

	warnings := resolveBuildOptions(cfg)

	if len(warnings) != 1 || warnings[0] != `build.options: unrecognized key "NOT_A_REAL_KEY"` {
		t.Errorf("expected unrecognized-key warning, got %v", warnings)
	}
}

func TestResolveBuildOptions_NoWarningsForKnownKeys(t *testing.T) {
	cfg := &types.NimpackConfig{}
	cfg.Build.Options = map[string]string{"ENTRYPOINT": "./cmd/myapp"}

	if warnings := resolveBuildOptions(cfg); len(warnings) != 0 {
		t.Errorf("expected no warnings, got %v", warnings)
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

	writeFile(t, dir, "with.yaml", withDeadFields)
	writeFile(t, dir, "without.yaml", withoutDeadFields)
	pathWith := filepath.Join(dir, "with.yaml")
	pathWithout := filepath.Join(dir, "without.yaml")

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
