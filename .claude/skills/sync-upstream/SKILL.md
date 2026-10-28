---
name: sync-upstream
description: Rebase the tenstorrent/zabbix provider fork onto a new tpretz/terraform-provider-zabbix release - fetch upstream, review its CHANGELOG/MIGRATING for breaking changes, fast-forward the upstream-v2 mirror, re-apply the tt: patch series from PATCHES.md, resolve conflicts, run build/vet/unit/acceptance tests on Zabbix 7.4 (plus upstream's gating matrix), update the manifest and consumer migration note, and open a draft PR. Use when someone says "new tpretz release", "sync upstream", "rebase on upstream", "upstream vX.Y.Z available".
---

# sync-upstream

TT-PATCH: TT-901. This repository is tpretz `v2` plus the patch series in
`PATCHES.md`. Read `TT-FORK.md` first if you have not. This skill re-creates that
series on a newer upstream and hands a reviewed-ready draft PR to a human.

## Guardrails (non-negotiable)

- **Never push to `main`** (or `master`) and never merge PRs. A human merges.
- **Never force-push** anything except the `sync/*` branch you created.
- **Never push tags, never create releases**, never run `release.yml`.
- **Never contribute upstream.** No PRs, issues, comments, forks or stars on
  tpretz or pepordev. Upstream is fetch-only; the script disables the push URL.
- `upstream-v2` only ever **fast-forwards to a tpretz commit**. Never commit to
  it, never merge a PR into it.
- Never print or commit secrets. Acceptance tests use the throwaway `Admin/zabbix`
  credentials of the local test stacks only.
- If a patch's *intent* no longer makes sense against the new upstream (for
  example, upstream now ships proxy groups), **stop and ask**. Do not invent a
  new design inside a sync.

## Procedure

### 0. Prerequisites

- Go matching `go.mod` (`make check-toolchain`), Terraform (`.tool-versions`),
  Docker with compose for `make testenv-up-*`, and `shellcheck`.
- A clean working tree on an up-to-date clone of `tenstorrent/terraform-provider-zabbix`.

### 1. See what upstream released (read-only)

```bash
scripts/sync-upstream.sh --dry-run                    # newest v2.x.y tag
scripts/sync-upstream.sh --dry-run --upstream-ref v2  # or upstream branch head
```

The dry run prints:

- the current base and the target;
- every upstream commit being taken;
- each `tt:` patch, together with the files upstream also touched.

Those files are the likely conflicts.

Then read upstream's notes for everything between the two versions:

```bash
git diff <old-base> <target> -- CHANGELOG.md MIGRATING.md
```

Write down three things:

- **breaking changes** that affect our consumers (TT-Public-Cloud, it/terraform);
- anything that **overlaps a patch's intent**. Each patch's "Drop when" in
  PATCHES.md says what to look for. Example: upstream adds `zabbix_proxygroup`,
  or adds `proxy_groupid`/`monitored_by` to hosts;
- new **Zabbix version gates** or test-matrix changes (`Makefile` `VERSIONS`/`GATED`).

If upstream now covers a patch, stop and report. Dropping a patch needs a
consumer migration plan and a human decision.

### 2. Apply the series

```bash
scripts/sync-upstream.sh --upstream-ref vX.Y.Z
```

This creates `sync/upstream-vX.Y.Z` at the upstream commit. It then cherry-picks
every `tt: TT-NNN` commit in PATCHES.md order and folds any
`fixup! tt: TT-NNN ...` commits into their patch. Conflicts confined to `docs/`
are resolved automatically by regenerating them.

**On a conflict**, the script exits 2 and names the patch.

1. Open that patch's section in `PATCHES.md`. It describes the intent and the
   upstream **anchor** each hook sits next to.
2. Re-place the hook at the semantically equivalent spot.
3. Keep the `// TT-PATCH: TT-NNN` tag on every line you add to an upstream file.
4. Keep upstream's code as upstream wrote it. Our change must stay a hook, not
   a rewrite.
5. If the hook needs more than a few lines now, move the logic into the
   patch's own `*_tt.go` file.
6. `git add` the files, then run `scripts/sync-upstream.sh --resume`.

   The sync branch starts at plain upstream, so until TT-901 has been re-applied
   the script is not in the working tree. Run the copy from the patch line:
   `git show origin/main:scripts/sync-upstream.sh > /tmp/tt-sync.sh && bash /tmp/tt-sync.sh --resume ...`.
   The script always re-executes itself from a temporary copy, so it is never
   affected by its own branch switches.

**`go.mod` / `go.sum` / `.tool-versions` conflicts (TT-005).** Never hand-merge
them. Run `git checkout --ours go.mod go.sum .tool-versions` (in a cherry-pick,
"ours" is the new upstream). Then re-apply the intent with the newest of
upstream's and our versions:

```bash
go mod edit -go=<max(upstream, 1.26.10)>
# set .tool-versions to the same version
go get golang.org/x/net@<max(upstream, v0.62.0)>
go mod tidy
govulncheck ./...
```

If upstream already meets or exceeds TT-005's pins and govulncheck is clean,
the patch has become empty. Run `git cherry-pick --skip`, remove TT-005 from
PATCHES.md, and say so in the PR. If govulncheck reports something new, extend
TT-005 only for that finding.

Compile breaks in `tt_*_test.go` or `*_tt.go` after a clean apply mean that
upstream renamed a helper or map we reference. The likely ones are
`proxySchemaBase`, `hostSchemaBase`, `updateOwner`, `removalOwner`,
`removalComputed`, `serverObject`, and `testAccCheckAllDestroyed`. Fix the
reference inside the owning patch:

```bash
git commit --fixup=<patch-sha>
GIT_SEQUENCE_EDITOR=true git rebase -i --autosquash <upstream-sha>   # non-interactive autosquash
```

### 3. Verify

```bash
gofmt -l . ; go build ./... ; go vet ./...
env -u ZABBIX_URL TF_ACC= go test ./... -count=1   # unit tests + schema, enum and coverage guards
govulncheck ./...                       # must be clean; new findings extend TT-005
env -u ZABBIX_URL make docs && git status --short docs/   # drift goes into the owning patch (fixup + autosquash)
make docs-validate
shellcheck scripts/*.sh
make testenv-up-74 && make test74       # acceptance on Zabbix 7.4: the release gate for us
make testenv-up-70 && make test70       # plus upstream's other gating versions
make testenv-up-60 && make test60
make testenv-down
```

Every test must pass, including upstream's own. The `TT*`/`*ProxyGroup*` tests
are ours: `TestAccResourceProxyGroup*`, `TestAccDataSourceProxyGroup`,
`TestAccResourceHostProxyGroup*`, and `TestAccImportByName`.

Watch for these two things:

- **A coverage guard failing** (`TestUpdateCoverageComplete`,
  `TestRemovalCoverageComplete`) because upstream added an attribute is
  upstream's to own. Ours register their coverage from `init()` in `tt_*_test.go`.
- **The R2/`monitored_by` warning in PATCHES.md (TT-003).** If upstream added
  either attribute to `hostSchemaBase`, our `init()` overwrites it. Diff the two
  and stop.

Without Docker you can still run the API-only acceptance subset against any
reachable Zabbix 7.4: export `ZABBIX_URL`/`ZABBIX_USER`/`ZABBIX_PASS`, then
`TF_ACC=1 go test ./provider/ -count=1`. Say plainly in the PR what was and was
not run.

### 4. Update the manifest and the consumer note

- Update the "Base" line in `PATCHES.md` (upstream version and SHA). Also update
  every section whose files, anchors or conflict likelihood changed, and record
  any conflict you resolved. Commit with
  `git commit --fixup=<TT-900 sha>` and autosquash, so that TT-900 stays one commit.
- Update `TT-MIGRATING-*.md`. If consumers must act, add a section
  "Upgrading from `<old>-tt.N` to `<new>-tt.1`" listing:
  - every upstream breaking change that touches our consumers' usage;
  - exact file paths in TT-Public-Cloud and it/terraform;
  - diffs.

  Do not edit the consumer repos.

### 5. Hand off

```bash
git merge -s ours --no-ff origin/main -m "sync: adopt upstream vX.Y.Z (tree = upstream + tt patches)"
git push -u origin sync/upstream-vX.Y.Z
git push origin <upstream-sha>:refs/heads/upstream-v2   # fast-forward only; never --force
gh pr create --draft --base main --head sync/upstream-vX.Y.Z ...
```

Push the mirror **before** the PR is merged. Until both have moved,
`origin/main` holds two patch generations that both descend from the old
mirror, and the script refuses to run ("two commits claim TT-001"). That
refusal is the safety net, not a bug.

The `-s ours` merge keeps the old `main` in history without taking any of its
content. It lets the PR merge into `main` with an ordinary merge commit, with no
force-push. The script's `--ancestry-path` logic ignores the superseded patch
generation on the next sync.

In the PR body:

- link the patch-only view, `compare/upstream-v2...sync/upstream-vX.Y.Z`;
- list upstream breaking changes, conflicts resolved, and test results per
  Zabbix version, with anything not run;
- add the consumer note delta;
- state the proposed release tag, `vX.Y.Z-tt.1` (TT-FORK.md § Versioning).

**Do not tag.**

## Final checklist

- [ ] Dry run reviewed; upstream CHANGELOG/MIGRATING read; overlaps with patch intents checked
- [ ] `upstream-v2` fast-forwarded to an exact tpretz commit (no force, no our commits)
- [ ] Every PATCHES.md ID applied exactly once, in order; no non-`tt:` commits
- [ ] Every edited line in an upstream file still carries its `TT-PATCH: <ID>` tag (`git grep -n TT-PATCH`)
- [ ] gofmt, build, vet, unit (incl. coverage guards), docs-check, docs-validate, shellcheck, govulncheck all clean
- [ ] Acceptance green on 7.4, and on 6.0/7.0 (upstream gating), or what was not run is stated
- [ ] PATCHES.md base line and sections updated (folded into TT-900)
- [ ] Consumer migration delta written (or "no consumer action" stated)
- [ ] Draft PR open against `main` with the `-s ours` adoption merge; tag proposed, **not pushed**
- [ ] Nothing pushed to `main`, nothing to tpretz/pepordev, no tags, no secrets in logs
