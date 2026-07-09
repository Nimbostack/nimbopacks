package phppack

import (
	"os"
	"path/filepath"
	"testing"
)

func writeComposerJSON(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, "composer.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// --- Detect ---

func TestDetect_NoPhp(t *testing.T) {
	p := &Pack{}
	res, err := p.Detect(t.Context(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Fatal("expected nil for non-PHP project")
	}
}

func TestDetect_Laravel(t *testing.T) {
	dir := t.TempDir()
	writeComposerJSON(t, dir, `{"require": {"laravel/framework": "^13.0"}}`)
	p := &Pack{}
	res, err := p.Detect(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected detection for Laravel project")
	}
	if res.SuggestedTemplate != "php-laravel" {
		t.Errorf("expected php-laravel template, got %s", res.SuggestedTemplate)
	}
}

func TestDetect_CakePHP(t *testing.T) {
	dir := t.TempDir()
	writeComposerJSON(t, dir, `{"require": {"cakephp/cakephp": "^5.0"}}`)
	p := &Pack{}
	res, err := p.Detect(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.SuggestedTemplate != "php-cakephp" {
		t.Fatalf("expected php-cakephp template, got %+v", res)
	}
}

func TestDetect_CodeIgniter(t *testing.T) {
	dir := t.TempDir()
	writeComposerJSON(t, dir, `{"require": {"codeigniter4/framework": "^4.7"}}`)
	p := &Pack{}
	res, err := p.Detect(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.SuggestedTemplate != "php-codeigniter" {
		t.Fatalf("expected php-codeigniter template, got %+v", res)
	}
	if res.Confidence != 0.9 {
		t.Errorf("expected confidence 0.9, got %v", res.Confidence)
	}
}

// --- GenerateConfig ---

func TestGenerateConfig_DocumentRootPerTemplate(t *testing.T) {
	p := &Pack{}
	cases := map[string]string{
		"php-laravel":     "/app/public",
		"php-cakephp":     "/app/webroot",
		"php-codeigniter": "/app/public",
		"php-composer":    "/app",
	}
	for tmpl, wantRoot := range cases {
		cfg, err := p.GenerateConfig(t.Context(), "", nil, tmpl)
		if err != nil {
			t.Fatalf("%s: %v", tmpl, err)
		}
		got := cfg.Image.Cmd[len(cfg.Image.Cmd)-1]
		if got != wantRoot {
			t.Errorf("%s: expected document root %s, got %s", tmpl, wantRoot, got)
		}
	}
}

func TestGenerateConfig_FrameworkEnv(t *testing.T) {
	p := &Pack{}
	cfg, err := p.GenerateConfig(t.Context(), "", nil, "php-laravel")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Image.Env["SESSION_DRIVER"] != "file" {
		t.Errorf("expected SESSION_DRIVER=file for php-laravel, got %+v", cfg.Image.Env)
	}
	if cfg.Build.Env["COMPOSER_CACHE_DIR"] != "/tmp/composer-cache" {
		t.Errorf("expected hyphenated COMPOSER_CACHE_DIR to match templates, got %s", cfg.Build.Env["COMPOSER_CACHE_DIR"])
	}
}

func TestDetect_GenericComposer(t *testing.T) {
	dir := t.TempDir()
	writeComposerJSON(t, dir, `{"require": {"monolog/monolog": "^3.0"}}`)
	p := &Pack{}
	res, err := p.Detect(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.SuggestedTemplate != "php-composer" {
		t.Fatalf("expected php-composer template, got %+v", res)
	}
}
