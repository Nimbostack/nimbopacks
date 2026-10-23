# Compliance documentation

Owner: **Compliance Manager** — not yet hired (charter Phase 2). This directory is a
placeholder so the structure exists when that role starts.

Compliance here means **supply-chain and open-source compliance**, backed by evidence.
It is not a certification exercise.

## What belongs here

A control matrix: framework → control → implementation → evidence link → owner → status.
Plus a CRA readiness memo, an SSDF self-attestation draft, a license compliance report
generated in CI, and a dependency policy.

## Frameworks in scope

| Framework | Target |
|---|---|
| SLSA v1.0 | Build L3 for releases |
| NIST SSDF (SP 800-218) | Full practice mapping |
| CISA Secure by Design pledge | Aligned with the pledge goals |
| OpenSSF Scorecard | ≥ 8.0 |
| OpenSSF Best Practices Badge | passing, then silver |
| OpenSSF S2C2F | Level 2 |
| SBOM — NTIA minimum elements | 100% of artifacts (SPDX 2.3 + CycloneDX 1.5+) |
| OpenVEX | Every suppressed CVE |
| EU Cyber Resilience Act | Readiness |
| NIST SP 800-190 / CIS Docker Benchmark | Applied to produced images |
| Licensing | Apache-2.0 compatible, with SPDX headers and a NOTICE file |

A control with no evidence link is not implemented, however good the intent.
