package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Nimbostack/nimbopacks/internal/pack/registry"
	"github.com/Nimbostack/nimbopacks/pkg/templates"

	// The packs register themselves, and every shipped sample names one of
	// them. Blank imports are how a binary opts into a pack set.
	_ "github.com/Nimbostack/nimbopacks/pkg/packs/dotnet"
	_ "github.com/Nimbostack/nimbopacks/pkg/packs/golang"
	_ "github.com/Nimbostack/nimbopacks/pkg/packs/java"
	_ "github.com/Nimbostack/nimbopacks/pkg/packs/node"
	_ "github.com/Nimbostack/nimbopacks/pkg/packs/python"
	_ "github.com/Nimbostack/nimbopacks/pkg/packs/webserver"
)

// goldenPath holds one line per shipped sample: the sample's path, then the
// SHA-256 of its melange.yaml and of its apko.yaml.
const goldenPath = "testdata/shipped-plans.sha256"

const samplesDir = "../../samples"

// shippedPlanDigests walks every nimpack.yaml under samples/, loads it, runs
// the pack named by the config, and hashes the two documents the wolfi backend
// writes. Canonical serialisation is yaml.Marshal, which sorts map keys.
func shippedPlanDigests(t *testing.T) map[string]string {
	t.Helper()

	digests := make(map[string]string)
	err := filepath.WalkDir(samplesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != DefaultFilename {
			return nil
		}
		dir := filepath.Dir(path)
		rel, err := filepath.Rel(samplesDir, dir)
		if err != nil {
			return err
		}

		cfg, err := Load(dir, "")
		if err != nil {
			return fmt.Errorf("%s: loading: %w", rel, err)
		}
		if cfg == nil {
			return fmt.Errorf("%s: no config loaded", rel)
		}
		if err := Validate(cfg); err != nil {
			return fmt.Errorf("%s: validating: %w", rel, err)
		}

		p := registry.Get(cfg.Build.Pack)
		if p == nil {
			return fmt.Errorf("%s: pack %q is not registered", rel, cfg.Build.Pack)
		}
		plan, err := p.Plan(t.Context(), dir, cfg)
		if err != nil {
			return fmt.Errorf("%s: planning: %w", rel, err)
		}

		melange, err := yaml.Marshal(plan.Melange)
		if err != nil {
			return fmt.Errorf("%s: marshalling melange: %w", rel, err)
		}
		apko, err := yaml.Marshal(plan.Apko)
		if err != nil {
			return fmt.Errorf("%s: marshalling apko: %w", rel, err)
		}
		digests[filepath.ToSlash(rel)] = sum(melange) + " " + sum(apko)
		return nil
	})
	if err != nil {
		t.Fatalf("walking samples: %v", err)
	}
	if len(digests) == 0 {
		t.Fatalf("no shipped configs found under %s", samplesDir)
	}
	return digests
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func formatDigests(digests map[string]string) string {
	keys := make([]string, 0, len(digests))
	for k := range digests {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s %s\n", k, digests[k])
	}
	return b.String()
}

// TestShippedConfigs_PlanEquivalence pins ADR-0004's central claim about A9,
// A10 and A11: "zero plan movement and zero digest movement for every config
// that ships today". The golden file was generated before the three changes
// landed, so a mismatch means a shipped sample's melange.yaml or apko.yaml
// moved — which is the breaking-change test from ADR-0004 D4 criterion 7,
// phrased on plan equivalence rather than on a digest the tool cannot hold
// stable.
//
// Regenerate deliberately, never reflexively: a diff here is either a bug or a
// breaking change that owes the reader a schema_version decision.
//
//	UPDATE_SHIPPED_PLANS=1 go test ./internal/config/
func TestShippedConfigs_PlanEquivalence(t *testing.T) {
	digests := shippedPlanDigests(t)
	got := formatDigests(digests)

	if os.Getenv("UPDATE_SHIPPED_PLANS") == "1" {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	if got == string(want) {
		return
	}

	// Report the samples that moved, not the whole file.
	wantLines := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(want)), "\n") {
		name, digest, _ := strings.Cut(line, " ")
		wantLines[name] = digest
	}
	for name, digest := range digests {
		switch w, ok := wantLines[name]; {
		case !ok:
			t.Errorf("sample %s is new; regenerate %s if that is intended", name, goldenPath)
		case w != digest:
			t.Errorf("sample %s: build plan changed\n  want: %s\n  got:  %s", name, w, digest)
		}
		delete(wantLines, name)
	}
	for name := range wantLines {
		t.Errorf("sample %s disappeared from %s", name, samplesDir)
	}
}

// TestShippedTemplates_LoadCleanly extends the same claim to the embedded
// templates, which are the other shipped configs: `nimbopacks init` writes one
// of these verbatim, so a template the loader would reject is a tool that
// scaffolds a project it then refuses to build.
func TestShippedTemplates_LoadCleanly(t *testing.T) {
	dir := t.TempDir()
	for _, tmpl := range templates.List() {
		content, err := templates.Render(tmpl.Name, templates.TemplateVars{
			ProjectName: "myapp",
			Version:     "1.0.0",
			Module:      "example.invalid/myapp",
			Entrypoint:  "./cmd/myapp",
		})
		if err != nil {
			t.Fatalf("rendering template %q: %v", tmpl.Name, err)
		}
		path := filepath.Join(dir, "nimpack.yaml")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := Load(dir, path)
		if err != nil {
			t.Errorf("template %q must load, got: %v", tmpl.Name, err)
			continue
		}
		if err := Validate(cfg); err != nil {
			t.Errorf("template %q must validate, got: %v", tmpl.Name, err)
			continue
		}
		if w := runAsWarnings(cfg); len(w) != 0 {
			t.Errorf("template %q must not scaffold a root container, got: %v", tmpl.Name, w)
		}
	}
}

// TestShippedConfigs_LoadCleanly is the other half of the claim: the three
// validations added for A9, A10 and A11 reject nothing that ships, and no
// shipped config asks for root.
func TestShippedConfigs_LoadCleanly(t *testing.T) {
	err := filepath.WalkDir(samplesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != DefaultFilename {
			return nil
		}
		dir := filepath.Dir(path)
		cfg, loadErr := Load(dir, "")
		if loadErr != nil {
			t.Errorf("%s: must load, got: %v", path, loadErr)
			return nil
		}
		if vErr := Validate(cfg); vErr != nil {
			t.Errorf("%s: must validate, got: %v", path, vErr)
			return nil
		}
		if w := runAsWarnings(cfg); len(w) != 0 {
			t.Errorf("%s: no shipped sample may request root, got: %v", path, w)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking samples: %v", err)
	}
}
