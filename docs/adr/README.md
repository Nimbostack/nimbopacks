# Architecture Decision Records

An ADR records **why** a structural decision was made, so that a year later nobody has
to guess. Copy `0000-template.md` to `NNNN-kebab-title.md` and fill it in.

Write an ADR when a change:

- alters a public contract — the `nimpack.yaml` schema, the `Pack` interface, the
  `backend.Backend` interface, the CLI surface, or the `update --check` exit codes;
- adds or removes a dependency that is hard to reverse;
- changes how an image is built, layered, or signed;
- is a decision you expect someone to question later.

Do not write an ADR for a bug fix, a refactor with no contract change, or a new pack
that uses the existing extension points.

## Index

| # | Title | Status |
|---|---|---|
| [0003](0003-dead-field-disposition.md) | Disposition the seven parsed-and-ignored `nimpack.yaml` fields before the v1 freeze | **Accepted** 2026-10-23 |
| [0005](0005-pinned-threshold-failon-validation.md) | Exclude pinned packages from the `update --check` threshold, and validate `--fail-on` | **Accepted** 2026-10-23 |

ADR-0001 (backend abstraction) and ADR-0002 (`schema_version` + compatibility policy)
are in flight on their own branches; add them to this index when they merge.

Planned, not yet written:

- **ADR-0004** — decouple `LayeringConfig`, `ApkoConfig` and `MelangeConfig` from
  `internal/types`; absorbs the `image.run_as` zero value, the `layering.strategy` enum,
  the `layering.budget` zero value and the three behaviours of `artifacts[].dest`
  (SCOPE §3 A9, A10, A11, A13).

## Rules

- Numbers run in order and are never reused or renumbered.
- An ADR is immutable once **Accepted**. To change the decision, write a new ADR and set
  the old one to `Superseded by NNNN`.
- An ADR that lists no downside and no rejected alternative is not finished. Every real
  decision costs something. Say what.
- The Principal Architect owns this directory and approves status changes.
