package update

import (
	"strings"
	"testing"

	"github.com/Nimbostack/nimbopacks/internal/types"
)

func TestBuildReport_GroupsByCVEPerPackage(t *testing.T) {
	matches := []GrypeMatch{
		{
			Vulnerability: GrypeVulnerability{ID: "CVE-2024-001", Severity: "High"},
			Artifact:      GrypeArtifact{Name: "openssl", Version: "3.1.4-r0"},
		},
		{
			Vulnerability: GrypeVulnerability{ID: "CVE-2024-002", Severity: "Critical"},
			Artifact:      GrypeArtifact{Name: "openssl", Version: "3.1.4-r0"},
		},
		{
			Vulnerability: GrypeVulnerability{ID: "CVE-2024-003", Severity: "Medium"},
			Artifact:      GrypeArtifact{Name: "curl", Version: "8.4.0-r0"},
		},
	}

	report := buildReport(matches, nil)

	if len(report.Packages) != 2 {
		t.Fatalf("want 2 packages, got %d", len(report.Packages))
	}
	if !report.HasCritical {
		t.Error("HasCritical should be true")
	}
	if !report.HasHigh {
		t.Error("HasHigh should be true")
	}
	// openssl should show worst severity: Critical
	var openssl *types.PackageUpdate
	for i := range report.Packages {
		if report.Packages[i].Package == "openssl" {
			openssl = &report.Packages[i]
		}
	}
	if openssl == nil {
		t.Fatal("openssl not in report")
	}
	if openssl.Severity != "Critical" {
		t.Errorf("openssl severity: got %q, want Critical", openssl.Severity)
	}
	if len(openssl.CVEs) != 2 {
		t.Errorf("openssl CVEs: got %v", openssl.CVEs)
	}
}

func TestBuildReport_NoCVEs(t *testing.T) {
	report := buildReport(nil, nil)
	if len(report.Packages) != 0 {
		t.Errorf("want 0 packages, got %d", len(report.Packages))
	}
	if !strings.Contains(report.Summary, "No CVEs") {
		t.Errorf("summary: got %q", report.Summary)
	}
}

func TestBuildReport_PinnedWarning(t *testing.T) {
	matches := []GrypeMatch{{
		Vulnerability: GrypeVulnerability{ID: "CVE-2024-999", Severity: "High"},
		Artifact:      GrypeArtifact{Name: "openssl", Version: "3.1.4-r0"},
	}}
	pinned := []string{"openssl=3.1.4-r0"}

	report := buildReport(matches, pinned)

	if len(report.Packages) != 1 {
		t.Fatalf("want 1 package, got %d", len(report.Packages))
	}
	if !report.Packages[0].Pinned {
		t.Error("openssl should be marked pinned")
	}
}

func TestSeverityRank(t *testing.T) {
	cases := []struct {
		s    string
		want int
	}{
		{"critical", 4},
		{"Critical", 4},
		{"high", 3},
		{"medium", 2},
		{"low", 1},
		{"negligible", 0},
		{"unknown", -1},
		{"", -1},
	}
	for _, c := range cases {
		if got := severityRank(c.s); got != c.want {
			t.Errorf("severityRank(%q) = %d, want %d", c.s, got, c.want)
		}
	}
}

func TestIsPinned(t *testing.T) {
	pinned := []string{"openssl=3.1.4-r0", "curl"}

	if !isPinned("openssl", "3.1.4-r0", pinned) {
		t.Error("openssl@3.1.4-r0 should be pinned")
	}
	if isPinned("openssl", "3.1.5-r0", pinned) {
		t.Error("openssl@3.1.5-r0 should not be pinned (different version)")
	}
	if !isPinned("curl", "8.4.0-r0", pinned) {
		t.Error("curl (any version) should be pinned")
	}
	if isPinned("zlib", "1.2.11-r0", pinned) {
		t.Error("zlib should not be pinned")
	}
}

func TestMeetsThreshold(t *testing.T) {
	criticalPkg := types.PackageUpdate{Package: "openssl", Severity: "Critical", CVEs: []string{"CVE-2024-001"}}
	highPkg := types.PackageUpdate{Package: "curl", Severity: "High", CVEs: []string{"CVE-2024-002"}}
	medPkg := types.PackageUpdate{Package: "zlib", Severity: "Medium", CVEs: []string{"CVE-2024-003"}}

	withCriticalAndHigh := &types.UpdateReport{Packages: []types.PackageUpdate{criticalPkg, highPkg}}
	if !MeetsThreshold(withCriticalAndHigh, "high") {
		t.Error("high threshold should trigger when High package present")
	}
	if !MeetsThreshold(withCriticalAndHigh, "critical") {
		t.Error("critical threshold should trigger when Critical package present")
	}

	highOnly := &types.UpdateReport{Packages: []types.PackageUpdate{highPkg}}
	if MeetsThreshold(highOnly, "critical") {
		t.Error("critical threshold should not trigger when only High present")
	}
	if !MeetsThreshold(highOnly, "high") {
		t.Error("high threshold should trigger when High present")
	}
	if !MeetsThreshold(highOnly, "medium") {
		t.Error("medium threshold should trigger when High present (High >= Medium)")
	}

	medOnly := &types.UpdateReport{Packages: []types.PackageUpdate{medPkg}}
	if MeetsThreshold(medOnly, "high") {
		t.Error("high threshold should not trigger when only Medium present")
	}
	if !MeetsThreshold(medOnly, "medium") {
		t.Error("medium threshold should trigger when Medium present")
	}

	empty := &types.UpdateReport{}
	if MeetsThreshold(empty, "low") {
		t.Error("empty report should not meet any threshold")
	}
	if MeetsThreshold(empty, "none") {
		t.Error("'none' threshold should never trigger")
	}
}
