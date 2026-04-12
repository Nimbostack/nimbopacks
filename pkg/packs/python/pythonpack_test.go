package pythonpack

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDetect_NoPythonFiles(t *testing.T) {
	p := &Pack{}
	res, err := p.Detect(t.Context(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Fatal("expected nil for non-Python project")
	}
}

func TestDetect_RequirementsTxt(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", "flask==3.0.0\n")

	p := &Pack{}
	res, err := p.Detect(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.Metadata["manager"] != "pip" {
		t.Errorf("expected manager=pip, got %s", res.Metadata["manager"])
	}
	if res.Metadata["framework"] != "flask" {
		t.Errorf("expected framework=flask, got %s", res.Metadata["framework"])
	}
}

func TestDetect_PyprojectTomlPoetry(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", "[tool.poetry]\nname = \"myapp\"\n\n[tool.poetry.dependencies]\nfastapi = \"^0.100\"\n")

	p := &Pack{}
	res, _ := p.Detect(t.Context(), dir)
	if res.Metadata["manager"] != "poetry" {
		t.Errorf("expected manager=poetry, got %s", res.Metadata["manager"])
	}
	if res.Metadata["framework"] != "fastapi" {
		t.Errorf("expected framework=fastapi, got %s", res.Metadata["framework"])
	}
	if res.SuggestedTemplate != "python-fastapi" {
		t.Errorf("expected python-fastapi, got %s", res.SuggestedTemplate)
	}
}

func TestDetect_DjangoSuggestsTemplate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", "django==4.2\n")

	p := &Pack{}
	res, _ := p.Detect(t.Context(), dir)
	if res.SuggestedTemplate != "python-django" {
		t.Errorf("expected python-django, got %s", res.SuggestedTemplate)
	}
}

func TestDetect_PlainPyFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.py", "print('hello')\n")

	p := &Pack{}
	res, err := p.Detect(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected detection from .py file")
	}
	if res.Confidence > 0.5 {
		t.Errorf("expected low confidence for bare .py file, got %f", res.Confidence)
	}
}

func TestDetect_FastAPIEntrypoint(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", "fastapi\nuvicorn\n")
	writeFile(t, dir, "app/main.py", "from fastapi import FastAPI\napp = FastAPI()\n")

	p := &Pack{}
	res, _ := p.Detect(t.Context(), dir)
	if res.Metadata["entrypoint"] != "app.main:app" {
		t.Errorf("expected app.main:app, got %s", res.Metadata["entrypoint"])
	}
}

func TestGenerateConfig_FastAPI(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", "fastapi\n")
	writeFile(t, dir, "main.py", "from fastapi import FastAPI\napp = FastAPI()\n")

	p := &Pack{}
	det, _ := p.Detect(t.Context(), dir)
	cfg, err := p.GenerateConfig(t.Context(), dir, det, "python-fastapi")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Build.Pack != "python" {
		t.Errorf("expected python pack, got %s", cfg.Build.Pack)
	}
	if cfg.Image.Entrypoint != "python3" {
		t.Errorf("expected python3 entrypoint, got %s", cfg.Image.Entrypoint)
	}
}

func TestPlan(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", "fastapi\n")

	p := &Pack{}
	det, _ := p.Detect(t.Context(), dir)
	cfg, _ := p.GenerateConfig(t.Context(), dir, det, "python-fastapi")
	plan, err := p.Plan(t.Context(), dir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Melange.Pipeline) < 2 {
		t.Errorf("expected at least 2 pipeline steps (install + copy), got %d", len(plan.Melange.Pipeline))
	}
}
