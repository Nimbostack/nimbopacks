package update

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/Nimbostack/nimbopacks/internal/types"
)

func makeReport(pkgs ...types.PackageUpdate) *types.UpdateReport {
	r := &types.UpdateReport{
		CheckedAt: time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC),
		Packages:  pkgs,
	}
	for _, p := range pkgs {
		switch strings.ToLower(p.Severity) {
		case "critical":
			r.HasCritical = true
		case "high":
			r.HasHigh = true
		}
	}
	if len(pkgs) == 0 {
		r.Summary = "No CVEs found"
	} else {
		r.Summary = "CVEs found"
	}
	return r
}

func TestFormatText_NoCVEs(t *testing.T) {
	var buf bytes.Buffer
	if err := Format(makeReport(), "text", &buf); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "No CVEs found") {
		t.Errorf("want 'No CVEs found', got:\n%s", got)
	}
}

func TestFormatText_WithCVEs(t *testing.T) {
	pkg := types.PackageUpdate{
		Package:        "openssl",
		CurrentVersion: "3.1.4-r0",
		LatestVersion:  "3.1.5-r0",
		CVEs:           []string{"CVE-2024-1234"},
		Severity:       "High",
	}
	var buf bytes.Buffer
	if err := Format(makeReport(pkg), "text", &buf); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "openssl") {
		t.Errorf("expected package name in output:\n%s", got)
	}
	if !strings.Contains(got, "CVE-2024-1234") {
		t.Errorf("expected CVE ID in output:\n%s", got)
	}
	if !strings.Contains(got, "HIGH") {
		t.Errorf("expected severity in output:\n%s", got)
	}
	if !strings.Contains(got, "nimbopacks build") {
		t.Errorf("expected remediation hint in output:\n%s", got)
	}
}

func TestFormatJSON_Valid(t *testing.T) {
	pkg := types.PackageUpdate{
		Package:  "curl",
		Severity: "Critical",
		CVEs:     []string{"CVE-2024-9999"},
	}
	var buf bytes.Buffer
	if err := Format(makeReport(pkg), "json", &buf); err != nil {
		t.Fatal(err)
	}
	var out types.UpdateReport
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if len(out.Packages) != 1 || out.Packages[0].Package != "curl" {
		t.Errorf("unexpected packages: %v", out.Packages)
	}
}

func TestFormatTable_Headers(t *testing.T) {
	pkg := types.PackageUpdate{
		Package:        "zlib",
		CurrentVersion: "1.2.11-r0",
		Severity:       "Medium",
		CVEs:           []string{"CVE-2024-5555"},
	}
	var buf bytes.Buffer
	if err := Format(makeReport(pkg), "table", &buf); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, col := range []string{"PACKAGE", "VERSION", "SEVERITY", "CVE"} {
		if !strings.Contains(got, col) {
			t.Errorf("table missing column %q:\n%s", col, got)
		}
	}
	if !strings.Contains(got, "zlib") {
		t.Errorf("table missing package name:\n%s", got)
	}
}

func TestFormatSARIF_Structure(t *testing.T) {
	pkg := types.PackageUpdate{
		Package:  "libssl",
		Severity: "High",
		CVEs:     []string{"CVE-2024-7777"},
	}
	var buf bytes.Buffer
	if err := Format(makeReport(pkg), "sarif", &buf); err != nil {
		t.Fatal(err)
	}

	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("SARIF is not valid JSON: %v", err)
	}
	if out["version"] != "2.1.0" {
		t.Errorf("SARIF version: got %v", out["version"])
	}
	runs, ok := out["runs"].([]any)
	if !ok || len(runs) == 0 {
		t.Fatal("SARIF missing runs")
	}
}

func TestFormat_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	err := Format(makeReport(), "xml", &buf)
	if err == nil {
		t.Error("expected error for unknown format")
	}
}
