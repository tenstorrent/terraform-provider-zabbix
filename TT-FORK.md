# tenstorrent/terraform-provider-zabbix: how this fork works

This is the source of the Terraform Registry provider **`tenstorrent/zabbix`**,
which TT-Public-Cloud and it/terraform consume. Tenstorrent owns it long-term.

The fork is **tpretz/terraform-provider-zabbix `v2`, unchanged, plus a small
patch set** listed in [PATCHES.md](./PATCHES.md). Nothing else differs from
upstream. We stay close to upstream so that taking a new upstream release is a
mechanical rebase, not a merge project.

**We do not contribute upstream.** No PRs, issues or comments go to tpretz or
pepordev. Every interaction with upstream is a read (fetch, release list).

## Branch model

| Branch | What it is | Who moves it |
|---|---|---|
| `upstream-v2` | An exact, **fast-forward-only mirror** of `tpretz/v2` at the commit we are currently based on. It never carries a commit of ours. | `scripts/sync-upstream.sh --push-mirror`, during a sync. Never force-pushed, never committed to. |
| `main` (proposed default) | `upstream-v2` plus the `tt:` patch series, in manifest order. Releases are tagged from here. | Merging a reviewed sync or feature PR, always with a merge commit. Never force-pushed. |
| `sync/upstream-<version>` | A sync in progress: a new `upstream-v2` plus the re-applied patches. | The sync procedure. A feature branch, so it may be force-pushed. |
| `legacy-v0.18` | The old default branch (`master`, v0.18.x, built on the archived `go-zabbix-api`), kept so that history and the `v0.*` tags stay reachable. | Nobody; frozen. |
| `master` | The current GitHub default branch. Once `main` is adopted it is identical to `legacy-v0.18`. | Nobody; freeze it, then retire it when `main` is the default. |

Rules:

- Never push to the default branch directly.
- Never force-push anything except your own feature or `sync/*` branches.
- Never push tags except as a deliberate release step.
- Never merge a PR *into* `upstream-v2`. That branch only fast-forwards to tpretz.

`main` only ever moves forward. A sync never force-pushes it. The sync branch is
the new `upstream-v2` plus the re-created patch series, and it ends with an **adoption
merge**:

```bash
git merge -s ours --no-ff origin/main
```

That merge keeps the old `main` in history but takes none of its content, so the
sync branch's tree is exactly "new upstream + patches". The sync PR then merges
into `main` with an ordinary merge commit, and the resulting tree is still
exactly "new upstream + patches". The sync script finds the *current* patch
generation with `git log --ancestry-path upstream-v2..main`. Superseded
generations do not descend from the new mirror commit, so they are ignored.

Allow only "Create a merge commit" for `main`. Squash and rebase merges would
rewrite the patch commits and break their one-commit-per-patch identity.

Between syncs, a change to an existing patch is a commit titled
`fixup! tt: TT-NNN ...`. The next sync folds it into TT-NNN automatically. A
new patch is a new `tt: TT-NNN` commit plus a PATCHES.md row.

### Adopting this model (one-off, human)

First, review and approve the initial PR, `v2-rebase-7.4` into `upstream-v2`, but
**do not merge it**: merging would put our commits on the mirror. Then:

1. Check the safety refs: `legacy-v0.18` must equal the current `master`
   (`fd8b27e`), and the `v0.*` tags must be present.
2. Create `main` from the reviewed branch. This creates a new branch, so no
   force-push is needed:
   `git push origin <reviewed-sha>:refs/heads/main`
3. In Settings → General, set the default branch to `main`.
4. In Settings → Branches, protect `main` (PR required, `tt-ci` checks
   required), and protect `upstream-v2` (restrict pushes to the sync
   maintainers, no deletions).
5. Leave **Issues disabled** (policy: no issues on this repo). Disable
   upstream's nightly workflow, which files a `nightly-failure` issue and would
   fail without Issues: `gh workflow disable nightly.yml`. This is repo state,
   not a code change, so it adds no upstream conflict.
6. Leave `master` alone, or delete it after a grace period, since it equals
   `legacy-v0.18`.

## Versioning and tags

The Registry needs a semver tag (`vX.Y.Z[-pre]`) for every release. We encode
the upstream base in the version:

```
v<upstream version>-tt.<n>        e.g.  v2.0.0-tt.1, v2.0.0-tt.2, v2.1.0-tt.1
```

- `<upstream version>` is the newest upstream release tag reachable from
  `upstream-v2` (`git describe` against upstream's tags). If `upstream-v2` is
  ahead of that tag, as now (`v2.0.0` + 2 commits), the version still names
  `v2.0.0`. The release notes say "based on tpretz v2 at `<sha>`".
- `<n>` starts at 1 for each upstream base and increments for each of our
  releases on that base.

**Tradeoff, and why we choose it anyway.** `-tt.N` is a semver *pre-release*
suffix. That has three consequences:

- Terraform never selects a pre-release through a range constraint.
  `version = "~> 2.0"` and `">= 2.0.0"` will **not** match `2.0.0-tt.1`.
  Consumers must pin exactly: `version = "2.0.0-tt.1"`.
- Pre-releases sort below the release: `2.0.0-tt.1 < 2.0.0`. `-tt.N` < `-tt.M`
  for N < M, which matters only to tools that compare versions.
- Dependency bots (Renovate/Dependabot) treat these as unstable and may not
  propose them by default.

Both consumer repos already pin exactly (`version = "0.18.1"`, in one mixin for
it/terraform), so the first point costs nothing. In return, every version says
which upstream it is built on, and it can never be mistaken for, or collide
with, a tpretz version number.

The alternative is plain semver on our own number line, for example
`v20.0.0` = "upstream 2.0 + ours". `~>` would work and tools would treat it as
stable, but the mapping to upstream would have to be looked up, and a scheme
like `v2.0.1` would collide with upstream's numbering. Switch to that only if a
consumer genuinely needs range constraints.

Upstream's `release.yml` triggers on tags matching `v2.*` and refuses anything
else. Our `v2.x.y-tt.n` tags match it. Old `v0.*` tags remain on `legacy-v0.18`;
do not reuse them.

## Release steps (human-approved; agents stop at step 3)

1. Make sure `main` is green. `tt-ci` must pass on 6.0/7.0/7.4. If the nightly
   workflow is enabled, check its run too.
2. Run the release gate locally if you want the 8.0 signal as well:
   `make release-gate`.
3. Prepare the release notes: the upstream version and SHA, the patch IDs, and
   any consumer-facing change. Update `TT-MIGRATING-*.md` if consumers must act.
4. **Human:** tag `main` and push the tag:
   `git tag -s v2.0.0-tt.1 -m "..." && git push origin v2.0.0-tt.1`.
5. Upstream's `release.yml` runs GoReleaser. It builds and signs the zip
   archives, SHA256SUMS and the manifest, and creates the GitHub release. The
   Registry picks up the release through its GitHub webhook.
6. Check that the version appears at registry.terraform.io/providers/tenstorrent/zabbix.
   Then open the consumer PRs that bump the exact pin.

### Secrets and Registry setup needed (none are in this repo)

| Item | Where | Notes |
|---|---|---|
| `GPG_PRIVATE_KEY` | repo Actions secret | ASCII-armoured private key. Upstream's `release.yml` uses this name. The old v0.18 workflow used `GPG_PRIVATE_KEY_BASE64` / `GPG_PASSPHRASE`, so **new secrets are required** even if the same key is reused. |
| `PASSPHRASE` | repo Actions secret | The key's passphrase. |
| GPG **public** key | Registry → tenstorrent namespace → Signing keys | It must match the private key above, or the Registry rejects the release. Check it is the key v0.18.x was signed with, or add the new one. |
| Registry ↔ GitHub link | registry.terraform.io publisher settings | The provider is already published from this repo. The repo was archived and unarchived, so check that the webhook still delivers. |
| `GITHUB_TOKEN` | automatic | Needs `contents: write` for the release, which is the default for tag-push workflows unless the org restricts it. |

GoReleaser does not mark `-tt.N` versions as GitHub pre-releases (no
`prerelease: auto`). The Registry still records them as semver pre-releases,
which is what makes exact pins necessary.

## Continuous integration

| Workflow | Origin | Triggers | What it verifies |
|---|---|---|---|
| `tt-ci.yml` | ours (TT-902) | every PR; pushes to `main`, `upstream-v2` | build, vet, gofmt, unit tests (including the schema, enum and coverage guards), `make docs-check`, shellcheck, and acceptance on Zabbix **6.0, 7.0, 7.4** (`make testenv-up-<v>` + `make test<v>`, upstream's compose stacks) |
| `ci.yml` | upstream | PRs; pushes to `v2` (a branch we do not have) | build, vet, unit, gofmt |
| `nightly.yml` | upstream | schedule (default branch only), manual | acceptance on 6.0/7.0/7.4/8.0 and a `nightly-failure` issue. Disabled by policy (it files issues, and Issues are off); see Moving to main. |
| `release.yml` | upstream | tag `v2.*` | GoReleaser + GPG signing (see above) |

Building from source needs **Go 1.26.10** or later (TT-005 raises upstream's
1.25.13 floor for security advisories). `.tool-versions` pins the exact version.

## Syncing with upstream

Ask Claude Code to "sync upstream" (skill: `.claude/skills/sync-upstream/SKILL.md`),
or follow that file by hand. The mechanical part is `scripts/sync-upstream.sh`.
The skill covers judgement calls such as conflicts, breaking-change review, and
consumer notes.

## go-zabbix-api

`tenstorrent/go-zabbix-api` is **not used** by this provider. v2 carries its
own client in `internal/zabbix`. The repo is only needed to build the
`legacy-v0.18` line. **Recommendation: archive it again**, with a README note
pointing here. Unarchive it only if a v0.18.x hotfix is ever needed.

## Upstream docs that do not apply here

Upstream's RELEASING.md, MAINTAINING.md and PLAN.md describe Dependabot, an
issue-filing nightly workflow and Issues being enabled. On this fork, by policy
(TT-006): no Dependabot, no issues, no automated upgrade flow. Upstream docs are
left unedited to keep syncs conflict-free; this section overrides them.
