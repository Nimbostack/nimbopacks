// Package config loads and validates nimpack.yaml configuration files.
package config

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/Nimbostack/nimbopacks/internal/types"
)

const DefaultFilename = "nimpack.yaml"

// knownBuildOptionKeys are the build.options keys recognized by shipped
// packs. Unrecognized keys are warned about, not discarded — a pack may add
// its own keys without a central schema change.
var knownBuildOptionKeys = map[string]bool{
	"ENTRYPOINT":      true, // go
	"CGO_ENABLED":     true, // go
	"GOFLAGS":         true, // go
	"JAR_DIR":         true, // java
	"OUTPUT_DIR":      true, // webserver
	"NGINX_CONF_PATH": true, // webserver
}

// resolveBuildOptions merges the deprecated build.env alias into
// build.options (options wins on key conflicts) and returns warnings for the
// alias usage and for any unrecognized option keys.
func resolveBuildOptions(cfg *types.NimpackConfig) []string {
	var warnings []string
	if len(cfg.Build.Env) > 0 {
		warnings = append(warnings, "build.env is deprecated; use build.options instead")
		merged := make(map[string]string, len(cfg.Build.Options)+len(cfg.Build.Env))
		maps.Copy(merged, cfg.Build.Env)
		maps.Copy(merged, cfg.Build.Options)
		cfg.Build.Options = merged
	}
	for k := range cfg.Build.Options {
		if !knownBuildOptionKeys[k] {
			warnings = append(warnings, fmt.Sprintf("build.options: unrecognized key %q", k))
		}
	}
	return warnings
}

// Validate checks that required fields are present in a loaded config.
func Validate(cfg *types.NimpackConfig) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	if cfg.Project.Name == "" {
		return errors.New("project.name is required")
	}
	if cfg.Project.Version == "" {
		return errors.New("project.version is required")
	}
	if cfg.Build.Pack == "" {
		return errors.New("build.pack is required")
	}
	return nil
}

func Load(srcDir, path string) (*types.NimpackConfig, error) {
	if path == "" {
		path = filepath.Join(srcDir, DefaultFilename)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg types.NimpackConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	for _, w := range resolveBuildOptions(&cfg) {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	return &cfg, nil
}
