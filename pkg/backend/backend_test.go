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

	oci := GetCapabilities("ocidirect")
	if oci.Reproducible {
		t.Error("ocidirect should not be reproducible")
	}
	if !oci.NoExternalDeps {
		t.Error("ocidirect should have no external deps")
	}
}

func TestNames(_ *testing.T) {
	names := Names()
	// At least the registered backends should be present when
	// init() runs from their packages. In a standalone test,
	// the map may be empty if the packages aren't imported.
	_ = names // Just verify it doesn't panic.
}
