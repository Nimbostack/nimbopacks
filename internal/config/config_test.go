package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestLoad_WarnsOnMissingSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "nimpack.yaml", "project:\n  name: myapp\n  version: 1.0.0\nbuild:\n  pack: golang\n")
	cfg, err := Load(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Loading stays lenient: the config is returned, and the warning is what tells the
	// author that the field becomes mandatory with schema "2".
	if cfg == nil || cfg.Project.Name != "myapp" {
		t.Fatalf("config should load: %+v", cfg)
	}
	if got := checkSchemaVersion(cfg); len(got) != 1 || !strings.Contains(got[0], "schema_version is missing") {
		t.Errorf("missing schema_version must warn, got %v", got)
	}
}

func TestLoad_WarnsOnUnknownSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "nimpack.yaml", "schema_version: \"9\"\nproject:\n  name: myapp\n  version: 1.0.0\nbuild:\n  pack: golang\n")
	cfg, err := Load(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := checkSchemaVersion(cfg); len(got) != 1 || !strings.Contains(got[0], "not supported") {
		t.Errorf("an unsupported schema_version must warn, got %v", got)
	}
}

func TestLoad_ReportsUnknownKeys(t *testing.T) {
	// The failure this guards: a misspelt key is discarded by the decoder, so the config
	// looks correct while the value it set is ignored by the build.
	data := []byte("schema_version: \"1\"\nproject:\n  name: myapp\n  version: 1.0.0\nbuild:\n  pack: golang\nimage:\n  lables:\n    a: b\n")
	warnings := strictWarnings(data)
	if len(warnings) != 1 {
		t.Fatalf("want one warning, got %v", warnings)
	}
	if !strings.Contains(warnings[0], "lables") {
		t.Errorf("the warning must name the offending key, got %q", warnings[0])
	}
	if strings.Contains(warnings[0], "\n") {
		t.Errorf("the warning must be one line, got %q", warnings[0])
	}
}

func TestLoad_NoWarningsForAValidConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "nimpack.yaml", "schema_version: \"1\"\nproject:\n  name: myapp\n  version: 1.0.0\nbuild:\n  pack: golang\n  options:\n    GOFLAGS: -mod=mod\nimage:\n  labels:\n    org.opencontainers.image.source: https://example.invalid/app\n")
	cfg, err := Load(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := checkSchemaVersion(cfg); len(got) != 0 {
		t.Errorf("valid schema_version must not warn: %v", got)
	}
	data, err := os.ReadFile(filepath.Join(dir, "nimpack.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if got := strictWarnings(data); len(got) != 0 {
		t.Errorf("a config that matches the schema must not warn: %v", got)
	}
}
