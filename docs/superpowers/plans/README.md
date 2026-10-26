# Implementation plans

A plan turns an approved design doc into ordered, reviewable work. It is about
**sequencing and verification**, not about what to build — that is settled in the
design doc.

File name: `YYYY-MM-DD-topic.md`.

## Template

```markdown
# <Topic> — plan

- **Date:** YYYY-MM-DD
- **Design doc:** link to ../specs/YYYY-MM-DD-topic-design.md
- **Owner:** role
- **Status:** Not started | In progress | Done

## Steps

Each step is one branch and one PR, small enough to review in one sitting, and leaves
the tree green. For each: what changes, which tests prove it, and what it depends on.

1. **<step>** — files touched, test that proves it, depends on: none
2. **<step>** — ...

## Verification

The exact commands that must pass before this is done:

- `task fmt && task lint && task test`
- affected samples build (`samples/affected-samples.sh`)
- any new test named in the design doc's test plan

## Rollback

How to undo this if it goes wrong after merge.

## Done when

The checklist from the design doc's goals, restated as things a reviewer can verify.
```
