// Package dotnetpack is the nimbopacks pack for .NET projects.
//
// It distinguishes between:
//   - Single project: one .csproj, suggests dotnet-webapi or dotnet-minimal-api
//   - Solution monorepo: .sln with multiple .csproj files, suggests dotnet-solution
//
// This is a key differentiator — most tools treat .NET monorepos poorly.
package dotnetpack

import (
	"context"
	"fmt"
	"maps"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Nimbostack/nimbopacks/internal/pack"
	"github.com/Nimbostack/nimbopacks/internal/pack/registry"
	"github.com/Nimbostack/nimbopacks/internal/types"
)

func init() { registry.Register(&Pack{}) }

type Pack struct{}

func (p *Pack) Name() string { return "dotnet" }

func (p *Pack) Detect(_ context.Context, srcDir string) (*types.DetectResult, error) {
	meta := make(map[string]string)

	// Check for solution file (monorepo).
	slnFiles, _ := pack.FindFiles(srcDir, "*.sln")
	hasSolution := len(slnFiles) > 0
	if hasSolution {
		meta["solution_file"] = filepath.Base(slnFiles[0])
	}

	// Find all .csproj files.
	csprojFiles, _ := pack.FindFilesRecursive(srcDir, "*.csproj")
	if len(csprojFiles) == 0 && !hasSolution {
		return nil, nil
	}

	meta["project_count"] = strconv.Itoa(len(csprojFiles))

	// Classify projects.
	var webProjects, workerProjects, libProjects []string
	for _, f := range csprojFiles {
		content, err := pack.ReadFile(srcDir, f)
		if err != nil {
			continue
		}
		dir := filepath.Dir(f)
		slashDir := filepath.ToSlash(dir)
		switch {
		case strings.Contains(content, "Microsoft.NET.Sdk.Web"):
			webProjects = append(webProjects, slashDir)
			meta["has_web"] = "true"
		case strings.Contains(content, "Microsoft.NET.Sdk.Worker"):
			workerProjects = append(workerProjects, slashDir)
			meta["has_worker"] = "true"
		default:
			libProjects = append(libProjects, slashDir)
		}
	}

	if len(webProjects) > 0 {
		meta["web_projects"] = strings.Join(webProjects, ",")
	}
	if len(workerProjects) > 0 {
		meta["worker_projects"] = strings.Join(workerProjects, ",")
	}

	// Detect target framework.
	for _, f := range csprojFiles {
		content, _ := pack.ReadFile(srcDir, f)
		if idx := strings.Index(content, "<TargetFramework>"); idx >= 0 {
			end := strings.Index(content[idx:], "</TargetFramework>")
			if end > 0 {
				tf := content[idx+len("<TargetFramework>") : idx+end]
				meta["target_framework"] = tf
				break
			}
		}
	}

	// Detect if it uses minimal API pattern.
	isMinimal := false
	if len(csprojFiles) == 1 {
		if content, err := pack.ReadFile(srcDir, "Program.cs"); err == nil {
			if strings.Contains(content, "WebApplication.CreateBuilder") && !strings.Contains(content, "class Startup") {
				isMinimal = true
				meta["minimal_api"] = "true"
			}
		}
	}

	// Determine confidence and suggested template.
	var confidence float64
	var suggested, summary string

	switch {
	case hasSolution && len(csprojFiles) > 1:
		confidence = 0.95
		suggested = "dotnet-solution"
		summary = fmt.Sprintf(".NET solution (%d projects: %d web, %d worker, %d lib)",
			len(csprojFiles), len(webProjects), len(workerProjects), len(libProjects))
	case len(webProjects) > 0 && isMinimal:
		confidence = 0.9
		suggested = "dotnet-minimal-api"
		summary = ".NET Minimal API"
	case len(webProjects) > 0:
		confidence = 0.9
		suggested = "dotnet-webapi"
		summary = ".NET Web API"
	case len(workerProjects) > 0:
		confidence = 0.85
		suggested = "dotnet-webapi" // closest match
		summary = ".NET Worker Service"
	default:
		confidence = 0.7
		suggested = "dotnet-webapi"
		summary = ".NET project"
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
	if sln, ok := meta["solution_file"]; ok {
		name = strings.TrimSuffix(sln, ".sln")
	}

	cfg := pack.BaseConfig(name, "0.0.1", p.Name(), templateName)

	// Set .NET-specific defaults.
	tf := meta["target_framework"]
	runtimeVer := "8"
	if strings.Contains(tf, "net9") {
		runtimeVer = "9"
	}

	cfg.Image.Packages = []string{
		fmt.Sprintf("dotnet-%s-runtime", runtimeVer),
		fmt.Sprintf("aspnet-%s-runtime", runtimeVer),
		"ca-certificates-bundle",
		"icu-libs",
	}
	cfg.Image.Env = map[string]string{
		"DOTNET_SYSTEM_GLOBALIZATION_INVARIANT": "false",
		"ASPNETCORE_URLS":                       "http://+:8080",
	}
	cfg.Image.Entrypoint = "dotnet"
	cfg.Image.Ports = []string{"8080"}
	cfg.Build.Dependencies = []string{fmt.Sprintf("dotnet-%s-sdk", runtimeVer), "build-base"}

	// Handle monorepo: generate artifacts from detected projects.
	if templateName == "dotnet-solution" {
		cfg.Build.Command = fmt.Sprintf("dotnet restore %s.sln", name)

		if webs, ok := meta["web_projects"]; ok {
			for proj := range strings.SplitSeq(webs, ",") {
				projName := filepath.Base(proj)
				cfg.Artifacts = append(cfg.Artifacts, types.Artifact{
					Name:    strings.ToLower(projName),
					Source:  proj,
					Command: fmt.Sprintf("dotnet publish %s -c Release -o /home/build/output/app/%s --no-self-contained", proj, strings.ToLower(projName)),
					Dest:    fmt.Sprintf("/app/%s", strings.ToLower(projName)),
				})
			}
		}
		if workers, ok := meta["worker_projects"]; ok {
			for proj := range strings.SplitSeq(workers, ",") {
				projName := filepath.Base(proj)
				cfg.Artifacts = append(cfg.Artifacts, types.Artifact{
					Name:    strings.ToLower(projName),
					Source:  proj,
					Command: fmt.Sprintf("dotnet publish %s -c Release -o /home/build/output/app/%s --no-self-contained", proj, strings.ToLower(projName)),
					Dest:    fmt.Sprintf("/app/%s", strings.ToLower(projName)),
				})
			}
		}

		if len(cfg.Artifacts) > 0 {
			cfg.Image.Cmd = []string{fmt.Sprintf("/app/%s/%s.dll", cfg.Artifacts[0].Name, cfg.Artifacts[0].Name)}
		}
	} else {
		cfg.Build.Command = "dotnet publish -c Release -o /home/build/output/app --no-self-contained"
		cfg.Image.Cmd = []string{fmt.Sprintf("/app/%s.dll", name)}
	}

	return &cfg, nil
}

func (p *Pack) Plan(_ context.Context, _ string, cfg *types.NimpackConfig) (*types.BuildPlan, error) {
	name := cfg.Project.Name
	version := cfg.Project.Version

	melange := pack.NewMelangeConfig(name, version, cfg.Build.Dependencies)
	melange.Package.Dependencies = types.MelangeDependencies{
		Runtime: cfg.Image.Packages,
	}

	// Build pipeline.
	if len(cfg.Artifacts) > 0 {
		// Monorepo: restore once, publish each artifact.
		if cfg.Build.Command != "" {
			melange.Pipeline = append(melange.Pipeline, types.MelangePipelineStep{
				Runs: cfg.Build.Command,
			})
		}
		for _, a := range cfg.Artifacts {
			cmd := a.Command
			if cmd == "" {
				cmd = fmt.Sprintf("dotnet publish %s -c Release -o /home/build/output%s --no-self-contained", a.Source, a.Dest)
			}
			melange.Pipeline = append(melange.Pipeline, types.MelangePipelineStep{Runs: cmd})
		}
	} else {
		melange.Pipeline = []types.MelangePipelineStep{
			{Runs: cfg.Build.Command},
		}
	}

	apko := pack.NewApkoConfig(name, cfg.Image.Entrypoint, cfg.Image.Packages)
	if len(cfg.Image.Cmd) > 0 {
		apko.Cmd = strings.Join(cfg.Image.Cmd, " ")
	}
	maps.Copy(apko.Environment, cfg.Image.Env)

	plan := &types.BuildPlan{Melange: melange, Apko: apko}
	pack.ApplyConfig(plan, cfg)
	return plan, nil
}
