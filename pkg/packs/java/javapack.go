// Package javapack is the nimbopacks pack for Java projects (Maven and Gradle).
package javapack

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

// javaHome is the install prefix of the Wolfi openjdk-21 / openjdk-21-jre
// packages. The JRE does not put `java` on PATH, so both the build env and the
// image entrypoint are derived from this path.
const javaHome = "/usr/lib/jvm/java-21-openjdk"

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

	// Scan build files for framework markers. Order matters: a more specific
	// framework (Quarkus, Micronaut, Spring WebFlux) wins over the generic
	// Spring Boot detection.
	confidence := 0.85
	var contents string
	var contentsSb43 strings.Builder
	for _, f := range []string{"pom.xml", "build.gradle", "build.gradle.kts"} {
		if c, err := pack.ReadFile(srcDir, f); err == nil {
			contentsSb43.WriteString(c)
		}
	}
	contents += contentsSb43.String()
	switch {
	case strings.Contains(contents, "quarkus"):
		meta["framework"] = "quarkus"
		suggested = "java-quarkus"
		confidence = 0.9
	case strings.Contains(contents, "micronaut"):
		meta["framework"] = "micronaut"
		suggested = "java-micronaut"
		confidence = 0.9
	case strings.Contains(contents, "spring-boot-starter-webflux"):
		meta["framework"] = "spring-boot-webflux"
		suggested = "java-webflux"
		confidence = 0.9
	case strings.Contains(contents, "spring-boot"):
		meta["framework"] = "spring-boot"
		confidence = 0.9
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
	// The Wolfi openjdk JRE does not put `java` on PATH, so use the absolute
	// path from JAVA_HOME rather than a bare "java".
	cfg.Image.Entrypoint = javaHome + "/bin/java"
	cfg.Image.Cmd = []string{"-jar", "/app/app.jar"}
	cfg.Image.Packages = []string{"openjdk-21-jre", "ca-certificates-bundle"}
	cfg.Image.Env = map[string]string{
		"JAVA_HOME": javaHome,
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

	// The Wolfi openjdk-21 package does not put `java` on PATH or export
	// JAVA_HOME, so mvn/gradle abort with "JAVA_HOME is not defined correctly".
	// Export it (from image.env if the user set it, else the Wolfi default)
	// before running the build command.
	buildJavaHome := javaHome
	if jh := cfg.Image.Env["JAVA_HOME"]; jh != "" {
		buildJavaHome = jh
	}
	buildCmd := fmt.Sprintf("export JAVA_HOME=%s\nexport PATH=\"$JAVA_HOME/bin:$PATH\"\n%s", buildJavaHome, cfg.Build.Command)

	melange.Pipeline = []types.MelangePipelineStep{
		{Runs: buildCmd},
		{Runs: "mkdir -p /home/build/output/app\ncp " + jarDir + "/*.jar /home/build/output/app/"},
		pack.InstallOutputStep(),
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
