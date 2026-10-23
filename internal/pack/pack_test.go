package pack_test

import (
	"reflect"
	"testing"

	"github.com/Nimbostack/nimbopacks/internal/pack"
	"github.com/Nimbostack/nimbopacks/internal/types"
)

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
