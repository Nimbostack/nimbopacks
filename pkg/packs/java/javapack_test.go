package javapack

import (
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

// --- Detect ---

func TestDetect_NoJava(t *testing.T) {
	p := &Pack{}
	res, err := p.Detect(t.Context(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if res != nil {
		t.Fatal("expected nil for non-Java project")
	}
}

func TestDetect_Maven(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pom.xml", "<project/>")

	p := &Pack{}
	res, err := p.Detect(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.SuggestedTemplate != "java-maven" {
		t.Errorf("expected java-maven, got %s", res.SuggestedTemplate)
	}
	if res.Metadata["build_tool"] != "maven" {
		t.Errorf("expected maven, got %s", res.Metadata["build_tool"])
	}
	if res.Confidence < 0.8 {
		t.Errorf("expected confidence >= 0.8, got %f", res.Confidence)
	}
}

func TestDetect_Gradle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "build.gradle", "plugins { id 'java' }")

	p := &Pack{}
	res, err := p.Detect(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.SuggestedTemplate != "java-gradle" {
		t.Errorf("expected java-gradle, got %s", res.SuggestedTemplate)
	}
	if res.Metadata["build_tool"] != "gradle" {
		t.Errorf("expected gradle, got %s", res.Metadata["build_tool"])
	}
}

func TestDetect_GradleKts(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "build.gradle.kts", "plugins { java }")

	p := &Pack{}
	res, err := p.Detect(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.SuggestedTemplate != "java-gradle" {
		t.Errorf("expected java-gradle for .kts, got %s", res.SuggestedTemplate)
	}
	if res.Metadata["build_tool"] != "gradle" {
		t.Errorf("expected gradle, got %s", res.Metadata["build_tool"])
	}
}

func TestDetect_SpringBoost(t *testing.T) {
	dir := t.TempDir()
	// spring-boot-starter-web contains the literal "spring-boot" substring.
	writeFile(t, dir, "build.gradle", `
dependencies {
    implementation 'org.springframework.boot:spring-boot-starter-web:3.2.0'
}`)

	p := &Pack{}
	res, err := p.Detect(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.Confidence < 0.9 {
		t.Errorf("expected confidence >= 0.9 for Spring, got %f", res.Confidence)
	}
	if res.Metadata["framework"] != "spring-boot" {
		t.Errorf("expected spring-boot framework, got %s", res.Metadata["framework"])
	}
	if res.SuggestedTemplate != "java-gradle" {
		t.Errorf("expected java-gradle for Spring/Gradle project, got %s", res.SuggestedTemplate)
	}
}

// --- GenerateConfig ---

func TestGenerateConfig_Maven(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pom.xml", "<project/>")

	p := &Pack{}
	det, _ := p.Detect(t.Context(), dir)
	cfg, err := p.GenerateConfig(t.Context(), dir, det, "java-maven")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Build.Pack != "java" {
		t.Errorf("expected java pack, got %s", cfg.Build.Pack)
	}
	if cfg.Build.Command != "mvn package -DskipTests" {
		t.Errorf("unexpected Maven command: %s", cfg.Build.Command)
	}
	if cfg.Build.Env["JAR_DIR"] != "target" {
		t.Errorf("expected JAR_DIR=target, got %q", cfg.Build.Env["JAR_DIR"])
	}
	hasMaven := false
	for _, d := range cfg.Build.Dependencies {
		if d == "maven" {
			hasMaven = true
		}
		if d == "gradle" {
			t.Error("gradle should not be in Maven deps")
		}
	}
	if !hasMaven {
		t.Error("expected maven in build dependencies")
	}
}

func TestGenerateConfig_Gradle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "build.gradle", "plugins { id 'java' }")

	p := &Pack{}
	det, _ := p.Detect(t.Context(), dir)
	cfg, err := p.GenerateConfig(t.Context(), dir, det, "java-gradle")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Build.Command != "./gradlew build -x test" {
		t.Errorf("unexpected Gradle command: %s", cfg.Build.Command)
	}
	if cfg.Build.Env["JAR_DIR"] != "build/libs" {
		t.Errorf("expected JAR_DIR=build/libs, got %q", cfg.Build.Env["JAR_DIR"])
	}
	for _, d := range cfg.Build.Dependencies {
		if d == "gradle" {
			t.Error("gradle system package should not be in deps — wrapper downloads it")
		}
		if d == "maven" {
			t.Error("maven should not be in Gradle deps")
		}
	}
}

// --- Plan ---

func TestPlan_Maven(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "pom.xml", "<project/>")

	p := &Pack{}
	det, _ := p.Detect(t.Context(), dir)
	cfg, _ := p.GenerateConfig(t.Context(), dir, det, "java-maven")
	plan, err := p.Plan(t.Context(), dir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Melange.Pipeline) != 2 {
		t.Fatalf("expected 2 pipeline steps, got %d", len(plan.Melange.Pipeline))
	}
	if plan.Melange.Pipeline[0].Runs != "mvn package -DskipTests" {
		t.Errorf("unexpected build step: %s", plan.Melange.Pipeline[0].Runs)
	}
	if !strings.Contains(plan.Melange.Pipeline[1].Runs, "target/") {
		t.Errorf("expected target/ in JAR copy step, got: %s", plan.Melange.Pipeline[1].Runs)
	}
	if len(plan.Melange.Package.Dependencies.Runtime) == 0 {
		t.Error("expected runtime dependencies to be set")
	}
}

func TestPlan_Gradle(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "build.gradle", "plugins { id 'java' }")

	p := &Pack{}
	det, _ := p.Detect(t.Context(), dir)
	cfg, _ := p.GenerateConfig(t.Context(), dir, det, "java-gradle")
	plan, err := p.Plan(t.Context(), dir, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Melange.Pipeline) != 2 {
		t.Fatalf("expected 2 pipeline steps, got %d", len(plan.Melange.Pipeline))
	}
	if plan.Melange.Pipeline[0].Runs != "./gradlew build -x test" {
		t.Errorf("unexpected build step: %s", plan.Melange.Pipeline[0].Runs)
	}
	if !strings.Contains(plan.Melange.Pipeline[1].Runs, "build/libs/") {
		t.Errorf("expected build/libs/ in JAR copy step, got: %s", plan.Melange.Pipeline[1].Runs)
	}
	if len(plan.Melange.Package.Dependencies.Runtime) == 0 {
		t.Error("expected runtime dependencies to be set")
	}
}
