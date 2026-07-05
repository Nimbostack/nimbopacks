# nimbopacks

[![CI](https://github.com/Nimbostack/nimbopacks/actions/workflows/ci.yml/badge.svg)](https://github.com/Nimbostack/nimbopacks/actions/workflows/ci.yml)
[![CodeQL](https://github.com/Nimbostack/nimbopacks/actions/workflows/codeql.yml/badge.svg)](https://github.com/Nimbostack/nimbopacks/actions/workflows/codeql.yml)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/Nimbostack/nimbopacks/badge)](https://scorecard.dev/viewer/?uri=github.com/Nimbostack/nimbopacks)
[![Go Report Card](https://goreportcard.com/badge/github.com/Nimbostack/nimbopacks)](https://goreportcard.com/report/github.com/Nimbostack/nimbopacks)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.24-00ADD8.svg)](https://go.dev)

Minimal, reproducible OCI container images from source — with built-in SBOMs and CVE scanning.

Nimbopacks builds images using [melange](https://github.com/chainguard-dev/melange) (APK package builder) and [apko](https://github.com/chainguard-dev/apko) (image assembler) from the [Wolfi](https://github.com/wolfi-dev) ecosystem. Every build emits an SBOM, and CVE scanning is a first-class workflow — patching is just a rebuild against Wolfi's continuously-updated packages.

`nimpack.yaml` is the single source of truth. Detection generates it; builds always read it.

## Quick Start

```bash
# Install
go install github.com/Nimbostack/nimbopacks/cmd/nimbopacks@latest

# Install build tools (one time)
nimbopacks toolchain install

# Scaffold config from a template
nimbopacks init go

# Or auto-detect and generate
nimbopacks init --detect

# Build
nimbopacks build

# Push to a registry
nimbopacks build --tag ghcr.io/myorg/myapp:v1.0.0 --push

# Scan for CVEs (exit 1 in CI if any meet the threshold)
nimbopacks update --check

# "Patch" = rebuild to pull the latest fixed Wolfi packages
nimbopacks build
```

## How It Works

```
                  ┌──────────────────┐
 nimbopacks init  │   nimpack.yaml   │  ← single source of truth
                  └────────┬─────────┘
                           │
                  ┌────────▼─────────┐
 nimbopacks build │  wolfi backend   │
                  │  ┌─────┐ ┌────┐  │
                  │  │mela-│ │apko│  │  ← melange builds APK,
                  │  │nge  │ │    │  │    apko assembles image
                  │  └──┬──┘ └─┬──┘  │
                  └─────┼──────┼─────┘
                        │      │
                 APK package  OCI image + SBOM
```

**Detection is a config generator, not the build path.** Builds always go through `nimpack.yaml`. This makes builds predictable and debuggable — you can always read the config to understand exactly what will happen.

## Templates

Templates are opinionated starting configs for specific project types:

```bash
nimbopacks templates

  dotnet:
    dotnet-blazor         .NET Blazor Web App (interactive server rendering)
    dotnet-grpc           .NET gRPC service (ASP.NET Core, HTTP/2)
    dotnet-minimal-api    .NET Minimal API (single file, lightweight)
    dotnet-solution       .NET Solution monorepo (multiple projects)
    dotnet-webapi         .NET Web API (single project)
    dotnet-worker         .NET Worker Service (background service, no HTTP port)
  go:
    go                    Go REST API (gin, echo, chi, or stdlib)
    go-grpc               Go gRPC service with protobuf
  java:
    java-gradle           Java Gradle application
    java-maven            Java Maven application
    java-micronaut        Micronaut 4 application (Maven, shaded jar)
    java-quarkus          Quarkus 3 application (Maven, uber-jar)
    java-webflux          Spring Boot WebFlux reactive app (Maven, Netty)
  node:
    node-express          Node.js Express API
    node-fastify          Node.js Fastify API
    node-hono             Node.js Hono API
    node-nestjs           NestJS application compiled with tsc
    node-nextjs           Next.js application with standalone output
  python:
    python-django         Python Django with gunicorn
    python-fastapi        Python FastAPI with uvicorn
  web:
    web-hugo              Hugo static site
    web-spa               SPA (React/Vue/Svelte/Angular) with nginx
    web-static            Plain static HTML site with nginx
```

## Samples

The [`samples/`](samples/) directory has a **working, buildable app for every
template** plus showcases of the enterprise capabilities — CVE scanning &
remediation, apko-native layering, and custom CA trust. Each sample is
`cd`-and-`nimbopacks build`-able with a README walking through build/run/view.

```bash
cd samples/go/rest && nimbopacks build
docker run --rm -p 8080:8080 go-rest-sample
```

See [samples/README.md](samples/README.md) for the full index across .NET, Go,
Java, Node.js, Python, and static-web stacks.

## Pushing Images

The `--tag` flag sets the full image reference; `--push` loads it into Docker and pushes:

```bash
# Push to GitHub Container Registry
nimbopacks build --tag ghcr.io/myorg/myapp:v1.2.3 --push

# Push to Docker Hub
nimbopacks build --tag myorg/myapp:latest --push

# Push to a private registry (with custom CA cert)
nimbopacks build \
  --tag registry.internal/myapp:v1.0.0 \
  --push \
  --ca-cert /path/to/ca.pem

# Build locally only (no push) — saves tarball to output/
nimbopacks build --tag myapp:dev
```

Without `--tag`, the image is tagged `<project-name>:latest`.

## Monorepo Support

For .NET solutions, or any multi-project setup:

```yaml
# nimpack.yaml
artifacts:
  - name: api
    source: src/MyApp.Api
    command: "dotnet publish src/MyApp.Api -c Release -o /home/build/output/app/api"
    dest: /app/api
  - name: worker
    source: src/MyApp.Worker
    command: "dotnet publish src/MyApp.Worker -c Release -o /home/build/output/app/worker"
    dest: /app/worker
```

## CVE Scanning

`nimbopacks update` scans the build's SBOM with [grype](https://github.com/anchore/grype).
Remediation is a rebuild — Wolfi ships patches continuously, so `nimbopacks build`
pulls the latest fixed packages.

```bash
# Scan and print findings (non-blocking, always exits 0)
nimbopacks update

  🔴 openssl                  → 3.2.1-r1     CVE-2024-0727
  🟠 ca-certificates-bundle   → 20240226-r0  CVE-2024-0567

# CI gate: exit 1 if any finding is at/above the threshold (default: high)
nimbopacks update --check
nimbopacks update --check --fail-on critical

# SARIF for GitHub Code Scanning
nimbopacks update --check --format sarif -o results.sarif

# Scan an existing SBOM without rebuilding
nimbopacks update --check --sbom ./output/sbom-x86_64.spdx.json

# Triage false positives with a grype policy (.grype.yaml in the project root is
# auto-detected; --grype-config points at a non-default location)
nimbopacks update --check --grype-config path/to/policy.yaml
```

**Exit codes:** `0` clean · `1` CVEs at/above threshold · `2` tool/config error.
**Env vars:** `NIMBOPACKS_FAIL_ON`, `NIMBOPACKS_FORMAT`, `NIMBOPACKS_GRYPE_CONFIG`, `NIMBOPACKS_GRYPE_DB_CACHE` (CI caching).

## Command Reference

| Command | Description |
|---|---|
| `nimbopacks init <template>` | Scaffold `nimpack.yaml` from a template |
| `nimbopacks init --detect` | Auto-detect project type and generate config |
| `nimbopacks detect` | Detect project type (read-only, no files written) |
| `nimbopacks detect --generate` | Detect and write `nimpack.yaml` |
| `nimbopacks build` | Build OCI image from `nimpack.yaml` |
| `nimbopacks build --tag <ref> --push` | Build and push to a registry |
| `nimbopacks update` | Scan for CVEs, print remediation (non-blocking) |
| `nimbopacks update --check` | Scan for CVEs, exit 1 if found at threshold |
| `nimbopacks templates` | List available templates |
| `nimbopacks packs` | List registered language packs |
| `nimbopacks status` | Show toolchain, backends, and packs |
| `nimbopacks toolchain install` | Download melange, apko, grype |
| `nimbopacks toolchain status` | Show installed versions |
| `nimbopacks toolchain upgrade` | Upgrade to latest |
| `nimbopacks toolchain remove` | Remove managed toolchain |
| `nimbopacks version` | Print version |

## Prerequisites

Nimbopacks manages its own toolchain — `nimbopacks toolchain install`
auto-downloads melange, apko, and grype (see the `toolchain` commands above).
Builds run melange in a sandbox (bubblewrap, or Docker where bubblewrap is
unavailable, e.g. WSL2). You can also install the tools manually:
[melange](https://github.com/chainguard-dev/melange),
[apko](https://github.com/chainguard-dev/apko),
[grype](https://github.com/anchore/grype).

## Using the Container Image

Every release publishes a pre-built OCI image to GitHub Container Registry. The image ships nimbopacks together with its full toolchain — melange, apko, grype, and bash — so no separate install step is needed.

```bash
# Pull the latest release
docker pull ghcr.io/nimbostack/nimbopacks:latest

# Run a build against your project directory
docker run --rm \
  -v $(pwd):/src \
  -w /src \
  ghcr.io/nimbostack/nimbopacks:latest \
  build

# Check for CVEs
docker run --rm \
  -v $(pwd):/src \
  -w /src \
  ghcr.io/nimbostack/nimbopacks:latest \
  update --check

# Pin to a specific version
docker pull ghcr.io/nimbostack/nimbopacks:v0.1.0
```

**CI example (GitHub Actions):**

```yaml
jobs:
  build:
    runs-on: ubuntu-latest
    container:
      image: ghcr.io/nimbostack/nimbopacks:latest
    steps:
      - uses: actions/checkout@v4
      - run: nimbopacks build --tag ghcr.io/myorg/myapp:${{ github.ref_name }} --push
      - run: nimbopacks update --check --format sarif -o results.sarif
      - uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: results.sarif
```

All release images are keyless-signed with [cosign](https://github.com/sigstore/cosign). Verify before running:

```bash
cosign verify ghcr.io/nimbostack/nimbopacks:latest \
  --certificate-identity-regexp="https://github.com/Nimbostack/nimbopacks/.github/workflows/release.yml" \
  --certificate-oidc-issuer="https://token.actions.githubusercontent.com"
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Adding a new language pack or template requires one new file each.

## Acknowledgements

Nimbopacks builds on top of excellent open source tools:

- **[melange](https://github.com/chainguard-dev/melange)** — APK package builder from source, by [Chainguard](https://chainguard.dev)
- **[apko](https://github.com/chainguard-dev/apko)** — OCI image assembler from APK packages, by [Chainguard](https://chainguard.dev)
- **[grype](https://github.com/anchore/grype)** — vulnerability scanner for container images and SBOMs, by [Anchore](https://anchore.com)
- **[Wolfi](https://github.com/wolfi-dev/os)** — supply chain-hardened Linux undistro, by [Chainguard](https://chainguard.dev)
