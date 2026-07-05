// Package pipeline wires detect → plan → build using the backend abstraction.
package pipeline

import (
	"context"
	"fmt"

	"github.com/Nimbostack/nimbopacks/internal/config"
	"github.com/Nimbostack/nimbopacks/internal/pack/registry"
	"github.com/Nimbostack/nimbopacks/pkg/backend"
)

type BuildOptions struct {
	SourceDir   string
	ConfigPath  string
	Tag         string
	OutputDir   string
	Push        bool
	BackendName string // "wolfi", or "" for auto-select
	CACertPath  string // CLI/env override for custom CA cert
}

func Build(ctx context.Context, opts BuildOptions) (*backend.ImageResult, error) {
	fmt.Println("🌩️  nimbopacks")
	fmt.Println()

	// 1. Load nimpack.yaml (required for builds).
	fmt.Println("[1/4] Loading config...")
	cfg, err := config.Load(opts.SourceDir, opts.ConfigPath)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if cfg == nil {
		return nil, fmt.Errorf("nimpack.yaml not found in %s; run `nimbopacks init` to generate one, or `nimbopacks detect` to auto-detect your project", opts.SourceDir)
	}

	// Merge CLI/env CA cert into config (CLI wins over yaml).
	if opts.CACertPath != "" {
		cfg.TLS.CACertPaths = append(cfg.TLS.CACertPaths, opts.CACertPath)
	}

	fmt.Printf("  → %s %s (pack: %s)\n", cfg.Project.Name, cfg.Project.Version, cfg.Build.Pack)

	// 2. Select backend.
	fmt.Println("[2/4] Selecting backend...")
	var b backend.Backend
	if opts.BackendName != "" {
		b, err = backend.Get(opts.BackendName)
		if err != nil {
			return nil, err
		}
		avail, reason := b.Available(ctx)
		if !avail {
			return nil, fmt.Errorf("backend %q not available: %s", opts.BackendName, reason)
		}
	} else {
		b, err = backend.AutoSelect(ctx)
		if err != nil {
			return nil, err
		}
	}
	caps := backend.GetCapabilities(b.Name())
	fmt.Printf("  → Backend: %s", b.Name())
	if caps.Reproducible {
		fmt.Print(" (reproducible, SBOM)")
	}
	fmt.Println()

	// 3. Plan.
	fmt.Println("[3/4] Planning build...")
	p := registry.Get(cfg.Build.Pack)
	if p == nil {
		return nil, fmt.Errorf("pack %q not found; available: %v", cfg.Build.Pack, registry.Names())
	}

	plan, err := p.Plan(ctx, opts.SourceDir, cfg)
	if err != nil {
		return nil, fmt.Errorf("plan: %w", err)
	}
	fmt.Printf("  → %d build steps, %d runtime packages\n",
		len(plan.Melange.Pipeline), len(plan.Apko.Contents.Packages))

	// 4. Build.
	fmt.Println("[4/4] Building...")

	if cfg.TLS.HasCustomCAs() {
		fmt.Println("  → Custom CA certificates configured")
	}

	buildResult, err := b.BuildPackage(ctx, plan, backend.BuildOpts{
		SourceDir: opts.SourceDir,
		TLS:       cfg.TLS,
	})
	if err != nil {
		return nil, fmt.Errorf("build: %w", err)
	}

	imgResult, err := b.AssembleImage(ctx, plan, buildResult, backend.ImageOpts{
		Tag:       opts.Tag,
		OutputDir: opts.OutputDir,
		Push:      opts.Push,
		TLS:       cfg.TLS,
		Layering:  cfg.Image.Layering,
	})
	if err != nil {
		return nil, fmt.Errorf("assemble: %w", err)
	}

	fmt.Println()
	fmt.Printf("  ✓ Image: %s\n", imgResult.TarballPath)
	fmt.Printf("  ✓ Tag:   %s\n", imgResult.Tag)
	if imgResult.SBOMPath != "" {
		fmt.Printf("  ✓ SBOM:  %s\n", imgResult.SBOMPath)
	}
	if imgResult.Digest != "" {
		fmt.Printf("  ✓ Digest: %s\n", imgResult.Digest)
	}

	return imgResult, nil
}
