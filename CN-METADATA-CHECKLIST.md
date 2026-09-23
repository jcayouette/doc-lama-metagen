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
| 7 | rancher/turtles-product-docs `versions@main` | `/home/scribe/projects/work/cn-metadata/turtles-product-docs/versions` | generated + pushed `add-en-meta-descriptions` (553 EN files; v0.28 source, copied to older; `stable` is a symlink to v0.27) | yes (`Good "git" signature` ED25519 work key) | https://github.com/rancher/turtles-product-docs/pull/284 |
| 8 | rancher/rancher-ai-product-docs `docs@main` | `/home/scribe/projects/work/cn-metadata/rancher-ai-product-docs/docs` | generated + pushed `add-en-meta-descriptions` (2 EN files added; 22 already had descriptions) | yes (`Good "git" signature` ED25519 work key) | https://github.com/rancher/rancher-ai-product-docs/pull/87 |
| 9 | rancher/stackstate-product-docs `docs@main` | `/home/scribe/projects/work/cn-metadata/stackstate-product-docs/docs` | generated + pushed `add-en-meta-descriptions` (8 EN files added; 217 already had descriptions) | yes (`Good "git" signature` ED25519 work key) | https://github.com/rancher/stackstate-product-docs/pull/423 |
| 10 | rancher/rancher-product-docs `versions@main` | `/home/scribe/projects/work/cn-metadata/rancher-product-docs/versions` | **paused 17 Sep 2026** — generator stopped; 1830 uncommitted EN files on `main` (do not revert). Resume by re-running the per-repo command; skip-existing keeps already-written pages. | | |
| 11 | rancher/fleet-product-docs `community-docs@main` | `/home/scribe/projects/work/cn-metadata/fleet-product-docs/community-docs` | pending | | |
| 12 | rancher/longhorn-product-docs `docs@main` | `/home/scribe/projects/work/cn-metadata/longhorn-product-docs/docs` | pending | | |
| 13 | rancher/elemental-product-docs `docs@main` | `/home/scribe/projects/work/cn-metadata/elemental-product-docs/docs` | pending | | |
| 14 | rancher/private-registry-product-docs `versions@main` | `/home/scribe/projects/work/cn-metadata/private-registry-product-docs/versions` | pending | | |
| 15 | kubewarden/docs `docs/admission-controller@main` | `/home/scribe/projects/work/cn-metadata/kubewarden/docs/admission-controller` | pending | | |
| 16 | kubewarden/docs `docs/sbom-scanner@main` | `/home/scribe/projects/work/cn-metadata/kubewarden/docs/sbom-scanner` | pending | | |
| 17 | rancher/runtime-enforcer-product-docs `versions@main` | `/home/scribe/projects/work/cn-metadata/runtime-enforcer-product-docs/versions` | pending (not cloned yet) | | |

Kubewarden: only `admission-controller` and `sbom-scanner`. Do not process `docs/kw` or other trees.

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

## Paused 17 Sep 2026 — resume here

Stopped `doc-meta-gen` for rancher-product-docs. Partial writes are uncommitted on `main` in `/home/scribe/projects/work/cn-metadata/rancher-product-docs`. Leave them in place.

### Rancher Manager snapshot

| Version | EN pages | With `:description:` | Still missing |
|---------|----------|----------------------|---------------|
| v2.16 (source) | 407 | 402 | 5 (warnings / skipped stubs) |
| v2.15 | 409 | 36 | 373 (not started) |
| v2.14 | 412 | 367 | 45 (stopped mid-copy) |
| v2.13 | 410 | 409 | 1 |
| v2.12 | 407 | 406 | 1 |
| v2.11 | 417 | 414 | 3 |
| srfa | 0 | 0 | 0 |

- Git: `main...origin/main`, **1830** modified `modules/en` files, no revdate edits, no non-EN files.
- Last written: `versions/v2.14/modules/en/pages/release-notes/v2.14.1.adoc`. Remaining v2.14 starts at `release-notes/v2.14.2.adoc` through troubleshooting, then all of v2.15.
- Log so far: Added 449, Copied 1381, Skipped 166, Warnings 5. HTML report at `reports/rancher.html` is incomplete (killed before finish).
- Do **not** commit or push this repo until the run completes.

### Resume tomorrow

```bash
cd /home/scribe/doc-lama-metagen/doc-meta-gen
./doc-meta-gen \
  --root /home/scribe/projects/work/cn-metadata/rancher-product-docs/versions \
  --type asciidoc \
  --lang en \
  --update-revdate=false \
  --html-log /home/scribe/doc-lama-metagen/reports/rancher.html \
  --report-title "rancher-product-docs Meta Descriptions"
```

Files that already have `:description:` are skipped. After generation: sign as `jcayouette` with `~/.ssh/id_ed25519_work`, push `add-en-meta-descriptions`, open PR with `reports/pr-body-rancher.md` (no Cursor footer).

Then continue pending repos in table order: fleet `community-docs`, longhorn, elemental, private-registry, kubewarden admission-controller, kubewarden sbom-scanner (not `docs/kw`), clone + runtime-enforcer.
