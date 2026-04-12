package templates

import (
	"strings"
	"testing"
)

func TestRender_GoAPI(t *testing.T) {
	content, err := Render("go", TemplateVars{
		ProjectName: "myservice",
		Version:     "1.0.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "name: myservice") {
		t.Error("expected project name")
	}
	if !strings.Contains(content, "version: 1.0.0") {
		t.Error("expected version")
	}
	if !strings.Contains(content, "pack: golang") {
		t.Error("expected go pack")
	}
}

func TestRender_DotnetSolution(t *testing.T) {
	content, err := Render("dotnet-solution", TemplateVars{
		ProjectName: "OrderSystem",
		Version:     "2.0.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "OrderSystem.Api") {
		t.Error("expected artifact references")
	}
	if !strings.Contains(content, "artifacts:") {
		t.Error("expected artifacts section")
	}
}

func TestRender_WebSPA(t *testing.T) {
	content, err := Render("web-spa", TemplateVars{
		ProjectName: "frontend",
		Version:     "1.0.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "nginx") {
		t.Error("expected nginx in web-spa template")
	}
	if !strings.Contains(content, "pack: web") {
		t.Error("expected web pack")
	}
}

func TestRender_Unknown(t *testing.T) {
	_, err := Render("nonexistent", TemplateVars{})
	if err == nil {
		t.Fatal("expected error for unknown template")
	}
}

func TestList_HasExpectedTemplates(t *testing.T) {
	all := List()
	names := make(map[string]bool)
	for _, tmpl := range all {
		names[tmpl.Name] = true
	}

	expected := []string{
		"go", "go-grpc",
		"python-fastapi", "python-django",
		"node-express", "node-nextjs",
		"java-maven", "java-gradle",
		"dotnet-webapi", "dotnet-solution", "dotnet-minimal-api",
		"web-static", "web-spa", "web-hugo",
	}
	for _, e := range expected {
		if !names[e] {
			t.Errorf("missing template: %s", e)
		}
	}
}

func TestListByPack(t *testing.T) {
	dotnet := ListByPack("dotnet")
	if len(dotnet) != 3 {
		t.Errorf("expected 3 dotnet templates, got %d", len(dotnet))
	}

	web := ListByPack("web")
	if len(web) != 3 {
		t.Errorf("expected 3 web templates, got %d", len(web))
	}

	java := ListByPack("java")
	if len(java) != 2 {
		t.Errorf("expected 2 java templates, got %d", len(java))
	}
}

func TestParsedMetadata(t *testing.T) {
	tmpl := Get("go")
	if tmpl == nil {
		t.Fatal("go not found")
	}
	if tmpl.Pack != "golang" {
		t.Errorf("expected pack=golang, got %s", tmpl.Pack)
	}
	if tmpl.Description == "" {
		t.Error("expected description")
	}
	if len(tmpl.Tags) == 0 {
		t.Error("expected tags")
	}
}

func TestRender_JavaMaven(t *testing.T) {
	content, err := Render("java-maven", TemplateVars{
		ProjectName: "myapp",
		Version:     "1.0.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "name: myapp") {
		t.Error("expected project name")
	}
	if !strings.Contains(content, "version: 1.0.0") {
		t.Error("expected version")
	}
	if !strings.Contains(content, "pack: java") {
		t.Error("expected java pack")
	}
	if !strings.Contains(content, "mvn") {
		t.Error("expected mvn build command")
	}
}

func TestRender_JavaGradle(t *testing.T) {
	content, err := Render("java-gradle", TemplateVars{
		ProjectName: "myapp",
		Version:     "1.0.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "name: myapp") {
		t.Error("expected project name")
	}
	if !strings.Contains(content, "version: 1.0.0") {
		t.Error("expected version")
	}
	if !strings.Contains(content, "pack: java") {
		t.Error("expected java pack")
	}
	if !strings.Contains(content, "gradlew") {
		t.Error("expected gradlew build command")
	}
}
