# Status and baselines

Measured numbers only. Every figure carries the command that produced it and the date it
was measured, so a reader can reproduce it or tell that it has gone stale.

| File | Purpose | Status |
|---|---|---|
| `baseline.md` | The Phase 0 starting measurements | To be written |

## What gets measured

- Test coverage, per package and total.
- OpenSSF Scorecard score, per check.
- CVE count per sample image, by severity.
- Image size per sample.
- Reproducibility: build twice, compare digests.

## Rules

- **No target is claimed until the baseline number exists.** "We aim for ≥ 80% coverage"
  is meaningless without knowing where we start.
- A number with no command next to it is a rumour. Write the command.
- When a number cannot be measured yet, say so and say why. An honest
  "not measurable: requires Docker, which is not installed" is worth more than a guess.
- Refresh on every release.
