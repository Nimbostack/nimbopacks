package update

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Nimbostack/nimbopacks/internal/config"
	"github.com/Nimbostack/nimbopacks/internal/pipeline"
	"github.com/Nimbostack/nimbopacks/internal/types"
	"github.com/Nimbostack/nimbopacks/pkg/backend"
)

// CheckOptions configures a CVE scan run.
type CheckOptions struct {
	// SourceDir is the project root (where nimpack.yaml lives).
	SourceDir string

	// ConfigPath overrides the default nimpack.yaml path.
	ConfigPath string

	// SBOMPath, if set, skips the build step and scans this SBOM directly.
	SBOMPath string

	// GrypeConfig overrides the grype policy file path.
	// Priority: this field > update.grype_config in nimpack.yaml > grype's default discovery (.grype.yaml in project root).
	// The NIMBOPACKS_GRYPE_CONFIG env var is resolved by the caller before populating this field.
	GrypeConfig string

	// DBCachePath overrides GRYPE_DB_CACHE_DIR.
	DBCachePath string

	// BackendName selects the build backend ("wolfi", "ocidirect", or "" for auto).
	BackendName string
}

// Check runs a CVE scan and returns a structured report.
// If opts.SBOMPath is set, the build step is skipped.
// Otherwise, the image is built first and the resulting SBOM or OCI tarball is scanned.
func Check(ctx context.Context, opts CheckOptions) (*types.UpdateReport, error) {
	// Load nimpack.yaml to get pinned packages and grype config.
	cfg, err := config.Load(opts.SourceDir, opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	if cfg == nil {
		return nil, fmt.Errorf("nimpack.yaml not found in %s", opts.SourceDir)
	}

	// Resolve grype config path (CLI flag > yaml field > default discovery by grype).
	grypeConfig := opts.GrypeConfig
	if grypeConfig == "" {
		grypeConfig = cfg.Update.GrypeConfig
	}

	// Resolve scan input.
	var input ScanInput
	if opts.SBOMPath != "" {
		input = ScanInput{Type: ScanSBOM, Path: opts.SBOMPath}
	} else {
		imgResult, err := buildForScan(ctx, opts)
		if err != nil {
			return nil, err
		}
		input, err = resolveScanInput(imgResult)
		if err != nil {
			return nil, err
		}
	}

	// Run grype.
	matches, err := RunGrype(ctx, input, grypeConfig, opts.DBCachePath)
	if err != nil {
		return nil, fmt.Errorf("grype scan: %w", err)
	}

	// Build report.
	report := buildReport(matches, cfg.Update.Pinned)
	return report, nil
}

// buildForScan calls pipeline.Build and returns the image result.
func buildForScan(ctx context.Context, opts CheckOptions) (*backend.ImageResult, error) {
	buildOpts := pipeline.BuildOptions{
		SourceDir:   opts.SourceDir,
		ConfigPath:  opts.ConfigPath,
		BackendName: opts.BackendName,
	}
	imgResult, err := pipeline.Build(ctx, buildOpts)
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}
	return imgResult, nil
}

// resolveScanInput picks the best grype input from the build result.
// Prefers SBOM (wolfi) over OCI archive (ocidirect).
func resolveScanInput(img *backend.ImageResult) (ScanInput, error) {
	if img.SBOMPath != "" {
		return ScanInput{Type: ScanSBOM, Path: img.SBOMPath}, nil
	}
	if img.TarballPath != "" {
		return ScanInput{Type: ScanOCIArchive, Path: img.TarballPath}, nil
	}
	return ScanInput{}, errors.New("no scannable artifact from build: neither SBOM nor OCI tarball found in build result")
}

// buildReport groups grype matches by package and produces an UpdateReport.
// Matches for the same package+version are merged; the worst severity wins.
func buildReport(matches []GrypeMatch, pinned []string) *types.UpdateReport {
	type key struct{ name, version string }
	grouped := map[key]*types.PackageUpdate{}
	var order []key

	for _, m := range matches {
		k := key{m.Artifact.Name, m.Artifact.Version}
		p, exists := grouped[k]
		if !exists {
			fix := ""
			if len(m.Vulnerability.Fix.Versions) > 0 {
				fix = m.Vulnerability.Fix.Versions[0]
			}
			p = &types.PackageUpdate{
				Package:        m.Artifact.Name,
				CurrentVersion: m.Artifact.Version,
				LatestVersion:  fix,
				UpdatedAt:      time.Now(),
				Pinned:         isPinned(m.Artifact.Name, m.Artifact.Version, pinned),
			}
			grouped[k] = p
			order = append(order, k)
		}
		p.CVEs = append(p.CVEs, m.Vulnerability.ID)
		if severityRank(m.Vulnerability.Severity) > severityRank(p.Severity) {
			p.Severity = m.Vulnerability.Severity
		}
	}

	report := &types.UpdateReport{
		CheckedAt: time.Now(),
	}
	for _, k := range order {
		pkg := *grouped[k]
		report.Packages = append(report.Packages, pkg)
		switch strings.ToLower(pkg.Severity) {
		case "critical":
			report.HasCritical = true
			report.HasHigh = true
		case "high":
			report.HasHigh = true
		}
	}

	if len(report.Packages) == 0 {
		report.Summary = "No CVEs found"
	} else {
		report.Summary = fmt.Sprintf("%d CVE(s) found across %d package(s)", countCVEs(report.Packages), len(report.Packages))
	}
	return report
}

func countCVEs(pkgs []types.PackageUpdate) int {
	n := 0
	for _, p := range pkgs {
		n += len(p.CVEs)
	}
	return n
}

// severityRank returns a numeric rank for severity comparison. Higher = more severe.
func severityRank(s string) int {
	switch strings.ToLower(s) {
	case "critical":
		return 4
	case "high":
		return 3
	case "medium":
		return 2
	case "low":
		return 1
	case "negligible":
		return 0
	default:
		return -1
	}
}

// isPinned reports whether pkg@version appears in the pinned list.
// "openssl=3.1.4-r0" pins a specific version.
// "openssl" (no version) pins all versions.
func isPinned(pkg, version string, pinned []string) bool {
	for _, p := range pinned {
		parts := strings.SplitN(p, "=", 2)
		if parts[0] != pkg {
			continue
		}
		if len(parts) == 1 {
			return true
		}
		if parts[1] == version {
			return true
		}
	}
	return false
}

// MeetsThreshold reports whether the report has CVEs at or above the failOn severity.
// failOn values: "critical", "high", "medium", "low", "none".
func MeetsThreshold(report *types.UpdateReport, failOn string) bool {
	if failOn == "none" {
		return false
	}
	threshold := severityRank(failOn)
	for _, p := range report.Packages {
		if severityRank(p.Severity) >= threshold {
			return true
		}
	}
	return false
}
