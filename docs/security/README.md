# Security documentation

Owner: **Security Engineer**. The disclosure policy itself lives in `/SECURITY.md` at
the repo root — that is the file reporters read, and it stays there.

## What belongs here

| File | Purpose | Status |
|---|---|---|
| `threat-model.md` | STRIDE threat model over the attack surfaces | Rev 1 (2026-10-23) — pending Principal Architect review |
| `vex/` | OpenVEX documents justifying not-affected CVE findings | Not started — blocked on threat-model item CI-7 |

## Surfaces the threat model must cover

- **Untrusted `nimpack.yaml`** — melange pipeline injection, `${{ }}` expansion.
- **Toolchain download integrity** — `internal/toolchain` fetches melange, apko, and
  grype. Every binary must be verified before execution.
- **TLS and custom CA handling** — `internal/utils/tlsutil`.
- **Container runner privileges** — the melange runner.
- **The GitHub Actions supply chain** — `.github/workflows/*`.
- **Untrusted source repositories** — URL parsing, `git` argument injection, `//subdir`
  path traversal. *Deferred: `internal/remote/` does not exist in this checkout yet.*

## Standing rules

- **CVE matching is grype's job.** Nimbopacks implements no suppression of its own.
  Triage goes through `.grype.yaml`, and a suppression needs a machine-readable OpenVEX
  justification.
- **Remediation SLA:** critical within 7 days, high within 30 days.
- **Private reports:** acknowledge within 72 hours. Never triage an unpatched
  vulnerability in a public issue thread, and never put a working proof-of-concept in a
  public repo or comment.
- **Security sign-off is required** on any PR touching `internal/toolchain`,
  `internal/utils/tlsutil`, `pkg/backend`, or `.github/workflows`.
