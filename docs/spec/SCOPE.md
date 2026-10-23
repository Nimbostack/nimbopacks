# nimpack.yaml v1 — Scope and Gap Report

**Status:** Scoping only. This document is *not* the specification; it is the
inventory and gap report that decides what the specification can safely freeze.

**Owner:** Principal Architect
**Date:** 2026-10-23
**Baseline commit:** `d6cdba5` (branch `docs/seed-docs-skeleton`), code identical to `main` `3f962f6`
**Source of truth read:** `internal/types/types.go` (346 lines) and every consumer of it. Not the README.

---

## 0. Executive summary

| Question | Answer |
|---|---|
| Addressable keys in the `nimpack.yaml` surface | **38** total — **31 leaf keys** plus 7 block keys (`project`, `build`, `image`, `image.layering`, `artifacts`, `update`, `tls`). All counts below are over the 31 leaf keys. |
| Keys with a test that asserts a downstream effect | **7 of 31** (23%) |
| Keys merely parsed or set by a test, no behavioural assertion | **7 of 31** |
| Keys with no test touching them at all | **17 of 31** (55%) |
| Keys **never read by any production code** | **6** — `build.context`, `build.template`, `image.labels`, `image.ports`, `update.auto_check`, `update.repositories` — **plus `schema_version` itself** |
| Fields whose documented meaning does not match the implementation | **7** — `build.template`, `build.env`, `image.layering` (absent-case default), `artifacts[].name`, `artifacts[].dest`, `update.auto_check` (default), `update.pinned`; plus `tls.ca_cert_paths` (§3 secondary) |
| `schema_version` validation in the loader | **None.** The field is written by `BaseConfig` and all 23 templates and never read. |
| Blocking ambiguities to resolve before freeze | **14** (§3) — **10 resolved** in ADR-0003, 4 (A9–A11, A13) carried to ADR-0004 |
| Estimated effort for spec + JSON Schema | **6.5 engineer-days**, of which 2.0 are decisions that need CTO/Engineering Lead input (§5) |

The headline risk is not missing text. It is that **22 of the 23 shipped templates
write `image.ports`, and nothing in the codebase ever reads it.** We are one
release away from freezing a v1 schema in which six fields are decoration. Once
`schema_version: "1"` is normative, implementing any of those six changes the
image digest for existing configs and is therefore a breaking change by our own
definition (§4). The disposition of those six fields must be decided *before*
the spec is written, not after.

**Update 2026-10-23:** that disposition is **decided** in
[ADR-0003](../adr/0003-dead-field-disposition.md) (Accepted) — three architect
calls plus six product calls recorded by the CTO, and the four measurements that
needed a build environment closed by NIMA-11. §3 below records the per-field
outcome. The headline changed with the measurement: apko has **no** input path
for exposed ports, so `image.ports` is reserved as a backend-capability gap and
stripped from the generators rather than implemented, and **no image digest moves
anywhere in ADR-0003**. The spec can now be written. The last carve-out — the
reproducibility clause — was settled the same day on NIMA-17: the wolfi backend
reports `Reproducible: false`, ephemeral per-build signing keys are kept
deliberately as a security property, and the v1 guarantee is therefore
**content-equivalence, not byte-equivalence**. That has a knock-on effect on §4.2
criterion 7, which is phrased in terms of digest stability — see §4.2.

---

## 1. Field inventory — the `nimpack.yaml` surface

Scope rule used: a field is "in the nimpack.yaml surface" if it carries a
`yaml:` tag and is reachable from `types.NimpackConfig`. Required/optional is
taken from what the loader actually enforces (`internal/config/config.go:18`),
not from the struct comments.

Legend for **Tested**: ✅ a test asserts behaviour driven by this field;
⚠️ a test merely parses or sets it without asserting a downstream effect;
❌ no test.

### 1.1 Root — `types.NimpackConfig` (`types.go:15`)

| YAML key | Go type | Req? | Default | What it actually does | Tested |
|---|---|---|---|---|---|
| `schema_version` | `string` | **Not enforced** | `"1"` when generated (`pack.go:216`) | **Nothing.** No code reads it. Not in `config.Validate`. | ❌ |
| `project` | `ProjectMeta` | yes (via children) | — | Package identity | — |
| `build` | `BuildConfig` | yes (via `build.pack`) | — | Selects the pack, shapes the melange pipeline | — |
| `image` | `ImageConfig` | no | zero struct | Shapes the apko config | — |
| `artifacts` | `[]Artifact` | no | `nil` → single artifact inferred | Monorepo fan-out. Honoured by **go** and **dotnet** only. | ⚠️ |
| `update` | `UpdateConfig` | no | zero struct | CVE scan behaviour | — |
| `tls` | `TLSConfig` | no | zero struct | Custom CA propagation | — |

### 1.2 `project` — `types.ProjectMeta` (`types.go:73`)

| YAML key | Go type | Req? | Default | What it actually does | Tested |
|---|---|---|---|---|---|
| `name` | `string` | **yes** (`config.go:22`) | — | melange package name (`pack.go:160`), apko package list entry (`pack.go:126`), default image tag `<name>:latest` (`wolfi.go:166`) | ✅ |
| `version` | `string` | **yes** (`config.go:25`) | — | melange package version (`pack.go:161`). Never validated as an apk version. | ✅ |
| `description` | `string` | no | `""` | melange package description (`pack.go:162`) | ❌ |

### 1.3 `build` — `types.BuildConfig` (`types.go:79`)

| YAML key | Go type | Req? | Default | What it actually does | Tested |
|---|---|---|---|---|---|
| `pack` | `string` | **yes** (`config.go:28`) | — | Registry lookup (`pipeline.go:71`). Free string; no enum, no validation until build time. | ✅ |
| `template` | `string` | no | `""` | **Nothing.** Written by `BaseConfig` and all 23 templates; never read. Comment claims `update` uses it to pull template updates — no such code exists. | ❌ |
| `command` | `string` | no | `""` | Overrides the pack's build step. Semantics differ per pack: go **replaces** the whole pipeline (`golangpack.go:115`), dotnet **prepends** a restore step in monorepo mode (`dotnetpack.go:241`), python/node use it as the sole `runs` step. | ⚠️ (java only) |
| `dependencies` | `[]string` | no | `nil` | Appended to melange build-time packages (`pack.go:164`) | ✅ (java) |
| `env` | `map[string]string` | no | `nil` | **Not what the comment says.** It is never passed to the melange build environment. It is a pack-specific keyword channel: `ENTRYPOINT` (go, `golangpack.go:109`), `JAR_DIR` (java, `javapack.go:113`), `OUTPUT_DIR` / `NGINX_CONF_PATH` (webserver, `webserverpack.go:163,190`). Any other key is silently discarded. | ⚠️ |
| `context` | `string` | no | `"."` per comment | **Nothing.** No code reads it. Monorepo subdirectory builds do not work through this field. | ❌ |

### 1.4 `image` — `types.ImageConfig` (`types.go:101`)

| YAML key | Go type | Req? | Default | What it actually does | Tested |
|---|---|---|---|---|---|
| `packages` | `[]string` | no | `nil` | Appended to apko contents (`pack.go:172`). Also doubles as melange *runtime* deps in go/node/python/dotnet. Dual-purpose, undocumented. | ❌ |
| `entrypoint` | `string` | no (should be) | `""` | apko `entrypoint.command` (`pack.go:170`). Empty → image with no entrypoint, no error. | ✅ |
| `cmd` | `[]string` | no | `nil` | Joined with spaces into apko `cmd` (`types.go:288`) by python, java, dotnet, node, webserver. **The go pack never reads it** (`golangpack.go:149`) — `cmd` is silently dropped for Go images. | ❌ |
| `env` | `map[string]string` | no | `nil` | Copied into apko `environment` (`pack.go:180`). Verified deterministic: `yaml.v3` emits map keys sorted, so this does not threaten digest stability. | ❌ |
| `run_as` | `uint32` | no | `65532` | apko `accounts.run-as` (`pack.go:177`). Guarded by `!= 0`, so **`run_as: 0` (root) is unrepresentable** — it silently means "unset". | ❌ |
| `labels` | `map[string]string` | no | `nil` | **Nothing.** `ApkoConfig` has no annotations field. OCI labels cannot be set at all. | ❌ |
| `ports` | `[]string` | no | `nil` | **Nothing.** Written by the dotnet and webserver packs (`dotnetpack.go:188`, `webserverpack.go:133`) and by **22 of 23 templates**. Never read. | ❌ |
| `layering` | `*LayeringConfig` | no | `nil` | Passed through to apko (`pipeline.go:104`, `wolfi.go:179`). | ❌ |

### 1.5 `image.layering` — `types.LayeringConfig` (`types.go:154`)

| YAML key | Go type | Req? | Default | What it actually does | Tested |
|---|---|---|---|---|---|
| `strategy` | `string` | yes if block present | none | Emitted verbatim into apko config. Only `origin` is valid; **no validation** — a typo reaches apko. | ❌ |
| `budget` | `int` | yes if block present | `0` (no `omitempty`) | Number of extra layers. `budget: 0` is emitted explicitly; behaviour undefined on our side. | ❌ |

Architectural note: `LayeringConfig` is used **both** as the nimpack.yaml schema
type and as the apko wire format (`types.go:294` embeds the same struct in
`ApkoConfig`). Our public schema is literally apko's. If apko changes its
layering keys, `nimpack.yaml` changes with it. This is a Wolfi assumption that
has leaked into the domain (`internal/types`) and it contradicts the
`CLAUDE.md` invariant "No Wolfi/Chainguard lock-in leaks outside
`pkg/backend/wolfi/`." Needs an ADR before freeze.

### 1.6 `artifacts[]` — `types.Artifact` (`types.go:179`)

| YAML key | Go type | Req? | Default | What it actually does | Tested |
|---|---|---|---|---|---|
| `name` | `string` | yes in practice | — | Comment says "used as the APK package name". **False** — the APK name is always `project.name` (`pack.go:160` overwrites it). In go it is the output binary name (`golangpack.go:140`); in dotnet it is a path component. | ⚠️ (dotnet) |
| `source` | `string` | yes in practice | — | go: `go/build` `packages` arg. dotnet: `dotnet publish` target. | ⚠️ (dotnet) |
| `dest` | `string` | no | `""` | Comment says "where the binary is installed in the image (e.g. `/usr/bin/worker`)". **In dotnet it is concatenated onto a staging path**: `-o /home/build/output%s` (`dotnetpack.go:248`) — so it must start with `/` and is relative to the staging root, not the image root. **The go pack ignores it entirely.** | ⚠️ |
| `command` | `string` | no | `""` | Replaces the per-artifact build step (go and dotnet) | ⚠️ |

Packs that ignore `artifacts` completely: **java, node, python, webserver**. A
monorepo config written against those packs silently builds only the root
artifact.

### 1.7 `update` — `types.UpdateConfig` (`types.go:194`)

| YAML key | Go type | Req? | Default | What it actually does | Tested |
|---|---|---|---|---|---|
| `auto_check` | `bool` | no | comment says `true`; **zero value is `false`** | **Nothing.** No production code reads it. CI behaviour is driven by the `--check` flag. Note the asymmetry with `tls.inject_into_image`, which correctly uses `*bool` to express "unset". | ⚠️ (parse-only, `types_test.go:28`) |
| `repositories` | `[]string` | no | `nil` | **Nothing.** No code reads it. | ❌ |
| `pinned` | `[]string` | no | `nil` | **Annotation only.** Sets `PackageUpdate.Pinned` on the report (`update.go:133`). It does **not** suppress the finding and `MeetsThreshold` ignores it (`update.go:218`), so a pinned package still fails `update --check`. The struct comment says "Pinned packages won't be updated automatically", which is not what happens. Format: `pkg` (all versions) or `pkg=version`. | ✅ |
| `grype_config` | `string` | no | `""` → grype's own discovery | Path to a grype policy file; CLI flag and `NIMBOPACKS_GRYPE_CONFIG` win over it (`update.go:52`) | ✅ |

Gap: `--fail-on` (the threshold that drives the public exit-code contract) has
**no nimpack.yaml field**. It is CLI/env only. Decide whether v1 adds
`update.fail_on`.

### 1.8 `tls` — `types.TLSConfig` (`types.go:38`)

| YAML key | Go type | Req? | Default | What it actually does | Tested |
|---|---|---|---|---|---|
| `ca_cert_paths` | `[]string` | no | `nil` | PEM files, absolute or project-relative. CLI `--ca-cert` is **appended**, not overriding (`pipeline.go:39`), despite the comment saying "CLI wins over yaml". | ❌ |
| `ca_dir_path` | `string` | no | `""` | Directory of `.pem`/`.crt` files (`tlsutil.go:36`) | ❌ |
| `inject_into_image` | `*bool` | no | `true` when nil (`types.go:61`) | Adds a melange step installing the CA bundle into the image trust store (`wolfi.go:115`) | ❌ |
| `insecure` | `bool` | no | `false` | Disables TLS verification entirely (`tlsutil.go:68`). No CLI gate, no warning banner, no test. | ❌ |

---

## 2. Types in `internal/types/` that are **excluded** from the nimpack.yaml spec

Per the acceptance criteria, these are stated explicitly rather than inventoried
as schema fields. None of them are `nimpack.yaml` input.

| Type | Lines | Why excluded | Still needs a contract? |
|---|---|---|---|
| `DetectResult` (`pack`, `confidence`, `summary`, `suggested_template`, `metadata`) | 215–231 | Output of `Detect()`, consumed by the CLI and `GenerateConfig()`. Never parsed from user YAML. | Yes — it is part of the **`Pack` interface** contract. `metadata` is an untyped `map[string]string` keyed by per-pack magic strings (`entrypoint`, `module_path`, `framework`, `build_tool`, `name`). Belongs in the Pack-interface ADR, not the nimpack.yaml spec. |
| `BuildPlan`, `MelangeConfig`, `MelangePackage`, `MelangeCopyright`, `MelangeDependencies`, `MelangeEnvironment`, `MelangeContents`, `MelangePipelineStep` | 237–281 | Internal IR serialised to `melange.yaml`. Not user-authored. | Only as the Pack↔backend boundary. Note `Epoch` is hardcoded to `0` (`pack.go:110`) and has no schema field — two builds of the same version are indistinguishable at the apk level. |
| `ApkoConfig`, `ApkoContents`, `ApkoEntrypoint`, `ApkoAccounts`, `ApkoUser`, `ApkoGroup` | 285–322 | Internal IR serialised to `apko.yaml`. | Same. These are apko's wire format living in the domain package — same leak as `LayeringConfig`. |
| `PackageUpdate`, `UpdateReport` | 328–346 | Not input — but they **are** the `nimbopacks update --format json` output, which CI consumes. | **Yes, and it is unversioned.** An undeclared public contract. Recommend a short appendix in the v1 spec or its own `docs/spec/update-report-v1.md`. Flagged as a gap, not folded into nimpack.yaml. |

Also excluded: `backend.BuildOpts/BuildResult/ImageOpts/ImageResult/Capabilities`
(`pkg/backend/backend.go`) — not in `internal/types/`, covered by ADR-0001.

---

## 3. Ambiguities that must be resolved before freeze

Each is file + field + exactly what is unclear. **D** = needs a product/owner
decision; **A** = architect can resolve and record.

**Status column** added 2026-10-23, revised the same day after the NIMA-11
build-environment verification: A1–A8, A12 and A14 are **resolved** in
[ADR-0003](../adr/0003-dead-field-disposition.md) (Accepted 2026-10-23); A9, A10,
A11 and A13 move to ADR-0004; A14's implementation moves to ADR-0005 because it
changes the `update --check` exit-code contract.

| # | File:line | Field | What is unclear | Kind | Resolved in |
|---|---|---|---|---|---|
| A1 | `types.go:17`, `config.go:18` | `schema_version` | No validation exists. Is it required? What does the loader do on `"2"`, on `"1.0"`, on absent? Today: all three are accepted and ignored. | **A** | **ADR-0003 — Implement** (validate, warn-then-error ramp). Decided: Principal Architect, 2026-10-23. |
| A2 | `types.go:98` | `build.context` | Dead field. Does v1 ship it as a promise (and implement it), reserve it, or delete it? Deleting breaks any config that already sets it; implementing changes the digest of any config that already sets it. | **D** | **ADR-0003 — Delete**. Decided: CTO, 2026-10-23. Free under the leniency corollary; migration note points at `artifacts[].source`. |
| A3 | `types.go:118` | `image.labels` | Dead field, and `ApkoConfig` has no annotations support, so implementing it is a backend change, not a schema change. | **D** | **ADR-0003 — Implement**. Decided: CTO, 2026-10-23. NIMA-11 confirmed apko accepts a top-level `annotations:` key, so this is a one-field `ApkoConfig` addition. |
| A4 | `types.go:121` | `image.ports` | Dead field **that we actively generate into 22 of 23 templates**. Users will reasonably read this as `EXPOSE`. Highest-exposure item in this report. | **D** | **ADR-0003 — Reserve** as a backend-capability gap, and strip from all 23 templates, 26 samples and both packs. Decided: CTO, 2026-10-23 (stated fallback). NIMA-11 proved the pinned apko has **no** exposed-ports input key. |
| A5 | `types.go:196` | `update.auto_check` | Dead, *and* its documented default (`true`) is the opposite of its zero value (`false`). If it ever becomes live, every config that omits it flips meaning. Should be `*bool` like `inject_into_image`. | **D** | **ADR-0003 — Delete**. Decided: CTO, 2026-10-23. CI scheduling belongs in the CI config; 50 files regenerate. |
| A6 | `types.go:199` | `update.repositories` | Dead. Also Wolfi-shaped ("additional Wolfi repos") in a backend-agnostic schema. | **A** | **ADR-0003 — Delete**. Decided: Principal Architect. |
| A7 | `types.go:85` | `build.template` | Dead, and its comment promises a feature (`update` pulls template updates) that does not exist. | **A** | **ADR-0003 — Reserve** (informational provenance, normatively non-behavioural). Decided: Principal Architect. |
| A8 | `types.go:94` | `build.env` | Documented as build environment variables; implemented as a per-pack keyword channel. Either implement it as real melange env (changes digests) or rename/narrow it. Unknown keys are silently dropped — no error, no warning. | **D** | **ADR-0003 — rename to `build.options`**, narrow the docs, warn on unknown keys, keep `build.env` as a deprecated alias through v1. Decided: CTO, 2026-10-23. A separate additive `build.environment` (real melange env, required by the Blockstax conformance target) is carried **open** as A8b. |
| A9 | `types.go:115`, `pack.go:177` | `image.run_as` | `uint32` + `!= 0` guard means `run_as: 0` cannot be expressed. Is root deliberately unrepresentable (defensible) or an accident? If deliberate, the spec must say so; if not, the type must become `*uint32`. | **A** | ADR-0004 (open) |
| A10 | `types.go:159` | `image.layering.strategy` | Free string with exactly one legal value and no validation. Should be an enum. Adding the enum later rejects configs that are accepted today → breaking. | **A** | ADR-0004 (open) |
| A11 | `types.go:174` | `image.layering.budget` | No `omitempty`, so a block with only `strategy` emits `budget: 0`. Is `0` "no extra layers", "apko default", or invalid? Unspecified on our side. | **A** | ADR-0004 (open) |
| A12 | `types.go:129` vs `backend.go:67` | `image.layering` absent | The two comments disagree: `types.go` says absent ⇒ "single layer (legacy behavior)"; `backend.go` says `nil` ⇒ "apko default (auto multi-layer)". One of them is wrong and it determines the digest of every image built without a `layering:` block. **Cannot be settled without a build — Docker is unavailable.** | **A** | **Resolved — single layer.** NIMA-11 measured it directly: no `layering:` block → 1 layer (apko's own history says *"This is an apko single-layer image"*); `strategy: origin, budget: 10` → 10 layers. `types.go:129` was right; the `backend.go:67` comment is corrected in PR #29. Recorded in ADR-0003. |
| A13 | `types.go:187`, `dotnetpack.go:248` | `artifacts[].dest` | Comment says image path; dotnet treats it as a suffix of the staging dir and requires a leading `/`; go ignores it. Three behaviours, one field. | **A** | ADR-0004 (open) |
| A14 | `types.go:202`, `update.go:218` | `update.pinned` | Comment says pinned packages "won't be updated"; implementation only tags the report and still fails `--check`. Since exit codes are a public contract, this is the gap most likely to surprise a CI consumer. | **D** | **Fix the code to match the comment** — pinned packages are excluded from the `--check` threshold. Decided: CTO, 2026-10-23, overriding the architect's recommendation to document current behaviour. Because it changes a public contract it ships as **ADR-0005**, carrying two conditions from ADR-0003: findings against pinned packages still appear in every report, and `pinned` means *held at a known version*, not *waived*. |

Secondary findings worth recording but not blocking the schema freeze:

- `update.go:222` — `MeetsThreshold` with an unrecognised `--fail-on` value
  yields rank `-1`, so **every** finding clears the threshold. A typo in
  `--fail-on` turns into "fail on everything" rather than an error. Affects the
  exit-code contract. **Decided: CTO, 2026-10-23 — validate the flag and exit
  `2`.** Ships with A14 in ADR-0005, same PR.
- `pipeline.go:38` — the CLI CA cert is appended to `tls.ca_cert_paths`; the
  comment claims the CLI wins. Harmless today, wrong as documentation.
- `wolfi.go:70` — `melange keygen` generates a **fresh RSA key on every build**.
  **Measured 2026-10-23 (NIMA-11): it does perturb the digest.** Signing one
  identical package with two fresh keys, then indexing and building with a pinned
  `--build-date`, produced different layer digests and diffIDs. Mechanism: an
  RSA-PKCS1v1.5 signature's DER encoding varies in length key-to-key, that length
  lands in the `S:` field of `/lib/apk/db/installed`, and that file is inside the
  layer tarball; the per-build public key under `/etc/apk/keys/` perturbs it
  independently. **Resolved on NIMA-17, 2026-10-23:** the flag is now `false`, and
  the ephemeral key is kept on purpose — long-lived signing material on a build
  host is a worse exposure than an unstable digest. Two items follow from that and
  are **not** yet done: the normative text must define the guarantee as
  content-equivalence rather than byte-equivalence, and `CLAUDE.md:7` /
  `README.md:10` still advertise "reproducible" with no qualifier, which now
  contradicts the code.
- Nothing passes `--source-date-epoch` to melange or apko; both inherit
  `SOURCE_DATE_EPOCH` from the ambient environment. So the digest is a function
  of (source, nimpack.yaml, toolchain versions, **ambient env**). The spec must
  state that explicitly, because the breaking-change rule in §4 is phrased in
  terms of digest stability.
- `config.go:46` — `yaml.Unmarshal` without `KnownFields`, so **unknown keys and
  typos are silently ignored**. `image.enttrypoint` produces an image with no
  entrypoint and no diagnostic. Strict vs. lenient decoding is a compatibility
  decision (strict decoding makes forward-compatible additions fail on older
  binaries); recommend *warn*, never *error*. **Decided: CTO, 2026-10-23 — warn to
  stderr, never error.** Recorded in ADR-0003 A1.

---

## 4. `schema_version` policy — proposal

Designed so a reviewer can apply it mechanically from a diff, without judgement.

### 4.1 Format

`schema_version` **MUST** be a string containing a decimal integer: `"1"`,
`"2"`. Not semver — a minor component would imply a compatibility axis we do not
intend to maintain. Quoting is mandatory so YAML does not coerce it to a number.

### 4.2 The breaking-change test

A change to `internal/types/` or to any code that reads it is **breaking** if
**any one** of these is true. Each is answerable by reading the diff.

1. **Removal/rename.** A `yaml:` tag is removed or changed.
2. **Type narrowing.** A field's type changes such that a YAML node accepted
   before is rejected now (scalar→sequence, string→int, adding `KnownFields`).
3. **New obligation.** A new required field, or an existing optional field
   becomes required.
4. **Default change.** The effective value when the field is omitted changes.
5. **Meaning change.** The same input yields a different `BuildPlan`.
6. **Value-set narrowing.** A previously-accepted value becomes invalid —
   *including introducing validation on a field that had none* (e.g. making
   `layering.strategy` an enum).
7. **Digest change.** For a fixed (source tree, `nimpack.yaml`, toolchain
   versions, `SOURCE_DATE_EPOCH`), the resulting image digest changes.

**Criterion 7 needs rephrasing before §4 goes normative** (added 2026-10-23).
NIMA-17 established that the wolfi backend is **not** byte-reproducible by
design: `melange keygen` mints an ephemeral signing key per build, the
signature's DER length varies key-to-key, and that length reaches the layer
tarball through the `S:` field of `/lib/apk/db/installed`. So two builds of an
*unchanged* config already produce different digests, and a criterion phrased
against the digest can neither be satisfied nor falsified by a reviewer reading a
diff — which was the whole point of §4.2.

Replace it with a criterion against the **`BuildPlan`** (`MelangeConfig` +
`ApkoConfig`), which is deterministic, diffable in a unit test, and the thing
that actually determines image content:

> **7′. Plan change.** For a fixed (source tree, `nimpack.yaml`, toolchain
> versions), the serialised `BuildPlan` changes.

Every disposition in ADR-0003 is plan-neutral as well as digest-neutral, so this
rephrasing changes none of them. It is recorded here rather than applied, because
promoting §4 to `COMPATIBILITY.md` is its own workstream (§5 item 5) and the
criteria should be renumbered once, there.

**Rule 7 corollary — "dead field activation":** implementing a field that is
currently parsed-and-ignored (`build.context`, `image.labels`, `image.ports`,
`update.auto_check`, `update.repositories`, `build.template`) changes the digest
of every existing config that sets it. It is therefore **breaking**, and it is
breaking *silently*, with no parse error to warn the user. These six fields
**MUST** each be dispositioned before v1 freezes — implement, delete, or declare
reserved-and-ignored in the normative text — because after the freeze each of
them costs a major version to fix.

**Leniency corollary to criterion 1** (added 2026-10-23 by
[ADR-0003](../adr/0003-dead-field-disposition.md); promote into
`COMPATIBILITY.md` when §4 goes normative): because `config.go:46` decodes
without `KnownFields`, **deleting a field that no production code reads has no
observable effect on any existing config** — the key keeps parsing into nothing,
the `BuildPlan` is unchanged, and the digest is unchanged. Such a deletion is
therefore **non-breaking**, subject to two conditions: **(a)** it holds only
while decoding is lenient, so if strict decoding ever lands (itself breaking
under criterion 2) that change's migration note MUST re-enumerate every key
deleted under this corollary; and **(b)** it does not extend to any field that
production code reads, where criterion 1 applies unchanged.

A change is **non-breaking** if it is none of 1–7, which in practice means:

- A new optional field whose zero value reproduces today's behaviour exactly.
- A new member added to an existing enum.
- Relaxed validation (a previously-rejected value becomes legal).
- Documentation that describes existing behaviour without changing it.

Pointer-to-bool (`*bool`) is the required pattern for any new boolean whose
intended default is `true`; a plain `bool` cannot distinguish "false" from
"absent" and bakes in a future rule-4 violation (see A5).

### 4.3 How the version moves

- The integer increments **only** on a breaking change, and at most once per
  release regardless of how many breaking changes it carries.
- Non-breaking additions do **not** move it. Field-level availability is
  documented as "since nimbopacks vX.Y", not as a schema version.
- `schema_version` is bumped in the same PR as the breaking change, together
  with a migration note at `docs/spec/migrations/<old>-to-<new>.md`. Neither
  half merges alone.

### 4.4 Loader behaviour (to be implemented; none of this exists today)

| Config value | Behaviour |
|---|---|
| absent | Warn, assume `"1"`. Error from the release that introduces `"2"`. |
| equals the supported version | Accept. |
| a supported older version | Accept, warn once, name the migration note. |
| a newer version than this binary knows | **Error.** "This config requires nimbopacks ≥ X; you are running Y." Never best-effort. |
| unparseable (`"1.0"`, `"v1"`, `1`) | Error, name the required format. |
| unknown keys anywhere | Warn listing each key and its path. Never error (see §3, strict-decoding note). |

### 4.5 Support window

- A `schema_version` is supported for **at least 12 months after its successor
  ships, and at least one full major nimbopacks release**, whichever is longer.
- A deprecated *field* within a live version keeps working, warns on every load,
  and may only be removed at the next `schema_version` bump — never within one.
- Dropping support for a version requires an ADR and a release-note entry; the
  error message must name the migration document.

### 4.6 Mechanical PR checklist

Reviewers paste this into any PR touching `internal/types/`:

- [ ] Diff checked against all seven breaking criteria in §4.2.
- [ ] If breaking: `schema_version` bumped, migration note added, ADR filed.
- [ ] If a new boolean defaulting to `true`: it is `*bool`.
- [ ] If a new field: a fixture proves that omitting it reproduces the previous plan byte-for-byte.
- [ ] If a new enum-ish string: the legal set is in the spec and the JSON Schema.
- [ ] Digest impact stated explicitly, even when it is "none".

---

## 5. Effort estimate — full spec + JSON Schema

| Workstream | Output | Effort | Blocks on |
|---|---|---|---|
| 1. Resolve the 14 ambiguities | ADR-0002 (`schema_version` + compatibility policy), [ADR-0003](../adr/0003-dead-field-disposition.md) (**Accepted 2026-10-23** — 10 of 14 resolved), ADR-0004 (`LayeringConfig`/`ApkoConfig`/`MelangeConfig` decoupling from apko; absorbs A9–A11 and A13), ADR-0005 (`update.pinned` + `--fail-on` validation; touches the exit-code contract) | **2.0 d** | **Unblocked.** The six product calls were decided by the CTO on 2026-10-23. |
| 2. Normative text | `docs/spec/nimpack-v1.md`, RFC 2119, 31 fields, one example per section | **1.5 d** | Workstream 1 |
| 3. JSON Schema | `docs/spec/nimpack.schema.json` + a `task spec-validate` target that validates every template and every sample against it | **1.0 d** | Workstream 2 |
| 4. Conformance tests | Table-driven tests in `internal/types` and `internal/config` that pin defaults, zero-value semantics, and round-trips; targets `internal/pack.ApplyConfig`, which implements the override semantics for six fields at **0% coverage** today | **1.5 d** | Can start in parallel with 2 |
| 5. Compatibility policy doc | `docs/spec/COMPATIBILITY.md` (§4 promoted to normative) + the PR checklist into `CONTRIBUTING.md` | **0.5 d** | Workstream 1 |
| **Subtotal** | | **6.5 d** | |
| 6. Digest-stability verification (A12, melange keygen) | ~~A double-build proving rebuild determinism and settling the default-layering question~~ | **done** | **Closed by NIMA-11, 2026-10-23.** A12 answered (absent ⇒ single layer); apko capability answered for both ports and annotations; `melange keygen` **does** perturb the digest. |
| 7. ~~Fix `Reproducible: true`~~ | Capability flag corrected to `false`; ephemeral signing keys kept deliberately | **done** | **Closed by NIMA-17, 2026-10-23.** Leaves two follow-ons: the §4.2 criterion-7 rephrasing below, and reconciling the "reproducible" claim in `CLAUDE.md:7` and `README.md:10` (product call → CTO). |
| 8. A8b — `build.environment` | A real melange build-env channel (`MelangeEnvironment.Environment`), required by the Blockstax conformance target | **0.5 d** | Additive and non-breaking, but a new public key — must land before the freeze or it costs a migration note. Needs CTO acceptance. |

Sequencing: workstream 1 gates everything. Workstream 4 is the one that should
start immediately regardless — it is pure debt, it is independent of the
decisions, and it is what makes the "0.0% coverage" number stop being true.

~~Recommendation: do **not** start the normative text until ADR-0003 lands.~~
**ADR-0003 landed 2026-10-23, and workstream 7 closed with it.** Workstream 2 may
start with no carve-out. Writing the RFC 2119 prose no longer risks blessing
decoration — every field it describes now has a recorded disposition, and
the two that are inert (`build.template`, `image.ports`) are inert *normatively*,
which is a thing a spec can state honestly.

---

## 6. What this report does not cover

- `internal/remote/` — does not exist in this checkout. No fields specced.
- The CLI surface (flags, env vars) beyond where it overlaps the schema
  (`--ca-cert`, `--fail-on`, `--grype-config`). The CLI is a separate public
  contract and deserves its own document.
- The `update --check` exit codes (`0`/`1`/`2`). Already a documented contract
  in `CLAUDE.md`; §3 flags two implementation gaps against it (`pinned`,
  unrecognised `--fail-on`).
- `pkg/backend` interface guarantees — ADR-0001.

---

## 7. Verification record

Every "never read" and "does not match the implementation" claim above was
re-checked against the working tree at `3f962f6` before this report was signed
off. Reproduce with:

```bash
# The six dead fields + schema_version: no production reader.
for f in SchemaVersion Context Template Labels Ports AutoCheck Repositories; do
  grep -rn "\.$f\b" --include='*.go' . | grep -v _test.go | grep -v internal/types/types.go
done
# image.ports: written by two packs, read by none.
grep -rn 'Image\.Ports' --include='*.go' .
# image.cmd: no reader in the go pack.
grep -rn 'Image\.Cmd' --include='*.go' pkg/packs/golang/
# 22 of 23 shipped templates emit image.ports.
ls pkg/templates/templates/*/*.yaml | wc -l
grep -l 'ports:' pkg/templates/templates/*/*.yaml | wc -l
```

Caveats on the grep method: `grep '\.Context\b'` also matches `context.Context`
in every function signature, and `grep '\.Repositories\b'` also matches
`ApkoContents.Repositories` (`wolfi.go:143`), which is a different struct. Both
were filtered by hand. After filtering, `BuildConfig.Context` and
`UpdateConfig.Repositories` have **zero** readers.

Confirmed directly in source, not inferred:

- `internal/config/config.go:18` — `Validate` enforces exactly three fields:
  `project.name`, `project.version`, `build.pack`. `schema_version` is not among them.
- `internal/config/config.go:46` — `yaml.Unmarshal` with no `KnownFields`, so
  unknown keys are silently dropped.
- `internal/pack/pack.go:177` — `if cfg.Image.RunAs != 0`, which is what makes
  `run_as: 0` unrepresentable.
- `pkg/packs/golang/golangpack.go:149` — the go pack builds its apko config from
  `Entrypoint`, `Packages`, and `Env` only; `cfg.Image.Cmd` is never read.
- `internal/update/update.go:180` — `severityRank` returns `-1` from its
  `default` branch, and `MeetsThreshold` (`:217`) compares `>= threshold`, so an
  unrecognised `--fail-on` matches every finding. `MeetsThreshold` does not
  consult `PackageUpdate.Pinned` at all; `pinned` only reaches the report
  (`update.go:80`) and the text formatter (`cmd/nimbopacks/main.go:633`).

Coverage measured this run with `go test -cover ./...`:

| Package | Coverage |
|---|---|
| `internal/types` | **0.0%** |
| `internal/pack` | **0.0%** — this is where `ApplyConfig` lives, the function that implements override semantics for six schema fields |
| `internal/config` | 45.0% |
| `internal/pipeline` | 34.0% |
| `pkg/backend/wolfi` | 16.2% |

The pack packages are well covered (86–100%), which is why seven fields reach ✅
and seven reach ⚠️: the assertions exist, but they live in pack tests and test
the pack's behaviour, not the schema's contract. Nothing pins a default or a
zero-value meaning. That is the gap workstream 4 in §5 closes.
