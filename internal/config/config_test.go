package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Nimbostack/nimbopacks/internal/types"
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
