package update

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/Nimbostack/nimbopacks/internal/types"
)

// Format writes report to w in the requested format.
// Valid formats: "text", "table", "json", "sarif".
func Format(report *types.UpdateReport, format string, w io.Writer) error {
	switch format {
	case "text":
		return formatText(report, w)
	case "table":
		return formatTable(report, w)
	case "json":
		return formatJSON(report, w)
	case "sarif":
		return formatSARIF(report, w)
	default:
		return fmt.Errorf("unknown format %q: must be text, table, json, or sarif", format)
	}
}

func formatText(r *types.UpdateReport, w io.Writer) error {
	fmt.Fprintf(w, "nimbopacks CVE scan — %s\n\n", r.CheckedAt.Format(time.RFC3339))

	if len(r.Packages) == 0 {
		fmt.Fprintln(w, "✓ No CVEs found")
		return nil
	}

	fmt.Fprintf(w, "Found vulnerabilities in %d package(s):\n\n", len(r.Packages))
	for _, p := range r.Packages {
		fmt.Fprintf(w, "  %s@%s  [%s]\n", p.Package, p.CurrentVersion, strings.ToUpper(p.Severity))
		for _, cve := range p.CVEs {
			fmt.Fprintf(w, "    → %s\n", cve)
		}
		if p.LatestVersion != "" && p.LatestVersion != p.CurrentVersion {
			fmt.Fprintf(w, "    Fix available: %s\n", p.LatestVersion)
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run `nimbopacks build` to rebuild with the latest Wolfi packages.")
	fmt.Fprintln(w, "Wolfi packages are updated continuously — a rebuild applies all patches.")
	return nil
}

func formatTable(r *types.UpdateReport, w io.Writer) error {
	fmt.Fprintf(w, "nimbopacks CVE scan — %s\n\n", r.CheckedAt.Format(time.RFC3339))

	if len(r.Packages) == 0 {
		fmt.Fprintln(w, "No CVEs found.")
		return nil
	}

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PACKAGE\tVERSION\tSEVERITY\tCVE\tFIX AVAILABLE")
	for _, p := range r.Packages {
		fix := p.LatestVersion
		if fix == "" || fix == p.CurrentVersion {
			fix = "-"
		}
		cves := strings.Join(p.CVEs, ", ")
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			p.Package, p.CurrentVersion, strings.ToUpper(p.Severity), cves, fix)
	}
	return tw.Flush()
}

func formatJSON(r *types.UpdateReport, w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// sarifLevel maps CVE severity to SARIF result level.
func sarifLevel(severity string) string {
	switch strings.ToLower(severity) {
	case "critical", "high":
		return "error"
	case "medium":
		return "warning"
	default:
		return "note"
	}
}

func formatSARIF(r *types.UpdateReport, w io.Writer) error {
	type sarifMessage struct {
		Text string `json:"text"`
	}
	type sarifShortDesc struct {
		Text string `json:"text"`
	}
	type sarifRuleProperties struct {
		Severity string `json:"severity"`
	}
	type sarifRule struct {
		ID               string              `json:"id"`
		ShortDescription sarifShortDesc      `json:"shortDescription"`
		HelpURI          string              `json:"helpUri"`
		Properties       sarifRuleProperties `json:"properties"`
	}
	type sarifDriver struct {
		Name    string      `json:"name"`
		Version string      `json:"version"`
		Rules   []sarifRule `json:"rules"`
	}
	type sarifTool struct {
		Driver sarifDriver `json:"driver"`
	}
	type sarifRegion struct {
		StartLine int `json:"startLine"`
	}
	type sarifArtifactLocation struct {
		URI string `json:"uri"`
	}
	type sarifPhysicalLocation struct {
		ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
		Region           sarifRegion           `json:"region"`
	}
	type sarifLocation struct {
		PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
	}
	type sarifResult struct {
		RuleID    string          `json:"ruleId"`
		Level     string          `json:"level"`
		Message   sarifMessage    `json:"message"`
		Locations []sarifLocation `json:"locations"`
	}
	type sarifRun struct {
		Tool    sarifTool     `json:"tool"`
		Results []sarifResult `json:"results"`
	}
	type sarifDoc struct {
		Schema  string     `json:"$schema"`
		Version string     `json:"version"`
		Runs    []sarifRun `json:"runs"`
	}

	rules := make([]sarifRule, 0)
	results := make([]sarifResult, 0)

	// One rule per unique CVE ID.
	seen := map[string]bool{}
	for _, p := range r.Packages {
		for _, cve := range p.CVEs {
			if !seen[cve] {
				seen[cve] = true
				rules = append(rules, sarifRule{
					ID:               cve,
					ShortDescription: sarifShortDesc{Text: fmt.Sprintf("%s in %s", cve, p.Package)},
					HelpURI:          "https://nvd.nist.gov/vuln/detail/" + cve,
					Properties:       sarifRuleProperties{Severity: p.Severity},
				})
			}
			results = append(results, sarifResult{
				RuleID:  cve,
				Level:   sarifLevel(p.Severity),
				Message: sarifMessage{Text: fmt.Sprintf("%s@%s: %s (%s)", p.Package, p.CurrentVersion, cve, p.Severity)},
				Locations: []sarifLocation{{
					PhysicalLocation: sarifPhysicalLocation{
						ArtifactLocation: sarifArtifactLocation{URI: "nimpack.yaml"},
						Region:           sarifRegion{StartLine: 1},
					},
				}},
			})
		}
	}

	doc := sarifDoc{
		Schema:  "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name:    "nimbopacks/grype",
				Version: "dev",
				Rules:   rules,
			}},
			Results: results,
		}},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(doc)
}
