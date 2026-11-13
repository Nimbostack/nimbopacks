package pack_test

import (
	"reflect"
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

// TestApplyConfig_PortsIgnored covers ADR-0003 A4's verification
// requirement: two configs differing only in image.ports must produce an
// identical ApkoConfig, since no backend-agnostic plan field carries ports.
func TestApplyConfig_PortsIgnored(t *testing.T) {
	base := func() *types.NimpackConfig {
		return &types.NimpackConfig{
			Project: types.ProjectMeta{Name: "app", Version: "1.0.0"},
			Image:   types.ImageConfig{Entrypoint: "/usr/bin/app"},
		}
	}

	cfgWithoutPorts := base()
	cfgWithPorts := base()
	cfgWithPorts.Image.Ports = []string{"8080"}

	planWithout := &types.BuildPlan{Apko: pack.NewApkoConfig("app", "", nil)}
	planWith := &types.BuildPlan{Apko: pack.NewApkoConfig("app", "", nil)}

	pack.ApplyConfig(planWithout, cfgWithoutPorts)
	pack.ApplyConfig(planWith, cfgWithPorts)

	if !reflect.DeepEqual(planWithout.Apko, planWith.Apko) {
		t.Errorf("ApkoConfig differs based on image.ports:\nwithout: %+v\nwith:    %+v", planWithout.Apko, planWith.Apko)
	}
}

// TestApplyConfig_RunAs is ADR-0004 §Verification item 3: the three states of
// image.run_as reach the apko config as three distinct run-as values. The
// `0` row is the one that failed before A9 — it rendered 65532, the UID the
// user had explicitly declined.
func TestApplyConfig_RunAs(t *testing.T) {
	ptr := func(u uint32) *uint32 { return &u }

	cases := []struct {
		name  string
		runAs *uint32
		want  string
	}{
		{name: "absent uses the non-root default", runAs: nil, want: "65532"},
		{name: "zero means root", runAs: ptr(0), want: "0"},
		{name: "an explicit uid", runAs: ptr(1000), want: "1000"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &types.NimpackConfig{
				Project: types.ProjectMeta{Name: "app", Version: "1.0.0"},
				Image:   types.ImageConfig{RunAs: tc.runAs},
			}
			plan := &types.BuildPlan{Apko: pack.NewApkoConfig("app", "", nil)}
			pack.ApplyConfig(plan, cfg)

			if got := plan.Apko.Accounts.RunAs; got != tc.want {
				t.Errorf("apko accounts.run-as: got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestBaseConfig_ScaffoldsNonRoot guards the generator side of A9: a config
// nimbopacks writes must still ask for the non-root UID explicitly, not leave
// the field absent and rely on a default elsewhere.
func TestBaseConfig_ScaffoldsNonRoot(t *testing.T) {
	cfg := pack.BaseConfig("app", "1.0.0", "golang", "go")
	if cfg.Image.RunAs == nil {
		t.Fatal("generated configs must set run_as")
	}
	if got := *cfg.Image.RunAs; got != pack.DefaultRunAs {
		t.Errorf("run_as: got %d, want %d", got, pack.DefaultRunAs)
	}
}
