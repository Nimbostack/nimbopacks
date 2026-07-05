// Package backend defines the interface between nimbopacks and the underlying
// build tools. This abstraction allows nimbopacks to support multiple build
// strategies while keeping the user-facing config (nimpack.yaml) identical —
// additional backends can be added by implementing Backend and registering it.
//
// # Available backends
//
//   - wolfi: melange + apko (full reproducibility, SBOM, atomic CVE patching)
package backend

import (
	"context"
	"errors"
	"fmt"

	"github.com/Nimbostack/nimbopacks/internal/types"
)

// Backend is the build strategy interface.
type Backend interface {
	// Name returns the backend identifier (e.g., "wolfi").
	Name() string

	// Available reports whether this backend can run on the current system.
	// For wolfi: checks if melange + apko are installed (or can be auto-installed).
	Available(ctx context.Context) (bool, string)

	// BuildPackage compiles the source code into a package artifact.
	// For wolfi: runs melange to produce an APK.
	BuildPackage(ctx context.Context, plan *types.BuildPlan, opts BuildOpts) (*BuildResult, error)

	// AssembleImage creates an OCI image from the build output.
	// For wolfi: runs apko to assemble a minimal image.
	AssembleImage(ctx context.Context, plan *types.BuildPlan, buildResult *BuildResult, opts ImageOpts) (*ImageResult, error)

	// CheckUpdates queries the package repository for available updates.
	// Returns packages that have newer versions or known CVEs.
	CheckUpdates(ctx context.Context, packages []string, cfg *types.UpdateConfig) ([]types.PackageUpdate, error)
}

// BuildOpts configures the package build phase.
type BuildOpts struct {
	SourceDir string
	WorkDir   string
	Platform  string // e.g., "x86_64"
	TLS       types.TLSConfig
}

// BuildResult is the output of BuildPackage.
type BuildResult struct {
	// PackagesDir is where built packages are stored (APK directory for wolfi).
	PackagesDir string

	// Artifacts are the individual build outputs.
	Artifacts []string

	// SigningKeyPath is the path to the signing key (wolfi only).
	SigningKeyPath string
}

// ImageOpts configures the image assembly phase.
type ImageOpts struct {
	Tag       string
	OutputDir string
	Push      bool
	TLS       types.TLSConfig
	Layering  *types.LayeringConfig // nil = apko default (auto multi-layer)
}

// ImageResult is the output of AssembleImage.
type ImageResult struct {
	// TarballPath is the path to the OCI image tarball.
	TarballPath string

	// Digest is the image digest (sha256:...).
	Digest string

	// Tag is the final image tag.
	Tag string

	// SBOMPath is the path to the generated SBOM (wolfi only).
	SBOMPath string
}

// --- Backend registry ---

var backends = make(map[string]Backend)

// Register adds a backend.
func Register(b Backend) {
	backends[b.Name()] = b
}

// Get returns a backend by name.
func Get(name string) (Backend, error) {
	b, ok := backends[name]
	if !ok {
		return nil, fmt.Errorf("backend %q not found; available: %v", name, Names())
	}
	return b, nil
}

// Names returns all registered backend names.
func Names() []string {
	var out []string
	for name := range backends {
		out = append(out, name)
	}
	return out
}

// AutoSelect picks the best available backend.
// Prefers wolfi if available, then any other registered backend that reports
// itself available.
func AutoSelect(ctx context.Context) (Backend, error) {
	// Prefer wolfi — it's the full experience.
	if b, ok := backends["wolfi"]; ok {
		if avail, _ := b.Available(ctx); avail {
			return b, nil
		}
	}

	// Fall back to any other available backend.
	for name, b := range backends {
		if name == "wolfi" {
			continue
		}
		if avail, _ := b.Available(ctx); avail {
			return b, nil
		}
	}

	return nil, errors.New("no backends available; install melange + apko, or run `nimbopacks toolchain install`")
}

// Capabilities describes what a backend supports.
type Capabilities struct {
	Reproducible   bool // Byte-for-byte identical rebuilds
	SBOM           bool // Automatic SBOM generation
	AtomicPatching bool // Can update single packages without full rebuild
	Sandboxed      bool // Build runs in a sandbox (bubblewrap/container)
	NoExternalDeps bool // Runs with zero external tool installs
}

// GetCapabilities returns what each backend supports.
// This is used by `nimbopacks status` and docs.
func GetCapabilities(name string) Capabilities {
	switch name {
	case "wolfi":
		return Capabilities{
			Reproducible:   true,
			SBOM:           true,
			AtomicPatching: true,
			Sandboxed:      true,
			NoExternalDeps: false, // Needs melange + apko (auto-installable)
		}
	default:
		return Capabilities{}
	}
}
