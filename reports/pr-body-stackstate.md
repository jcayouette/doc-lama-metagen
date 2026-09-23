## Summary
- Adds English (`modules/en`) meta descriptions in ASD-STE100 style
- Leaves `:revdate:` unchanged
- Does not change trailing newlines at end of file

## Test plan
- [ ] Confirm only `modules/en` pages changed
- [ ] Confirm no `:revdate:` / `:page-revdate:` edits
- [ ] Confirm no end-of-file newline-only diffs
