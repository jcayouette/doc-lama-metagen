#!/bin/bash
set -u
REPO=/home/scribe/projects/work/cn-metadata/suse-edge.github.io
TOOL=/home/scribe/doc-lama-metagen/doc-meta-gen/doc-meta-gen
WT=/home/scribe/projects/work/cn-metadata/edge-wt
export GIT_SSH_COMMAND='ssh -i /home/scribe/.ssh/id_ed25519_work -o IdentitiesOnly=yes'
mkdir -p "$WT"

for ver in 3.7 3.6 3.5 3.4 3.3 3.2 3.1; do
  echo "===== release-$ver $(date -Is) ====="
  dest="$WT/release-$ver"
  branch="add-en-meta-descriptions-$ver"
  if [ ! -d "$dest/.git" ] && [ ! -f "$dest/.git" ]; then
    git -C "$REPO" worktree add -b "$branch" "$dest" "upstream/release-$ver" || { echo "FAIL worktree $ver"; continue; }
  fi
  if [ -d "$dest/docs/en" ]; then
    root="$dest/docs/en"
  else
    root="$dest/asciidoc"
  fi
  echo "root $root"
  "$TOOL" --root "$root" --lang en --update-revdate=false -a product="SUSE Edge" \
    > "/home/scribe/doc-lama-metagen/reports/edge-$ver.log" 2>&1 || echo "tool exit $? for $ver"
  for f in edge-book/versions.adoc edge-book/links.adoc edge-book/attributes.adoc template.adoc; do
    rel="${root#$dest/}/$f"
    if [ -f "$dest/$rel" ]; then
      git -C "$dest" checkout -- "$rel" || true
    fi
  done
  if git -C "$dest" diff -U0 | grep -E -q '^[+-]:(page-)?revdate:'; then
    echo "REVDATE CHANGE $ver — not committing"
    continue
  fi
  git -C "$dest" add -A
  if git -C "$dest" diff --cached --quiet; then
    echo "no changes $ver"
    continue
  fi
  git -C "$dest" -c gpg.format=ssh -c user.signingkey=/home/scribe/.ssh/id_ed25519_work -c commit.gpgsign=true \
    commit -S -m "$(cat <<EOF
Add English meta descriptions to SUSE Edge $ver docs.

Each English page now has a finished description, and revision dates stay unchanged.
EOF
)" || { echo "FAIL commit $ver"; continue; }
  git -C "$dest" push -u origin "HEAD:$branch" || { echo "FAIL push $ver"; continue; }
  gh pr create --repo suse-edge/suse-edge.github.io \
    --base "release-$ver" \
    --head "jcayouette:$branch" \
    --title "Add English meta descriptions" \
    --body "$(cat <<EOF
## Summary
- Add an English \`:description:\` to the SUSE Edge $ver documentation pages.
- Leave \`:revdate:\` and \`:page-revdate:\` unchanged.
- Leave generated version and link attribute files unchanged.

## Test plan
- [ ] Confirm each changed page has one \`:description:\` and the same \`:revdate:\` as before.
- [ ] Confirm \`edge-book/versions.adoc\`, \`links.adoc\`, and \`attributes.adoc\` are unchanged.
- [ ] Preview one page and check the description matches the page.

EOF
)" || echo "FAIL pr $ver"
  echo "DONE $ver"
done
echo "ALL DONE $(date -Is)"
