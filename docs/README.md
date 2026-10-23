# Nimbopacks documentation

Where each kind of document lives, and who owns it.

| Directory | Holds | Owner |
|---|---|---|
| `adr/` | Architecture Decision Records — one per interface or structural decision | Principal Architect |
| `spec/` | The normative `nimpack.yaml` specification and its JSON Schema | Principal Architect |
| `security/` | Threat model, CVE operations, OpenVEX documents | Security Engineer |
| `compliance/` | Control matrix, evidence, licensing reports | Compliance Manager (not yet hired) |
| `status/` | Measured baselines and release evidence snapshots | QA & Test Engineer (not yet hired) |
| `superpowers/specs/` | Design docs, written before non-trivial code | The author; the Architect reviews |
| `superpowers/plans/` | Implementation plans that follow an approved design doc | The author |

`CLAUDE.md` at the repo root is the authoritative description of the architecture and
the pack interface. Nothing here overrides it. If a document here disagrees with
`CLAUDE.md`, `CLAUDE.md` wins and the document is wrong.

## The flow for a non-trivial change

1. An issue with acceptance criteria.
2. A design doc in `superpowers/specs/`, reviewed by the Principal Architect.
3. A plan in `superpowers/plans/`.
4. Implementation, test-first, on a feature branch.
5. `task fmt && task lint && task test`, plus affected samples.
6. Peer review. Security sign-off if the change touches `internal/toolchain`,
   `internal/utils/tlsutil`, `pkg/backend`, or `.github/workflows`.
7. A PR with a Conventional Commit title.

Small changes skip steps 2 and 3. "Small" means: no new public contract, no schema
change, no new dependency, and no change to how an image is built.

## Naming

- ADRs: `adr/NNNN-kebab-title.md`, numbered in order, never renumbered.
- Design docs: `superpowers/specs/YYYY-MM-DD-topic-design.md`.
- Plans: `superpowers/plans/YYYY-MM-DD-topic.md`.
