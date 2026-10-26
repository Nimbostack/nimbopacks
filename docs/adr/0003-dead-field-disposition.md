# 0003. Disposition the seven parsed-and-ignored `nimpack.yaml` fields before the v1 freeze

- **Status:** Accepted — 2026-10-23
- **Date:** 2026-10-23 (revised same day after the NIMA-11 build-environment verification)
- **Deciders:** Principal Architect (A1, A6, A7); CTO (A2, A3, A4, A5, and the two adjacent calls A8, A14)
- **Affects:** `internal/types/types.go`, `internal/config/config.go`, `internal/pack/pack.go`,
  `pkg/packs/dotnet/dotnetpack.go`, `pkg/packs/webserver/webserverpack.go`,
  `pkg/templates/templates/**` (23 files), `samples/**/nimpack.yaml` (27 files),
  `docs/spec/nimpack-v1.md` (unwritten), `docs/spec/nimpack.schema.json` (unwritten)
- **Informed by:** NIMA-11 build-environment verification (apko v1.4.12, melange v0.63.1)
- **Supersedes nothing. Gates:** the normative v1 spec text (workstream 2 of `docs/spec/SCOPE.md` §5)

---

## Context

`docs/spec/SCOPE.md` inventoried the 31 leaf keys of the `nimpack.yaml` surface against
the code that reads them. Six keys are parsed by the loader and read by **no production
code at all**, and `schema_version` is a seventh — written by `pack.BaseConfig`
(`internal/pack/pack.go:216`) and by all 23 shipped templates, validated nowhere.

Exposure, re-verified against the working tree at `3f962f6`:

| Field | `types.go` | Templates writing it | Samples writing it | Also written by |
|---|---|---|---|---|
| `image.ports` | `:121` | **22 of 23** | **26 of 27** | dotnet pack `:188`, webserver pack `:133` |
| `build.template` | `:85` | **23 of 23** | 27 of 27 | `pack.BaseConfig:223` |
| `update.auto_check` | `:196` | **23 of 23** (all `true`) | **27 of 27** (all `true`) | `pack.BaseConfig:229` |
| `schema_version` | `:17` | **23 of 23** (all `"1"`) | 27 of 27 | `pack.BaseConfig:216` |
| `build.context` | `:98` | 0 | 0 | — |
| `image.labels` | `:118` | 0 | 0 | — |
| `update.repositories` | `:199` | 0 | 0 | — |

Two facts about the loader drive everything below.

**Fact 1 — decoding is lenient.** `internal/config/config.go:46` calls `yaml.Unmarshal`
with no `KnownFields`. Unknown keys are silently discarded. There is no warning and no
error.

**Fact 2 — `ApplyConfig` never touches these fields.** `internal/pack/pack.go:156-180`
copies project metadata, build dependencies, `image.entrypoint`, `image.packages`,
`image.run_as` and `image.env` onto the plan. `ports`, `labels`, `context`, `template`,
`auto_check` and `repositories` are not read there or anywhere else outside
`internal/types/types.go` and tests.

### Why this gates the spec

`docs/spec/SCOPE.md` §4.2 criterion 7 makes a change breaking if, for a fixed source
tree, `nimpack.yaml`, toolchain version and `SOURCE_DATE_EPOCH`, the resulting image
digest changes. Implementing any currently-dead field does exactly that for every config
that already sets it — and it does it *silently*, because Fact 1 means the user never saw
a diagnostic telling them the key was inert.

So the pre-freeze window is the **only** time any of these can be implemented for free.
Once `schema_version: "1"` is normative, each one costs a major version. That asymmetry,
not the merits of any individual field, is why this ADR has to land before the normative
text is written.

### The leniency corollary (proposed amendment to SCOPE §4.2)

Fact 1 has a consequence the §4.2 criteria do not currently capture:

> **Deleting a field that no production code reads has no observable effect on any
> existing config.** The key keeps parsing (into nothing), the `BuildPlan` is unchanged,
> and the digest is unchanged.

Criterion 1 ("a `yaml:` tag is removed or changed ⇒ breaking") is a proxy for observable
harm. For a dead field under lenient decoding the harm is zero, so **deletion of a dead
field is classified non-breaking in v1**, subject to two conditions that MUST be recorded
alongside the policy:

- **(a)** It holds only while decoding is lenient. If strict decoding ever lands — itself
  breaking under criterion 2 — every key deleted under this corollary becomes a hard
  parse error, and that change's migration note MUST re-enumerate them.
- **(b)** It does not extend to any field that production code reads. For those,
  criterion 1 applies unchanged.

This corollary is what makes "delete" cheap and "implement" expensive, and it is the
single biggest input to the five deletions/implementations below.

---

## Decision

One disposition per field. `Implement` = wire it to real behaviour before the freeze.
`Delete` = remove the `yaml:` tag and the Go field. `Reserve` = the JSON Schema accepts
the field and the normative text states exactly when it has no effect — either *never*
(A7, inert by definition) or *under a backend that cannot express it* (A4). A Reserved
field is never silently inert: the loader or the backend says so on stderr.

| # | Field | Disposition | Digest impact | `schema_version` consequence | Decided by |
|---|---|---|---|---|---|
| A1 | `schema_version` | **Implement** (validate, warn-then-error ramp) | **None** — validation only, no plan change | Stays `"1"`. Ramp designed so no config that loads today stops loading. | Principal Architect, 2026-10-23 |
| A3 | `image.labels` | **Implement** — maps 1:1 onto apko's `annotations:` key | **Changes** only for configs that set it — zero shipped templates, zero shipped samples | None. Schema field already exists; this is a backend addition. | CTO, 2026-10-23 |
| A4 | `image.ports` | **Reserve** — declarative intent, emitted only by backends that can express it; wolfi cannot. Strip from all 23 templates and both packs. | **None.** apko has no input path to `ExposedPorts`, so nothing can change. | None | CTO, 2026-10-23 (stated fallback; the condition fired) |
| A7 | `build.template` | **Reserve** (informational provenance, normatively non-behavioural) | **None**, now or ever — the spec forbids it from reaching the plan | None | Principal Architect, 2026-10-23 |
| A2 | `build.context` | **Delete** | **None** — no reader, no shipped writer | None under the leniency corollary | CTO, 2026-10-23 |
| A5 | `update.auto_check` | **Delete** | **None** — no reader | None under the leniency corollary. Requires regenerating 23 templates + 27 samples. | CTO, 2026-10-23 |
| A6 | `update.repositories` | **Delete** | **None** — no reader, no shipped writer | None under the leniency corollary | Principal Architect, 2026-10-23 |

**Net effect on the freeze: zero digest movement.** The verification in NIMA-11 removed
the one disposition that would have changed digests (A4), and A3 — the only remaining
`Implement` with any digest reach — is set by no shipped template and no shipped sample.
Every image nimbopacks builds today keeps its digest across this ADR. That was not true of
the draft, and it is the single most consequential thing the verification bought.

### A1 — `schema_version`: Implement, with a warn-then-error ramp

Implement `docs/spec/SCOPE.md` §4.4 loader behaviour, with one amendment: in the first
release that introduces validation, **every** malformed or unknown value warns rather
than errors. The error path activates only in the release that introduces `"2"`.

Rationale: introducing validation on a field that had none is criterion-6 breaking as
§4.2 is written. The ramp makes the first release observably non-breaking — no config
that loads today stops loading — and defers the hard failure to the `"2"` boundary, where
a version bump is happening anyway and the error message can name a migration note that
actually exists.

Concretely, for the v1 release:

- absent → warn, assume `"1"`
- `"1"` → accept silently
- anything else (`"2"`, `"1.0"`, `"v1"`, unquoted `1`) → **warn**, assume `"1"`, name the
  required format
- unknown keys anywhere → warn listing each key and its path, never error

From the release that introduces `"2"`: absent → error; a newer-than-known version →
error; unparseable → error. Unknown keys stay a warning permanently.

`schema_version` is **REQUIRED** in the normative text from v1 even though the loader only
warns on its absence. "Required by the spec, warned by the loader" is a deliberate and
stateable position; silently accepting absence forever is not.

### A3 — `image.labels`: Implement, mapped onto apko's `annotations:`

`ApkoConfig` (`types.go:285`) has no annotations field, so this is a backend change rather
than a schema change — the schema field already exists and is already the right shape.

**Verified (NIMA-11).** `annotations:` is a real top-level key in the pinned apko's
`ImageConfiguration` (apko v1.4.12). A trial build setting
`org.opencontainers.image.source` and `.description` produced **both** OCI
index/manifest-level `annotations` and image-config `config.Labels` carrying the same
pairs. So the implementation is a one-field mapping:

```go
// internal/types/types.go — ApkoConfig
Annotations map[string]string `yaml:"annotations,omitempty"`
```

copied from `cfg.Image.Labels` in `pack.ApplyConfig` (`internal/pack/pack.go:156`). No
further backend work, no post-processing of the OCI config.

Implement it because an enterprise-trustworthy 1.0 that cannot set
`org.opencontainers.image.source`, `.revision`, `.licenses` or `.created` has a real
supply-chain attribution gap. Those annotations are how a consumer traces an image back
to its source, and they are exactly what the Chainguard / distroless / Docker Hardened
Images comparison set already emit. The conformance target named on NIMA-15
(`Nimbostack/blockstax-containers`) sets **9 to 11** OCI annotations on each of its four
images; without A3, that target can never be expressed in `nimpack.yaml`, which is a
sharper argument than the general one and is why this is the one `Implement` left standing.

Normative shape: keys MUST be emitted in sorted order so the digest does not depend on Go
map iteration order — the reproducibility lens applies here directly, and a `map` is
exactly the construct that violates it if emitted naively.

Digest impact is near-zero in practice: no shipped template and no shipped sample sets
`labels`, so no existing build changes. Any config that *does* set it today is getting a
silently-ignored key, and the field going live is the fix, not the regression.

### A4 — `image.ports`: Reserve as declarative intent; strip from templates and packs

This was the highest-exposure item in the report: 22 of 23 templates and 26 of 27 samples
write it, two packs generate it, and nothing reads it. The draft of this ADR said
*Implement as OCI `config.ExposedPorts`*, conditional on apko being able to express it.

**Verified (NIMA-11): it cannot.** The full top-level `yaml:` tag inventory extracted from
the pinned apko binary (v1.4.12) contains no `ports`, `exposed-ports`, `expose` or any
equivalent. The `ExposedPorts` symbol present in the binary belongs to the *output*
`go-containerregistry` `v1.Config` struct — the OCI config apko writes — and apko has no
input path that populates it. It is always empty.

The CTO's standing condition therefore fires: *"If apko's `ImageConfiguration` has no
exposed-ports key, implementing means post-processing the OCI config — do not do that for
v1. In that case fall back to Reserve, and strip `image.ports` from all 23 templates in
the same PR."* That is the disposition.

**Why Reserve and not Delete**, given this ADR's own bias toward deletion and the CTO's
rule that a field must either do something or not exist: deleting `image.ports` would mean
removing a near-universal container concept from a deliberately backend-agnostic schema
*because the one backend we happen to ship cannot express it*. That is the hexagonal leak
this project exists to avoid, running in the opposite direction — an adapter limitation
reaching back into the domain. `CLAUDE.md`: *"No Wolfi/Chainguard lock-in leaks outside
`pkg/backend/wolfi/`."* A BuildKit or Dockerfile-shaped second backend emits `EXPOSE` in
one line; a schema that cannot say "this service listens on 8080" would be strictly less
implementable than a Dockerfile, and `nimpack.yaml` is meant to be an implementable spec,
not this tool's config file.

So the normative text is conditional, not a no-op:

> `image.ports` declares the ports the containerised workload listens on. An
> implementation MUST emit them as exposed-port metadata **where the target image format
> and build backend support it**, and MUST otherwise ignore them. Declared ports MUST NOT
> affect any other part of the build. The `wolfi` backend in v1 does not support
> exposed-port metadata and MUST warn on stderr, once per build, that it is ignoring a
> declared `image.ports`.

The warning rides the same stderr diagnostic channel A1 introduces for unknown keys; it
is not a new mechanism, and it is what keeps this from being a silent no-op.

This is the "say plainly that it documents intent and produces no image metadata" the CTO
asked for after the Blockstax review, with the backend-capability clause added so the
field is not frozen as permanently inert.

**Strip it from the generators anyway.** 22 templates, 26 samples, `dotnetpack.go:188` and
`webserverpack.go:133` stop writing it. Nimbopacks must not generate, into a user's config,
a key that the backend it is about to invoke will ignore — that is the "we wrote it, so
users believe it" problem the CTO named, and the fix is to stop writing it rather than to
stop having it. A user who writes `ports:` by hand has made a choice; a user who finds it
pre-filled has been told something false. Digest impact of the strip: **none** — no reader,
so no plan changes.

One discipline this buys: `image.ports` is now the worked example for the first
**backend-capability gap** in the spec. The v1 text needs a general mechanism for "field X
is unsupported by backend Y" regardless, because a second backend will produce more of
them in the other direction, and it is better to design that mechanism against a field
that costs nothing than against one that does.

### A7 — `build.template`: Reserve as informational provenance

Keep the field. Delete the comment at `types.go:84` that promises
`nimbopacks update` pulls template updates — no such code exists and none is planned.

Normative text: `build.template` records the template a config was generated from. It is
**informational**. Implementations MUST NOT let its value affect the `BuildPlan`, the
melange config, the apko config, or the image digest.

Reserve is right here and nowhere else in this ADR, because this is the one dead field
with a real non-behavioural purpose: it is a provenance trail, written by `BaseConfig` and
all 23 templates, and it is useful to a human reading a generated config even with no code
behind it. Deleting it would mean regenerating 23 templates to remove information that is
true and worth keeping. Implementing the promised update-pulling feature is a whole
product nobody has asked for.

Writing "MUST NOT affect the build" into the spec also converts the field from an
*undecided* no-op into a *guaranteed* one, which removes it from the criterion-7 trap
permanently: there is no future release in which activating it is tempting.

### A2 — `build.context`: Delete

Zero readers, zero shipped writers. The comment at `types.go:96` promises a monorepo
subdirectory build that does not work.

Delete rather than implement, because `artifacts[].source` is **already** the monorepo
mechanism (honoured by the go and dotnet packs) and two overlapping ways to say "build
from this subdirectory" is a type-design defect — a config setting both would have no
defined precedence. Per the type-design lens, prefer one representable way to express a
thing over two that can disagree.

Migration note (one line): *`build.context` was never implemented. Use `artifacts[].source`
to build from a subdirectory.* Under the leniency corollary no existing config breaks;
nobody needs to act.

### A5 — `update.auto_check`: Delete

The nastiest of the seven. Documented default is `true` (`types.go:195`); the Go zero value
is `false`; the tag has no `omitempty`, so it is serialised into every generated config;
and all 23 templates plus all 27 samples write `true` explicitly. If it ever went live,
every hand-written config that omitted it would silently flip to "off".

Delete it, because what it claims to control is not nimbopacks' business. "Runs
`nimbopacks update --check` in CI" is a property of the CI workflow, not of the image. The
`--check` flag already expresses it, at the layer that can act on it. A field in the image
config that tries to control whether a pipeline invokes a command is a layering violation:
`nimpack.yaml` describes the artifact, not the process that builds it.

Keeping it would require changing it to `*bool` to avoid the criterion-4 default flip —
a type change on a field that does nothing — and would then commit us to implementing
pipeline control in the image schema.

Cost, stated plainly: this is the only deletion with real mechanical work. 23 templates and
27 samples must drop the key, and `pack.BaseConfig:229` must stop emitting it. All 50 files
set the same value, so the edit is mechanical and reviewable as a single diff.

Migration note: *`update.auto_check` was never implemented. Invoke `nimbopacks update --check`
from your CI workflow instead.*

### A6 — `update.repositories`: Delete

Zero readers, zero shipped writers, and the comment at `types.go:198` says "additional
**Wolfi** repos". That is a backend name in `internal/types`, which is the domain package —
the hexagonal lens says the core must not know which adapter is loaded, and `CLAUDE.md`
states it directly: *"No Wolfi/Chainguard lock-in leaks outside `pkg/backend/wolfi/`."*

Delete it. If per-backend repository configuration is needed later, the extension point is
a backend-namespaced block — `backend: { wolfi: { repositories: [...] } }` — resolved by
the backend adapter, not a top-level key in the backend-agnostic schema. That shape also
survives a second backend, which the current one does not: a BuildKit backend has no
concept of an apk repository, and `update.repositories` would be dead weight in its
configs forever.

This is recorded here rather than in ADR-0004 (`LayeringConfig` decoupling) because the
field is dead and deleting it is free; ADR-0004 deals with the leaks that are load-bearing.

---

## Adjacent product calls carried for the same sign-off

These two are **not** dead fields — both are read by production code — so they are outside
this ADR's normative scope. They are the other two of the six calls `docs/spec/SCOPE.md` §3
marks **D**, and batching them into one sign-off round avoids a second one.

**A8 — `build.env` (`types.go:94`). Decided: CTO, 2026-10-23 — rename to `build.options`.**
Documented as build environment variables; implemented as a per-pack keyword channel
(`ENTRYPOINT` for go, `JAR_DIR` for java, `OUTPUT_DIR`/`NGINX_CONF_PATH` for webserver).
Unknown keys are silently discarded. The decision: rename the field to `build.options`,
narrow the documentation to describe the keyword channel it actually is, warn on
unrecognised keys, and keep `build.env` as a **deprecated alias** accepted throughout v1.
Digest impact: **none**, provided the recognised keyword set is unchanged.

The rename is a type-design fix, not cosmetics: the current name is the reason nobody
noticed that `MelangeEnvironment` (`types.go:267`) has only `Contents` and therefore has
nowhere to put an environment variable even if we wanted to.

*Carried open — A8b.* The NIMA-15 Blockstax review found the conformance target needs
**real** build-time environment variables (`GOMODCACHE: /var/cache/melange`), which
`build.options` by construction will not provide. The CTO's A8 ruling explicitly says not
to turn `build.env` into real melange env; that ruling and the Blockstax requirement are
compatible, because the answer is a *new* field rather than a reinterpreted one:

- add `Environment map[string]string` to `types.MelangeEnvironment`, mirroring melange's
  own `environment.environment` block;
- surface it as a new `build.environment` key with no default, so an absent key emits
  nothing and no existing digest moves;
- keep it strictly disjoint from `build.options`, so the two can never disagree.

This is additive and non-breaking (§4.2: a new optional key whose zero value preserves
current behaviour), but it is a new public schema field, so it needs its own issue and an
ADR note before the freeze. It is **not** decided here.

**A14 — `update.pinned` (`types.go:201`, `update.go:218`). Decided: CTO, 2026-10-23 — fix
the code to match the comment.** The comment says pinned packages "won't be updated
automatically"; `MeetsThreshold` ignores `Pinned` entirely, so a pinned package still
fails `update --check`. The CTO's ruling: pinned packages MUST be excluded from the
`--check` threshold — today's documentation is false, and that makes it a bug rather than
a schema question.

*Architect's concern, recorded once and not re-litigated:* this makes nimbopacks itself
suppress a finding from an exit code, and the standing invariant is *"Nimbopacks
implements no CVE suppression of its own — defer to grype"* (`CLAUDE.md`). It also changes
`update --check` exit codes for existing configs that set `pinned`, and those exit codes
are a public contract (`0` clean, `1` CVEs at or above threshold, `2` tool/config error).
Two conditions keep the decision inside the invariant, and the implementing ADR MUST carry
both:

1. **Report, don't hide.** A finding against a pinned package MUST still appear in every
   output format, flagged as pinned. Only its contribution to the exit code changes.
   Suppression removes evidence; this removes a gate. They are different acts and the spec
   must say which one `pinned` is.
2. **Name the semantics.** `pinned` is an *operator acknowledgement that this package is
   held at a known version*, not a vulnerability waiver. `.grype.yaml` remains the only
   place a CVE is waived.

Because it modifies a public contract, A14 ships as **ADR-0005** with its own migration
note, per this ADR's own rule, not as a line item here.

**Two further calls made by the CTO in the same ruling**, recorded so they are not lost:

- **`--fail-on` typo must be an error.** `severityRank` returning `-1` (`update.go:180`, used by `MeetsThreshold` at `:222`)
  means a misspelled threshold clears on *everything* — silent and backwards. Validate the
  flag and exit `2`. Same contract, same PR as A14/ADR-0005.
- **Unknown keys warn, never error.** Confirmed as decided; it is already the v1 behaviour
  specified in A1 above, and the rejected alternative (strict `KnownFields` decoding) is
  recorded below.

---

## Consequences

**Good**

- The v1 spec can be written. Seven fields stop being undecided, and the normative text
  will not bless decoration.
- **No image digest moves.** Every disposition is digest-neutral for every config that
  exists today. The freeze can be taken without a re-pin, which is the outcome the draft
  of this ADR could not promise.
- Three Wolfi-, monorepo- and pipeline-shaped keys leave the backend-agnostic schema,
  which moves `internal/types` measurably closer to passing the second-backend test.
- The leniency corollary gives reviewers a mechanical rule for the next dead field, which
  there will be.
- `image.labels` closes a real supply-chain attribution gap rather than papering over it,
  and the NIMA-11 measurement reduced it from "a backend change" to a four-line one.
- `image.ports` forces the spec to grow an explicit **backend-capability gap** concept in
  v1, against a field where getting it wrong costs nothing. Without A4 that concept would
  have arrived for the first time alongside a second backend, under pressure.

**Bad**

- Deleting `update.auto_check` touches 50 files, and stripping `image.ports` touches 22
  templates, 26 samples and two packs. Both are mechanical, but together they are a very
  wide diff that will conflict with anything else editing templates or samples. Land them
  as two separate PRs, auto_check first.
- **Two fields now ship as Reserved** (`build.template`, `image.ports`) against the CTO's
  stated rule that a field must either do something or not exist. `build.template` is
  normatively inert forever; `image.ports` is inert under the only backend we ship. Both
  are defended above, but a reader of the v1 spec will encounter two keys that do nothing
  when they run the tool, and "the spec is backend-agnostic" is a thin answer to someone
  who only has one backend.
- The leniency corollary buys cheap deletions by leaning on a loader behaviour we have
  separately decided to change (warn on unknown keys). The warnings will name keys we
  deliberately removed, and users will reasonably ask why. The migration notes must answer
  that.
- A14's resolution puts nimbopacks in the business of deciding which findings gate an exit
  code. The two conditions above keep it inside the "CVE policy is grype's" invariant, but
  the invariant is now qualified rather than absolute, and future requests to extend
  `pinned` into real suppression will cite this ADR.
- A8b (`build.environment`) is identified but undecided, so the schema is not actually
  complete at the end of this ADR. One known gap remains, named.
- **The reproducibility claim turned out to be false**, and the fix was to withdraw the
  claim rather than earn it (NIMA-17). Not caused by this ADR, but it lands in the same
  window, and it means §4.2 criterion 7 — which defines "breaking" in terms of digest
  stability — rests on a property the tool does not have. The criterion needs rephrasing
  against the `BuildPlan` instead. Every disposition here is plan-neutral as well as
  digest-neutral, so nothing above has to change, but the policy text does.

**Neutral**

- `schema_version` stays `"1"`. Nothing here moves it — that is the whole point of doing it
  before the freeze.
- Template and sample regeneration is required either way; the dispositions only change
  which keys the regeneration touches.
- `internal/types` loses three fields (`build.context`, `update.auto_check`,
  `update.repositories`), renames one (`build.env` → `build.options`, alias retained), and
  gains one (`ApkoConfig.Annotations`, not part of the nimpack.yaml surface). The leaf-key
  count in `docs/spec/SCOPE.md` §0 drops from 31 to 28.

---

## Alternatives considered

**Reserve all seven.** Cheapest: no code changes, no template edits, no digest movement,
and the spec ships on time. Rejected because it ships a documented no-op that nimbopacks
*itself writes* into the user's config — which is worse than being undecided, because it
is then deliberate. It also does not escape the trap, it defers it: every reserved field
is still criterion-7 breaking to activate later, so this alternative converts a
one-release decision into a permanent one. The two fields that do end up Reserved are
Reserved for reasons that do not generalise: one is provenance, one is a backend-capability
gap, and both are stripped from the generators so we stop asserting them on the user's
behalf.

**Delete all seven.** Maximally clean: the schema contains only fields with behaviour, and
under the leniency corollary nothing breaks. Rejected for `image.labels`, which is an
attribution requirement for the supply-chain posture this project explicitly targets and
which the conformance target uses 9–11 times per image. Also rejected for
`build.template`, which is useful provenance.

**Delete `image.ports`, since apko cannot express it.** The strongest remaining
alternative, and the one the draft of this ADR named as its own fallback — the CTO's rule
is that a field must either do something or not exist, and after NIMA-11 we know
`image.ports` can do nothing under the shipped backend. Rejected because the premise is
backwards: `nimpack.yaml` is specified to be backend-agnostic and implementable by others
(`CLAUDE.md` design goals), so the capability of one adapter is not a reason to amputate
the domain. Exposed ports are expressible in OCI, in Dockerfiles, in BuildKit and in
Compose; a schema that cannot state them is strictly less expressive than a Dockerfile,
and re-adding the key at v2 would then be a schema addition with a migration note rather
than a free one. The real defect was that we *generated* the key, and the strip fixes that
without the amputation. This is the one place in this ADR where the hexagonal lens argues
*for* keeping an inert field.

**Post-process the OCI config to implement `image.ports` anyway.** Technically possible:
rewrite `config.ExposedPorts` in the image config after apko writes it. Rejected — the CTO
ruled it out for v1 and the architecture agrees. It means nimbopacks mutating an artifact
apko produced, after apko computed its SBOM over it, which puts the SBOM and the image out
of agreement and breaks the attestation chain for a convenience field. It would also be
the first piece of image-assembly logic living outside the backend, directly against
"apko-native layering / no custom layering logic" and the spirit behind it.

**Implement all seven.** Rejected. `build.context` would create a second monorepo mechanism
that can disagree with `artifacts[].source`. `update.auto_check` would put CI-invocation
policy inside the image config. `update.repositories` would hard-code a Wolfi concept into
the domain package, breaking the `CLAUDE.md` no-lock-in invariant. Each one is a worse
schema than the one we have, not a better one.

**Defer the whole question until after the v1 freeze and handle it in v2.** Rejected on
arithmetic. Deferring converts five free changes into five that each need a
`schema_version` bump, a migration note and an ADR, and it means v1 ships a normative
document that cannot honestly describe what seven of its own fields do.

**Add strict decoding (`KnownFields`) now so dead keys become loud.** Rejected for v1. It is
criterion-2 breaking, it makes forward-compatible additions fail on older binaries — a
config written for v1.3 would hard-fail on a v1.0 binary instead of degrading — and it
would immediately reject the three keys this ADR deletes. The warn-on-unknown-keys path in
A1 gets most of the benefit with none of the breakage. Revisit at the `"2"` boundary.

**Split this into seven ADRs, one per field.** Rejected. The decisions share a single
forcing argument (the pre-freeze window) and a single enabling fact (lenient decoding);
split apart, each one reads as an arbitrary preference and the reviewer cannot see that
the deletions are what pay for the implementations. `update.pinned` *is* split out (above)
precisely because it does not share that argument — it touches the exit-code contract
instead.

---

## Verification

How we will know this decision is holding:

1. **Dead-field regression test.** A test in `internal/config` that loads a fixture setting
   every deleted key (`build.context`, `update.auto_check`, `update.repositories`) and
   asserts the load succeeds and the resulting `BuildPlan` is byte-identical to the same
   fixture without them. This pins the leniency corollary so a future strict-decoding change
   fails loudly here first.
2. **`build.template` non-behaviour test.** Two configs differing only in `build.template`
   MUST produce identical `MelangeConfig` and `ApkoConfig`. This is the enforcement of
   "informational".
3. **`schema_version` table test.** One case per row of the A1 ramp table: absent, `"1"`,
   `"2"`, `"1.0"`, `"v1"`, unquoted `1` — asserting warn-vs-error per release phase.
4. **Template/sample conformance.** The `task spec-validate` target planned in
   `docs/spec/SCOPE.md` §5 workstream 3 validates all 23 templates and all 27 samples
   against `nimpack.schema.json`. A template still emitting a deleted key fails CI. Extend
   it with one extra assertion for A4: **no template, sample or pack may emit
   `image.ports`** while the shipped backend cannot express it. Reserve only stays honest
   if the generators stop writing the field, and that is the thing most likely to silently
   regress when the next template is contributed.
5. **`image.ports` ignored-with-warning test.** Two configs differing only in
   `image.ports` MUST produce identical `ApkoConfig`, and the one that sets it MUST emit
   the backend-capability warning. This is what distinguishes A4's Reserve from the
   undocumented no-op it replaces.
6. **`image.labels` determinism test.** The same `labels` map, built twice, MUST produce
   byte-identical apko `annotations:` output. A Go `map` round-tripped naively does not
   guarantee this, and it is the only new map-to-output path this ADR introduces — the
   reproducibility lens applied to its own decision.
7. **Review gate.** The §4.6 mechanical PR checklist already requires "digest impact stated
   explicitly, even when it is none". Add one line to it: *if the PR gives behaviour to a
   previously-inert field, it cites this ADR or supersedes it.*
8. **Grep guard.** The §7 verification loop in `docs/spec/SCOPE.md` re-run in CI: no field
   in `internal/types` may be written by `pack.BaseConfig` or a template while having zero
   production readers. This is what would have caught all seven a year ago. `image.ports`
   is the deliberate exception and must be allow-listed by name, with this ADR cited — an
   exception that is not written down is how the next dead field gets in.

Items 1–3 are new tests in `internal/config` and `internal/types`, both at **0.0%** and
**45.0%** coverage today, so this ADR also retires part of the coverage debt
`docs/spec/SCOPE.md` §5 workstream 4 identifies.

---

## Build-environment verification — results (NIMA-11, closed 2026-10-23)

The draft of this ADR handed four measurements to whoever had a build environment, because
none of them were answerable from a checkout without Docker. Three were answered anyway:
`apko build` and `melange keygen`/`sign`/`index` do not need a container runtime, only
`melange build` does. All four rows are now closed.

| Question | Result | Effect on this ADR |
|---|---|---|
| **A12** — absent `image.layering`: single layer, or apko-default multi-layer? | **Single layer.** A no-`layering:` build produced 1 layer, with apko's own history entry reading *"This is an apko single-layer image"*; `layering: {strategy: origin, budget: 10}` produced 10, grouped by origin. `types.go:129` was right, `pkg/backend/backend.go:67` was wrong; the comment is corrected in PR #29. | **Closed.** Normative consequence: an absent `image.layering` block MUST mean a single-layer image. ADR-0004 records the `LayeringConfig` decoupling, not this question. |
| **apko capability: exposed ports** | **No.** No `ports`/`exposed-ports`/`expose` key anywhere in the pinned apko's top-level `ImageConfiguration` tag set (v1.4.12). The `ExposedPorts` symbol in the binary belongs to the output `v1.Config`, which apko never populates from input. | **A4 takes its stated fallback.** See A4 above. |
| **apko capability: annotations** | **Yes**, key is `annotations:`. A trial build emitted both manifest-level OCI `annotations` and image-config `config.Labels`. | **A3 confirmed as Implement**, and reduced to a one-field mapping. |
| **melange keygen digest perturbation** (`wolfi.go:70` generates a fresh RSA key per build) | **Yes, it perturbs the digest.** Signing one identical package with two fresh keys, then indexing and building with a pinned `--build-date`, produced different layer digests and diffIDs. Mechanism: the DER encoding of an RSA-PKCS1v1.5 signature varies in length key-to-key, the `.apk` size lands in the `S:` field of `/lib/apk/db/installed`, and that file is inside the layer tarball. The per-build public key under `/etc/apk/keys/` perturbs it independently. | **`Reproducible: true` at `backend.go:151` was false.** Resolved outside this ADR on NIMA-17: the flag is now `false`, and ephemeral per-build signing keys are **kept deliberately**. |

Two consequences leave this ADR rather than sitting in it:

- **Reproducibility (NIMA-17, closed 2026-10-23).** The wolfi backend advertised a
  capability it does not have. The resolution chose the security property over the
  marketing one: `Reproducible` is now `false`, and melange keeps generating an ephemeral
  signing key per build rather than reusing a pinned one, because long-lived signing
  material on a build host is a worse exposure than a non-reproducible digest. I agree
  with that trade. Two consequences the v1 spec must carry:

  - The normative text defines nimbopacks' guarantee as **content-equivalence, not
    byte-equivalence** — same source plus same `nimpack.yaml` plus same toolchain gives an
    image with the same package set, SBOM and configuration, but not the same digest. It
    must say so in the same breath as `SOURCE_DATE_EPOCH`, or readers will assume the
    stronger claim.
  - **`CLAUDE.md:7` and `README.md:10` both say "reproducible" with no qualifier, and the
    code now says otherwise.** That is a contract-first defect: the top-line product claim
    contradicts the capability flag. It is a product call, not an architect call — raised
    separately.

  This also softens §4.2 criterion 7, which defines a breaking change in terms of digest
  stability. If digests are not stable build-to-build anyway, the criterion needs
  rephrasing in terms of the *plan* — `MelangeConfig` + `ApkoConfig` — rather than the
  digest. ADR-0004 or the compatibility-policy doc owns that; it does not change any
  disposition here, because every disposition in this ADR is plan-neutral too.
- **Environment inputs.** NIMA-11 also confirms the §3 secondary finding that nothing
  passes `--source-date-epoch` through to melange or apko — both inherit it ambiently. The
  v1 reproducibility clause must enumerate the ambient environment as an input, or the
  tool must pass it explicitly. Same issue.

### Still open after this ADR

| Item | Issue | Owner | Next step |
|---|---|---|---|
| **Product claim vs. capability flag** — `CLAUDE.md:7` and `README.md:10` advertise "reproducible" images; `backend.go` now reports `Reproducible: false`. | NIMA-22 | CTO | Decide what the headline claim becomes, then reconcile both files and the v1 spec's reproducibility clause in one change. |
| **A8b** — a real `build.environment` channel mapping to melange's `environment.environment`, required by the Blockstax conformance target (`GOMODCACHE`). Additive and non-breaking, but a new public schema field. | NIMA-21 | CTO to accept; Principal Architect to spec | Own issue + a short ADR note before the freeze. Must land before `schema_version: "1"` is normative or it needs a migration note. |
| **A9, A10, A11, A13** — `image.run_as` zero value, `layering.strategy` enum, `layering.budget` zero value, `artifacts[].dest` three behaviours. | NIMA-18 | Principal Architect | **ADR-0004**, together with the `LayeringConfig` / `ApkoConfig` / `MelangeConfig`-in-`internal/types` decoupling the CTO asked be raised separately. |
| **A14 implementation** — exclude pinned packages from the `--check` threshold, and make a `--fail-on` typo exit `2`. | NIMA-19 | Engineering | **ADR-0005** plus the code change, carrying the two conditions stated above. |

---

## Sign-off record

| # | Field | Disposition | Decided by | Date |
|---|---|---|---|---|
| A1 | `schema_version` | Implement — warn-then-error ramp | Principal Architect | 2026-10-23 |
| A2 | `build.context` | Delete | CTO | 2026-10-23 |
| A3 | `image.labels` | Implement — apko `annotations:` | CTO | 2026-10-23 |
| A4 | `image.ports` | Reserve — backend-capability gap; strip from generators | CTO (stated fallback, condition verified by NIMA-11) | 2026-10-23 |
| A5 | `update.auto_check` | Delete | CTO | 2026-10-23 |
| A6 | `update.repositories` | Delete | Principal Architect | 2026-10-23 |
| A7 | `build.template` | Reserve — informational | Principal Architect | 2026-10-23 |
| A8 | `build.env` (adjacent) | Rename to `build.options`; alias retained through v1; warn on unknown keys | CTO | 2026-10-23 |
| A8b | `build.environment` (new) | **Undecided** — carried to its own issue | — | — |
| A14 | `update.pinned` (adjacent) | Fix the code: exclude pinned from the `--check` threshold | CTO | 2026-10-23 |

A2, A3, A4, A5, A8 and A14 were decided by the CTO on the record in the NIMA-9 thread on
2026-10-23, as the six product calls this ADR was explicitly told not to make alone. A4's
entry is a conditional decision whose condition was resolved by NIMA-11 on the same day:
the CTO pre-authorised the fallback, the verification showed apko cannot express exposed
ports, and the fallback is what is recorded. The item-verdict interaction raised on NIMA-9
(`66ec95dd-1a28-420d-8755-8acd6255c488`) was superseded by that comment and withdrawn.

The code that implements these dispositions is **NIMA-20**, to be landed as four separate
PRs (deletions, the `image.ports` strip, `image.labels`, the `build.env` rename) before
`schema_version: "1"` goes normative.

Every row of the decision table now carries a name and a date, so the ADR is **Accepted**
and workstream 2 of `docs/spec/SCOPE.md` §5 — the normative v1 text — is **fully
unblocked**. The reproducibility clause, the last carve-out, was settled by NIMA-17 on the
same day: it is now a clause with a known answer (content-equivalent, not byte-identical)
rather than an open question.

This table is the record required by the review bar: a decision that shapes future work
belongs in an ADR, not only in a comment thread. From here the ADR is immutable — the
single undecided row (A8b) is resolved by a *new* ADR, not by editing this one.
