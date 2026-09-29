## Summary
- Adds English (`en/adoc`) meta descriptions in ASD-STE100 style
- Leaves `:revdate:` unchanged
- Does not change trailing newlines at end of file

## Test plan
- [ ] Confirm only English pages changed
- [ ] Confirm no `:revdate:` / `:page-revdate:` edits
- [ ] Confirm no end-of-file newline-only diffs
