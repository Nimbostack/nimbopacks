// Package javapack is the nimbopacks pack for Java projects (Maven and Gradle).
package javapack

import (
	"context"
	"maps"
	"strings"

	"github.com/Nimbostack/nimbopacks/internal/pack"
	"github.com/Nimbostack/nimbopacks/internal/pack/registry"
	"github.com/Nimbostack/nimbopacks/internal/types"
)

func init() { registry.Register(&Pack{}) }

type Pack struct{}

func (p *Pack) Name() string { return "java" }

func (p *Pack) Detect(_ context.Context, srcDir string) (*types.DetectResult, error) {
	meta := make(map[string]string)
	var buildTool string
	switch {
	case pack.FileExists(srcDir, "pom.xml"):
		buildTool = "maven"
	case pack.FileExists(srcDir, "build.gradle"), pack.FileExists(srcDir, "build.gradle.kts"):
		buildTool = "gradle"
	default:
		return nil, nil
	}
	meta["build_tool"] = buildTool

	suggested := "java-maven"
	if buildTool == "gradle" {
		suggested = "java-gradle"
	}

	confidence := 0.85
	for _, f := range []string{"pom.xml", "build.gradle", "build.gradle.kts"} {
		if c, err := pack.ReadFile(srcDir, f); err == nil && strings.Contains(c, "spring-boot") {
			meta["framework"] = "spring-boot"
			confidence = 0.9
		}
	}

	return &types.DetectResult{
		PackName:          p.Name(),
		Confidence:        confidence,
		Summary:           "Java project (" + buildTool + ")",
		SuggestedTemplate: suggested,
		Metadata:          meta,
	}, nil
}

func (p *Pack) GenerateConfig(_ context.Context, _ string, det *types.DetectResult, tmpl string) (*types.NimpackConfig, error) {
	cfg := pack.BaseConfig("app", "0.0.1", p.Name(), tmpl)
	if det.Metadata["build_tool"] == "gradle" {
		cfg.Build.Command = "./gradlew build -x test"
		cfg.Build.Dependencies = []string{"openjdk-21", "build-base"}
		cfg.Build.Env = map[string]string{"JAR_DIR": "build/libs"}
	} else {
		cfg.Build.Command = "mvn package -DskipTests"
		cfg.Build.Dependencies = []string{"openjdk-21", "maven", "build-base"}
		cfg.Build.Env = map[string]string{"JAR_DIR": "target"}
	}
	cfg.Image.Entrypoint = "java"
	cfg.Image.Cmd = []string{"-jar", "/app/app.jar"}
	cfg.Image.Packages = []string{"openjdk-21-jre", "ca-certificates-bundle"}
	cfg.Image.Env = map[string]string{
		"JAVA_HOME": "/usr/lib/jvm/java-21-openjdk",
		"JAVA_OPTS": "-XX:+UseContainerSupport -XX:MaxRAMPercentage=75.0",
	}
	return &cfg, nil
}

func (p *Pack) Plan(_ context.Context, _ string, cfg *types.NimpackConfig) (*types.BuildPlan, error) {
	melange := pack.NewMelangeConfig(cfg.Project.Name, cfg.Project.Version, cfg.Build.Dependencies)
	melange.Package.Dependencies = types.MelangeDependencies{
		Runtime: cfg.Image.Packages,
	}

	jarDir := "target" // Maven default; safe fallback for hand-authored configs
	if dir, ok := cfg.Build.Env["JAR_DIR"]; ok && dir != "" {
		jarDir = dir
	}

	melange.Pipeline = []types.MelangePipelineStep{
		{Runs: cfg.Build.Command},
		{Runs: "mkdir -p /home/build/output/app\ncp " + jarDir + "/*.jar /home/build/output/app/"},
	}

	apko := pack.NewApkoConfig(cfg.Project.Name, cfg.Image.Entrypoint, cfg.Image.Packages)
	if len(cfg.Image.Cmd) > 0 {
		apko.Cmd = strings.Join(cfg.Image.Cmd, " ")
	}
	maps.Copy(apko.Environment, cfg.Image.Env)

	plan := &types.BuildPlan{Melange: melange, Apko: apko}
	pack.ApplyConfig(plan, cfg)
	return plan, nil
}
