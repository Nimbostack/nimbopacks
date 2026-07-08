package phppack

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
	writeFile(t, dir, "composer.json", `{"require": {"laravel/framework": "^13.0"}}`)
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
	writeFile(t, dir, "composer.json", `{"require": {"cakephp/cakephp": "^5.0"}}`)
	p := &Pack{}
	res, err := p.Detect(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.SuggestedTemplate != "php-cakephp" {
		t.Fatalf("expected php-cakephp template, got %+v", res)
	}
}

func TestDetect_GenericComposer(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "composer.json", `{"require": {"monolog/monolog": "^3.0"}}`)
	p := &Pack{}
	res, err := p.Detect(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil || res.SuggestedTemplate != "php-composer" {
		t.Fatalf("expected php-composer template, got %+v", res)
	}
}
