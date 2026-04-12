package registry

import (
	"context"
	"testing"

	"github.com/Nimbostack/nimbopacks/internal/types"
)

type mockPack struct {
	name   string
	result *types.DetectResult
}

func (m *mockPack) Name() string { return m.name }
func (m *mockPack) Detect(_ context.Context, _ string) (*types.DetectResult, error) {
	return m.result, nil
}

func (m *mockPack) GenerateConfig(_ context.Context, _ string, _ *types.DetectResult, _ string) (*types.NimpackConfig, error) {
	return nil, nil
}

func (m *mockPack) Plan(_ context.Context, _ string, _ *types.NimpackConfig) (*types.BuildPlan, error) {
	return nil, nil
}

func TestDetectAll_SortsByConfidence(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	Register(&mockPack{name: "low", result: &types.DetectResult{PackName: "low", Confidence: 0.3}})
	Register(&mockPack{name: "high", result: &types.DetectResult{PackName: "high", Confidence: 0.9}})
	Register(&mockPack{name: "mid", result: &types.DetectResult{PackName: "mid", Confidence: 0.6}})

	results, err := DetectAll(t.Context(), "/tmp")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3, got %d", len(results))
	}
	if results[0].Pack.Name() != "high" {
		t.Errorf("expected 'high' first, got %s", results[0].Pack.Name())
	}
}

func TestDetectAll_SkipsNil(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	Register(&mockPack{name: "nope", result: nil})
	Register(&mockPack{name: "yes", result: &types.DetectResult{PackName: "yes", Confidence: 0.8}})

	results, _ := DetectAll(t.Context(), "/tmp")
	if len(results) != 1 {
		t.Fatalf("expected 1, got %d", len(results))
	}
}

func TestDetectBest(t *testing.T) {
	ResetForTesting()
	defer ResetForTesting()

	Register(&mockPack{name: "weak", result: &types.DetectResult{PackName: "weak", Confidence: 0.5}})
	Register(&mockPack{name: "strong", result: &types.DetectResult{PackName: "strong", Confidence: 0.95}})

	best, _ := DetectBest(t.Context(), "/tmp")
	if best == nil || best.Pack.Name() != "strong" {
		t.Errorf("expected 'strong'")
	}
}
