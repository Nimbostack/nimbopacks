// Package update implements the CVE scanning workflow for nimbopacks.
// It orchestrates grype invocation, report building, and output formatting.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Nimbostack/nimbopacks/internal/toolchain"
)

// ScanType identifies the kind of input to pass to grype.
type ScanType string

const (
	ScanSBOM       ScanType = "sbom"
	ScanOCIArchive ScanType = "oci-archive"
)

// ScanInput describes what grype should scan.
type ScanInput struct {
	Type ScanType
	Path string
}

// String returns the grype-format target string (e.g. "sbom:/path/to/sbom.json").
func (s ScanInput) String() string {
	return string(s.Type) + ":" + s.Path
}

// GrypeOutput is the top-level structure of grype's JSON output.
type GrypeOutput struct {
	Matches []GrypeMatch `json:"matches"`
}

// GrypeMatch is one vulnerability-package pairing from grype.
type GrypeMatch struct {
	Vulnerability GrypeVulnerability `json:"vulnerability"`
	Artifact      GrypeArtifact      `json:"artifact"`
}

// GrypeVulnerability holds CVE metadata from grype.
type GrypeVulnerability struct {
	ID          string   `json:"id"`
	Severity    string   `json:"severity"`
	Description string   `json:"description"`
	Fix         GrypeFix `json:"fix"`
}

// GrypeFix describes the available fix for a vulnerability.
type GrypeFix struct {
	Versions []string `json:"versions"`
	State    string   `json:"state"` // "fixed", "not-fixed", "wont-fix", "unknown"
}

// GrypeArtifact is the package affected by the vulnerability.
type GrypeArtifact struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"` // "apk", "deb", "rpm", etc.
}

// execGrype is the grype command executor. Overridden in tests.
var execGrype = func(ctx context.Context, args []string, env []string) ([]byte, error) {
	grype := grypePath()
	if grype == "" {
		return nil, errors.New("grype not found; run `nimbopacks toolchain install`")
	}
	cmd := exec.CommandContext(ctx, grype, args...)
	cmd.Env = append(os.Environ(), env...)
	return cmd.Output()
}

// grypePath resolves the grype binary: managed toolchain first, then PATH.
func grypePath() string {
	managed := filepath.Join(toolchain.BinDir(), "grype")
	if _, err := os.Stat(managed); err == nil {
		return managed
	}
	if path, err := exec.LookPath("grype"); err == nil {
		return path
	}
	return ""
}

// RunGrype invokes grype against the given scan input and returns the parsed matches.
//
//   - grypeConfig: path to a .grype.yaml policy file. Empty = grype uses its default discovery.
//   - dbCachePath: overrides GRYPE_DB_CACHE_DIR. Empty = grype default (~/.cache/grype).
func RunGrype(ctx context.Context, input ScanInput, grypeConfig, dbCachePath string) ([]GrypeMatch, error) {
	args := []string{
		input.String(),
		"--output", "json",
		"--quiet",
	}
	if grypeConfig != "" {
		args = append(args, "--config", grypeConfig)
	}

	var env []string
	if dbCachePath != "" {
		env = append(env, "GRYPE_DB_CACHE_DIR="+dbCachePath)
	}

	out, err := execGrype(ctx, args, env)
	if err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return nil, fmt.Errorf("grype: %w", err)
		}
		// exit code 1 means vulnerabilities found — continue to parse output
	}

	var result GrypeOutput
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parsing grype output: %w", err)
	}

	return result.Matches, nil
}
