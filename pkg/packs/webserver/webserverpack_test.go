package webpack

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestDetect_NoWeb(t *testing.T) {
	p := &Pack{}
	res, _ := p.Detect(context.Background(), t.TempDir())
	if res != nil {
		t.Fatal("expected nil for non-web project")
	}
}

func TestDetect_StaticHTML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "index.html", "<html><body>hello</body></html>")

	p := &Pack{}
	res, err := p.Detect(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.Metadata["output_dir"] != "." {
		t.Errorf("expected output_dir=., got %s", res.Metadata["output_dir"])
	}
	if res.SuggestedTemplate != "web-static" {
		t.Errorf("expected web-static, got %s", res.SuggestedTemplate)
	}
}

func TestDetect_PreBuiltDist(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "dist/index.html", "<html></html>")

	p := &Pack{}
	res, _ := p.Detect(context.Background(), dir)
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.Metadata["output_dir"] != "dist" {
		t.Errorf("expected dist, got %s", res.Metadata["output_dir"])
	}
}

func TestDetect_ViteReact(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "vite.config.ts", "export default {}")
	writeFile(t, dir, "package.json", `{"dependencies": {"react": "^18.0.0"}}`)

	p := &Pack{}
	res, _ := p.Detect(context.Background(), dir)
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.Metadata["framework"] != "react" {
		t.Errorf("expected react, got %s", res.Metadata["framework"])
	}
	if res.Metadata["build_cmd"] != "npm ci && npm run build" {
		t.Errorf("unexpected build cmd: %s", res.Metadata["build_cmd"])
	}
	if res.Confidence < 0.9 {
		t.Errorf("expected >= 0.9, got %f", res.Confidence)
	}
}

func TestDetect_Angular(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "angular.json", "{}")

	p := &Pack{}
	res, _ := p.Detect(context.Background(), dir)
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.Metadata["framework"] != "angular" {
		t.Errorf("expected angular, got %s", res.Metadata["framework"])
	}
}

func TestDetect_Hugo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "hugo.toml", "baseURL = 'https://example.com'")
	if err := os.MkdirAll(filepath.Join(dir, "content"), 0o755); err != nil {
		t.Fatal(err)
	}

	p := &Pack{}
	res, _ := p.Detect(context.Background(), dir)
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.Metadata["framework"] != "hugo" {
		t.Errorf("expected hugo, got %s", res.Metadata["framework"])
	}
	if res.Metadata["output_dir"] != "public" {
		t.Errorf("expected public, got %s", res.Metadata["output_dir"])
	}
}

func TestDetect_CustomNginxConf(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "index.html", "<html></html>")
	writeFile(t, dir, "nginx.conf", "worker_processes 1;")

	p := &Pack{}
	res, _ := p.Detect(context.Background(), dir)
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.Metadata["has_nginx_conf"] != "true" {
		t.Error("expected has_nginx_conf=true")
	}
}

func TestDetect_NginxConfInSubdir(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "index.html", "<html></html>")
	writeFile(t, dir, "config/nginx.conf", "worker_processes 1;")

	p := &Pack{}
	res, _ := p.Detect(context.Background(), dir)
	if res.Metadata["has_nginx_conf"] != "true" {
		t.Error("expected nginx conf detection in config/")
	}
	if res.Metadata["nginx_conf_path"] != "config/nginx.conf" {
		t.Errorf("expected config/nginx.conf, got %s", res.Metadata["nginx_conf_path"])
	}
}

func TestPlan_DefaultNginxConf(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "index.html", "<html></html>")

	p := &Pack{}
	det, _ := p.Detect(context.Background(), dir)
	cfg, _ := p.GenerateConfig(context.Background(), dir, det, "web-static")
	plan, err := p.Plan(context.Background(), dir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Should have steps for: copy files, nginx.conf, mime.types, directories
	if len(plan.Melange.Pipeline) < 4 {
		t.Errorf("expected at least 4 pipeline steps, got %d", len(plan.Melange.Pipeline))
	}

	// Check that default nginx.conf is embedded.
	foundNginxConf := false
	foundMimeTypes := false
	for _, step := range plan.Melange.Pipeline {
		if strings.Contains(step.Runs, "worker_processes auto") {
			foundNginxConf = true
		}
		if strings.Contains(step.Runs, "text/html") {
			foundMimeTypes = true
		}
	}
	if !foundNginxConf {
		t.Error("expected default nginx.conf in pipeline")
	}
	if !foundMimeTypes {
		t.Error("expected default mime.types in pipeline")
	}
}

func TestPlan_WithBuildStep(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "vite.config.ts", "export default {}")
	writeFile(t, dir, "package.json", `{"dependencies": {"react": "^18"}}`)

	p := &Pack{}
	det, _ := p.Detect(context.Background(), dir)
	cfg, _ := p.GenerateConfig(context.Background(), dir, det, "web-spa")
	plan, err := p.Plan(context.Background(), dir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	// First step should be the build command.
	if !strings.Contains(plan.Melange.Pipeline[0].Runs, "npm") {
		t.Errorf("expected build step first, got: %s", plan.Melange.Pipeline[0].Runs)
	}
}
