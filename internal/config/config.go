// Package config loads and validates nimpack.yaml configuration files.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"
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
	return validateLayering(cfg.Image.Layering)
}

// validateLayering enforces ADR-0004 A10 and A11 on the image.layering block.
//
// These reject rather than warn, which is the one place this loader departs
// from the warn-then-error ramp ADR-0003 A1 set for schema_version. A1's ramp
// exists so that no config which builds today stops building, and every config
// rejected here already fails: apko exits 1 on an unrecognized strategy, and a
// budget outside [1, 126] either duplicates a state that has its own spelling
// or asks for an image no runtime will pull. Validating at load moves an
// existing failure earlier and names the legal values while doing it.
func validateLayering(l *types.LayeringConfig) error {
	if l == nil {
		return nil
	}
	if l.Strategy != types.LayeringStrategyOrigin {
		if l.Strategy == "" {
			return fmt.Errorf("image.layering.strategy is required when image.layering is set; supported: %s",
				strategyList())
		}
		return fmt.Errorf("image.layering.strategy %q is not supported; supported: %s",
			string(l.Strategy), strategyList())
	}
	if l.Budget == nil {
		return fmt.Errorf("image.layering.budget is required when image.layering is set, and must be between %d and %d; omit image.layering entirely for a single-layer image",
			types.MinLayeringBudget, types.MaxLayeringBudget)
	}
	if *l.Budget < types.MinLayeringBudget {
		return fmt.Errorf("image.layering.budget is %d; it must be at least %d (budget: 0 builds the same two-layer image as budget: 1, and omitting image.layering entirely is how to ask for a single layer)",
			*l.Budget, types.MinLayeringBudget)
	}
	if *l.Budget > types.MaxLayeringBudget {
		return fmt.Errorf("image.layering.budget is %d; it must be at most %d, because the total layer count is the budget plus one metadata layer and container runtimes cap at 127 layers",
			*l.Budget, types.MaxLayeringBudget)
	}
	return nil
}

func strategyList() string {
	names := make([]string, 0, len(types.LayeringStrategies))
	for _, s := range types.LayeringStrategies {
		names = append(names, strconv.Quote(string(s)))
	}
	return strings.Join(names, ", ")
}

// runAsWarnings reports the stderr diagnostic ADR-0004 A9 requires when a
// config asks for UID 0. Root stays expressible — a schema meant to be
// implemented by others should not be less expressive than a Dockerfile's
// USER root — but it is never silent.
func runAsWarnings(cfg *types.NimpackConfig) []string {
	if cfg.Image.RunAs == nil || *cfg.Image.RunAs != 0 {
		return nil
	}
	name := cfg.Project.Name
	if name == "" {
		name = "this project"
	}
	return []string{fmt.Sprintf("image.run_as is 0: the container built from %s will run as root", name)}
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
	warnings = append(warnings, runAsWarnings(&cfg)...)
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s\n", w)
	}
	// Layering is rejected here rather than warned about, and it is the only
	// thing Load refuses: see validateLayering. Validate applies the same rule
	// for callers that construct a config without going through the loader.
	if err := validateLayering(cfg.Image.Layering); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &cfg, nil
}
