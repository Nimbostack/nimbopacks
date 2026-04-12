package update

import (
	"context"
	"fmt"
	"testing"
)

func TestRunGrype_ParsesSBOMMatches(t *testing.T) {
	old := execGrype
	defer func() { execGrype = old }()

	execGrype = func(_ context.Context, _ []string, _ []string) ([]byte, error) {
		return []byte(`{
			"matches": [
				{
					"vulnerability": {
						"id": "CVE-2024-1234",
						"severity": "High",
						"description": "Test vuln",
						"fix": {"versions": ["3.1.5-r0"], "state": "fixed"}
					},
					"artifact": {"name": "openssl", "version": "3.1.4-r0", "type": "apk"}
				}
			]
		}`), nil
	}

	matches, err := RunGrype(context.Background(), ScanInput{Type: ScanSBOM, Path: "/fake/sbom.json"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("want 1 match, got %d", len(matches))
	}
	m := matches[0]
	if m.Vulnerability.ID != "CVE-2024-1234" {
		t.Errorf("ID: got %q", m.Vulnerability.ID)
	}
	if m.Vulnerability.Severity != "High" {
		t.Errorf("Severity: got %q", m.Vulnerability.Severity)
	}
	if m.Artifact.Name != "openssl" {
		t.Errorf("Artifact.Name: got %q", m.Artifact.Name)
	}
	if m.Vulnerability.Fix.State != "fixed" {
		t.Errorf("Fix.State: got %q", m.Vulnerability.Fix.State)
	}
}

func TestRunGrype_OCIArchiveInput(t *testing.T) {
	old := execGrype
	defer func() { execGrype = old }()

	var capturedArgs []string
	execGrype = func(_ context.Context, args []string, _ []string) ([]byte, error) {
		capturedArgs = args
		return []byte(`{"matches":[]}`), nil
	}

	_, err := RunGrype(context.Background(), ScanInput{Type: ScanOCIArchive, Path: "/fake/image.tar"}, "", "")
	if err != nil {
		t.Fatal(err)
	}

	if len(capturedArgs) == 0 || capturedArgs[0] != "oci-archive:/fake/image.tar" {
		t.Errorf("args[0]: got %v", capturedArgs)
	}
}

func TestRunGrype_GrypeConfigPassedThrough(t *testing.T) {
	old := execGrype
	defer func() { execGrype = old }()

	var capturedArgs []string
	execGrype = func(_ context.Context, args []string, _ []string) ([]byte, error) {
		capturedArgs = args
		return []byte(`{"matches":[]}`), nil
	}

	_, err := RunGrype(context.Background(), ScanInput{Type: ScanSBOM, Path: "/sbom.json"}, "/project/.grype.yaml", "")
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for i, a := range capturedArgs {
		if a == "--config" && i+1 < len(capturedArgs) && capturedArgs[i+1] == "/project/.grype.yaml" {
			found = true
		}
	}
	if !found {
		t.Errorf("--config flag not found in args: %v", capturedArgs)
	}
}

func TestRunGrype_DBCacheEnvSet(t *testing.T) {
	old := execGrype
	defer func() { execGrype = old }()

	var capturedEnv []string
	execGrype = func(_ context.Context, _ []string, env []string) ([]byte, error) {
		capturedEnv = env
		return []byte(`{"matches":[]}`), nil
	}

	_, err := RunGrype(context.Background(), ScanInput{Type: ScanSBOM, Path: "/sbom.json"}, "", "/ci/grype-cache")
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, e := range capturedEnv {
		if e == "GRYPE_DB_CACHE_DIR=/ci/grype-cache" {
			found = true
		}
	}
	if !found {
		t.Errorf("GRYPE_DB_CACHE_DIR not found in env: %v", capturedEnv)
	}
}

func TestRunGrype_EmptyOutput_ReturnsError(t *testing.T) {
	old := execGrype
	defer func() { execGrype = old }()

	execGrype = func(_ context.Context, _ []string, _ []string) ([]byte, error) {
		return []byte{}, fmt.Errorf("grype: exit status 2")
	}

	_, err := RunGrype(context.Background(), ScanInput{Type: ScanSBOM, Path: "/sbom.json"}, "", "")
	if err == nil {
		t.Error("expected error when grype output is empty")
	}
}
