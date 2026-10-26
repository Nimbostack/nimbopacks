# Design docs

A design doc is written **before** non-trivial code and reviewed by the Principal
Architect. It answers "what are we building and why this shape" — not "how do we
sequence the work", which belongs in a plan under `../plans/`.

File name: `YYYY-MM-DD-topic-design.md`.

## Template

```markdown
# <Topic> — design

- **Date:** YYYY-MM-DD
- **Author:** role
- **Reviewer:** Principal Architect
- **Status:** Draft | In review | Approved | Superseded
- **Issue:** link

## Problem

What is broken or missing, for whom, and what it costs today. Include the evidence —
an error, a measurement, a user report. Not "we should have X".

## Goals

Numbered, testable. Each one should be something you can later point at and say
"done" or "not done".

## Non-goals

What this deliberately does not do. This is where scope creep gets stopped early.

## Invariants in play

Which rules from `CLAUDE.md` this design must not break, quoted. Then say how the
design respects each one. If the design needs to break one, stop and write an ADR first.

## Design

The shape of the solution. Types, interfaces, file layout, control flow. Show the
important signatures. Say which existing extension point you are using, or which new
one you are adding and why the existing ones did not fit.

## Alternatives considered

Each with the specific reason it was rejected.

## Risks

What could go wrong, how likely, and what we would do about it.

## Test plan

What proves this works. Unit tests, golden tests, a sample build, a regression test for
the bug that prompted this. Name them.

## Open questions

Things that need an answer before implementation starts, with who owes the answer.
```
