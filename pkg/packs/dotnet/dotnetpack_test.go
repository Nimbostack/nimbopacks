package dotnetpack

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

func TestDetect_NoDotnet(t *testing.T) {
	p := &Pack{}
	res, _ := p.Detect(context.Background(), t.TempDir())
	if res != nil {
		t.Fatal("expected nil for non-.NET project")
	}
}

func TestDetect_SingleWebAPI(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "MyApp.csproj", `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>`)
	writeFile(t, dir, "Program.cs", `var builder = WebApplication.CreateBuilder(args);
var app = builder.Build();
app.MapGet("/", () => "Hello");
app.Run();`)

	p := &Pack{}
	res, err := p.Detect(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.SuggestedTemplate != "dotnet-minimal-api" {
		t.Errorf("expected dotnet-minimal-api, got %s", res.SuggestedTemplate)
	}
	if res.Metadata["minimal_api"] != "true" {
		t.Error("expected minimal_api metadata")
	}
	if res.Metadata["target_framework"] != "net8.0" {
		t.Errorf("expected net8.0, got %s", res.Metadata["target_framework"])
	}
}

func TestDetect_SolutionMonorepo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "MyApp.sln", "Microsoft Visual Studio Solution File")
	writeFile(t, dir, "src/MyApp.Api/MyApp.Api.csproj", `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup>
</Project>`)
	writeFile(t, dir, "src/MyApp.Worker/MyApp.Worker.csproj", `<Project Sdk="Microsoft.NET.Sdk.Worker">
  <PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup>
</Project>`)
	writeFile(t, dir, "src/MyApp.Core/MyApp.Core.csproj", `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup>
</Project>`)

	p := &Pack{}
	res, _ := p.Detect(context.Background(), dir)
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.SuggestedTemplate != "dotnet-solution" {
		t.Errorf("expected dotnet-solution, got %s", res.SuggestedTemplate)
	}
	if res.Confidence < 0.9 {
		t.Errorf("expected high confidence for solution, got %f", res.Confidence)
	}
	if res.Metadata["project_count"] != "3" {
		t.Errorf("expected 3 projects, got %s", res.Metadata["project_count"])
	}
	if res.Metadata["has_web"] != "true" {
		t.Error("expected has_web")
	}
	if res.Metadata["has_worker"] != "true" {
		t.Error("expected has_worker")
	}
}

func TestGenerateConfig_Solution(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "MyApp.sln", "solution")
	writeFile(t, dir, "src/MyApp.Api/MyApp.Api.csproj", `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup>
</Project>`)
	writeFile(t, dir, "src/MyApp.Worker/MyApp.Worker.csproj", `<Project Sdk="Microsoft.NET.Sdk.Worker">
  <PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup>
</Project>`)

	p := &Pack{}
	det, _ := p.Detect(context.Background(), dir)
	cfg, err := p.GenerateConfig(context.Background(), dir, det, "dotnet-solution")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Artifacts) < 2 {
		t.Errorf("expected at least 2 artifacts, got %d", len(cfg.Artifacts))
	}
	// Check that web and worker projects are represented.
	foundWeb := false
	foundWorker := false
	for _, a := range cfg.Artifacts {
		if a.Source == "src/MyApp.Api" {
			foundWeb = true
		}
		if a.Source == "src/MyApp.Worker" {
			foundWorker = true
		}
	}
	if !foundWeb {
		t.Error("expected web project in artifacts")
	}
	if !foundWorker {
		t.Error("expected worker project in artifacts")
	}
}

func TestPlan_Monorepo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "MyApp.sln", "solution")
	writeFile(t, dir, "src/MyApp.Api/MyApp.Api.csproj", `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup><TargetFramework>net8.0</TargetFramework></PropertyGroup>
</Project>`)

	p := &Pack{}
	det, _ := p.Detect(context.Background(), dir)
	cfg, _ := p.GenerateConfig(context.Background(), dir, det, "dotnet-solution")
	plan, err := p.Plan(context.Background(), dir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Should have restore step + one publish per artifact.
	expectedSteps := 1 + len(cfg.Artifacts) // restore + N publishes
	if len(plan.Melange.Pipeline) != expectedSteps {
		t.Errorf("expected %d pipeline steps, got %d", expectedSteps, len(plan.Melange.Pipeline))
	}
}
