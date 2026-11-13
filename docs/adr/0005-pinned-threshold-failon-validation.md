# 0005. Count pinned packages toward the `update --check` threshold, and validate `--fail-on`

- **Status:** Accepted — 2026-10-23; decision 1 amended 2026-10-26
- **Date:** 2026-10-23
- **Deciders:** CTO
- **Affects:** `internal/update/update.go`, `internal/update/format.go`, `cmd/nimbopacks/main.go`
  (the `update --check` exit-code contract documented in `CLAUDE.md`)

## Context

Split out of ADR-0003 because both changes here modify the `update --check`
**exit-code contract**, which is public: `CLAUDE.md` documents `0` clean, `1` CVEs
found at or above `--fail-on`, `2` tool/config error. A contract change needs its own
record, separate from the field-disposition work in ADR-0003.

Two independent bugs were found in `internal/update/update.go`:

1. **`update.pinned` has no effect on `--check`.** `types.go:201` documents pinned
   packages as "won't be updated automatically" — an operator acknowledgement that a
   package is deliberately held at a known version. But `MeetsThreshold` (`update.go:218`,
   pre-fix) never consulted `PackageUpdate.Pinned` when deciding whether a finding
   crosses the threshold. A pinned package's CVE still failed `--check` every time,
   with no way to make CI green short of unpinning (defeating the point of pinning) or
   lowering `--fail-on` for the whole scan (hiding unrelated findings too). The doc
   comment was a promise the code didn't keep — this is a bug, not a new schema
   question.

2. **An unrecognised `--fail-on` value silently turns the gate into "fail on any
   finding".** `severityRank`
   (`update.go:180`) returns `-1` for any string it doesn't recognise. `MeetsThreshold`
   compares each finding's rank against that `-1` threshold, and every severity
   (`negligible` and up, rank ≥ 0) is `≥ -1`. So `--fail-on critcial` (typo) does not
   error: it makes every finding clear the threshold, so the gate fails on the first
   report that has any finding at all and passes on an empty one. That is noise where a
   typo should have been an error, and it is indistinguishable from a real vulnerability
   gate doing its job. A flag typo must be caught, not silently reinterpreted.

The standing invariant that both fixes must respect: *"CVE policy is grype's —
nimbopacks implements no suppression of its own"* (`CLAUDE.md`, CVE Scanning section).
`.grype.yaml` is the only place a CVE is waived. Neither fix may remove a finding from
the report; they may only change how a finding maps to an exit code.

## Decision

1. **Amended 2026-10-26 — a pinned package counts toward the threshold like any other.**
   The finding is unchanged: it still appears in `report.Packages`, still counts in
   `report.Summary`, and every output format (text, table, JSON, SARIF) still renders a
   `pinned` marker. What changed is the exit code.

   As first written, this decision excluded `Pinned == true` from `MeetsThreshold`. Review
   found that the exclusion is exactly what the standing invariant in the Context forbids: a
   pin is usually taken *before* a CVE is disclosed, so the exclusion let a pin keep a
   critical finding out of the gate — a suppression implemented here, not in grype. The
   distinction the original text leaned on ("acknowledgement, not a waiver") does not survive
   contact with the exit code: a finding that cannot fail the build has been waived.
   Suppression stays grype's job, through `.grype.yaml` and an OpenVEX justification.

2. Add `ValidFailOn(failOn string) bool`, checking against the five documented values
   (`critical`, `high`, `medium`, `low`, `none`), case-insensitive. `cmd/nimbopacks`
   calls it before building or scanning anything, when `--check` is set, and returns
   a plain error (not the `errCVEThreshold` sentinel) on failure. `main()`'s existing
   dispatch — `errCVEThreshold` → exit 1, any other error → exit 2 — then gives an
   unrecognised `--fail-on` the tool/config exit code, matching the table in
   `CLAUDE.md`, with no change to `main()` itself.

## Consequences

**Good:**
- `update.pinned` now does what its own doc comment has always said it does for
  `--check`, closing the gap between documentation and behavior.
- A CI job can hold a known, patched-on-a-schedule package at a pinned version without
  the pin itself being penalized twice (once by losing the auto-update, once by
  failing every scan).
- A misspelled `--fail-on` now fails loudly and immediately (exit 2, before any build
  or grype invocation) instead of changing gate behavior with no warning.

**Bad:**
- This is a breaking change to the exit-code contract for two specific inputs: a CI
  job that currently relies on `update.pinned` entries failing `--check` (e.g. as a
  forced reminder) will see those checks start passing; a CI job with a pre-existing
  `--fail-on` typo that happened to produce a usable (if accidental) result will start
  exiting 2 instead. Both are intentional — the old behavior was a bug in each case —
  but either can break a pipeline that depended on it. See Migration note.
- Pinned findings still appear in output, so `grype`/report-based alerting that isn't
  threshold-based (e.g. "any CVE in the JSON report") is unaffected and unchanged;
  only `MeetsThreshold`-gated paths change. Teams filtering on `has_critical`/`has_high`
  rather than `--check`'s own exit code will not see the pinned exclusion and may need
  to add their own `pinned` check if they want equivalent behavior.

**Neutral:**
- `severityRank` is unchanged; `ValidFailOn` is a separate, additive check so the two
  failure modes (unrecognised severity string vs. threshold comparison) stay
  independently testable.

## Alternatives considered

- **Silently clamp an unrecognised `--fail-on` to a safe default (e.g. `high`).**
  Rejected: this still changes behavior without telling the operator, and picks a
  threshold they didn't ask for. An explicit exit 2 forces the CI config to be
  corrected, which is the only fix that doesn't guess at intent.
- **Suppress pinned findings from the report entirely (what ADR-0003's "pinned means
  waived" framing would imply).** Rejected by the CTO: that would make `update.pinned`
  a second, informal CVE-suppression mechanism in addition to `.grype.yaml`, which
  directly contradicts the "no suppression of its own" invariant. Reporting but not
  gating keeps `pinned` as acknowledgement, not waiver.
- **Make pinned-exclusion configurable (a flag like `--ignore-pinned`).** Rejected:
  `update.pinned` already is the opt-in signal — a pinned entry only exists because an
  operator added it to `nimpack.yaml`. A second flag to decide whether the first one
  counts adds a knob with no clear second use case.

## Verification

- `internal/update/update_test.go`: `TestMeetsThreshold_ExcludesPinnedPackages`
  (a pinned-only report never meets any threshold; a mixed report still meets it on
  the unpinned finding) and `TestValidFailOn` (all five recognised values, several
  case variants, and several typos/garbage strings).
- `internal/update/format_test.go`: `TestFormatText_FlagsPinned`,
  `TestFormatTable_FlagsPinned`, `TestFormatSARIF_FlagsPinned`,
  `TestFormatJSON_IncludesPinned` — each asserts the finding is present and marked
  pinned, not removed.
- `cmd/nimbopacks/main_test.go`: `TestUpdateCmd_RejectsUnrecognisedFailOn` — asserts
  an unrecognised `--fail-on` value returns an error that is not `errCVEThreshold`
  (so `main()` routes it to exit 2, not exit 1).
- `task fmt && task lint && task test` gate this on every change to `internal/update`
  or `cmd/nimbopacks`.

## Migration note

For a CI job that sets `update.pinned` today:
- Before: a pinned package with a CVE at or above `--fail-on` always failed `--check`.
- After: that same pinned package no longer fails `--check` on its own. The finding
  still prints in every format and is flagged `pinned`, so dashboards and SARIF
  consumers keep full visibility — only the exit code changes. If a team was using
  this failure as a forced "come back and revisit this pin" reminder, switch to a
  dashboard/report check on the `pinned` flag, or track it outside `--check`.

For a CI job with a misspelled `--fail-on` (e.g. `--fail-on critcial`):
- Before: the typo was silently absorbed; `MeetsThreshold` effectively matched
  everything, so the job likely failed (or passed) in a way that didn't reflect the
  intended threshold.
- After: `nimbopacks update --check --fail-on critcial` now exits 2 immediately with
  `invalid --fail-on value "critcial": must be one of critical, high, medium, low, none`,
  before any build or scan runs. Fix the spelling in the CI config; no other change is
  needed. Recognised values (`critical`, `high`, `medium`, `low`, `none`, any case)
  behave exactly as before.
