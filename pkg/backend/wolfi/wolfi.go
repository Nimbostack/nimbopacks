// Package wolfi implements the melange + apko build backend.
package wolfi

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Nimbostack/nimbopacks/internal/cache"
	"github.com/Nimbostack/nimbopacks/internal/types"
	tlsutil "github.com/Nimbostack/nimbopacks/internal/utils"
	"github.com/Nimbostack/nimbopacks/pkg/backend"
)

func init() {
	backend.Register(&Backend{})
}

// Backend implements the wolfi build strategy.
type Backend struct{}

func (b *Backend) Name() string { return "wolfi" }

func (b *Backend) Available(_ context.Context) (bool, string) {
	// Check for melange.
	if _, err := exec.LookPath("melange"); err != nil {
		// Check if toolchain manager has it.
		managed := toolchainPath("melange")
		if _, err := os.Stat(managed); err != nil {
			return false, "melange not found; run `nimbopacks toolchain install` or install manually"
		}
	}

	// Check for apko.
	if _, err := exec.LookPath("apko"); err != nil {
		managed := toolchainPath("apko")
		if _, err := os.Stat(managed); err != nil {
			return false, "apko not found; run `nimbopacks toolchain install` or install manually"
		}
	}

	return true, "melange + apko available"
}

func (b *Backend) BuildPackage(ctx context.Context, plan *types.BuildPlan, opts backend.BuildOpts) (*backend.BuildResult, error) {
	workDir := opts.WorkDir
	if workDir == "" {
		var err error
		workDir, err = cache.NewBuildWorkDir()
		if err != nil {
			return nil, err
		}
	}

	// Write melange.yaml.
	melangePath := filepath.Join(workDir, "melange.yaml")
	if err := writeYAML(melangePath, plan.Melange); err != nil {
		return nil, fmt.Errorf("writing melange config: %w", err)
	}

	// Generate signing key.
	keyPath := filepath.Join(workDir, "melange.rsa")
	if err := runTool(ctx, "melange", "keygen", keyPath); err != nil {
		return nil, fmt.Errorf("melange keygen: %w", err)
	}

	// Build APK.
	packagesDir := filepath.Join(workDir, "packages")
	platform := opts.Platform
	if platform == "" {
		platform = "x86_64"
	}

	args := []string{
		"build", melangePath,
		"--source-dir", opts.SourceDir,
		"--out-dir", packagesDir,
		"--signing-key", keyPath,
		"--arch", platform,
	}

	if runner := detectMelangeRunner(); runner != "" {
		args = append(args, "--runner", runner)
		fmt.Printf("  → Runner: %s (bubblewrap not available)\n", runner)
	}

	if opts.TLS.HasCustomCAs() {
		pemData, err := tlsutil.LoadCACerts(opts.SourceDir, opts.TLS)
		if err != nil {
			return nil, fmt.Errorf("preparing CA certs: %w", err)
		}
		if len(pemData) > 0 {
			const bundleName = ".nimbopacks-ca-bundle.pem"
			hostBundle := filepath.Join(opts.SourceDir, bundleName)
			if err := os.WriteFile(hostBundle, pemData, 0o644); err != nil {
				return nil, fmt.Errorf("writing CA bundle into workspace: %w", err)
			}
			defer os.Remove(hostBundle)
			guestBundle := "/home/build/" + bundleName

			envFile := filepath.Join(workDir, "melange.env")
			if err := writeEnvFile(envFile, tlsutil.MelangeEnvVars(guestBundle)); err != nil {
				return nil, fmt.Errorf("writing melange env file: %w", err)
			}
			args = append(args, "--env-file", envFile)

			// If injecting into image, add a pipeline step.
			if opts.TLS.ShouldInjectIntoImage() {
				step := tlsutil.ImageCACertInstallStep(guestBundle)
				if step != nil {
					plan.Melange.Pipeline = append(plan.Melange.Pipeline, *step)
					// Re-write melange.yaml with the extra step.
					if err := writeYAML(melangePath, plan.Melange); err != nil {
						return nil, fmt.Errorf("re-writing melange config with CA cert step: %w", err)
					}
				}
			}
		}
	}

	if err := runTool(ctx, "melange", args...); err != nil {
		return nil, fmt.Errorf("melange build: %w", err)
	}

	return &backend.BuildResult{
		PackagesDir:    packagesDir,
		SigningKeyPath: keyPath,
	}, nil
}

func (b *Backend) AssembleImage(ctx context.Context, plan *types.BuildPlan, buildResult *backend.BuildResult, opts backend.ImageOpts) (*backend.ImageResult, error) {
	workDir := filepath.Dir(buildResult.PackagesDir)

	// Patch the local repo path.
	apkoCfg := plan.Apko
	for i, repo := range apkoCfg.Contents.Repositories {
		if repo == "/home/build/packages" {
			apkoCfg.Contents.Repositories[i] = buildResult.PackagesDir
		}
	}

	// Write apko.yaml.
	apkoPath := filepath.Join(workDir, "apko.yaml")
	if err := writeYAML(apkoPath, apkoCfg); err != nil {
		return nil, fmt.Errorf("writing apko config: %w", err)
	}

	// Determine output.
	outputDir := opts.OutputDir
	if outputDir == "" {
		outputDir = filepath.Join(workDir, "output")
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating output dir: %w", err)
	}

	tag := opts.Tag
	if tag == "" {
		tag = plan.Melange.Package.Name + ":latest"
	}

	tarball := filepath.Join(outputDir, "image.tar")

	// Build image with apko.
	// Since apko v0.27.0, adding a "layering" section to the config enables
	// multi-layer images using the "origin" strategy. Without it, apko
	// produces a single layer (legacy behavior).
	//
	// Reference: https://github.com/chainguard-dev/apko/blob/main/docs/layering.md

	// Pass through layering config from nimpack.yaml into the apko config.
	if opts.Layering != nil {
		apkoCfg.Layering = opts.Layering
		fmt.Printf("  → Layering: strategy=%s, budget=%d\n", opts.Layering.Strategy, opts.Layering.Budget)
	}

	// Re-write apko.yaml with layering config included.
	if err := writeYAML(apkoPath, apkoCfg); err != nil {
		return nil, fmt.Errorf("writing apko config: %w", err)
	}

	fmt.Println("  → Building image with apko...")

	args := []string{
		"build", apkoPath,
		tag, tarball,
	}

	// Constrain apko to the architecture melange actually built.
	// Without --arch, apko attempts every architecture Docker knows about,
	// and Wolfi only publishes x86_64 / arm64 — the rest 404.
	if arch := builtArch(buildResult.PackagesDir); arch != "" {
		args = append(args, "--arch", arch)
	}

	if buildResult.SigningKeyPath != "" {
		args = append(args, "--keyring-append", buildResult.SigningKeyPath+".pub")
	}

	// Direct apko to write SBOMs into the output directory so we can find them.
	args = append(args, "--sbom-path", outputDir)

	if err := runTool(ctx, "apko", args...); err != nil {
		return nil, fmt.Errorf("apko build: %w", err)
	}

	// Find the arch-specific SBOM apko writes as sbom-<arch>.spdx.json.
	// This file contains the complete package list and is what grype needs.
	sbomPath := findApkoSBOM(outputDir)

	result := &backend.ImageResult{
		TarballPath: tarball,
		Tag:         tag,
		SBOMPath:    sbomPath,
	}

	// Push if requested.
	if opts.Push {
		if err := runTool(ctx, "docker", "load", "-i", tarball); err != nil {
			return result, fmt.Errorf("docker load: %w", err)
		}
		if err := runTool(ctx, "docker", "push", tag); err != nil {
			return result, fmt.Errorf("docker push: %w", err)
		}
	}

	return result, nil
}

func (b *Backend) CheckUpdates(_ context.Context, packages []string, _ *types.UpdateConfig) ([]types.PackageUpdate, error) {
	// Query Wolfi APKINDEX for each package.
	// In production this parses the actual APKINDEX.tar.gz.
	var updates []types.PackageUpdate
	for _, pkg := range packages {
		// Placeholder: in a real implementation, compare installed vs latest.
		updates = append(updates, types.PackageUpdate{
			Package:   pkg,
			UpdatedAt: time.Now(),
		})
	}
	return updates, nil
}

// --- Helpers ---

// findApkoSBOM returns the path to the arch-specific SBOM written by apko into dir,
// i.e. "sbom-<arch>.spdx.json". Returns "" if not found.
func findApkoSBOM(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		name := e.Name()
		// apko writes sbom-<arch>.spdx.json and sbom-index.spdx.json.
		// The arch-specific file has the full package inventory.
		if len(name) > 5 && name[:5] == "sbom-" && name != "sbom-index.spdx.json" {
			return filepath.Join(dir, name)
		}
	}
	return ""
}

// builtArch returns the architecture subdirectory created by melange inside
// packagesDir (e.g. "x86_64"), or "" if it cannot be determined.
// apko accepts APK arch names (x86_64, aarch64) for --arch and maps them to OCI
// names (x86_64, arm64) internally, so we can pass this value directly.
func builtArch(packagesDir string) string {
	entries, err := os.ReadDir(packagesDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			return e.Name()
		}
	}
	return ""
}

// detectMelangeRunner returns the --runner value to pass to melange, or "" to
// use melange's compiled-in default (bubblewrap).
//
// On systems where bubblewrap is unavailable (e.g. WSL2, rootless containers)
// we fall back to docker. If neither is found we return "" and let melange
// produce its own error — it may have other runners configured (lima, kubernetes).
func detectMelangeRunner() string {
	if _, err := exec.LookPath("bwrap"); err == nil {
		return "" // bubblewrap is present; use melange's default
	}
	if _, err := exec.LookPath("docker"); err == nil {
		return "docker"
	}
	return ""
}

// toolchainPath returns the managed binary path.
func toolchainPath(tool string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".nimbopacks", "toolchain", "bin", tool)
}

// runTool executes a tool, checking managed path first.
func runTool(ctx context.Context, name string, args ...string) error {
	// Check managed toolchain first.
	managed := toolchainPath(name)
	if _, err := os.Stat(managed); err == nil {
		name = managed
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// writeEnvFile writes env vars as KEY=VALUE lines for melange's --env-file.
// Keys are sorted so the output is deterministic (reproducible builds).
func writeEnvFile(path string, env map[string]string) error {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%s\n", k, env[k])
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func writeYAML(path string, v any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := yaml.NewEncoder(f)
	enc.SetIndent(2)
	return enc.Encode(v)
}
