## Summary

<!-- What does this PR do? Link the issue it resolves if applicable (Closes #N). -->

## Type of change

- [ ] Bug fix
- [ ] New language pack
- [ ] New template
- [ ] Feature
- [ ] Docs / chore

## Testing

<!-- How did you test this? Paste the commands and what they returned. -->

- [ ] `task test` passes
- [ ] New tests added (required for new packs and features)
- [ ] Manually tested against a real project

## Checklist

- [ ] Commit messages follow [Conventional Commits](https://www.conventionalcommits.org) (`feat:`, `fix:`, `docs:`, etc.), subject within 72 characters
- [ ] No work-item keys, board links or internal paths in the commits, the diff or this description — this repository is public
- [ ] `task hooks` installed, or the same rules checked by hand
- [ ] New packs call `registry.Register()` in `init()` and are imported in `cmd/nimbopacks/main.go`
- [ ] New templates include frontmatter comments (`# Template:`, `# Pack:`, `# Description:`, `# Tags:`)
