# Nimbopacks — architecture and conventions

Everything a contributor needs before touching this codebase: the shape of the system, the
rules that are not negotiable, and the traps that have already cost someone a day. The file
is named `CLAUDE.md` because coding agents pick it up automatically, which makes it the one
document every contributor and every tool reads; nothing in it depends on a particular tool.

## What is Nimbopacks

Nimbopacks builds minimal OCI images from source using **melange** (APK package
builder) and **apko** (image assembler) from the Wolfi/Chainguard ecosystem. Every
image gets an SBOM and is scanned for CVEs with grype. The workflow is config-first:
`nimpack.yaml` is the single source of truth, and detection generates that config —
it never drives builds directly.

Builds are **not** byte-for-byte reproducible today, and the code says so:
`backend.Capabilities.Reproducible` is `false` for the wolfi backend. melange signs
each build with a freshly generated key, the signature size reaches the image through
`/lib/apk/db/installed`, and the digest moves between two otherwise identical builds.
Keeping the ephemeral key is a deliberate trade — long-lived signing material on a
build host is the worse exposure. See
[ADR-0003](docs/adr/0003-dead-field-disposition.md) for the measurement and the
reasoning. The rest of the constraint still binds, and reviewers hold changes to it:
nothing may put wall-clock time, network response order, Go map iteration order, or
the build hostname into a build's output.

## Project Structure

```
cmd/nimbopacks/        CLI entry point (cobra); blank-imports packs + the wolfi backend
internal/
  types/               nimpack.yaml schema, BuildPlan, apko/melange config structs
  pack/                Pack interface + shared helpers; pack/registry/ self-registration
  config/              nimpack.yaml loader
  pipeline/            Orchestration: config → detect → plan → build
  toolchain/           Auto-download/manage melange + apko + grype binaries
  update/              CVE scanning: grype invocation, SBOM resolution, formatters
  cache/               Build work-dir + cache management
  utils/               tlsutil: custom CA certificate handling
pkg/
  packs/               Language packs, one dir each: dotnet, generic, golang, java, node, python, webserver
  templates/           Embedded nimpack.yaml templates (go:embed) + loader
  backend/             Build backend abstraction; backend/wolfi/ is the only implementation
samples/               Buildable example app per template + enterprise showcases
```

Note: packs, templates, and backend live under `pkg/` (public); everything else
is `internal/`.

## Architecture

- **Config-first:** Builds always read `nimpack.yaml`. Detection only generates it.
- **Self-registering packs:** Each pack calls `registry.Register()` in `init()`.
  Adding a language = one new dir under `pkg/packs/` + one blank import in `cmd/`.
- **Backend abstraction:** The `backend.Backend` interface separates build
  strategy from pack logic. wolfi (melange + apko) is the only backend today;
  the interface + registry let others be added without touching packs/pipeline.
- **Embedded templates:** Standalone YAML in `pkg/templates/templates/`, loaded
  via `go:embed`. Contributors add templates without writing Go.
- **apko-native layering:** Multi-layer images use apko's `layering` config
  (`strategy: origin`, `budget: N`). No custom layering logic.
- **CVE scanning via grype:** managed by the toolchain. `update --check` builds,
  scans the apko SBOM, and exits non-zero on CVEs at/above `--fail-on`. Plain
  `update` is non-blocking. See `internal/update/`.

## Build and Test

```bash
task build          # → bin/
task test           # all tests   (test-verbose, test-cover also available)
task lint           # golangci-lint (--fix)
task fmt            # gofumpt
task install        # go install
task clean
```

## The Pack Interface

```go
type Pack interface {
    Name() string
    Detect(ctx, srcDir) (*DetectResult, error)                          // fast, no side effects
    GenerateConfig(ctx, srcDir, detected, tmpl) (*NimpackConfig, error)  // produces nimpack.yaml
    Plan(ctx, srcDir, cfg) (*BuildPlan, error)                          // nimpack.yaml → melange + apko
}
```

Shared helpers in `internal/pack/`: `FileExists`, `ReadFile`,
`FindFilesRecursive`, `BaseConfig`, `NewMelangeConfig`, `NewApkoConfig`,
`InstallOutputStep`, `ApplyConfig`.

### Adding a pack

1. `pkg/packs/<name>/<name>pack.go`, implement `Pack`, call `registry.Register(&Pack{})` in `init()`.
2. Blank-import it in `cmd/nimbopacks/main.go`.
3. Add templates under `pkg/templates/templates/<name>/` and tests alongside the pack.

### Adding a template

Create `pkg/templates/templates/<pack>/<name>.yaml` with frontmatter, then it's
picked up by `go:embed` on next build:

```yaml
# Template: my-template
# Pack: go
# Description: Short description
# Tags: web, api
---
schema_version: "1"
...
```

Substitution vars: `{{ .ProjectName }}`, `{{ .Version }}`, `{{ .Module }}`,
`{{ .Framework }}`, `{{ .Entrypoint }}`.

## Key Types

- `types.NimpackConfig` — root nimpack.yaml schema; the central data structure.
- `types.DetectResult` — detection output, includes `SuggestedTemplate`.
- `types.BuildPlan` — `MelangeConfig` + `ApkoConfig`.
- `types.LayeringConfig` — mirrors apko's `layering` (`strategy` + `budget`).
- `types.TLSConfig` — custom CA certs that propagate to build, image, and pushes.

## Conventions

- Packs are self-contained; all language logic stays in `pkg/packs/<name>/`.
- Core packages (`types`, `pipeline`, `backend`, `pack`) are language-agnostic —
  no switch-on-language.
- `Detect()` must be fast: no network, no shelling out, no source mutation.
  Return `nil, nil` (not an error) when the project doesn't match.
- Scaffold with `pack.BaseConfig`, `NewMelangeConfig`, `NewApkoConfig`; packs
  that stage output under `/home/build/output` must append
  `pack.InstallOutputStep()` (only `${{targets.destdir}}` is packaged).
- End `Plan()` with `pack.ApplyConfig(plan, cfg)` to merge user overrides.

Adding a pack/template should **not** require touching `internal/types/`,
`internal/pipeline/`, `pkg/backend/`, or `pkg/templates/templates.go`.

## External Tools (managed by `internal/toolchain/`)

- **melange** — builds APKs from source.
- **apko** — assembles OCI images + SBOMs from APKs.
- **grype** (`anchore/grype`) — scans apko SBOMs for CVEs.
- **Wolfi** — package ecosystem (`https://packages.wolfi.dev/os`).
- apko layering uses the `origin` strategy (v0.27.0+):
  https://github.com/chainguard-dev/apko/blob/main/docs/layering.md

## CVE Scanning (`nimbopacks update`)

`internal/update/`: `update.go` (orchestration), `grype.go` (subprocess + JSON),
`format.go` (text/table/json/sarif). Run `nimbopacks update --help` for the full
flag/env-var list. Non-obvious behavior:

- **`--check` vs plain `update`:** both build → locate SBOM → `grype sbom:<path>` →
  format. `--check` exits non-zero on CVEs at/above `--fail-on` (default `high`);
  plain `update` always exits 0.
- **Exit codes:** `0` clean, `1` CVEs at/above threshold, `2` tool/config error.
- **Monorepo:** each `artifacts:` entry is scanned separately; exit code = worst severity.
- **Triage:** a project-root `.grype.yaml` is honored natively (grype runs from
  the source dir). Override location via `--grype-config` /
  `NIMBOPACKS_GRYPE_CONFIG` / `update.grype_config` (`UpdateConfig.GrypeConfig`).
  Nimbopacks implements no CVE suppression of its own — defer to grype.

## Testing Patterns

- Pack tests use `t.TempDir()` + a `writeFile` helper for fixtures.
- Registry tests call `ResetForTesting()` to isolate global state.
- Template tests assert rendered output contains expected strings.

## Design Goals

Apache-2.0. The backend abstraction avoids Wolfi/Chainguard lock-in; packs and
templates are built for community contribution with minimal Go; `nimpack.yaml` is
meant to be an implementable spec, not just this tool's config.
