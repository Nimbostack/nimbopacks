// Package golangpack is the nimbopacks pack for Go projects.
package golangpack

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"strings"

	"github.com/Nimbostack/nimbopacks/internal/pack"
	"github.com/Nimbostack/nimbopacks/internal/pack/registry"
	"github.com/Nimbostack/nimbopacks/internal/types"
)

func init() { registry.Register(&Pack{}) }

type Pack struct{}

func (p *Pack) Name() string { return "golang" }

func (p *Pack) Detect(_ context.Context, srcDir string) (*types.DetectResult, error) {
	if !pack.FileExists(srcDir, "go.mod") {
		return nil, nil
	}

	confidence := 0.8
	meta := make(map[string]string)

	if content, err := pack.ReadFile(srcDir, "go.mod"); err == nil {
		for line := range strings.SplitSeq(content, "\n") {
			line = strings.TrimSpace(line)
			if mod, ok := strings.CutPrefix(line, "module "); ok {
				meta["module_path"] = mod
			}
			if ver, ok := strings.CutPrefix(line, "go "); ok {
				meta["go_version"] = ver
			}
		}
	}

	if ep := findEntrypoint(srcDir); ep != "" {
		meta["entrypoint"] = ep
		confidence = 0.9
	}

	fw := detectFramework(srcDir)
	if fw != "" {
		meta["framework"] = fw
		confidence = 0.95
	}

	// Suggest template based on framework.
	suggested := "go"
	if fw == "grpc" || fw == "connect" {
		suggested = "go-grpc"
	}

	summary := "Go project"
	if fw != "" {
		summary = fmt.Sprintf("Go/%s project", fw)
	}

	return &types.DetectResult{
		PackName:          p.Name(),
		Confidence:        confidence,
		Summary:           summary,
		SuggestedTemplate: suggested,
		Metadata:          meta,
	}, nil
}

func (p *Pack) GenerateConfig(_ context.Context, _ string, detected *types.DetectResult, templateName string) (*types.NimpackConfig, error) {
	meta := detected.Metadata

	name := "app"
	if mod, ok := meta["module_path"]; ok {
		parts := strings.Split(mod, "/")
		name = parts[len(parts)-1]
	}

	cfg := pack.BaseConfig(name, "0.0.1", p.Name(), templateName)
	cfg.Build.Env = map[string]string{
		"CGO_ENABLED": "0",
		"GOFLAGS":     "-trimpath",
	}
	cfg.Image.Entrypoint = "/usr/bin/" + name
	cfg.Image.Packages = []string{"ca-certificates-bundle"}

	if ep, ok := meta["entrypoint"]; ok {
		cfg.Build.Env["ENTRYPOINT"] = ep
	}

	return &cfg, nil
}

func (p *Pack) Plan(_ context.Context, srcDir string, cfg *types.NimpackConfig) (*types.BuildPlan, error) {
	name := cfg.Project.Name
	version := cfg.Project.Version

	melange := pack.NewMelangeConfig(name, version, []string{"go", "build-base"})
	melange.Package.Dependencies = types.MelangeDependencies{
		Runtime: cfg.Image.Packages,
	}

	// Determine build target.
	// Prefer explicit ENTRYPOINT from config; fall back to scanning the source.
	target := "."
	if ep, ok := cfg.Build.Env["ENTRYPOINT"]; ok && ep != "" {
		target = ep
	} else if ep := findEntrypoint(srcDir); ep != "" {
		target = ep
	}

	if cfg.Build.Command != "" {
		melange.Pipeline = []types.MelangePipelineStep{{Runs: cfg.Build.Command}}
	} else {
		melange.Pipeline = []types.MelangePipelineStep{
			{
				Uses: "go/build",
				With: map[string]string{
					"packages": target,
					"output":   name,
					"ldflags":  "-s -w",
				},
			},
		}
	}

	// Handle multi-artifact builds.
	if len(cfg.Artifacts) > 0 {
		melange.Pipeline = nil
		for _, a := range cfg.Artifacts {
			if a.Command != "" {
				melange.Pipeline = append(melange.Pipeline, types.MelangePipelineStep{Runs: a.Command})
			} else {
				melange.Pipeline = append(melange.Pipeline, types.MelangePipelineStep{
					Uses: "go/build",
					With: map[string]string{
						"packages": a.Source,
						"output":   a.Name,
						"ldflags":  "-s -w",
					},
				})
			}
		}
	}

	apko := pack.NewApkoConfig(name, cfg.Image.Entrypoint, cfg.Image.Packages)
	maps.Copy(apko.Environment, cfg.Image.Env)

	plan := &types.BuildPlan{Melange: melange, Apko: apko}
	pack.ApplyConfig(plan, cfg)
	return plan, nil
}

func findEntrypoint(srcDir string) string {
	if entries, err := pack.FindFiles(srcDir, "cmd/*/main.go"); err == nil && len(entries) > 0 {
		// filepath.Base(filepath.Dir(...)) extracts the command name without
		// assuming "/" as the path separator.
		name := filepath.Base(filepath.Dir(entries[0]))
		return fmt.Sprintf("./cmd/%s", name)
	}
	if pack.FileExists(srcDir, "main.go") {
		return "."
	}
	return ""
}

func detectFramework(srcDir string) string {
	frameworks := map[string]string{
		"github.com/gin-gonic/gin": "gin", "github.com/labstack/echo": "echo",
		"github.com/gofiber/fiber": "fiber", "github.com/go-chi/chi": "chi",
		"google.golang.org/grpc": "grpc", "connectrpc.com/connect": "connect",
	}
	content, err := pack.ReadFile(srcDir, "go.sum")
	if err != nil {
		content, _ = pack.ReadFile(srcDir, "go.mod")
	}
	for path, name := range frameworks {
		if strings.Contains(content, path) {
			return name
		}
	}
	return ""
}
