# Contributing to Nimbopacks

## Architecture

```
nimbopacks/
├── cmd/
│   └── nimbopacks/          # CLI entry point (cobra)
├── internal/
│   ├── types/               # Shared types and nimpack.yaml schema
│   ├── pack/                # Pack interface + shared helpers
│   │   └── registry/        # Global pack registry (self-registration via init())
│   ├── cache/               # Build artifact cache
│   ├── config/              # nimpack.yaml loader
│   ├── pipeline/            # Orchestration: config → detect → plan → build
│   ├── toolchain/           # Auto-download melange, apko, grype
│   ├── update/              # CVE scanning (grype invocation, SBOM resolution)
│   └── utils/               # Shared utilities (tlsutil)
├── pkg/
│   ├── backend/             # Build backend abstraction
│   │   └── wolfi/           # melange + apko backend
│   ├── packs/               # ← one directory per language
│   │   ├── dotnet/
│   │   ├── golang/
│   │   ├── java/
│   │   ├── node/
│   │   ├── python/
│   │   └── webserver/
│   └── templates/           # nimpack.yaml templates
│       └── templates/       # Standalone YAML files (go:embed)
```

**Key principle:** Detection generates config. Builds read config. No magic.

## Adding a New Language Pack

Touch 2 files:

1. Create `pkg/packs/<lang>/<lang>pack.go`
2. Add one blank import to `cmd/nimbopacks/main.go`

### The Pack Interface

```go
type Pack interface {
    Name() string
    Detect(ctx, srcDir) (*DetectResult, error)
    GenerateConfig(ctx, srcDir, detected, templateName) (*NimpackConfig, error)
    Plan(ctx, srcDir, cfg) (*BuildPlan, error)
}
```

**Detect** — Fast, read-only. Returns `SuggestedTemplate` to guide `nimbopacks init`.

**GenerateConfig** — Produces a `nimpack.yaml` from detection results. Called by `nimbopacks init --detect`.

**Plan** — Reads `nimpack.yaml`, produces melange + apko configs. This is the build path.

### Example

```go
package ruby

import (
    "context"
    "github.com/Nimbostack/nimbopacks/internal/pack"
    "github.com/Nimbostack/nimbopacks/internal/pack/registry"
    "github.com/Nimbostack/nimbopacks/internal/types"
)

func init() { registry.Register(&Pack{}) }

type Pack struct{}

func (p *Pack) Name() string { return "ruby" }

func (p *Pack) Detect(ctx context.Context, srcDir string) (*types.DetectResult, error) {
    if !pack.FileExists(srcDir, "Gemfile") {
        return nil, nil
    }
    return &types.DetectResult{
        PackName: "ruby", Confidence: 0.8,
        Summary: "Ruby project", SuggestedTemplate: "ruby-rails",
    }, nil
}

func (p *Pack) GenerateConfig(...) (*types.NimpackConfig, error) { ... }
func (p *Pack) Plan(...) (*types.BuildPlan, error) { ... }
```

Then in `cmd/nimbopacks/main.go`:

```go
_ "github.com/Nimbostack/nimbopacks/pkg/packs/ruby"
```

## Adding a New Template

Add a YAML file under `pkg/templates/templates/<pack>/`:

```yaml
# Template: ruby-rails
# Pack: ruby
# Description: Ruby on Rails application
# Tags: web, api
---
schema_version: "1"
project:
  name: {{ .ProjectName }}
...
```

The `go:embed` directive in `pkg/templates/` picks it up automatically at next build. No Go code changes needed.

## Design Principles

**Packs are self-contained.** All language-specific logic lives in `pkg/packs/<lang>/`. The core packages (`types`, `pipeline`, `backend`, `pack`) are language-agnostic — adding a new language should never require touching them. If you find you need a new top-level config field, open an issue first to discuss the schema change.

**Detect is fast and read-only.** `Detect()` must return quickly with no side effects — it runs on every `nimbopacks init --detect` call and is designed to be safe to run across many directories. No network calls, no shelling out, no writing files. Return `nil, nil` if the project doesn't match.

**Builds are config-driven.** Detection exists to generate `nimpack.yaml`. Builds always read it. This makes builds predictable and auditable — the config is the contract, not the detection result.

**Use the shared helpers.** `pack.FileExists`, `pack.ReadFile`, `pack.NewMelangeConfig`, `pack.NewApkoConfig`, and `pack.ApplyConfig` exist to keep pack code consistent. Prefer them over writing your own equivalents.

## Commit Messages

This project uses [Conventional Commits](https://www.conventionalcommits.org):

```
<type>[optional scope]: <description>
```

Allowed types: `feat`, `fix`, `docs`, `chore`, `refactor`, `ci`

```
feat: add ruby pack
fix(toolchain): handle missing melange binary
docs: update contributing guide
refactor(pipeline): simplify plan execution
```

CI enforces this on every PR. To catch violations locally:

```bash
task hooks
```

## Running Tests

```bash
task test                                        # all tests
task test-verbose                                # all tests with verbose output
go test ./pkg/packs/dotnet/...                   # one pack
go test -v -run TestDetect_Solution ./...        # one test
```

Pack tests create temp directories with `t.TempDir()` and write fixture files with a `writeFile` helper. See any existing pack test for the pattern.
