## Summary
- Adds or replaces English (`modules/en`) meta descriptions in ASD-STE100 style for all pages under `docs/latest` (advanced, configure, develop, setup, stackpacks, suse-cloud-observability, use, and top-level pages)
- Overwrites placeholder descriptions such as `SUSE Observability` / `SUSE Observability Self-hosted`
- Leaves `:revdate:` unchanged
- Does not change trailing newlines at end of file

## Test plan
- [ ] Confirm only `docs/latest/modules/en` pages changed
- [ ] Confirm `advanced`, `configure`, `develop`, `setup`, `stackpacks`, `suse-cloud-observability`, and `use` all have real page descriptions (not product-name placeholders)
- [ ] Confirm descriptions are one sentence, about 120–160 characters
- [ ] Confirm no `:revdate:` / `:page-revdate:` edits
- [ ] Confirm no end-of-file newline-only diffs
