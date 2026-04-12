package golangpack

import (
	"context"
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

func TestDetect_NoGoMod(t *testing.T) {
	p := &Pack{}
	res, err := p.Detect(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Fatal("expected nil")
	}
}

func TestDetect_BasicProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module github.com/example/myapp\n\ngo 1.22\n")
	writeFile(t, dir, "main.go", "package main\nfunc main() {}\n")

	p := &Pack{}
	res, err := p.Detect(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected result")
	}
	if res.Confidence < 0.8 {
		t.Errorf("low confidence: %f", res.Confidence)
	}
	if res.Metadata["module_path"] != "github.com/example/myapp" {
		t.Errorf("wrong module: %s", res.Metadata["module_path"])
	}
	if res.SuggestedTemplate != "go" {
		t.Errorf("expected go template, got %s", res.SuggestedTemplate)
	}
}

func TestDetect_CmdLayout(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module github.com/example/multi\ngo 1.22\n")
	writeFile(t, dir, "cmd/server/main.go", "package main\nfunc main() {}\n")

	p := &Pack{}
	res, _ := p.Detect(context.Background(), dir)
	if res.Metadata["entrypoint"] != "./cmd/server" {
		t.Errorf("expected ./cmd/server, got %s", res.Metadata["entrypoint"])
	}
	if res.Confidence < 0.9 {
		t.Errorf("expected >= 0.9, got %f", res.Confidence)
	}
}

func TestDetect_GRPCSuggestsTemplate(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module example.com/svc\ngo 1.22\nrequire google.golang.org/grpc v1.60.0\n")

	p := &Pack{}
	res, _ := p.Detect(context.Background(), dir)
	if res.SuggestedTemplate != "go-grpc" {
		t.Errorf("expected go-grpc, got %s", res.SuggestedTemplate)
	}
}

func TestGenerateConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module github.com/example/myapp\ngo 1.22\n")

	p := &Pack{}
	det, _ := p.Detect(context.Background(), dir)
	cfg, err := p.GenerateConfig(context.Background(), dir, det, "go")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Project.Name != "myapp" {
		t.Errorf("expected myapp, got %s", cfg.Project.Name)
	}
	if cfg.Build.Pack != "golang" {
		t.Errorf("expected go pack, got %s", cfg.Build.Pack)
	}
}

func TestPlan(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module github.com/example/myapp\ngo 1.22\n")

	p := &Pack{}
	det, _ := p.Detect(context.Background(), dir)
	cfg, _ := p.GenerateConfig(context.Background(), dir, det, "go")
	plan, err := p.Plan(context.Background(), dir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Melange.Package.Name != "myapp" {
		t.Errorf("expected myapp, got %s", plan.Melange.Package.Name)
	}
	if len(plan.Melange.Pipeline) == 0 {
		t.Error("expected pipeline steps")
	}
	if plan.Apko.Entrypoint.Command != "/usr/bin/myapp" {
		t.Errorf("expected /usr/bin/myapp, got %s", plan.Apko.Entrypoint.Command)
	}
}
