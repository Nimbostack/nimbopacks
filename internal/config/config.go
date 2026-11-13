// Package config loads and validates nimpack.yaml configuration files.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

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

// checkSchemaVersion reports whether the config states a schema version this release
// understands. ADR-0003 A1 introduces validation with a warn-then-error ramp: every
// malformed or unknown value warns in this release, and the error path arrives only with
// the release that introduces "2". A missing version is the same shape of problem — the
// field has never been enforced, so most configs on disk still omit it.
func checkSchemaVersion(cfg *types.NimpackConfig) []string {
	switch cfg.SchemaVersion {
	case "":
		return []string{`schema_version is missing; assuming "1" (it becomes required in the release that introduces "2")`}
	case "1":
		return nil
	default:
		return []string{fmt.Sprintf("schema_version %q is not supported by this release; loading leniently", cfg.SchemaVersion)}
	}
}

// strictWarnings decodes the document a second time with KnownFields, which fails on a key
// that no struct field claims, and turns that failure into a warning. Decoding stays
// lenient — the point is that a typo such as `labels:` under `image:` is reported instead of
// being silently dropped, which is how a config appears to set a value the build then
// ignores.
func strictWarnings(data []byte) []string {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var strict types.NimpackConfig
	err := dec.Decode(&strict)
	if err == nil {
		return nil
	}
	msg := strings.Join(strings.Fields(strings.ReplaceAll(err.Error(), "yaml: unmarshal errors:", "")), " ")
	return []string{"config does not match the schema: " + msg}
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
	warnings := append(checkSchemaVersion(&cfg), strictWarnings(data)...)
	warnings = append(warnings, resolveBuildOptions(&cfg)...)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	return &cfg, nil
}
