// Package pack defines the Pack interface.
//
// A Pack knows how to:
//  1. Detect if a source directory matches its language
//  2. Generate a nimpack.yaml from detection results
//  3. Create a build plan from a nimpack.yaml
//
// Detection is a convenience — it generates a config file.
// Builds always go through the config file.
//
//	Detection path:  source → Detect() → GenerateConfig() → nimpack.yaml
//	Build path:      nimpack.yaml → Plan() → melange + apko
package pack

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"github.com/Nimbostack/nimbopacks/internal/types"
)

// Pack is the interface every language module implements.
type Pack interface {
	// Name returns a unique identifier (e.g., "go", "python", "dotnet").
	Name() string

	// Detect inspects a source directory. Returns nil, nil if no match.
	Detect(ctx context.Context, srcDir string) (*types.DetectResult, error)

	// GenerateConfig produces a nimpack.yaml from detection results + template.
	// This is called by `nimbopacks init` and `nimbopacks detect --generate`.
	GenerateConfig(ctx context.Context, srcDir string, detected *types.DetectResult, templateName string) (*types.NimpackConfig, error)

	// Plan creates a build plan from a nimpack.yaml. This is the build path.
	Plan(ctx context.Context, srcDir string, cfg *types.NimpackConfig) (*types.BuildPlan, error)
}

// --- Shared helpers for pack authors ---

func FileExists(srcDir, filename string) bool {
	_, err := os.Stat(filepath.Join(srcDir, filename))
	return err == nil
}

func ReadFile(srcDir, filename string) (string, error) {
	data, err := os.ReadFile(filepath.Join(srcDir, filename))
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func FindFiles(srcDir, pattern string) ([]string, error) {
	return filepath.Glob(filepath.Join(srcDir, pattern))
}

func FindFilesRecursive(srcDir, pattern string) ([]string, error) {
	skipDirs := map[string]bool{
		"vendor": true, "node_modules": true, ".git": true,
		"__pycache__": true, ".venv": true, "venv": true,
		"target": true, ".cargo": true, "dist": true,
		"build": true, "bin": true, "obj": true, "packages": true,
	}
	var matches []string
	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if strings.HasPrefix(name, ".") || skipDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}
		matched, _ := filepath.Match(pattern, info.Name())
		if matched {
			rel, _ := filepath.Rel(srcDir, path)
			matches = append(matches, rel)
		}
		return nil
	})
	return matches, err
}

func ContainsLine(srcDir, filename string, pred func(string) bool) (bool, error) {
	content, err := ReadFile(srcDir, filename)
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(content, "\n") {
		if pred(strings.TrimSpace(line)) {
			return true, nil
		}
	}
	return false, nil
}

// --- Build plan helpers ---

func NewMelangeConfig(name, version string, buildDeps []string) types.MelangeConfig {
	return types.MelangeConfig{
		Package: types.MelangePackage{
			Name:    name,
			Version: version,
			Epoch:   0,
			Copyright: []types.MelangeCopyright{
				{License: "Apache-2.0"},
			},
		},
		Environment: types.MelangeEnvironment{
			Contents: types.MelangeContents{
				Keyring:      []string{"https://packages.wolfi.dev/os/wolfi-signing.rsa.pub"},
				Repositories: []string{"https://packages.wolfi.dev/os"},
				Packages:     Dedupe(append([]string{"wolfi-baselayout"}, buildDeps...)),
			},
		},
	}
}

func NewApkoConfig(name, entrypoint string, runtimeDeps []string) types.ApkoConfig {
	packages := append([]string{"wolfi-baselayout", name}, runtimeDeps...)
	return types.ApkoConfig{
		Contents: types.ApkoContents{
			Keyring: []string{"https://packages.wolfi.dev/os/wolfi-signing.rsa.pub"},
			Repositories: []string{
				"https://packages.wolfi.dev/os",
				"/home/build/packages",
			},
			Packages: Dedupe(packages),
		},
		Entrypoint: types.ApkoEntrypoint{Command: entrypoint},
		Accounts: types.ApkoAccounts{
			RunAs: "65532",
			Users: []types.ApkoUser{
				{Username: "nonroot", UID: 65532, GID: 65532},
			},
			Groups: []types.ApkoGroup{
				{Groupname: "nonroot", GID: 65532},
			},
		},
		Environment: make(map[string]string),
	}
}

// ApplyConfig applies nimpack.yaml overrides onto a build plan.
//
// Packs are responsible for constructing their own pipeline steps (including
// incorporating cfg.Build.Command into the appropriate step). ApplyConfig
// handles all other user overrides: metadata, extra dependencies, image
// packages, run-as UID, and environment variables.
func ApplyConfig(plan *types.BuildPlan, cfg *types.NimpackConfig) {
	if cfg == nil {
		return
	}
	plan.Melange.Package.Name = cfg.Project.Name
	plan.Melange.Package.Version = cfg.Project.Version
	plan.Melange.Package.Description = cfg.Project.Description

	if len(cfg.Build.Dependencies) > 0 {
		plan.Melange.Environment.Contents.Packages = Dedupe(
			append(plan.Melange.Environment.Contents.Packages, cfg.Build.Dependencies...),
		)
	}
	if cfg.Image.Entrypoint != "" {
		plan.Apko.Entrypoint.Command = cfg.Image.Entrypoint
	}
	if len(cfg.Image.Packages) > 0 {
		plan.Apko.Contents.Packages = Dedupe(
			append(plan.Apko.Contents.Packages, cfg.Image.Packages...),
		)
	}
	if cfg.Image.RunAs != 0 {
		plan.Apko.Accounts.RunAs = fmt.Sprintf("%d", cfg.Image.RunAs)
	}
	maps.Copy(plan.Apko.Environment, cfg.Image.Env)
}

func Dedupe(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}

// BaseConfig returns a minimal nimpack.yaml skeleton.
func BaseConfig(name, version, packName, templateName string) types.NimpackConfig {
	return types.NimpackConfig{
		SchemaVersion: "1",
		Project: types.ProjectMeta{
			Name:    name,
			Version: version,
		},
		Build: types.BuildConfig{
			Pack:     packName,
			Template: templateName,
		},
		Image: types.ImageConfig{
			RunAs: 65532,
			Env:   make(map[string]string),
		},
		Update: types.UpdateConfig{
			AutoCheck: true,
		},
	}
}
