# Nimbopacks Samples

A working sample for every nimbopacks template, plus a showcase that walks
through the CVE patching workflow end-to-end.

Each sample directory contains a real (small) application, a `nimpack.yaml`
that fully describes the build, and a `README.md` with build/run/view steps.
You can `cd` into any directory and run `nimbopacks build` directly — no
detection step required.

## Quick start

```bash
# One-time toolchain install
nimbopacks toolchain install

# Pick any sample and build
cd samples/go/rest
nimbopacks build
docker run --rm -p 8080:8080 go-rest-sample
```

## Verifying samples

[`verify-builds.sh`](verify-builds.sh) builds each sample, runs the resulting image, and
probes it (HTTP, gRPC h2c, or a worker log pattern) — proving the samples *run*,
not just that they build. The [Samples CI workflow](../.github/workflows/samples.yml)
runs it per-sample on relevant changes and weekly.

```bash
task build                       # build the nimbopacks binary first
./samples/verify-builds.sh              # verify every sample
./samples/verify-builds.sh go/rest node/express   # or just specific ones
```

## Index

### .NET

| Sample | Template | What it shows |
|---|---|---|
| [dotnet/minimal-api](dotnet/minimal-api) | `dotnet-minimal-api` | Single-file ASP.NET Core minimal API |
| [dotnet/webapi](dotnet/webapi) | `dotnet-webapi` | ASP.NET Core Web API with a controller |
| [dotnet/monorepo](dotnet/monorepo) | `dotnet-solution` | **Monorepo** — a `.sln` with two projects (`Api` + `Worker`) packaged into one image via `artifacts:` |
| [dotnet/grpc](dotnet/grpc) | `dotnet-grpc` | ASP.NET Core gRPC service over HTTP/2 (h2c) |
| [dotnet/worker](dotnet/worker) | `dotnet-worker` | Background `Worker` service (no HTTP port) |
| [dotnet/blazor](dotnet/blazor) | `dotnet-blazor` | Blazor Web App with interactive server rendering |

### Go

| Sample | Template | What it shows |
|---|---|---|
| [go/rest](go/rest) | `go` | Stdlib HTTP server |
| [go/grpc](go/grpc) | `go-grpc` | gRPC service with reflection enabled |

### Java

| Sample | Template | What it shows |
|---|---|---|
| [java/maven](java/maven) | `java-maven` | Spring Boot MVC (servlet) app built with Maven |
| [java/gradle](java/gradle) | `java-gradle` | Spring Boot MVC (servlet) app built with Gradle |
| [java/quarkus](java/quarkus) | `java-quarkus` | Quarkus 3 REST app (Maven, uber-jar) |
| [java/micronaut](java/micronaut) | `java-micronaut` | Micronaut 4 app on Netty (Maven, shaded jar) |
| [java/webflux](java/webflux) | `java-webflux` | Spring Boot WebFlux reactive app on Netty (Maven) |

### Node.js

| Sample | Template | What it shows |
|---|---|---|
| [node/express](node/express) | `node-express` | Express HTTP API |
| [node/fastify](node/fastify) | `node-fastify` | Fastify HTTP API |
| [node/hono](node/hono) | `node-hono` | Hono API served by `@hono/node-server` |
| [node/nestjs](node/nestjs) | `node-nestjs` | NestJS API compiled with `tsc` |
| [node/nextjs](node/nextjs) | `node-nextjs` | Next.js with `output: "standalone"` |

### Python

| Sample | Template | What it shows |
|---|---|---|
| [python/fastapi](python/fastapi) | `python-fastapi` | FastAPI served by uvicorn |
| [python/django](python/django) | `python-django` | Django served by gunicorn |

### Web

| Sample | Template | What it shows |
|---|---|---|
| [web/static](web/static) | `web-static` | Plain static HTML served by nginx |
| [web/spa](web/spa) | `web-spa` | Vite + React SPA built and served by nginx |
| [web/hugo](web/hugo) | `web-hugo` | Hugo static site built and served by nginx |
| [web/custom-nginx](web/custom-nginx) | `web-static` | Static site served with a project-supplied `nginx.conf` via `NGINX_CONF_PATH` |

### Showcase

| Sample | What it shows |
|---|---|
| [showcase/cve-patching](showcase/cve-patching) | End-to-end CVE workflow: SBOM → `nimbopacks update --check` severity gate (exit codes) → remediate via rebuild, pin bump, or `.grype.yaml` triage policy → SARIF for GitHub Code Scanning |
| [showcase/layering](showcase/layering) | apko-native multi-layer images via `image.layering` — `strategy: origin` + `budget` tuning |
| [showcase/custom-ca-certs](showcase/custom-ca-certs) | Injecting custom CA certificates via `tls.ca_cert_paths` for corporate proxies, private registries, and internal HTTPS |

## Enterprise capabilities these samples demonstrate

Every sample is a small app, but together they exercise the properties you need
for production container images:

- **Minimal, distroless runtime images** — built from Wolfi packages with no
  shell or package manager in the final image (single-binary entrypoint).
- **Non-root by default** — every image runs as UID `65532`.
- **Automatic SBOMs** — each build emits an SPDX SBOM (`output/sbom-*.spdx.json`).
- **CVE gating & triage** — `nimbopacks update --check` scans the SBOM with grype,
  fails CI on a severity threshold, emits SARIF, and honors a `.grype.yaml`
  triage policy. See [showcase/cve-patching](showcase/cve-patching).
- **Reproducible builds** — melange + apko produce deterministic, signed packages.
- **apko-native layering** — tune layer count/grouping for cache efficiency. See
  [showcase/layering](showcase/layering).
- **Custom CA trust** — inject corporate/private CAs into the build and image for
  proxies and private registries. See [showcase/custom-ca-certs](showcase/custom-ca-certs).
- **Config-first & polyglot** — one `nimpack.yaml` schema across 6 language
  ecosystems, including a .NET **monorepo** packaged into a single image.

## Conventions used in every sample

- Project names are kebab-case and end in `-sample` so the resulting Docker
  image is unambiguous (`go-rest-sample`, `dotnet-webapi-sample`, …).
- All HTTP samples listen on port `8080` and expose `/` and `/healthz`.
- All images run as a non-root user (`run_as: 65532`).
- All `nimpack.yaml` files enable `update.auto_check: true` so a build
  surfaces CVEs in the resulting image.
