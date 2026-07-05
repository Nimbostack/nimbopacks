package dotnetpack

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

func TestDetect_NoDotnet(t *testing.T) {
	p := &Pack{}
	res, _ := p.Detect(t.Context(), t.TempDir())
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
	res, err := p.Detect(t.Context(), dir)
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

func TestDetect_Grpc(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "MyGrpc.csproj", `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Grpc.AspNetCore" Version="2.66.0" />
  </ItemGroup>
</Project>`)
	writeFile(t, dir, "Protos/greet.proto", `syntax = "proto3";
service Greeter { rpc SayHello (HelloRequest) returns (HelloReply); }`)
	writeFile(t, dir, "Program.cs", `var builder = WebApplication.CreateBuilder(args);
builder.Services.AddGrpc();
var app = builder.Build();
app.Run();`)

	p := &Pack{}
	res, err := p.Detect(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.SuggestedTemplate != "dotnet-grpc" {
		t.Errorf("expected dotnet-grpc, got %s", res.SuggestedTemplate)
	}
	if res.Metadata["has_grpc"] != "true" {
		t.Error("expected has_grpc metadata")
	}
}

func TestDetect_Blazor(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "MyBlazor.csproj", `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>`)
	writeFile(t, dir, "Program.cs", `var builder = WebApplication.CreateBuilder(args);
builder.Services.AddRazorComponents();
var app = builder.Build();
app.Run();`)
	writeFile(t, dir, "Components/Pages/Home.razor", `@page "/"
<h1>Hello</h1>`)

	p := &Pack{}
	res, err := p.Detect(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.SuggestedTemplate != "dotnet-blazor" {
		t.Errorf("expected dotnet-blazor, got %s", res.SuggestedTemplate)
	}
	if res.Metadata["has_blazor"] != "true" {
		t.Error("expected has_blazor metadata")
	}
}

func TestDetect_Worker(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "MyWorker.csproj", `<Project Sdk="Microsoft.NET.Sdk.Worker">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>`)
	writeFile(t, dir, "Program.cs", `var builder = Host.CreateApplicationBuilder(args);
builder.Services.AddHostedService<Worker>();
var host = builder.Build();
host.Run();`)

	p := &Pack{}
	res, err := p.Detect(t.Context(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if res == nil {
		t.Fatal("expected detection")
	}
	if res.SuggestedTemplate != "dotnet-worker" {
		t.Errorf("expected dotnet-worker, got %s", res.SuggestedTemplate)
	}
	if res.Metadata["has_worker"] != "true" {
		t.Error("expected has_worker metadata")
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
	res, _ := p.Detect(t.Context(), dir)
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
	det, _ := p.Detect(t.Context(), dir)
	cfg, err := p.GenerateConfig(t.Context(), dir, det, "dotnet-solution")
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
	det, _ := p.Detect(t.Context(), dir)
	cfg, _ := p.GenerateConfig(t.Context(), dir, det, "dotnet-solution")
	plan, err := p.Plan(t.Context(), dir, cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Should have restore step + one publish per artifact + the install step.
	expectedSteps := 1 + len(cfg.Artifacts) + 1 // restore + N publishes + install-to-destdir
	if len(plan.Melange.Pipeline) != expectedSteps {
		t.Errorf("expected %d pipeline steps, got %d", expectedSteps, len(plan.Melange.Pipeline))
	}
}
