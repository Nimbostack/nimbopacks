# 0004. Separate the `nimpack.yaml` schema types from the backend wire formats, and close the four layering/artifact ambiguities

- **Status:** Accepted — 2026-10-23
- **Date:** 2026-10-23
- **Deciders:** Principal Architect (the boundary, A9, A10, A11, A13). `docs/spec/SCOPE.md` §3
  marks all four ambiguities **A** — architect resolves and records.
- **Affects:** `internal/types/types.go`, `internal/pack/pack.go`, `internal/config/config.go`,
  `internal/pipeline/pipeline.go`, `pkg/backend/backend.go`, `pkg/backend/wolfi/wolfi.go`,
  `pkg/packs/golang/golangpack.go`, `pkg/packs/dotnet/dotnetpack.go`,
  `docs/spec/nimpack-v1.md` (unwritten), `docs/spec/nimpack.schema.json` (unwritten)
- **Amends:** [ADR-0001](0001-backend-abstraction.md) §1 (the frozen `backend.Backend` baseline —
  `ImageOpts.Layering` changes meaning, not shape) and §6 (names the first instalment of the L1 fix).
  Takes the §4.2 criterion-7 rephrasing [ADR-0003](0003-dead-field-disposition.md) left open.
- **Does not supersede anything.** **Gates:** the normative v1 spec text
  (workstream 2 of `docs/spec/SCOPE.md` §5).

---

## Context

The CTO raised this in the NIMA-9 ruling of 2026-10-23: `types.LayeringConfig` is
simultaneously the public `nimpack.yaml` schema type (`types.go:133`) and apko's wire format
(`types.go:294`), and `ApkoConfig`/`MelangeConfig` live in `internal/types` at all. `CLAUDE.md`:
*"No Wolfi/Chainguard lock-in leaks outside `pkg/backend/wolfi/`."*

### The conflation, precisely

One Go struct, `types.LayeringConfig` (`types.go:154-175`), plays three roles:

| Role | Site | What it means there |
|---|---|---|
| **Public schema** | `ImageConfig.Layering` (`types.go:133`) — parsed from user YAML | A `nimpack.yaml` key the user writes and we will freeze |
| **Port argument** | `backend.ImageOpts.Layering` (`backend.go:67`) | What the orchestrator hands any backend |
| **Adapter wire format** | `ApkoConfig.Layering` (`types.go:294`), written to `apko.yaml` | Bytes apko parses |

The struct's own doc comment says it *"matches apko's native layering configuration exactly"*
(`types.go:136`). That is the defect stated in one line: our public schema is defined as
"whatever apko's is". The plumbing confirms it — `pipeline.go:103` copies
`cfg.Image.Layering` into `ImageOpts`, and `wolfi.go:180` assigns the *same pointer* into the
apko config. There is no translation step anywhere, because there are no two types to
translate between.

### What the freeze actually freezes — and what it does not

This distinction decides the scope of this ADR, so it is stated before the decision.

`schema_version: "1"` freezes **the set of types that are parsed from user YAML**. It says
nothing about types that only ever travel inside the process. `docs/spec/SCOPE.md` §2 already
excludes `MelangeConfig`, `ApkoConfig` and their satellites from the spec on exactly that
ground: they are internal IR, never user-authored.

So the pre-freeze hazard is **not** "melange and apko structs live in `internal/types`". That
is [ADR-0001](0001-backend-abstraction.md)'s **L1**, it is already accepted as known debt, and
it is movable at any time without a `schema_version` bump because no user config names those
types. Treating it as a freeze deadline would be over-claiming.

The hazard is narrower and sharper: **exactly one struct is both a frozen public schema type
and an unfrozen adapter wire format.** On the day `schema_version: "1"` goes normative, the
shape of `LayeringConfig` becomes a promise to every future implementer of `nimpack.yaml` —
and that same shape is today contractually defined as apko's. If apko renames `budget`, we
either break our own frozen schema or carry a translation we did not design. The translation
is coming either way; the only question is whether we write it while it is free.

`ApkoConfig` and `MelangeConfig` are in this ADR because the CTO asked for them in the same
breath, and because the rule that prevents the next `LayeringConfig` has to cover them. They
are not in it because they block the freeze.

### Why the full L1 fix is not taken here

ADR-0001 §6 names the target: a backend-neutral `BuildPlan` — ordered build steps, declared
inputs and outputs, abstract package requirements — with `MelangeConfig`/`ApkoConfig` becoming
the wolfi adapter's *rendering* of that plan. It also says why it does not happen as a side
effect: it is a breaking change to `types.BuildPlan`, the `Pack` interface and all seven packs.

Nothing has changed since. There is still no second backend, and the architecture lens this
project runs on says *an abstraction with exactly one implementation is an untested
abstraction*. Designing the neutral plan now, blind, would replace one untested abstraction
with two. ADR-0001's sequencing stands: L1 is driven by a real second backend.

What *can* land now, free, is the part that the freeze makes expensive later.

### Correction to `docs/spec/SCOPE.md`'s framing of A13

SCOPE §1.6 records `artifacts[].dest` as *"three behaviours, one field"*: comment says image
path, dotnet treats it as a staging-dir suffix requiring a leading `/`, go ignores it. The
first two are not actually different behaviours, and the record should say so.

`dotnetpack.go:249` emits `-o /home/build/output%s`. `pack.InstallOutputStep()`
(`pack.go:195-199`) then runs `cp -a /home/build/output/. "${{targets.destdir}}/"`, and melange
packages `${{targets.destdir}}` as the filesystem root. So `dest: /app/api` stages to
`/home/build/output/app/api` and lands at image path `/app/api`. **The dotnet pack already
implements the documented meaning**; the string concatenation is the mechanism, not a second
semantic. The leading-`/` requirement is an artifact of `%s` concatenation, and an absolute
image path begins with `/` anyway.

The real inventory is therefore:

| Pack | `artifacts[].dest` | Verdict |
|---|---|---|
| dotnet | Honoured — resolves to the image path, via the staging copy | Conforms |
| go | **Ignored.** `golangpack.go:137-144` passes `a.Source` and `a.Name` to melange's `go/build` and never reads `a.Dest` | Pack defect |
| java, node, python, webserver | Ignore `artifacts` **entirely** — a monorepo config silently builds only the root artifact | The dangerous one |

That reframing changes the disposition: A13 is one normative definition plus two **pack
defects**, not a schema ambiguity needing a product call.

### What was measured for this ADR

Docker is unavailable (`AGENTS.md`), but — as NIMA-11 established — `apko build` does not need
it, only `melange build` does. The pinned toolchain is present at
`~/.nimbopacks/toolchain/bin/` and `packages.wolfi.dev` is reachable, so the four questions this
ADR would otherwise have deferred were answered directly against **apko v1.4.12** and the pinned
melange. Full results and commands are in **Measurements** below; each disposition cites them
inline. Nothing in this ADR rests on an unverified reading of apko's behaviour.

---

## Decision

### D1 — Name the boundary: one role per type, enforced by a rule

Three roles, three type populations, stated so a reviewer can apply it mechanically:

| Population | Lives in | Parsed from user YAML? | Frozen by `schema_version: "1"`? | Example |
|---|---|---|---|---|
| **Schema types** | `internal/types`, schema section | Yes | **Yes** | `NimpackConfig`, `ImageConfig`, `LayeringConfig`, `Artifact`, `TLSConfig` |
| **Port types** | `pkg/backend` | No | No — but changing them needs an ADR (ADR-0001 §1) | `BuildOpts`, `ImageOpts`, `BuildResult`, `ImageResult` |
| **Wire types** | `internal/types`, wire section (today); the adapter (target state) | No | **No** | `MelangeConfig`, `ApkoConfig`, `ApkoLayering`, and everything else under `types.go:233-322` |

**The rule, effective immediately:**

> No type in `internal/types` may be both a schema type and a wire type. A struct reachable
> from `NimpackConfig` MUST NOT be reachable from `MelangeConfig` or `ApkoConfig`, and vice
> versa. Field-identical twins are correct and expected; shared structs are not.

This is the generalisation of ADR-0001 §3 ("no new leak may be added") to the specific failure
mode that produced `LayeringConfig`. It is enforced by a test, not by review memory — see
**Verification** item 1.

**Port types carry schema types, never wire types.** `ImageOpts.Layering` stays
`*types.LayeringConfig`, which after D2 is unambiguously *our* type with *our* documented
semantics. This is the hexagonal reading: a port speaks the domain's language, and the adapter
translates at its own edge. It retires the `ImageOpts.Layering` half of ADR-0001's **L6**.

### D2 — Split `LayeringConfig` from `ApkoLayering`

Two structs, field-identical today, free to diverge tomorrow.

```go
// internal/types/types.go — schema section. The public nimpack.yaml type.
type LayeringStrategy string

const LayeringStrategyOrigin LayeringStrategy = "origin"

type LayeringConfig struct {
    Strategy LayeringStrategy `yaml:"strategy" json:"strategy"`
    Budget   *int             `yaml:"budget"   json:"budget"`
}

// internal/types/types.go — wire section, beside ApkoConfig. apko's format.
type ApkoLayering struct {
    Strategy string `yaml:"strategy"`
    Budget   int    `yaml:"budget"`
}
```

`ApkoConfig.Layering` becomes `*ApkoLayering`. The translation is a function in the **adapter**,
`pkg/backend/wolfi/`, called from `AssembleImage` where `wolfi.go:180` does the bare pointer
assignment today. It is the only place that knows `nimpack`'s `origin` maps to apko's `origin`.

Three further corrections ride along, because they are the same four lines of code:

1. **The adapter owns `ApkoConfig.Layering`.** No pack may set it. Today
   `plan.Apko.Layering` is a second, unspecified channel into the same output that no pack uses
   and nothing forbids; a pack that set it would bypass the schema entirely. One representable
   way to say a thing (type-design lens).
2. **Delete the comment at `types.go:136`** — *"matches apko's native layering configuration
   exactly"*. After the split it is false by construction, and it is the sentence that caused
   this.
3. **Fix `backend.go:67`.** The comment reads `nil = apko default (auto multi-layer)`. NIMA-11
   measured absent-layering as a **single** layer; ADR-0003 records it. PR #29 already corrects
   this line — noted here so the two changes do not collide.

**Digest impact: none.** The emitted `apko.yaml` bytes are identical for every config that is
valid today. This is a Go type split with no change to serialised output.

**`schema_version` consequence: none.** Stays `"1"`.

### D3 — `ApkoConfig` and `MelangeConfig` stay in `internal/types`, under D1's rule, with a named exit

Moving them to `pkg/backend/wolfi/` *today* would make every pack import the wolfi backend —
`Pack.Plan()` returns `*types.BuildPlan` and all seven packs construct melange and apko structs
directly. The domain would then depend on the adapter, which is the hexagonal violation
inverted and strictly worse than the current one. Moving them to a third package
(`pkg/plan/melange`) changes an import path and nothing else: the coupling is identical and we
would have spent a PR to make the dependency graph prettier without making it shallower. ADR-0001
already rejected that class of fix by name ("fix only the cheap leaks and declare victory").

So: they stay, they are **excluded from the v1 spec** (SCOPE §2, unchanged), they are **not
frozen** by `schema_version: "1"`, and D1's rule now prevents a schema type ever being welded to
one again. L1 remains open and remains ADR-0001 §6's, sequenced against a real second backend.

The v1 spec text MUST say this explicitly, in one sentence, so a future implementer does not
read `internal/types/types.go` and conclude that melange YAML is part of the contract:

> The `nimpack.yaml` schema is defined by this document. The intermediate structures a
> particular implementation uses to drive its build tools are not part of the contract and may
> change in any release.

### D4 — Amend SCOPE §4.2 criterion 7: breaking is defined on the plan, not the digest

ADR-0003 left this open and named ADR-0004 or the compatibility policy as its owner. Taking it
here, because every "digest impact" line in this ADR depends on it.

Criterion 7 currently reads: *a change is breaking if, for a fixed source tree, `nimpack.yaml`,
toolchain version and `SOURCE_DATE_EPOCH`, the resulting image digest changes.* NIMA-17
established that nimbopacks' digests are **not** stable build-to-build — `wolfi.go:70` generates a
fresh RSA signing key per build, and that perturbs layer digests. A criterion phrased on a
property the tool does not have is unusable.

Replacement:

> **Criterion 7.** A change is breaking if, for a fixed source tree, `nimpack.yaml`, toolchain
> version and environment, **the build plan the active backend receives** is not byte-identical
> after canonical serialisation. For the `wolfi` backend that plan is the generated `melange.yaml`
> plus `apko.yaml`. Digest equivalence follows from plan equivalence modulo the per-build signing
> key, and is the user-visible consequence, not the test.

This is backend-parameterised on purpose — naming `MelangeConfig` and `ApkoConfig` in the
universal breaking-change test would re-commit the leak D1 just forbade. Every disposition in
ADR-0003 and in this ADR is plan-neutral as well as digest-neutral, so nothing already decided
has to be revisited.

### The four ambiguities

| # | Field | Disposition | Digest impact | `schema_version` consequence |
|---|---|---|---|---|
| A9 | `image.run_as` | **`*uint32`.** `run_as: 0` becomes valid and means root, with a stderr warning. Absent means the 65532 default. | **None** for any shipped config — all 50 templates and samples set `65532`. Changes for a config setting `run_as: 0`, which today silently gets 65532. | Stays `"1"`. **Breaking if deferred** past the freeze: it changes the meaning of an accepted input. |
| A10 | `image.layering.strategy` | **Closed enum**, one member in v1: `origin`. Validated at load; unknown value is an error. The enum is *nimbopacks'*, not apko's. | **None.** Validation only. | Stays `"1"`. **Breaking if deferred** (SCOPE §4.2 criterion 6: introduces validation where there was none). |
| A11 | `image.layering.budget` | **`*int`, REQUIRED when the `layering` block is present, MUST be ≥ 1.** Absent or `0` is a load error. No default. | **None.** The one shipped config that sets `layering` (`samples/showcase/layering/nimpack.yaml`) sets `budget: 8`. Measured: the missing `omitempty` is digest-neutral — apko treats absent and `0` identically. | Stays `"1"`. **Breaking if deferred**, same criterion. Reject-now keeps adding a default later free. |
| A13 | `artifacts[].dest` | **One normative definition** — the absolute path in the image where the artifact's output is installed; MUST begin with `/`; validated at load. Plus two pack defects filed: go MUST honour it, and a pack that does not support `artifacts` MUST error rather than silently build the root artifact. | **None** for any shipped config. The only two configs setting `dest` are dotnet, already conforming, already `/`-prefixed. | Stays `"1"`. **Breaking if deferred** on both halves — new validation, and `dest` going live in the go pack changes meaning for a config that sets it. |

**Net effect on the freeze: zero plan movement and zero digest movement for every config that
ships today**, matching ADR-0003's record. Each row is free before the freeze and costs a major
version after it. That asymmetry is the only reason this ADR is urgent.

### A9 — `image.run_as`: `*uint32`, root representable, root warned

`types.go:115` is `uint32` with `omitempty`; `pack.go:177` guards on `cfg.Image.RunAs != 0`. So
`run_as: 0` is indistinguishable from absent and silently yields 65532.

**It is an accident, not a policy.** The evidence is that nothing anywhere states a
root-is-forbidden rule: there is no error, no warning, no doc comment, and no test. A deliberate
prohibition that produces a *different UID than the user asked for*, silently, is not a
prohibition — it is the worst available outcome, because the user believes they configured root
and got 65532, or believes they configured nothing and cannot tell. "A zero value that silently
means something is a future bug" (type-design lens) is the exact diagnosis.

So make the three states representable: absent (default 65532), `0` (root), `N` (that UID).

**Allow root rather than reject it**, on the precedent this project already set. ADR-0003 A4
kept `image.ports` in the schema against the CTO's "a field must either do something or not
exist" rule, with the reasoning that `nimpack.yaml` is specified to be backend-agnostic and
implementable by others, so *"a schema that cannot state them is strictly less expressive than a
Dockerfile"*. UID 0 is the same shape of argument and a stronger one — `USER root` is one line in
every competing format. Refusing root is a *policy*, and policy belongs in a lint or admission
layer that can be turned on per organisation, not welded into the type system of a schema we are
asking other people to implement.

**The backend can express it.** Measured (M4): apko accepts `accounts.run-as: "0"` and emits
`config.User: "0"` in the OCI config. So "root is unrepresentable" is purely our accident, not a
constraint handed to us — there is no backend-capability gap to hide behind.

The warning is what keeps the minimal-image posture honest:

> An implementation MUST emit a warning on stderr when `image.run_as` is `0`, naming the image
> tag and stating that the container will run as root.

This rides the same stderr diagnostic channel ADR-0003 A1 introduced for unknown keys and A4 for
backend-capability gaps. No new mechanism.

Mechanically: `RunAs *uint32`; `pack.ApplyConfig` guards on `!= nil`; the 65532 default stays
where it already is, in `pack.NewApkoConfig` (`pack.go:138`). `pack.BaseConfig:226` continues to
write `65532` into generated configs. `omitempty` on a pointer behaves correctly, so the emitted
YAML is unchanged.

### A10 — `image.layering.strategy`: a closed enum that is ours

A free string with exactly one legal value and no validation. A typo reaches apko.

Disposition: `type LayeringStrategy string` with one constant, validated at load, **error** on an
unknown value.

**Why error here, when ADR-0003 A1 chose a warn-then-error ramp for `schema_version`.** A1's ramp
exists to honour "no config that loads today stops loading". The set of configs newly rejected
here is `{layering.strategy ∉ {origin}}`, and **measured (M3): every member of that set already
fails.** `strategy: orgin` makes apko exit `1` with `unrecognized layering strategy "orgin"`. So
validating at load does not reject a single config that works today — it moves an existing
failure earlier, from after the package resolution and download to before the build starts, and
replaces apko's message with one that names the legal values. A1's rule is about preserving
working configs, not preserving every parse, so it is satisfied with room to spare.

**The enum is nimbopacks', not apko's.** The spec defines `origin` as *"group packages by the
upstream source that produced them, so packages that change together share a layer"* — a
statement about package provenance, which Debian, RPM and buildpack ecosystems all have. It is
not defined as "whatever apko calls `origin`". The wolfi adapter maps it to apko's `origin`; that
mapping is the adapter's business and lives in the translation function from D2. A backend that
cannot implement a requested strategy MUST reject it, reusing the backend-capability-gap
mechanism ADR-0003 A4 introduced for `image.ports`.

Adding a second strategy later is a non-breaking addition (a new enum member accepts configs that
were previously rejected). Adding the *enum* later is breaking. That is the whole argument for
doing it now.

### A11 — `image.layering.budget`: required, ≥ 1, no default

`types.go:174` has no `omitempty`, so `layering: {strategy: origin}` marshals `budget: 0` into
`apko.yaml`. SCOPE §1.6 records the behaviour of `0` as *"undefined on our side"*. It is now
measured (M2), and the result changes the reasoning, so the facts come first.

**Measured, apko v1.4.12, nine-package image, `SOURCE_DATE_EPOCH=0`:**

| `image.layering` | apko log | Total layers in the manifest |
|---|---|---|
| block absent | — (single-layer path) | **1** |
| `strategy: origin` only | `Building 1 layers with budget 0` | **2** |
| `strategy: origin, budget: 0` | `Building 1 layers with budget 0` | **2** |
| `budget: 1` | `Building 1 layers with budget 1` | **2** |
| `budget: 2` | `Building 2 layers with budget 2` | **3** |
| `budget: 8` | `Building 8 layers with budget 8` | **9** |
| `budget: 40` | `Building 38 layers with budget 40` | **39** |

Four facts fall out, and three of them correct the record:

1. **`types.go:161-163` is right.** Total layers = content layers + 1, the `+1` being the "top"
   metadata layer. ADR-0003's verification table reports NIMA-11 measuring `budget: 10` → 10
   layers; that is apko's **log line**, which counts content layers, not the manifest. Both
   observations are correct and they were counting different things. The comment stands; the
   ADR-0003 table row should be read as "10 content layers, 11 total".
2. **Content layers are capped by the number of origin groups**, not by budget alone:
   `budget: 40` yielded 38. So budget is an upper bound, not a target.
3. **The missing `omitempty` is digest-neutral.** apko treats an absent `budget` and `budget: 0`
   identically — same log line, same layer count, same diffIDs. SCOPE flagged the missing tag as
   a hazard; measured, it is cosmetic. Recorded so nobody spends a PR on it expecting a digest
   change.
4. **`budget: 0` is NOT equivalent to omitting the `layering` block.** This is the one that
   matters: omitting gives **1** layer, `budget: 0` gives **2**. They are different images with
   different digests. The intuitive reading — "zero budget means no layering" — is false.

Disposition: `*int`; when the `layering` block is present, `budget` is **REQUIRED** and MUST be
≥ 1. Absent is an error; `0` is an error.

- **`0` is an error, and fact 4 is why it is not simply redefined as "single layer".** We cannot
  specify `budget: 0` to mean "one layer" without either lying about what the backend does or
  making the adapter rewrite the config behind the user's back. And we should not specify it as
  what it actually produces either, because `budget: 0` and `budget: 1` are the same image
  (both → 2 layers): that is two spellings of one state with no precedence rule, the type-design
  defect `build.context` was deleted for in ADR-0003 A2. Requiring ≥ 1 leaves exactly one
  spelling of every reachable state — omit the block for 1 layer, `budget: 1` for 2, and so on.
- **Absent is an error rather than defaulted,** because any default we pick is a guess ("required
  fields over optional-with-defaults where the default is a guess"). The obvious candidate is
  Chainguard's recommended `10`, cited at `types.go:168` — a recommendation about *their*
  catalogue, not a fact about the user's image. Fact 2 sharpens this: on a small image a budget
  of 10 silently becomes whatever the group count is, so the default would not even mean what it
  says.
- Rejecting now keeps the door open: because no valid config omits `budget`, introducing a
  default in a later v1 release is a **non-breaking** addition. Defaulting now and changing the
  default later would be breaking. Reject-now is the forward-compatible order.
- Implementations SHOULD reject a budget above 126. Container runtimes cap at 127 layers total,
  total is budget + 1 (fact 1), and the spec should not let a config request an unpullable image.
  SHOULD rather than MUST, because the cap is a runtime property and not ours to freeze.

Normative text, now sayable precisely:

> `budget` is the maximum number of content layers the implementation may create for this image,
> in addition to one final layer holding image metadata. The resulting count MAY be lower than
> `budget` when there are fewer distinct package groups than the budget allows. `budget` MUST be
> an integer ≥ 1 and is REQUIRED whenever `image.layering` is present. To request a single-layer
> image, omit `image.layering` entirely.

### A13 — `artifacts[].dest`: one definition, two pack defects

Normative text:

> `artifacts[].dest` is the absolute path, **in the image**, at which this artifact's build
> output is installed. It MUST begin with `/`. An implementation that supports `artifacts` MUST
> honour `dest`. If `dest` is absent, the installation path is pack-defined and the pack MUST
> document it.

Validated at load: non-empty `dest` MUST begin with `/`. The two shipped configs that set it
(`pkg/templates/templates/dotnet/dotnet-solution.yaml:42,46` and
`samples/dotnet/monorepo/nimpack.yaml:44,48`) already comply, so validation is free.

**Why "image path" and not "staging path".** It is what the field comment has always said, it is
what the commented-out example in the go template says (`go.yaml:34` — `dest: /usr/bin/server`),
and per the Context correction it is what the dotnet pack already does. Specifying the staging
directory instead would promote `/home/build/output` — melange's sandbox layout, ADR-0001's
**L4** — from an implementation detail to a frozen part of the public schema. That is precisely
the leak this ADR exists to stop, and it would be the single worst thing in the v1 document.

Two pack defects follow, neither of which is a schema question. Both are **code**, filed as
follow-up work, and both are listed with their digest impact because they must land before the
freeze:

1. **The go pack MUST honour `dest`.** `golangpack.go:137-144` passes `packages` and `output` to
   melange's `go/build` pipeline and drops `a.Dest`. **Measured (M1):** the pinned melange's
   `go/build` pipeline takes `prefix` (default `usr`) and `install-dir` (default `bin`), and its
   own documentation states *"The final install location inside the apk will be in prefix /
   install-dir / output"*. So the mapping is exact and needs no new mechanism:
   `dest: /usr/bin/server` → `prefix: usr`, `install-dir: bin`, `output: server`. Note that
   `go/build`'s default is already `/usr/bin/<output>`, which is why the go template's
   commented-out example says `dest: /usr/bin/server` — the documented default and the ignored
   field happen to agree, which is exactly why nobody noticed. Digest impact: none for shipped
   configs — the go template's `artifacts` block is commented out and no go sample sets `dest`.
   Breaking if deferred.
2. **A pack that does not support `artifacts` MUST error.** java, node, python and webserver
   ignore the block entirely, so a monorepo config under those packs *silently builds only the
   root artifact* and ships an image missing most of what the user asked for. Silence is the
   defect; the user has no way to discover it short of inspecting the image. The rule:

   > If `artifacts` is non-empty and the selected pack does not support it, the implementation
   > MUST fail with a diagnostic naming the pack, and MUST NOT build.

   Digest impact: none for shipped configs — only the dotnet template and the dotnet monorepo
   sample use `artifacts`. Breaking if deferred: turning a build into an error is criterion-6.

This is the same failure class as `dest`-in-go — a per-pack capability that the schema promises
and some packs do not keep — so the v1 spec gets one mechanism, not two. It is the third
instance of the backend/pack-capability gap concept (after `image.ports` and `layering.strategy`),
which is enough recurrence to make it a first-class section of the spec rather than three
footnotes.

---

## What the second-backend test looks like after this ADR

The acceptance criterion asks for this concretely, and "it feels more decoupled" is not an
answer. The test is a compiling artifact:

```
pkg/backend/internal/probe/probe.go   // a no-op Backend, registered only in tests
```

It implements `backend.Backend` and asserts, per field of `BuildOpts`, `ImageOpts`,
`BuildResult` and `ImageResult`, whether a non-wolfi backend can act on that field **without
importing or understanding any `Melange*` or `Apko*` type**. The test records the result as two
counts and fails if the usable count goes down.

Scored against the port as it stands:

| Port surface | Usable by a second backend? |
|---|---|
| `BuildOpts.SourceDir`, `.WorkDir`, `.Platform`, `.TLS` | Yes |
| `ImageOpts.Tag`, `.OutputDir`, `.Push`, `.TLS` | Yes |
| `ImageOpts.Layering` | **Yes, after D2.** `LayeringStrategy` + a layer budget are OCI-level concepts with nimbopacks-defined semantics. Before D2: no — it was apko's struct. |
| `ImageResult.Digest`, `.Tag`, `.TarballPath`, `.SBOMPath` | Yes |
| `BuildResult.PackagesDir`, `.Artifacts`, `.SigningKeyPath` | Partly — `SigningKeyPath` is documented "wolfi only" (**L6**) |
| `BuildPlan.Melange`, `BuildPlan.Apko` | **No.** This is **L1**, and it is the entire build instruction set |
| `CheckUpdates` | Dead weight — no caller, stub implementation (ADR-0001 §5) |

So the honest score after this ADR: **a second backend can configure an image and cannot build
one.** That is one line better than ADR-0001's baseline, in the one place the freeze made
urgent, and the probe makes the remaining distance a number in CI instead of an opinion in a
review thread. The number only goes to zero when L1 is fixed, which this ADR does not do and
does not claim to.

---

## Consequences

**Good**

- The one struct that would have frozen apko's wire format into our public schema stops being
  that struct, while it is still free to split. After the freeze this costs a major version.
- D1 converts "watch out for leaks" into a mechanical rule with a test behind it. The next
  `LayeringConfig` fails CI instead of reaching an ADR a year later.
- All four ambiguities close with **zero plan movement and zero digest movement** for every
  config that ships today, so the freeze still needs no re-pin. ADR-0003's clean record survives.
- `image.run_as` stops silently substituting a UID the user did not ask for. That is a security-
  adjacent correctness bug, not only a type-design one.
- The `artifacts`-silently-ignored defect is now a specified error. It was the most damaging
  silent failure in the inventory — an image missing most of the user's binaries, with no
  diagnostic.
- Criterion 7 becomes a test someone can actually run, on a property the tool actually has.
- The backend/pack-capability gap has now recurred three times (`image.ports`,
  `layering.strategy`, `artifacts`). Three instances is enough to design the mechanism once, in
  the spec, instead of per-field.
- **Every behavioural claim in this ADR is measured**, not inferred from doc comments — which is
  what the inventory it descends from was written to stop. Three of the four measurements
  corrected something: `budget: 0` is not "no layering", the missing `omitempty` is harmless,
  and `types.go:162` was right where ADR-0003's table appeared to contradict it.
- The probe backend turns "is the abstraction real?" into a CI number.

**Bad**

- **This ADR does not fix L1**, which is the leak that actually blocks a second backend.
  `MelangeConfig` and `ApkoConfig` are still in `internal/types`, still reachable from the `Pack`
  interface, still the only build instruction set a backend receives. The CTO asked for
  `ApkoConfig`/`MelangeConfig` to leave `internal/types` and the answer here is "not yet, and
  here is the rule that stops it getting worse". That is a smaller answer than the question.
- **D2 creates two field-identical structs**, which looks like duplication and will attract a
  well-meaning "refactor" PR proposing to merge them. The test in Verification 1 is the only
  thing standing between that PR and re-creating this exact defect; a reviewer who does not read
  this ADR will see the test as pedantry.
- **Three new validation errors** (`layering.strategy`, `layering.budget`, `dest` leading `/`)
  land in a loader that has never rejected anything. Users who hit them will experience a tool
  that got stricter in a patch release. The mitigation — none of the newly-rejected configs
  works today — is correct but is an argument, not an experience.
- **A9 makes root expressible.** Someone will write `run_as: 0` into production because the
  warning scrolled past in CI output. The alternative was a schema that cannot say what every
  competing format can say, but this is a real cost and it is the one decision here that a
  security reviewer may want to revisit with an admission-policy layer behind it.
- **A11 rejects a state that works.** `budget: 0` builds a valid two-layer image today; after
  this ADR it is a load error. Nobody is known to rely on it and it is indistinguishable from
  `budget: 1`, but it is the only place in this ADR that takes away something functional rather
  than something broken.
- **A user asking "how many layers will I get?" still gets "at most this many."** Fact 2 of the
  A11 measurement — the cap by origin-group count — means `budget` is an upper bound, so the
  normative text cannot promise an exact number. That is apko's behaviour, not a gap in the
  spec, but it will surprise someone comparing two images built from the same config with
  different package sets.
- **A13 files two pack defects this ADR does not fix.** The normative text is correct the day it
  lands; two of the six packs are not. Until the follow-up ships, the spec describes behaviour
  the go pack does not have, which is exactly the documentation-reality gap SCOPE was written to
  find.
- Making `Budget` and `RunAs` pointers is a small ergonomic tax on every call site and on anyone
  writing a test fixture. `*int` fields are the thing Go programmers complain about most, and we
  now have three (`InjectIntoImage`, `RunAs`, `Budget`).

**Neutral**

- `schema_version` stays `"1"`. Every row is decided before the freeze precisely so it never
  moves for these.
- `internal/types` gains two types (`LayeringStrategy`, `ApkoLayering`) and changes three field
  types (`ImageConfig.RunAs`, `LayeringConfig.Strategy`, `LayeringConfig.Budget`). The leaf-key
  count in SCOPE §0 is unchanged — no key is added or removed, only given a type and a rule.
- ADR-0001's L-numbering is unchanged. **L6** shrinks (its `ImageOpts.Layering` clause is
  retired, `SigningKeyPath`/`SBOMPath` remain); **L1** is untouched. Reviewers keep citing the
  same numbers.

---

## Alternatives considered

**Move `MelangeConfig` and `ApkoConfig` into `pkg/backend/wolfi/` now, as the CTO's wording
suggests.** The direct reading of the request. Rejected because `Pack.Plan()` returns
`*types.BuildPlan` and all seven packs construct these structs directly, so the move makes
`internal/pack` and every pack import the wolfi adapter. The domain would depend on the adapter:
the same hexagonal violation, pointing the other way, and harder to reverse because it would be
spread across seven packages instead of one. The prerequisite is a neutral `BuildPlan`, which is
ADR-0001 §6 and needs a second backend to design against.

**Move them to a neutral third package — `pkg/plan/melange`.** Satisfies the letter of "not in
`internal/types`" with no import inversion. Rejected as cosmetic: packs still construct melange
structs, backends still receive melange YAML, and the dependency graph is exactly as deep. It
would let us claim the invariant holds when it does not, which is worse than ADR-0001's position
of recording honestly that it does not. ADR-0001 rejected this class by name.

**Do the full L1 fix now — neutral `BuildPlan`, wolfi renders it.** The only alternative that
actually satisfies the invariant. Rejected on sequencing, not on merit: it breaks the `Pack`
interface and all seven packs, it has no second backend to validate against, and designing a
backend-neutral build IR with one known consumer produces a second untested abstraction. ADR-0001
§6 already reasoned this and nothing has changed. It also collides with the freeze — doing it now
would move `schema_version` twice (once for the plan change, once for whatever the second backend
teaches us), and ADR-0001 §6 explicitly asks that it move once.

**Keep one `LayeringConfig` and simply document that it is apko's.** Zero code. Rejected because
it freezes an external project's wire format as our public schema. `nimpack.yaml` is meant to be
an implementable spec, not this tool's config file; "implement apko's layering block" is not a
specification we can hand to a BuildKit implementer. It also makes any future apko rename a
breaking change to *our* schema, triggered by someone else's release.

**A9: keep `uint32` and reject `run_as: 0` with an error.** Also makes illegal states
unrepresentable, and it is the stronger security posture — root is never expressible, full stop.
Genuinely close. Rejected for consistency with ADR-0003 A4, which kept `image.ports` in the schema
on the explicit ground that a backend-agnostic schema must not be less expressive than a
Dockerfile. `USER root` is expressible everywhere we are benchmarked against, legitimate uses
exist (a base image others extend, a privileged init that drops down), and the right instrument
for "this organisation forbids root" is a policy check that can be configured, not a type that
cannot. The warning gives most of the deterrence at none of the expressiveness cost.

**A9: keep `uint32` and document that `0` means unset.** The cheapest option — write down the
current behaviour and move on. Rejected because it writes a bug into a normative document. A user
who writes `run_as: 0` gets UID 65532 and no diagnostic; blessing that in the spec makes it
permanent and makes every future implementer reproduce it.

**A11: default `budget` to 10 when the `layering` block is present.** Chainguard's recommendation,
already cited in our own doc comment, and friendlier than an error. Rejected because the
recommendation is about Chainguard's catalogue, not the user's image, and because of the
compatibility ordering: with absent-is-an-error, adding a default later is free; with
absent-defaults-to-10, changing it later is breaking. Reject-now strictly dominates.

**A11: treat `budget: 0` as "single layer", equivalent to omitting the block.** Permissive and
intuitive, and it was this ADR's working assumption until the measurement. Rejected on fact 4 of
M2: omitting the block gives **1** layer and `budget: 0` gives **2**, so the equivalence is
simply false. Specifying it anyway would require the wolfi adapter to delete a `layering` block
the user wrote — rewriting the config behind their back to make our documentation true. The
weaker version (accept `0`, document it as "two layers") was also rejected, because `budget: 1`
already produces that image and two spellings of one state with no precedence rule is the defect
`build.context` was deleted for in ADR-0003 A2.

**A13: specify `dest` as the staging path, matching the dotnet implementation's mechanics.**
Arguably the most faithful description of what the code does. Rejected because it writes
`/home/build/output` — melange's sandbox layout, ADR-0001 **L4** — into the frozen public schema.
A BuildKit backend has no such directory and could never implement the field. It is also
unnecessary: the Context correction shows the mechanism already resolves to the image path, so
the backend-neutral definition and the existing behaviour agree.

**Split this into four ADRs, one per ambiguity, plus one for the boundary.** Rejected for the
reason ADR-0003 gave: they share one forcing argument (the pre-freeze window) and, here, one
subject — two of the four *are* the struct being split, and A13's disposition depends on D1's
rule about not freezing adapter paths. Split apart, the reviewer cannot see that A10 and A11 are
consequences of D2 rather than independent preferences.

**Defer everything to v2.** Rejected on the same arithmetic as ADR-0003: four free changes become
four that each need a version bump, a migration note and an ADR, and v1 ships a normative
document that cannot say what `budget: 0` or `run_as: 0` mean.

---

## Verification

1. **Schema/wire separation test** (`internal/types`). Reflective walk of `NimpackConfig` and of
   `MelangeConfig`/`ApkoConfig`, collecting every reachable named struct type. The two sets MUST
   be disjoint. This is D1's enforcement, it fails today, and it is the test that stops the
   "merge these two identical structs" PR. Failure message cites this ADR.
2. **Layering translation test** (`pkg/backend/wolfi`). A `types.LayeringConfig{Strategy: origin,
   Budget: 8}` MUST render byte-identical `apko.yaml` layering bytes to the pre-split code. This
   pins the digest-neutrality claim for the one shipped config that uses layering.
3. **`run_as` table test** (`internal/pack`). Absent → `run-as: "65532"`; `0` → `run-as: "0"`
   plus a stderr warning; `1000` → `run-as: "1000"`. The `0` row is the one that fails today.
4. **Layering validation table test** (`internal/config`). `strategy: origin` + `budget: 8` →
   accepted; `strategy: orgin` → error naming the legal values; block present with no `budget` →
   error; `budget: 0` → error; `budget: 127` → rejected (total would exceed the 127-layer runtime
   cap). One case per normative sentence.
4a. **Layering contract test against the real apko** (`samples/`, runs only where the toolchain
   is installed). Re-runs M2's table — absent / `1` / `2` / `8` and asserts 1 / 2 / 3 / 9 total
   layers. This is what catches apko changing its layering semantics under us, which is the
   failure D2's split exists to contain: after the split the fix would be in the adapter's
   translation function, and this test is what points at it.
5. **`dest` validation and honouring tests.** Load-time: `dest: app/api` (no leading `/`) → error;
   `dest: /app/api` → accepted. Pack-level: the go pack with `artifacts[].dest` set MUST produce a
   plan installing to that path, and java/node/python/webserver with a non-empty `artifacts` MUST
   return an error naming the pack.
6. **The probe backend** (`pkg/backend/internal/probe`). Implements `Backend`, registered only in
   tests, with a table asserting which port fields are usable without any `Melange*`/`Apko*` type.
   The usable count is asserted and may only increase. This is the second-backend test made
   executable; it is the one check here that measures the *remaining* distance rather than the
   distance covered.
7. **Review gate.** Extend SCOPE §4.6's mechanical PR checklist with one line: *a PR adding a
   field to `internal/types` states which of the three populations in ADR-0004 D1 it belongs to.*
   D1's rule only holds if the question is asked at the point a type is created.
8. **Grep guard, extended.** SCOPE §7's loop already runs in CI. Add: no identifier matching
   `Melange|Apko` may appear in `internal/pipeline` or in `pkg/backend/backend.go`. That catches
   L7 and L6 regressions and is the cheapest possible proxy for "the core does not know which
   adapter is loaded".

Items 1, 3, 4 and 5 are new tests in `internal/types`, `internal/pack` and `internal/config` —
the last at **0.0%** coverage — so this ADR, like ADR-0003, retires part of the coverage debt in
SCOPE §5 workstream 4.

---

## Measurements

Run 2026-10-23 against the pinned toolchain (`~/.nimbopacks/toolchain/bin/`), **apko v1.4.12**,
`packages.wolfi.dev/os`, `SOURCE_DATE_EPOCH=0`, `--arch x86_64`. The probe image is a nine-package
Wolfi image (`wolfi-baselayout`, `ca-certificates-bundle`, `openssl`, `glibc-locale-en`, `tzdata`,
`curl`, `jq`, `bash`, `coreutils`) — deliberately the package list from
`samples/showcase/layering/nimpack.yaml`, so the numbers describe a config we actually ship. Layer
counts are `tar tf <image>.tar | grep -c '\.tar\.gz$'`, cross-checked against the OCI manifest's
`layers` array. No Docker was used or needed; `apko build` runs unprivileged.

| # | Question | Result | Effect on this ADR |
|---|---|---|---|
| **M1** | Does melange's `go/build` pipeline expose an install path, and under what key? | **Yes.** Inputs `prefix` (default `usr`) and `install-dir` (default `bin`); the pipeline's own text reads *"The final install location inside the apk will be in prefix / install-dir / output"*. | A13's go-pack fix is a direct mapping, no new mechanism. The pipeline's default is already `/usr/bin/<output>`, which is why the ignored `dest` went unnoticed. |
| **M2** | What does `budget: 0` do, what does absent do, and is total = budget + 1? | See the table in **A11**. Absent block → 1 layer; `budget` absent ≡ `0` ≡ `1` → 2 layers; `2` → 3; `8` → 9; `40` → 39 (capped at 38 content layers by origin-group count). | Four facts, three of them corrections. Most importantly **`budget: 0` ≠ omitting the block**, which killed this ADR's original A11 reasoning. |
| **M3** | Does apko reject an unknown layering strategy, or pass it through? | **Rejects.** `strategy: orgin` → exit `1`, `building "amd64" layer: unrecognized layering strategy "orgin"`. | A10's load-time enum rejects nothing that works today; it only moves an existing failure earlier with a better message. |
| **M4** | Can apko express `run-as: 0`? | **Yes.** `accounts.run-as: "0"` builds and emits `config.User: "0"` in the OCI config. | A9 has no backend-capability excuse. Root is unrepresentable purely because of our `!= 0` guard. |

M2 also reconciles an apparent contradiction between `types.go:161-163` and ADR-0003's
verification table: apko's `Building N layers` log counts **content** layers, the manifest counts
**total**. Both prior observations were right about different numbers. No correction to ADR-0003
is required; it is immutable and its row is accurate as written.

---

## Still open after this ADR

| Item | Owner | Next step |
|---|---|---|
| **L1 — the neutral `BuildPlan`.** `MelangeConfig`/`ApkoConfig` remain in `internal/types` and remain the only build instruction set a backend receives. Not frozen by `schema_version: "1"`; movable at any time. | Principal Architect | Stays ADR-0001 §6. Sequenced against a real second backend, not against the freeze. The probe backend (Verification 6) is the instrument that says when it is worth doing. |
| **Pack defect: the go pack ignores `artifacts[].dest`.** Mapping confirmed by M1. | Engineering | Code change + test. Must land before the freeze or `dest` going live in go becomes breaking. |
| **Pack defect: java, node, python and webserver silently ignore `artifacts`.** | Engineering | Return an error naming the pack. Must land before the freeze. |
| **ADR-0001 §4 — `Capabilities` as a sixth interface method** (L5, `GetCapabilities`'s `switch` on backend name inside the port). Untouched here; it is the other thing a second backend cannot do without editing `pkg/backend`. | Principal Architect | Its own ADR, as ADR-0001 §4 requires. |

---

## Sign-off record

| # | Subject | Disposition | Decided by | Date |
|---|---|---|---|---|
| D1 | Schema / port / wire type populations | Rule stated, test-enforced | Principal Architect | 2026-10-23 |
| D2 | `LayeringConfig` ↔ `ApkoLayering` | Split; adapter translates; adapter owns `ApkoConfig.Layering` | Principal Architect | 2026-10-23 |
| D3 | `ApkoConfig`/`MelangeConfig` in `internal/types` | Stay; excluded from the spec; not frozen; L1 remains ADR-0001 §6's | Principal Architect | 2026-10-23 |
| D4 | SCOPE §4.2 criterion 7 | Rephrased on plan-equivalence, not digest-equivalence | Principal Architect | 2026-10-23 |
| A9 | `image.run_as` | `*uint32`; `0` valid, means root, warns | Principal Architect | 2026-10-23 |
| A10 | `image.layering.strategy` | Closed nimbopacks-owned enum; one member `origin`; error on unknown | Principal Architect | 2026-10-23 |
| A11 | `image.layering.budget` | `*int`; REQUIRED with the block; MUST be ≥ 1; no default | Principal Architect | 2026-10-23 |
| A13 | `artifacts[].dest` | Absolute image path, MUST start with `/`; two pack defects filed | Principal Architect | 2026-10-23 |

All four ambiguities are marked **A** in `docs/spec/SCOPE.md` §3 — architect resolves and
records — so no product sign-off is outstanding. The boundary decisions amend ADR-0001, which
reserves its own changes to a later ADR; this is that ADR.

The code implementing D2, A9, A10, A11 and A13's validation is a follow-up issue, as are the two
pack defects. From here this ADR is immutable: a change to any decision above is a **new** ADR
that supersedes it, not an edit.
