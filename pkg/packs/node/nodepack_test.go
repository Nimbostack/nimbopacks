package nodepack

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

func TestDetect_NoPackageJSON(t *testing.T) {
	p := &Pack{}
	res, err := p.Detect(t.Context(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Fatal("expected nil for non-Node project")
	}
}

func TestDetect_BasicNodeProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"myapp","dependencies":{}}`)

	p := &Pack{}
	res, err := p.Detect(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.Confidence < 0.8 {
		t.Errorf("expected confidence >= 0.8, got %f", res.Confidence)
	}
	if res.Metadata["name"] != "myapp" {
		t.Errorf("expected name=myapp, got %s", res.Metadata["name"])
	}
	if res.Metadata["pm"] != "npm" {
		t.Errorf("expected pm=npm, got %s", res.Metadata["pm"])
	}
}

func TestDetect_NextJSSuggestsTemplate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"site","dependencies":{"next":"14.0.0","react":"18.0.0"}}`)

	p := &Pack{}
	res, _ := p.Detect(t.Context(), dir)
	if res.SuggestedTemplate != "node-nextjs" {
		t.Errorf("expected node-nextjs, got %s", res.SuggestedTemplate)
	}
	if res.Metadata["framework"] != "nextjs" {
		t.Errorf("expected framework=nextjs, got %s", res.Metadata["framework"])
	}
}

func TestDetect_TypeScriptProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"api","dependencies":{"express":"4.18.0"}}`)
	writeFile(t, dir, "tsconfig.json", `{"compilerOptions":{}}`)

	p := &Pack{}
	res, _ := p.Detect(t.Context(), dir)
	if res.Metadata["typescript"] != "true" {
		t.Error("expected typescript=true")
	}
}

func TestDetect_YarnLockfileDetected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"app","dependencies":{}}`)
	writeFile(t, dir, "yarn.lock", "# yarn lockfile v1\n")

	p := &Pack{}
	res, _ := p.Detect(t.Context(), dir)
	if res.Metadata["pm"] != "yarn" {
		t.Errorf("expected pm=yarn, got %s", res.Metadata["pm"])
	}
}

func TestGenerateConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"myapi","dependencies":{"express":"4.18.0"}}`)

	p := &Pack{}
	det, _ := p.Detect(t.Context(), dir)
	cfg, err := p.GenerateConfig(t.Context(), dir, det, "node-express")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Name != "myapi" {
		t.Errorf("expected myapi, got %s", cfg.Project.Name)
	}
	if cfg.Build.Pack != "node" {
		t.Errorf("expected node pack, got %s", cfg.Build.Pack)
	}
	if cfg.Image.Entrypoint != "node" {
		t.Errorf("expected node entrypoint, got %s", cfg.Image.Entrypoint)
	}
}

func TestPlan(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{"name":"myapi","dependencies":{}}`)

	p := &Pack{}
	det, _ := p.Detect(t.Context(), dir)
	cfg, _ := p.GenerateConfig(t.Context(), dir, det, "node-express")
	plan, err := p.Plan(t.Context(), dir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Melange.Package.Name != "myapi" {
		t.Errorf("expected myapi, got %s", plan.Melange.Package.Name)
	}
	if len(plan.Melange.Pipeline) < 2 {
		t.Errorf("expected at least 2 pipeline steps (build + copy), got %d", len(plan.Melange.Pipeline))
	}
}
