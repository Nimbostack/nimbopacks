// Package config loads and validates nimpack.yaml configuration files.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/Nimbostack/nimbopacks/internal/types"
)

const DefaultFilename = "nimpack.yaml"

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
	return &cfg, nil
}
