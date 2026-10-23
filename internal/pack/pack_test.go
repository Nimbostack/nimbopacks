package pack_test

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Nimbostack/nimbopacks/internal/pack"
	"github.com/Nimbostack/nimbopacks/internal/types"
)

// TestApplyConfig_LabelsBecomeAnnotations covers ADR-0003 A3: image.labels
// maps 1:1 onto apko's annotations: key.
func TestApplyConfig_LabelsBecomeAnnotations(t *testing.T) {
	cfg := &types.NimpackConfig{
		Project: types.ProjectMeta{Name: "app", Version: "1.0.0"},
		Image: types.ImageConfig{
			Labels: map[string]string{
				"org.opencontainers.image.source":      "https://example.invalid/app",
				"org.opencontainers.image.description": "demo",
			},
		},
	}
	plan := &types.BuildPlan{Apko: pack.NewApkoConfig("app", "", nil)}
	pack.ApplyConfig(plan, cfg)

	if got, want := plan.Apko.Annotations["org.opencontainers.image.source"], "https://example.invalid/app"; got != want {
		t.Errorf("Annotations[source]: got %q, want %q", got, want)
	}
	if got, want := plan.Apko.Annotations["org.opencontainers.image.description"], "demo"; got != want {
		t.Errorf("Annotations[description]: got %q, want %q", got, want)
	}
}

// TestApplyConfig_LabelsDeterministic covers ADR-0003 §Verification item 3:
// the same map built twice produces byte-identical annotations: output. Go
// map iteration order is randomized per-run, so this only passes if the
// emitted YAML is sorted rather than relying on map order.
func TestApplyConfig_LabelsDeterministic(t *testing.T) {
	labels := map[string]string{
		"org.opencontainers.image.source":      "https://example.invalid/app",
		"org.opencontainers.image.description": "demo",
		"org.opencontainers.image.licenses":    "Apache-2.0",
		"org.opencontainers.image.revision":    "abc123",
	}

	render := func() []byte {
		cfg := &types.NimpackConfig{
			Project: types.ProjectMeta{Name: "app", Version: "1.0.0"},
			Image:   types.ImageConfig{Labels: labels},
		}
		plan := &types.BuildPlan{Apko: pack.NewApkoConfig("app", "", nil)}
		pack.ApplyConfig(plan, cfg)
		out, err := yaml.Marshal(plan.Apko.Annotations)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		return out
	}

	first := render()
	for range 5 {
		if got := render(); string(got) != string(first) {
			t.Fatalf("annotations output not deterministic:\nfirst: %s\ngot:   %s", first, got)
		}
	}
}
