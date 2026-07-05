// Package nodepack is the nimbopacks pack for Node.js projects.
package nodepack

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"

	"github.com/Nimbostack/nimbopacks/internal/pack"
	"github.com/Nimbostack/nimbopacks/internal/pack/registry"
	"github.com/Nimbostack/nimbopacks/internal/types"
)

func init() { registry.Register(&Pack{}) }

type Pack struct{}

func (p *Pack) Name() string { return "node" }

func (p *Pack) Detect(_ context.Context, srcDir string) (*types.DetectResult, error) {
	if !pack.FileExists(srcDir, "package.json") {
		return nil, nil
	}
	meta := map[string]string{"pm": detectPM(srcDir)}
	confidence := 0.8
	pkg := parsePkg(srcDir)
	if pkg.Name != "" {
		meta["name"] = pkg.Name
	}
	fw := detectFW(pkg.Deps)
	if fw != "" {
		meta["framework"] = fw
		confidence = 0.9
	}
	if pack.FileExists(srcDir, "tsconfig.json") {
		meta["typescript"] = "true"
	}
	suggested := "node-express"
	switch fw {
	case "nextjs":
		suggested = "node-nextjs"
	case "nestjs":
		suggested = "node-nestjs"
	case "fastify":
		suggested = "node-fastify"
	case "hono":
		suggested = "node-hono"
	}
	return &types.DetectResult{
		PackName: p.Name(), Confidence: confidence,
		Summary:           fmt.Sprintf("Node.js/%s (%s)", fw, meta["pm"]),
		SuggestedTemplate: suggested, Metadata: meta,
	}, nil
}

func (p *Pack) GenerateConfig(_ context.Context, _ string, detected *types.DetectResult, tmpl string) (*types.NimpackConfig, error) {
	name := detected.Metadata["name"]
	if name == "" {
		name = "app"
	}
	cfg := pack.BaseConfig(name, "0.0.1", p.Name(), tmpl)
	cfg.Image.Packages = []string{"nodejs-20", "ca-certificates-bundle"}
	cfg.Image.Entrypoint = "node"
	cfg.Image.Env = map[string]string{"NODE_ENV": "production"}
	cfg.Build.Command = "npm ci && npm run build"
	return &cfg, nil
}

func (p *Pack) Plan(_ context.Context, _ string, cfg *types.NimpackConfig) (*types.BuildPlan, error) {
	melange := pack.NewMelangeConfig(cfg.Project.Name, cfg.Project.Version, []string{"nodejs-20", "npm", "build-base"})
	melange.Package.Dependencies = types.MelangeDependencies{Runtime: cfg.Image.Packages}
	melange.Pipeline = []types.MelangePipelineStep{
		{Runs: cfg.Build.Command},
		// Stage the whole source tree (including node_modules) under /app, then
		// install it into the package root (${{targets.destdir}} — the only thing
		// melange actually packages). The find excludes the staging dir itself;
		// a plain `cp -r .` would recurse into it and busybox cp exits non-zero.
		{Runs: "mkdir -p /home/build/output/app\nfind . -mindepth 1 -maxdepth 1 ! -name output -exec cp -r {} /home/build/output/app/ \\;"},
		pack.InstallOutputStep(),
	}
	apko := pack.NewApkoConfig(cfg.Project.Name, cfg.Image.Entrypoint, cfg.Image.Packages)
	if len(cfg.Image.Cmd) > 0 {
		apko.Cmd = strings.Join(cfg.Image.Cmd, " ")
	}
	maps.Copy(apko.Environment, cfg.Image.Env)
	plan := &types.BuildPlan{Melange: melange, Apko: apko}
	pack.ApplyConfig(plan, cfg)
	return plan, nil
}

func detectPM(d string) string {
	switch {
	case pack.FileExists(d, "bun.lockb"):
		return "bun"
	case pack.FileExists(d, "pnpm-lock.yaml"):
		return "pnpm"
	case pack.FileExists(d, "yarn.lock"):
		return "yarn"
	default:
		return "npm"
	}
}

type pkgJSON struct {
	Name string            `json:"name"`
	Deps map[string]string `json:"dependencies"`
}

func parsePkg(d string) pkgJSON {
	c, err := pack.ReadFile(d, "package.json")
	if err != nil {
		return pkgJSON{}
	}
	var p pkgJSON
	if err := json.Unmarshal([]byte(c), &p); err != nil {
		return pkgJSON{}
	}
	return p
}

func detectFW(deps map[string]string) string {
	for _, fw := range []struct{ p, n string }{{"next", "nextjs"}, {"express", "express"}, {"fastify", "fastify"}, {"hono", "hono"}, {"@nestjs/core", "nestjs"}} {
		if _, ok := deps[fw.p]; ok {
			return fw.n
		}
	}
	return ""
}
