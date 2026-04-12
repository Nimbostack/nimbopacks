// Package types defines the shared data structures for nimbopacks.
//
// These types are the contract between packs, templates, the builder, and the updater.
// They should change rarely.
package types

import "time"

// ============================================================================
// nimpack.yaml — the single source of truth for builds
// ============================================================================

// NimpackConfig is the root of nimpack.yaml.
// This is REQUIRED for builds. Detection generates it; templates scaffold it.
type NimpackConfig struct {
	// Version of the nimpack.yaml schema.
	SchemaVersion string `yaml:"schema_version" json:"schema_version"`

	Project ProjectMeta `yaml:"project" json:"project"`
	Build   BuildConfig `yaml:"build" json:"build"`
	Image   ImageConfig `yaml:"image" json:"image"`

	// Artifacts supports monorepos — multiple build outputs from one source tree.
	// If empty, a single artifact is inferred from Build settings.
	Artifacts []Artifact `yaml:"artifacts,omitempty" json:"artifacts,omitempty"`

	// Update configures the `nimbopacks update` CVE patching workflow.
	Update UpdateConfig `yaml:"update,omitempty" json:"update,omitempty"`

	// TLS configures custom CA certificates for environments behind
	// corporate proxies or private registries with self-signed certs.
	TLS TLSConfig `yaml:"tls,omitempty" json:"tls,omitempty"`
}

// TLSConfig handles custom CA certificates.
// These propagate to every layer: melange builds, apko image assembly,
// the final image's trust store, registry pushes, and toolchain downloads.
type TLSConfig struct {
	// CACertPaths are paths to PEM-encoded CA certificate files.
	// Can be absolute or relative to the project root.
	// Example: ["certs/corporate-ca.pem", "/etc/ssl/certs/internal-ca.pem"]
	CACertPaths []string `yaml:"ca_cert_paths,omitempty" json:"ca_cert_paths,omitempty"`

	// CADirPath is a directory containing PEM CA cert files.
	// All .pem and .crt files in the directory are loaded.
	// Example: "/usr/local/share/ca-certificates"
	CADirPath string `yaml:"ca_dir_path,omitempty" json:"ca_dir_path,omitempty"`

	// InjectIntoImage controls whether custom CAs are added to the
	// final image's trust store so the running application trusts
	// the same CAs. Default: true.
	InjectIntoImage *bool `yaml:"inject_into_image,omitempty" json:"inject_into_image,omitempty"`

	// Insecure disables TLS verification entirely.
	// USE ONLY FOR DEBUGGING. Never in production.
	Insecure bool `yaml:"insecure,omitempty" json:"insecure,omitempty"`
}

// ShouldInjectIntoImage returns whether CAs should be added to the final image.
// Defaults to true.
func (t TLSConfig) ShouldInjectIntoImage() bool {
	if t.InjectIntoImage == nil {
		return true
	}
	return *t.InjectIntoImage
}

// HasCustomCAs returns true if any custom CA config is set.
func (t TLSConfig) HasCustomCAs() bool {
	return len(t.CACertPaths) > 0 || t.CADirPath != ""
}

type ProjectMeta struct {
	Name        string `yaml:"name" json:"name"`
	Version     string `yaml:"version" json:"version"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type BuildConfig struct {
	// Pack is the pack that handles this build (e.g., "go", "python", "dotnet").
	Pack string `yaml:"pack" json:"pack"`

	// Template is the template this config was generated from (e.g., "go", "dotnet-solution").
	// Informational — used by `nimbopacks update` to pull template updates.
	Template string `yaml:"template,omitempty" json:"template,omitempty"`

	// Command overrides the pack's default build command.
	Command string `yaml:"command,omitempty" json:"command,omitempty"`

	// Dependencies are extra APK packages needed at build time.
	Dependencies []string `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`

	// Env sets environment variables during the build.
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`

	// Context is the build context path relative to the project root.
	// Defaults to "." — useful for monorepos where the app is in a subdirectory.
	Context string `yaml:"context,omitempty" json:"context,omitempty"`
}

type ImageConfig struct {
	// Packages are APK packages to include in the final image.
	Packages []string `yaml:"packages" json:"packages"`

	// Entrypoint is the image ENTRYPOINT.
	Entrypoint string `yaml:"entrypoint" json:"entrypoint"`

	// Cmd is the image CMD (optional).
	Cmd []string `yaml:"cmd,omitempty" json:"cmd,omitempty"`

	// Env are environment variables baked into the image.
	Env map[string]string `yaml:"env,omitempty" json:"env,omitempty"`

	// RunAs is the UID for the non-root user (default: 65532).
	RunAs uint32 `yaml:"run_as,omitempty" json:"run_as,omitempty"`

	// Labels are OCI annotations.
	Labels map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`

	// Ports to expose.
	Ports []string `yaml:"ports,omitempty" json:"ports,omitempty"`

	// Layering controls apko's native multi-layer image assembly.
	//
	// Since apko v0.27.0, images can be split into intelligent layers
	// grouped by package origin. Packages from the same upstream build
	// share a layer, so when one package updates only its layer changes.
	//
	// Without this section, apko produces a single layer (legacy behavior).
	// Add it to opt into multi-layer images.
	//
	// Reference: https://github.com/chainguard-dev/apko/blob/main/docs/layering.md
	Layering *LayeringConfig `yaml:"layering,omitempty" json:"layering,omitempty"`
}

// LayeringConfig matches apko's native layering configuration exactly.
//
// apko groups packages by "origin" — packages built from the same upstream
// source land in the same layer. The top N groups (by installed size) each
// get their own layer, remaining packages overflow into a shared layer,
// and a final "top" layer captures OS metadata (installed db, apk world, etc.).
//
// Only two fields: strategy and budget. That's it.
// apko deliberately keeps this simple — no per-package pinning, no manual
// layer assignment. The heuristics handle it.
//
// Example:
//
//	layering:
//	  strategy: origin
//	  budget: 10
//
// Reference: https://github.com/chainguard-dev/apko/blob/main/docs/layering.md
type LayeringConfig struct {
	// Strategy is the layering algorithm. Currently only "origin" is supported.
	// "origin" groups packages by their build origin — packages from the same
	// upstream source (e.g., gcc, openssl) land in the same layer because
	// they change together.
	Strategy string `yaml:"strategy" json:"strategy"`

	// Budget is the number of additional layers apko will create.
	// The total layer count will be budget + 1 (the +1 is the "top" layer
	// containing OS metadata like the installed db).
	//
	// apko takes the top N package groups by installed size and gives each
	// its own layer. Remaining groups overflow into a single shared layer.
	//
	// Chainguard found budget=10 to be a good default — it eliminated ~70%
	// of unique layer data across their catalog. Increase for application
	// images that won't be used as bases. Decrease for base images where
	// consumers will add their own layers on top.
	//
	// Container runtimes cap at 127 layers total, so leave headroom.
	Budget int `yaml:"budget" json:"budget"`
}

// Artifact defines one build output in a monorepo.
// Each artifact can target a different subdirectory and produce a different binary.
type Artifact struct {
	// Name of the artifact (used as the APK package name).
	Name string `yaml:"name" json:"name"`

	// Source path relative to the project root (e.g., "src/MyApi" for .NET, "./cmd/worker" for Go).
	Source string `yaml:"source" json:"source"`

	// Dest is where the binary is installed in the image (e.g., "/usr/bin/worker").
	Dest string `yaml:"dest" json:"dest"`

	// Command overrides the build command for this specific artifact.
	Command string `yaml:"command,omitempty" json:"command,omitempty"`
}

// UpdateConfig controls the CVE patching workflow.
type UpdateConfig struct {
	// AutoCheck runs `nimbopacks update --check` in CI (default: true).
	AutoCheck bool `yaml:"auto_check" json:"auto_check"`

	// Repositories are additional Wolfi repos to check for updates.
	Repositories []string `yaml:"repositories,omitempty" json:"repositories,omitempty"`

	// Pinned packages won't be updated automatically.
	Pinned []string `yaml:"pinned,omitempty" json:"pinned,omitempty"`

	// GrypeConfig is the path to a grype policy file (.grype.yaml).
	// Relative to the project root. If empty, grype looks for .grype.yaml
	// in the working directory (project root) automatically.
	GrypeConfig string `yaml:"grype_config,omitempty" json:"grype_config,omitempty"`
}

// ============================================================================
// Detection types — used by packs to report what they found
// ============================================================================

// DetectResult is returned by a pack's Detect() method.
type DetectResult struct {
	// PackName identifies which pack produced this result.
	PackName string `yaml:"pack" json:"pack"`

	// Confidence from 0.0 to 1.0.
	Confidence float64 `yaml:"confidence" json:"confidence"`

	// Summary is a human-readable description (e.g., "Go/gin API with cmd/ layout").
	Summary string `yaml:"summary,omitempty" json:"summary,omitempty"`

	// SuggestedTemplate is the template this detection recommends.
	// The user can accept it or pick a different one.
	SuggestedTemplate string `yaml:"suggested_template,omitempty" json:"suggested_template,omitempty"`

	// Metadata is pack-specific data passed to GenerateConfig().
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// ============================================================================
// Build plan — intermediate representation between config and melange/apko
// ============================================================================

type BuildPlan struct {
	Melange MelangeConfig `yaml:"melange" json:"melange"`
	Apko    ApkoConfig    `yaml:"apko" json:"apko"`
}

// --- Melange ---

type MelangeConfig struct {
	Package     MelangePackage        `yaml:"package"`
	Environment MelangeEnvironment    `yaml:"environment"`
	Pipeline    []MelangePipelineStep `yaml:"pipeline"`
}

type MelangePackage struct {
	Name         string              `yaml:"name"`
	Version      string              `yaml:"version"`
	Epoch        int                 `yaml:"epoch"`
	Description  string              `yaml:"description,omitempty"`
	Copyright    []MelangeCopyright  `yaml:"copyright,omitempty"`
	Dependencies MelangeDependencies `yaml:"dependencies,omitempty"`
}

type MelangeCopyright struct {
	License string `yaml:"license"`
}

type MelangeDependencies struct {
	Runtime []string `yaml:"runtime,omitempty"`
}

type MelangeEnvironment struct {
	Contents MelangeContents `yaml:"contents"`
}

type MelangeContents struct {
	Keyring      []string `yaml:"keyring,omitempty"`
	Repositories []string `yaml:"repositories"`
	Packages     []string `yaml:"packages"`
}

type MelangePipelineStep struct {
	Uses string            `yaml:"uses,omitempty"`
	With map[string]string `yaml:"with,omitempty"`
	Runs string            `yaml:"runs,omitempty"`
}

// --- Apko ---

type ApkoConfig struct {
	Contents   ApkoContents   `yaml:"contents"`
	Entrypoint ApkoEntrypoint `yaml:"entrypoint"`
	// Cmd is a space-joined string because apko's YAML format takes cmd as a
	// string, not a sequence. Packs convert []string from ImageConfig.Cmd with
	// strings.Join before assigning here.
	Cmd         string            `yaml:"cmd,omitempty"`
	Accounts    ApkoAccounts      `yaml:"accounts"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Layering    *LayeringConfig   `yaml:"layering,omitempty"`
}

type ApkoContents struct {
	Keyring      []string `yaml:"keyring,omitempty"`
	Repositories []string `yaml:"repositories"`
	Packages     []string `yaml:"packages"`
}

type ApkoEntrypoint struct {
	Command string `yaml:"command"`
}

type ApkoAccounts struct {
	RunAs  string      `yaml:"run-as"`
	Users  []ApkoUser  `yaml:"users"`
	Groups []ApkoGroup `yaml:"groups"`
}

type ApkoUser struct {
	Username string `yaml:"username"`
	UID      uint32 `yaml:"uid"`
	GID      uint32 `yaml:"gid"`
}

type ApkoGroup struct {
	Groupname string `yaml:"groupname"`
	GID       uint32 `yaml:"gid"`
}

// ============================================================================
// Update types — CVE patching
// ============================================================================

type PackageUpdate struct {
	Package        string    `json:"package"`
	CurrentVersion string    `json:"current_version"`
	LatestVersion  string    `json:"latest_version"`
	CVEs           []string  `json:"cves,omitempty"`
	Severity       string    `json:"severity,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
	// Pinned is true when this package version appears in update.pinned.
	// A rebuild will NOT fix this CVE unless the pin is removed or bumped.
	Pinned bool `json:"pinned,omitempty"`
}

type UpdateReport struct {
	CheckedAt   time.Time       `json:"checked_at"`
	Packages    []PackageUpdate `json:"packages"`
	HasCritical bool            `json:"has_critical"`
	HasHigh     bool            `json:"has_high"`
	Summary     string          `json:"summary"`
}
