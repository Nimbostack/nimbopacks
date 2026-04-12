package types_test

import (
	"testing"

	"github.com/Nimbostack/nimbopacks/internal/types"
	"gopkg.in/yaml.v3"
)

func TestUpdateConfig_GrypeConfig(t *testing.T) {
	raw := `
update:
  auto_check: true
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
	if !cfg.Update.AutoCheck {
		t.Error("AutoCheck: want true")
	}
}

func TestUpdateConfig_Roundtrip(t *testing.T) {
	cfg := types.NimpackConfig{}
	cfg.Update.GrypeConfig = "security/grype.yaml"
	cfg.Update.AutoCheck = true

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
	if !got.Update.AutoCheck {
		t.Error("roundtrip AutoCheck: want true")
	}
}
