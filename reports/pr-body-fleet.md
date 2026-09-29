## Summary
- Adds English meta descriptions in ASD-STE100 style for Fleet community docs (`modules/ROOT`)
- Generated from the latest semantic version and copied onto matching paths in older versions
- Leaves `:revdate:` unchanged
- Does not change trailing newlines at end of file

## Test plan
- [ ] Confirm only English community-docs pages changed
- [ ] Confirm no `:revdate:` / `:page-revdate:` edits
- [ ] Confirm no end-of-file newline-only diffs
