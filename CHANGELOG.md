# Changelog

Notable changes, newest first. This project follows [semantic versioning](https://semver.org/);
while the major version is 0, a change to the configuration surface is a minor bump.

## 0.2.2 — 2026-11-16

### Fixed

- **The container image publishes.** It has been missing since 0.2.0: the release job installed melange through
  an action that fails on current runners (`qemu-user-static` was renamed on Ubuntu 24.04; the fallback then
  died restarting `apparmor.service`), and underneath that, `apko.yaml` pinned versions Wolfi no longer carries
  (`apko~1.2.2-r1`, `melange~0.48.2-r1`, `grype~0.111.0-r2`). The job now installs both tools from upstream
  release archives with pinned SHA-256 digests, enables rootless bubblewrap with the sysctl this repo already
  uses elsewhere, and pins the image contents to versions that exist.
- A `vX.Y.Z-rc.N` tag no longer produces an image whose APK reports `0.0.0`. Prerelease tags map to apk's
  underscore form (`0.2.3-rc.1` becomes `0.2.3_rc1`); any tag shape that cannot be mapped fails the job instead
  of publishing a mislabelled artifact.
- The release job can be dry-run from a branch. `workflow_dispatch` builds the image without cutting a release,
  always under a run-scoped `ci-test-<run_id>` tag, so a manual run cannot reach `latest` or overwrite a release.

## 0.2.1 — 2026-11-15

### Fixed

- The container image publishes again. The release job failed while setting up melange, before any
  image work started: the setup action installs `qemu-user-static`, which Ubuntu 24.04 replaced with
  `qemu-user-binfmt`, so the step died with "no installation candidate". The job now runs on 22.04,
  which still has that package. Binaries, SBOMs and checksums were unaffected in 0.2.0; only the OCI
  image was missing.

## 0.2.0 — 2026-11-15

### Upgrading from 0.1.0

**Read this before upgrading.** No configuration that builds with 0.1.0 stops building, but several
fields changed meaning and the tool now tells you about problems it used to ignore.

- **Three `nimpack.yaml` fields are gone:** `build.context`, `update.auto_check` and
  `update.repositories`. None of them did anything — they were parsed and never read. A config that
  still sets them loads unchanged and produces an identical build plan; nothing needs editing.
- **`build.env` is now `build.options`.** The old key keeps working as a deprecated alias and prints a
  warning. If a config sets both, `options` wins. Rename at your convenience.
- **`image.ports` is reserved.** apko's image configuration has no exposed-ports input, so this field
  was accepted and then dropped on the floor. It is now reserved: setting it emits one warning per
  build and the value is not carried into the image. Use your orchestrator's service definition for
  ports; a container image has no say in what is published.
- **`image.labels` now reaches the image.** Labels are mapped onto apko annotations, so they appear in
  the image config where tools can read them.
- **The loader validates more and says more.** A missing or unreadable `schema_version` warns, an
  unrecognised key warns and names itself instead of being silently discarded, and `image.run_as: 0`
  warns because the container will run as root. These warn now and become errors with schema `"2"`.
- **`image.layering` is validated strictly, which is a departure from the above.** `strategy` must be
  `origin`, and `budget` is required, at least 1 and at most 126. Every config this rejects already
  failed inside apko, so the build was going to fail either way — it now fails earlier with the legal
  values named.

### Changed

- The Go toolchain is 1.26.11. The 1.24 line no longer receives security backports, and the standard
  library fixes for 26 reachable vulnerabilities live between go1.25.8 and go1.26.10.
- `govulncheck` now **fails the build** on a reachable vulnerability. It ran in report-only mode in
  0.1.0, which is the same as not running it.

### Fixed

- Building the `cve-patching` sample no longer fails. It pinned `openssl=3.0.7-r0`, a build Wolfi has
  since garbage-collected; the pin is removed and the sample still teaches pinning and remediation in
  its text.
- A pinned package no longer escapes the update check. Pinning recorded a version and then exempted it
  from the threshold, so a package pinned before a disclosure could stay silent about a critical
  vulnerability — the exact waiver the ADR warns against. A pin is metadata now, not an exemption.
- The apko layering default is documented correctly: no `layering` block produces a single layer.

### Security

- Every GitHub Action is pinned to a commit SHA.
- The release job no longer restores the Go module cache. It runs with write access to releases, so it
  is the reader that must not consume a cache an earlier pull-request run could have written.
- `step-security/harden-runner` runs in audit mode, and secret scanning covers the full history rather
  than the tip.

## 0.1.0 — 2026-07-05

First tagged release.
