# Nimbopacks documentation

Where each kind of document lives, and who owns it.

| Directory | Holds | Owner |
|---|---|---|
| `adr/` | Architecture Decision Records — one per interface or structural decision | Principal Architect |
| `spec/` | The normative `nimpack.yaml` specification and its JSON Schema | Principal Architect |
| `security/` | Threat model, CVE operations, OpenVEX documents | Security Engineer |
| `compliance/` | Control matrix, evidence, licensing reports | Compliance Manager |
| `status/` | Measured baselines and release evidence snapshots | QA & Test Engineer |

`CLAUDE.md` at the repo root is the authoritative description of the architecture and
the pack interface. Nothing here overrides it. If a document here disagrees with
`CLAUDE.md`, `CLAUDE.md` wins and the document is wrong.

## The flow for a non-trivial change

1. A design note — the problem, the shape of the change and the alternatives considered.
   Anything that adds a public contract, changes the schema, adds a dependency or changes
   how an image is built needs one; the pull request is a fine place for it.
2. Implementation, test-first, on a feature branch.
3. `task fmt && task lint && task test`, plus the affected samples.
4. Peer review. Security sign-off if the change touches `internal/toolchain`,
   `internal/utils/tlsutil`, `pkg/backend`, or `.github/workflows`.
5. A PR with a Conventional Commit title.

Small changes — no new public contract, no schema change, no new dependency, no change to
how an image is built — go straight to step 2.

## Naming

- ADRs: `adr/NNNN-kebab-title.md`, numbered in order, never renumbered.
- Security documents: `security/<topic>.md`.
- Status snapshots: `status/YYYY-MM-DD-<topic>.md`.
