# CN metadata generation checklist

Scope: English (`modules/en`) only. `--root` is the product path below so other trees in the same clone are not touched. Always pass `--update-revdate=false`. Generate from the latest semantic version (or `next`/`dev`/`latest` if there is no semver) and copy matching paths to older versions.

Work identity: `jcayouette` / `jcayouette@suse.com`  
Signing: SSH key `~/.ssh/id_ed25519_work` (`jcayouette@suse.com`)  
Push: `ssh -i ~/.ssh/id_ed25519_work` (GitHub host alias `github.com-work`)

| # | Upstream | Local `--root` | Status | Signed | PR |
|---|----------|----------------|--------|--------|-----|
| 1 | rancher/harvester-product-docs `versions@main` | `/home/scribe/projects/work/cn-metadata/harvester-product-docs/versions` | generated + pushed `add-en-meta-descriptions` (535 EN files; v1.9 source, copied to v1.5–v1.8) | yes (`Good "git" signature` ED25519 work key) | https://github.com/rancher/harvester-product-docs/pull/315 |
| 2 | rancher/k3s-product-docs `docs@main` | `/home/scribe/projects/work/cn-metadata/k3s-product-docs/docs` | generated + pushed `add-en-meta-descriptions` (67 EN files; single `latest`) | yes (`Good "git" signature` ED25519 work key) | https://github.com/rancher/k3s-product-docs/pull/290 |
| 3 | rancher/rancher-developer-access-product-docs `docs@main` | `/home/scribe/projects/work/cn-metadata/rancher-developer-access-product-docs/docs` | generated + pushed `add-en-meta-descriptions` (2 EN files; single `latest`) | yes (`Good "git" signature` ED25519 work key) | https://github.com/rancher/rancher-developer-access-product-docs/pull/55 |
| 4 | rancher/k3k-product-docs `versions@main` | `/home/scribe/projects/work/cn-metadata/k3k-product-docs/versions` | generated + pushed `add-en-meta-descriptions` (108 EN files; v1.3.0 source, copied to older) | yes (`Good "git" signature` ED25519 work key) | https://github.com/rancher/k3k-product-docs/pull/203 |
| 5 | rancher/neuvector-product-docs `docs@main` | `/home/scribe/projects/work/cn-metadata/neuvector-product-docs/docs` | generated + pushed `add-en-meta-descriptions` (364 EN files; 5.6 source, copied to 5.3–5.5) | yes | https://github.com/rancher/neuvector-product-docs/pull/181 |
| 6 | rancher/rke2-product-docs `versions@main` | `/home/scribe/projects/work/cn-metadata/rke2-product-docs/versions` | generated + pushed `add-en-meta-descriptions` (70 EN files; single `latest`) | yes | https://github.com/rancher/rke2-product-docs/pull/243 |
| 7 | rancher/turtles-product-docs `versions@main` | `/home/scribe/projects/work/cn-metadata/turtles-product-docs/versions` | PR 284 merged 18 Sep (553 files, v0.11–v0.28). Follow-up: 43 short stub pages (`intro.adoc`, `changelogs/index.adoc`) still missing — filling now. `stable` is a symlink to v0.27. | yes | https://github.com/rancher/turtles-product-docs/pull/284 |
| 8 | rancher/rancher-ai-product-docs `docs@main` | `/home/scribe/projects/work/cn-metadata/rancher-ai-product-docs/docs` | generated + pushed `add-en-meta-descriptions` (2 EN files added; 22 already had descriptions) | yes (`Good "git" signature` ED25519 work key) | https://github.com/rancher/rancher-ai-product-docs/pull/87 |
| 9 | rancher/stackstate-product-docs `docs@main` | `/home/scribe/projects/work/cn-metadata/stackstate-product-docs/docs` | PR 423 updated: 225 EN pages in advanced/configure/develop/setup/stackpacks/suse-cloud-observability/use (plus root pages); placeholders overwritten; 120–160 chars | yes | https://github.com/rancher/stackstate-product-docs/pull/423 |
| 10 | rancher/rancher-product-docs `versions@main` | `/home/scribe/projects/work/cn-metadata/rancher-product-docs/versions` | generated + pushed `add-en-meta-descriptions` (2244 EN files; v2.16 source, copied to v2.11–v2.15) | yes (`Good "git" signature` ED25519 work key) | https://github.com/rancher/rancher-product-docs/pull/1472 |
| 11 | rancher/fleet-product-docs `community-docs@main` | `/home/scribe/projects/work/cn-metadata/fleet-product-docs/community-docs` | generated + pushed `add-en-meta-descriptions` (502 ROOT files; v0.16 source, copied to older + next; `--lang all` because module is ROOT) | yes (`Good "git" signature` ED25519 work key) | https://github.com/rancher/fleet-product-docs/pull/396 |
| 12 | rancher/longhorn-product-docs `docs@main` | `/home/scribe/projects/work/cn-metadata/longhorn-product-docs/docs` | generated + pushed `add-en-meta-descriptions` (EN pages per version 1.9–1.13; source 1.13, copied older) | yes | https://github.com/rancher/longhorn-product-docs/pull/456 |
| 13 | rancher/elemental-product-docs `docs@main` | `/home/scribe/projects/work/cn-metadata/elemental-product-docs/docs` | generated + pushed `add-en-meta-descriptions` (394 EN pages 1.5–1.10 plus examples; source 1.10, copied older) | yes | https://github.com/rancher/elemental-product-docs/pull/156 |
| 14 | rancher/private-registry-product-docs `en/adoc@main` | `/home/scribe/projects/work/cn-metadata/private-registry-product-docs/en/adoc` | generated + pushed `add-en-meta-descriptions` (15 English `en/adoc` files; layout is `en/adoc`, not `versions/`) | yes | https://github.com/rancher/private-registry-product-docs/pull/49 |
| 15 | kubewarden/docs `docs/admission-controller@main` | `/home/scribe/projects/work/cn-metadata/kubewarden/docs/admission-controller` | remaining Gatekeeper migration pages filled (1.28–1.31); all EN pages 1.28–1.39 have `:description:` | yes | https://github.com/kubewarden/docs/pull/919 |
| 16 | kubewarden/docs `docs/sbom-scanner@main` | `/home/scribe/projects/work/cn-metadata/kubewarden/docs/sbom-scanner` | remaining quickstart pages filled (0.12–0.13); all EN pages 0.11–0.13 have `:description:` | yes | https://github.com/kubewarden/docs/pull/919 |
| 17 | rancher/runtime-enforcer-product-docs `docs@main` | `/home/scribe/projects/work/cn-metadata/runtime-enforcer-product-docs/docs` | generated + pushed `add-en-meta-descriptions` (EN pages 0.7–0.9; source 0.9, copied older). Layout is `docs/version-*`, not `versions/`. | yes | https://github.com/rancher/runtime-enforcer-product-docs/pull/30 |
| 18 | kubewarden/docs `docs/runtime-enforcer@main` | `/home/scribe/projects/work/cn-metadata/kubewarden/docs/runtime-enforcer` | generated + pushed `add-en-meta-descriptions-enforcers` (0.8–0.11 EN pages; source 0.11, copied older) | yes | https://github.com/kubewarden/docs/pull/921 |
| 19 | kubewarden/docs `docs/network-enforcer@main` | `/home/scribe/projects/work/cn-metadata/kubewarden/docs/network-enforcer` | generated + pushed `add-en-meta-descriptions-enforcers` (0.2–0.3 EN pages; source 0.3, copied older) | yes | https://github.com/kubewarden/docs/pull/921 |

Kubewarden: process `admission-controller`, `sbom-scanner`, `runtime-enforcer`, and `network-enforcer`. Do not process `docs/kw`.

## Per-repo command

```bash
./doc-meta-gen \
  --root <Local --root> \
  --type asciidoc \
  --lang en \
  --update-revdate=false \
  --html-log /home/scribe/doc-lama-metagen/reports/<name>.html \
  --report-title "<name> Meta Descriptions"
```

## Sign-off checks (each repo)

- [ ] Only `modules/en` pages changed
- [ ] No `:revdate:` / `:page-revdate:` edits
- [ ] Commit signed with work SSH key (`git log --show-signature`)
- [ ] Push as `jcayouette` via work key
- [ ] PR opened against upstream `main` from the fork

## Notes 28 Sep 2026 (user correction)

- Turtles was treated as finished last week, but the remaining work is the **43 stub pages** the generator skipped (too little body text). PR https://github.com/rancher/turtles-product-docs/pull/284 is **merged** (v0.11–v0.28). Follow-up PR for the stubs; do not reopen 284.
- `stable` → symlink to `v0.27`; git will not list `versions/stable/*.adoc` as separate files.
- StackState PR https://github.com/rancher/stackstate-product-docs/pull/423 only shows `setup/` plus README/classic because **advanced, configure, use, and most of setup already had placeholder descriptions** (`:description: SUSE Observability`). Skip-existing left them unchanged. Fix: strip those placeholders and generate real STE100 sentences on the same branch.
- Do **not** continue remaining repos (elemental, private-registry, kubewarden, runtime-enforcer) until turtles stubs + stackstate are submitted.
- Longhorn: 661 local uncommitted files; leave them; do not push.
- Rancher Manager PR 1472 and Fleet PR 396 were opened 28 Sep from the checklist; leave them unless asked to revert.

## Paused 17 Sep 2026 — rancher-product-docs (completed 28 Sep)

Rancher Manager generation was paused mid-run on 17 Sep and finished 28 Sep. PR: https://github.com/rancher/rancher-product-docs/pull/1472
