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
| [0004](0004-schema-wire-format-separation.md) | Separate the `nimpack.yaml` schema types from the backend wire formats, and close the four layering/artifact ambiguities | **Accepted** 2026-10-23 |
| [0005](0005-pinned-threshold-failon-validation.md) | Count pinned packages toward the `update --check` threshold, and validate `--fail-on` | **Accepted** 2026-10-23, decision 1 amended 2026-10-26 |

Numbers 0001 and 0002 are reserved: 0001 for the `backend.Backend` abstraction baseline
and 0002 for the `schema_version` and compatibility policy. Both are referenced by the
ADRs below but are not in the repository yet.

Planned, not yet written:

- **`Capabilities` as a sixth `backend.Backend` method** — retires leak L5
  (`GetCapabilities` switches on backend name inside the port). Required by ADR-0001 §4,
  restated as still-open by ADR-0004.

## Rules

- Numbers run in order and are never reused or renumbered.
- An ADR is immutable once **Accepted**. To change the decision, write a new ADR and set
  the old one to `Superseded by NNNN`.
- An ADR that lists no downside and no rejected alternative is not finished. Every real
  decision costs something. Say what.
- The Principal Architect owns this directory and approves status changes.
