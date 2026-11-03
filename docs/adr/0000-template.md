# NNNN. Short title in the imperative

- **Status:** Proposed | Accepted | Superseded by NNNN
- **Date:** YYYY-MM-DD
- **Deciders:** names or roles
- **Affects:** the files, packages, or contracts this touches

## Context

What forced this decision? State the problem, the constraints, and the invariants from
`CLAUDE.md` that are in play. Facts only — no advocacy yet. A reader who knows nothing
about the issue should understand the pressure after this section.

## Decision

What we are doing, in the imperative: "Pass the git ref after `--` and reject refs
matching `^-`." Be specific enough that someone could implement it from this paragraph.

## Consequences

**Good:** what this buys us.

**Bad:** what it costs us. Every decision costs something — migration work, performance,
a narrower future, more code to maintain. If this section is empty, the ADR is not
finished.

**Neutral:** what simply changes.

## Alternatives considered

For each one: what it was, and the specific reason it was rejected. "We considered X but
rejected it" is not a reason. "We considered X but it would require `internal/pipeline`
to switch on the pack name, which breaks the language-agnostic core invariant" is.

## Verification

How we will know this decision is holding: a test, a lint rule, a CI check, or a review
gate. A decision with no enforcement decays.
