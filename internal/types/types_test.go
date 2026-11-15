package types_test

import (
	"strings"
	"testing"

	"github.com/Nimbostack/nimbopacks/internal/types"
	"gopkg.in/yaml.v3"
)

func TestUpdateConfig_GrypeConfig(t *testing.T) {
	raw := `
update:
  grype_config: config/grype-policy.yaml
  pinned:
    - openssl=3.1.4-r0
`
	var cfg types.NimpackConfig
	if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Update.GrypeConfig != "config/grype-policy.yaml" {
		t.Errorf("GrypeConfig: got %q, want %q", cfg.Update.GrypeConfig, "config/grype-policy.yaml")
	}
	if len(cfg.Update.Pinned) != 1 || cfg.Update.Pinned[0] != "openssl=3.1.4-r0" {
		t.Errorf("Pinned: got %v", cfg.Update.Pinned)
	}
}

func TestUpdateConfig_Roundtrip(t *testing.T) {
	cfg := types.NimpackConfig{}
	cfg.Update.GrypeConfig = "security/grype.yaml"

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	var got types.NimpackConfig
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Update.GrypeConfig != "security/grype.yaml" {
		t.Errorf("roundtrip GrypeConfig: got %q", got.Update.GrypeConfig)
	}
}

// TestApkoConfig_LayeringBytes is ADR-0004 §Verification item 2: the layering
// block the wolfi backend writes into apko.yaml must be byte-identical before
// and after A10 and A11 changed the field types. A named string type and a
// *int marshal exactly as the string and int they replaced, so the one shipped
// config that sets layering (samples/showcase/layering, budget 8) produces the
// same apko input and therefore the same image.
func TestApkoConfig_LayeringBytes(t *testing.T) {
	budget := 8
	cfg := types.ApkoConfig{
		Layering: &types.LayeringConfig{
			Strategy: types.LayeringStrategyOrigin,
			Budget:   &budget,
		},
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	const want = "layering:\n    strategy: origin\n    budget: 8\n"
	if got := string(data); !strings.Contains(got, want) {
		t.Errorf("apko layering bytes changed:\nwant to contain:\n%s\ngot:\n%s", want, got)
	}
}

// TestImageConfig_RunAsThreeStates covers ADR-0004 A9 at the schema level:
// absent, zero and a UID must decode to three distinguishable values, and an
// absent field must still marshal away rather than re-appearing as run_as: 0.
func TestImageConfig_RunAsThreeStates(t *testing.T) {
	decode := func(t *testing.T, raw string) *uint32 {
		t.Helper()
		var cfg types.NimpackConfig
		if err := yaml.Unmarshal([]byte(raw), &cfg); err != nil {
			t.Fatal(err)
		}
		return cfg.Image.RunAs
	}

	if got := decode(t, "image:\n  packages: []\n"); got != nil {
		t.Errorf("an absent run_as must decode to nil, got %d", *got)
	}
	if got := decode(t, "image:\n  run_as: 0\n"); got == nil || *got != 0 {
		t.Errorf("run_as: 0 must decode to 0, got %v", got)
	}
	if got := decode(t, "image:\n  run_as: 1000\n"); got == nil || *got != 1000 {
		t.Errorf("run_as: 1000 must decode to 1000, got %v", got)
	}

	var cfg types.NimpackConfig
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "run_as") {
		t.Errorf("an absent run_as must not be emitted:\n%s", data)
	}
}
