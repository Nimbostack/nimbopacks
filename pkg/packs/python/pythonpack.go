// Package pythonpack is the nimbopacks pack for Python projects.
package pythonpack

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/Nimbostack/nimbopacks/internal/pack"
	"github.com/Nimbostack/nimbopacks/internal/pack/registry"
	"github.com/Nimbostack/nimbopacks/internal/types"
)

func init() { registry.Register(&Pack{}) }

type Pack struct{}

func (p *Pack) Name() string { return "python" }

func (p *Pack) Detect(_ context.Context, srcDir string) (*types.DetectResult, error) {
	var confidence float64
	meta := make(map[string]string)

	switch {
	case pack.FileExists(srcDir, "pyproject.toml"):
		confidence = 0.85
		meta["manager"] = detectManager(srcDir)
	case pack.FileExists(srcDir, "Pipfile"):
		confidence = 0.8
		meta["manager"] = "pipenv"
	case pack.FileExists(srcDir, "requirements.txt"):
		confidence = 0.75
		meta["manager"] = "pip"
	case pack.FileExists(srcDir, "setup.py"):
		confidence = 0.7
		meta["manager"] = "setuptools"
	default:
		files, _ := pack.FindFilesRecursive(srcDir, "*.py")
		if len(files) == 0 {
			return nil, nil
		}
		confidence = 0.4
		meta["manager"] = "pip"
	}

	fw := detectFW(srcDir)
	if fw != "" {
		meta["framework"] = fw
		confidence += 0.05
	}
	if ep := detectEntrypoint(srcDir, fw); ep != "" {
		meta["entrypoint"] = ep
	}

	suggested := "python-fastapi"
	if fw == "django" {
		suggested = "python-django"
	}

	summary := fmt.Sprintf("Python project (%s)", meta["manager"])
	if fw != "" {
		summary = fmt.Sprintf("Python/%s (%s)", fw, meta["manager"])
	}

	return &types.DetectResult{
		PackName: p.Name(), Confidence: confidence,
		Summary: summary, SuggestedTemplate: suggested, Metadata: meta,
	}, nil
}

func (p *Pack) GenerateConfig(_ context.Context, _ string, detected *types.DetectResult, templateName string) (*types.NimpackConfig, error) {
	cfg := pack.BaseConfig("app", "0.0.1", p.Name(), templateName)
	cfg.Image.Packages = []string{"python-3.12", "ca-certificates-bundle"}
	cfg.Image.Env = map[string]string{"PYTHONDONTWRITEBYTECODE": "1", "PYTHONUNBUFFERED": "1"}
	cfg.Build.Dependencies = []string{"python-3.12", "py3-pip", "build-base"}
	cfg.Build.Command = "pip install --no-cache-dir -r requirements.txt"
	ep := detected.Metadata["entrypoint"]
	fw := detected.Metadata["framework"]
	switch fw {
	case "fastapi":
		cfg.Image.Entrypoint = "python3"
		if ep == "" {
			ep = "app.main:app"
		}
		cfg.Image.Cmd = []string{"-m", "uvicorn", ep, "--host=0.0.0.0"}
		cfg.Image.Packages = append(cfg.Image.Packages, "py3-uvicorn")
	case "django":
		cfg.Image.Entrypoint = "python3"
		cfg.Image.Cmd = []string{"-m", "gunicorn", "config.wsgi", "--bind=0.0.0.0:8000"}
		cfg.Image.Packages = append(cfg.Image.Packages, "py3-gunicorn")
	default:
		cfg.Image.Entrypoint = "python3"
		if ep != "" {
			cfg.Image.Cmd = []string{ep}
		}
	}
	return &cfg, nil
}

func (p *Pack) Plan(_ context.Context, _ string, cfg *types.NimpackConfig) (*types.BuildPlan, error) {
	melange := pack.NewMelangeConfig(cfg.Project.Name, cfg.Project.Version, cfg.Build.Dependencies)
	melange.Package.Dependencies = types.MelangeDependencies{Runtime: cfg.Image.Packages}
	melange.Pipeline = []types.MelangePipelineStep{
		{Runs: cfg.Build.Command},
		{Runs: "mkdir -p /home/build/output/app\ncp -r . /home/build/output/app/"},
	}
	apko := pack.NewApkoConfig(cfg.Project.Name, cfg.Image.Entrypoint, cfg.Image.Packages)
	maps.Copy(apko.Environment, cfg.Image.Env)
	plan := &types.BuildPlan{Melange: melange, Apko: apko}
	pack.ApplyConfig(plan, cfg)
	return plan, nil
}

func detectManager(srcDir string) string {
	content, _ := pack.ReadFile(srcDir, "pyproject.toml")
	switch {
	case strings.Contains(content, "[tool.poetry]"):
		return "poetry"
	case strings.Contains(content, "[tool.pdm]"):
		return "pdm"
	default:
		return "pip"
	}
}

func detectFW(srcDir string) string {
	for _, f := range []string{"requirements.txt", "pyproject.toml", "Pipfile"} {
		content, err := pack.ReadFile(srcDir, f)
		if err != nil {
			continue
		}
		lower := strings.ToLower(content)
		switch {
		case strings.Contains(lower, "fastapi"):
			return "fastapi"
		case strings.Contains(lower, "django"):
			return "django"
		case strings.Contains(lower, "flask"):
			return "flask"
		}
	}
	return ""
}

func detectEntrypoint(srcDir, fw string) string {
	if fw == "fastapi" {
		for _, f := range []string{"app/main.py", "main.py", "app.py"} {
			if pack.FileExists(srcDir, f) {
				return strings.ReplaceAll(strings.TrimSuffix(f, ".py"), "/", ".") + ":app"
			}
		}
	}
	for _, f := range []string{"app.py", "main.py", "server.py"} {
		if pack.FileExists(srcDir, f) {
			return f
		}
	}
	return ""
}
