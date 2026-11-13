package backend

import (
	"testing"
)

func TestGetCapabilities(t *testing.T) {
	wolfi := GetCapabilities("wolfi")
	// Not reproducible: melange signs every build with a fresh RSA key, which changes the
	// .apk encoding, the installed-database size field and therefore the image digest. The
	// capability said true until a double-build measured otherwise.
	if wolfi.Reproducible {
		t.Error("wolfi must not claim reproducible builds; per-build signing keys move the digest")
	}
	if !wolfi.SBOM {
		t.Error("wolfi should have SBOM")
	}
	if !wolfi.AtomicPatching {
		t.Error("wolfi should have atomic patching")
	}
	if wolfi.ExposedPorts {
		t.Error("wolfi should not claim exposed-port support (apko has no input key for it)")
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
