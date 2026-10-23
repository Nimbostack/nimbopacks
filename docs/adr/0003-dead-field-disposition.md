# 0003. Disposition the seven parsed-and-ignored `nimpack.yaml` fields before the v1 freeze

- **Status:** Proposed — architect calls decided, product calls pending sign-off
- **Date:** 2026-10-23
- **Deciders:** Principal Architect (A1, A6, A7); CTO / Board (A2, A3, A4, A5, and the two adjacent calls A8, A14)
- **Affects:** `internal/types/types.go`, `internal/config/config.go`, `internal/pack/pack.go`,
  `pkg/templates/templates/**` (23 files), `samples/**/nimpack.yaml` (27 files),
  `docs/spec/nimpack-v1.md` (unwritten), `docs/spec/nimpack.schema.json` (unwritten)
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
`Delete` = remove the `yaml:` tag and the Go field. `Reserve` = normative text says the
field is accepted and has no effect on the build; the JSON Schema accepts it.

| # | Field | Disposition | Digest impact | `schema_version` consequence | Decided by |
|---|---|---|---|---|---|
| A1 | `schema_version` | **Implement** (validate, warn-then-error ramp) | **None** — validation only, no plan change | Stays `"1"`. Ramp designed so no config that loads today stops loading. | Principal Architect — **decided** |
| A4 | `image.ports` | **Implement** as OCI `config.ExposedPorts` | **Changes** for 26 of 27 samples and any config from 22 of 23 templates | **None if merged before the freeze.** After the freeze it costs a bump. | CTO / Board — **pending** |
| A3 | `image.labels` | **Implement** as OCI annotations | **Changes** only for configs that set it — zero shipped templates, zero shipped samples | None. Schema field already exists; this is a backend addition. | CTO / Board — **pending** |
| A7 | `build.template` | **Reserve** (informational provenance, normatively non-behavioural) | **None**, now or ever — the spec forbids it from reaching the plan | None | Principal Architect — **decided** |
| A2 | `build.context` | **Delete** | **None** — no reader, no shipped writer | None under the leniency corollary | CTO / Board — **pending** |
| A5 | `update.auto_check` | **Delete** | **None** — no reader | None under the leniency corollary. Requires regenerating 23 templates + 27 samples. | CTO / Board — **pending** |
| A6 | `update.repositories` | **Delete** | **None** — no reader, no shipped writer | None under the leniency corollary | Principal Architect — **decided** |

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

### A4 — `image.ports`: Implement as OCI `config.ExposedPorts`

This is the highest-exposure item in the report: 22 of 23 templates and 26 of 27 samples
write it, two packs generate it, and nothing reads it. A user reading a nimbopacks-generated
`nimpack.yaml` will read `ports: ["8080"]` as `EXPOSE 8080`. Today it means nothing.

Implement it as OCI image-config `ExposedPorts`. Normative value format: `"<port>"` or
`"<port>/<tcp|udp>"`; bare port defaults to `tcp`; the emitted set MUST be sorted so the
digest does not depend on YAML sequence order beyond the user's own ordering.

This is the only disposition in this ADR that changes an image digest, and it changes it
for almost every image anyone has built with a template. That is precisely why it must
land **before** the freeze: the digest change is unavoidable in any future where the field
means what users already think it means, and the pre-freeze window is the only time it
does not also cost a `schema_version` bump and a migration note.

**Blocked on one verification (see Open items):** whether the pinned apko (v0.27.0 line)
can express exposed ports in its image configuration. apko is a downloaded binary, not a
Go dependency, so its schema cannot be inspected from this checkout and there is no Docker
here. If apko cannot express it, this disposition falls back to **Delete** — which costs
edits to 22 templates and 26 samples — and `Reserve` remains rejected either way.

### A3 — `image.labels`: Implement as OCI annotations

`ApkoConfig` (`types.go:285`) has no annotations field, so this is a backend change rather
than a schema change — the schema field already exists and is already the right shape.

Implement it because an enterprise-trustworthy 1.0 that cannot set
`org.opencontainers.image.source`, `.revision`, `.licenses` or `.created` has a real
supply-chain attribution gap. Those annotations are how a consumer traces an image back
to its source, and they are exactly what the Chainguard / distroless / Docker Hardened
Images comparison set already emit.

Digest impact is near-zero in practice: no shipped template and no shipped sample sets
`labels`, so no existing build changes. Any config that *does* set it today is getting a
silently-ignored key, and the field going live is the fix, not the regression.

Same apko-capability caveat as A4. The fallback if apko cannot express annotations is
**not** Delete — it is to carry the field as Reserved and open a separate issue against
the backend, because the requirement is real even if today's backend cannot meet it. That
asymmetry with A4 is deliberate: `ports` is convenience metadata, attribution is not.

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

**A8 — `build.env` (`types.go:94`).** Documented as build environment variables; implemented
as a per-pack keyword channel (`ENTRYPOINT` for go, `JAR_DIR` for java,
`OUTPUT_DIR`/`NGINX_CONF_PATH` for webserver). Unknown keys are silently discarded.
*Recommendation:* fix the documentation to describe the keyword channel, and add a warning
for unrecognised keys. Do **not** implement it as real melange environment variables in v1 —
that changes the digest of every config that sets it *and* collides with the keyword names
already in use, so there is no safe migration. A real build-env channel should be a new,
separately-named field.

**A14 — `update.pinned` (`types.go:201`, `update.go:217`).** The comment says pinned packages
"won't be updated automatically"; the implementation only tags the report and
`MeetsThreshold` ignores `Pinned` entirely, so a pinned package still fails `update --check`.
*Recommendation:* document the current behaviour (annotation only) and do **not** change
`MeetsThreshold`. Making `pinned` suppress a finding would change `update --check` exit
codes for existing configs, and exit codes are a public contract (`CLAUDE.md`: `0` clean,
`1` CVEs at or above threshold, `2` tool/config error). Suppression is grype's job via
`.grype.yaml`, per the standing "CVE policy is grype's" invariant.

If the decision on A14 is anything other than "document current behaviour", it needs its
own ADR, because it then modifies a public contract.

---

## Consequences

**Good**

- The v1 spec can be written. Seven fields stop being undecided, and the normative text
  will not bless decoration.
- The one unavoidable digest change (`image.ports`) is paid inside the pre-freeze window,
  where it costs nothing, instead of at a major version later.
- Three Wolfi-, layering- and pipeline-shaped keys leave the backend-agnostic schema,
  which moves `internal/types` measurably closer to passing the second-backend test.
- The leniency corollary gives reviewers a mechanical rule for the next dead field, which
  there will be.
- `image.labels` closes a real supply-chain attribution gap rather than papering over it.

**Bad**

- `image.ports` changes the digest of essentially every image built from a template or
  sample. Anyone pinning a digest across this release re-pins. There is no way to have the
  field mean what users think it means without paying this.
- Deleting `update.auto_check` touches 50 files. It is mechanical, but it is a wide diff to
  review and it will conflict with anything else editing templates or samples.
- Two implementations (A4, A3) depend on an apko capability nobody here can verify. One of
  them has a Delete fallback that is strictly more expensive than the primary choice, so
  the verification result can increase the cost of this ADR after it is accepted.
- The leniency corollary buys cheap deletions by leaning on a loader behaviour we have
  separately recommended changing (warn on unknown keys). The warnings will name keys we
  deliberately removed, and users will reasonably ask why. The migration notes must answer
  that.
- `build.template` ships as a documented no-op. That is a smell, and the spec will say so
  out loud.
- Four of these seven decisions (A2–A5) are not the architect's to make, so this ADR is
  `Proposed` until signed and the spec stays blocked until then.

**Neutral**

- `schema_version` stays `"1"`. Nothing here moves it — that is the whole point of doing it
  before the freeze.
- Template and sample regeneration is required either way; the dispositions only change
  which keys the regeneration touches.
- `internal/types` loses three fields and gains no new ones, so the leaf-key count in
  `docs/spec/SCOPE.md` §0 drops from 31 to 28.

---

## Alternatives considered

**Reserve all seven.** Cheapest: no code changes, no template edits, no digest movement,
and the spec ships on time. Rejected because it ships a documented no-op in 22 of 23
templates. A user who reads the spec learns that the `ports` key nimbopacks itself wrote
into their config does nothing — which is worse than it being undocumented, because now
it is deliberate. It also does not escape the trap, it defers it: every reserved field is
still criterion-7 breaking to activate later, so this alternative converts a one-release
decision into a permanent one.

**Delete all seven.** Maximally clean: the schema contains only fields with behaviour, and
under the leniency corollary nothing breaks. Rejected for `image.ports` and `image.labels`
specifically. Ports is information the user already took the trouble to write and that two
packs already infer correctly; throwing it away to avoid a one-time digest change is the
wrong trade when the window to make that change for free is open right now. Labels are an
attribution requirement for the supply-chain posture this project is explicitly targeting.
Also rejected for `build.template`, which is useful provenance.

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
   against `nimpack.schema.json`. A template still emitting a deleted key fails CI.
5. **Review gate.** The §4.6 mechanical PR checklist already requires "digest impact stated
   explicitly, even when it is none". Add one line to it: *if the PR gives behaviour to a
   previously-inert field, it cites this ADR or supersedes it.*
6. **Grep guard.** The §7 verification loop in `docs/spec/SCOPE.md` re-run in CI: no field
   in `internal/types` may be written by `pack.BaseConfig` or a template while having zero
   production readers. This is what would have caught all seven a year ago.

Items 1–3 are new tests in `internal/config` and `internal/types`, both at **0.0%** and
**45.0%** coverage today, so this ADR also retires part of the coverage debt
`docs/spec/SCOPE.md` §5 workstream 4 identifies.

---

## Open items — not decided here, handed off

| Item | Why it cannot be settled here | Owner | Unblock action |
|---|---|---|---|
| **A12** — does an absent `image.layering` block mean single layer (`types.go:129`) or apko-default multi-layer (`backend.go:67`)? The two comments disagree, and the answer is the digest of every image built without a `layering:` block. | Requires a real apko build. **No Docker in this environment.** | Whoever has a build environment | Build one sample with and without a `layering:` block; compare layer counts and digests. Record the result in ADR-0004. |
| **apko capability: exposed ports** | apko is a downloaded binary, not a Go dependency; its config schema is not inspectable from this checkout and there is no network. | Same | Inspect the pinned apko (v0.27.0 line) image-configuration schema for an exposed-ports key. Result selects A4's primary (Implement) or fallback (Delete). |
| **apko capability: annotations** | Same | Same | Same inspection for an annotations key. A negative result moves A3 to Reserve plus a backend issue; it does not move it to Delete. |
| **melange keygen digest perturbation** (`wolfi.go:70` generates a fresh RSA key per build) | Needs a double-build to see whether the signature segment reaches the final digest. | Same | Build the same sample twice with a fixed `SOURCE_DATE_EPOCH`; compare digests. If they differ, the `Reproducible: true` capability claim at `backend.go:151` is false and the spec's reproducibility clause must be rewritten. |

A1, A6, A7 are decided and need no further input. A2–A5 are recorded as **Proposed**
pending CTO / Board sign-off; this ADR moves to **Accepted** only when every row of the
decision table has a name and a date in its "Decided by" column.
