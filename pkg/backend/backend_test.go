package backend

import (
	"testing"
)

func TestGetCapabilities(t *testing.T) {
	wolfi := GetCapabilities("wolfi")
	if !wolfi.Reproducible {
		t.Error("wolfi should be reproducible")
	}
	if !wolfi.SBOM {
		t.Error("wolfi should have SBOM")
	}
	if !wolfi.AtomicPatching {
		t.Error("wolfi should have atomic patching")
	}

	// Unknown backends report zero-value capabilities.
	if (GetCapabilities("nonexistent") != Capabilities{}) {
		t.Error("unknown backend should report zero-value capabilities")
	}
}

func TestNames(_ *testing.T) {
	names := Names()
	// At least the registered backends should be present when
	// init() runs from their packages. In a standalone test,
	// the map may be empty if the packages aren't imported.
	_ = names // Just verify it doesn't panic.
}
