#!/usr/bin/env bash
# TT-PATCH: TT-901 -- tenstorrent addition. See PATCHES.md and
# .claude/skills/sync-upstream/SKILL.md, which is the procedure this script
# serves. It does the mechanical parts of an upstream sync only:
#
#   1. fetch tpretz (read-only) and origin
#   2. resolve the upstream ref to sync to, and check upstream-v2 can
#      fast-forward to it
#   3. list our tt: patch commits and check them against PATCHES.md
#   4. create sync/upstream-<version> from the new upstream and cherry-pick
#      the patches in manifest order, regenerating docs/ on docs-only
#      conflicts and stopping on anything else
#   5. optionally fast-forward origin/upstream-v2 (never forced)
#
# It never pushes anything but the ff-only mirror (and only with
# --push-mirror), never pushes tags, never touches main, and never talks to
# upstream except to fetch.

set -euo pipefail

# The script lives in TT-901, which is not in the working tree while earlier
# patches are being applied (the sync branch starts at plain upstream). Run
# from a copy outside the tree so --resume always works, even mid-series.
if [ -z "${TT_SYNC_COPY:-}" ]; then
	copy="${TMPDIR:-/tmp}/tt-sync-upstream.$$.sh"
	cp "$0" "$copy"
	TT_SYNC_COPY=1 exec bash "$copy" "$@"
fi

UPSTREAM_URL="https://github.com/tpretz/terraform-provider-zabbix"
UPSTREAM_REMOTE="tpretz"
UPSTREAM_BRANCH="v2"
MIRROR_BRANCH="upstream-v2"
TAG_NS="refs/upstream-tags"   # upstream tags live here, never in refs/tags

target_ref=""
from_ref=""
dry_run=0
push_mirror=0
resume=0
regen_docs=1

usage() {
	cat <<'USAGE'
usage: scripts/sync-upstream.sh [options]

  --upstream-ref REF  upstream tag (e.g. v2.1.0) or "v2" (branch head) to sync to.
                      Default: the newest v2.* release tag on tpretz.
  --from REF          ref carrying the current tt: patches.
                      Default: origin/main, else origin/master.
  --dry-run           report what would happen; change nothing.
  --resume            continue a sync after resolving a conflict by hand
                      (finish the cherry-pick with `git cherry-pick --continue` first,
                      or let this script do it).
  --no-regen-docs     do not auto-resolve docs/-only conflicts with `make docs`.
  --push-mirror       after a clean apply, fast-forward origin/upstream-v2 to the
                      new upstream commit (plain push; git refuses a non-ff).
  -h, --help          this text.

Exit codes: 0 done, 2 stopped on a conflict that needs a human, 1 error.
USAGE
}

die() { echo "error: $*" >&2; exit 1; }
note() { echo "==> $*"; }

while [ $# -gt 0 ]; do
	case "$1" in
	--upstream-ref) target_ref="${2:?}"; shift 2 ;;
	--from) from_ref="${2:?}"; shift 2 ;;
	--dry-run) dry_run=1; shift ;;
	--resume) resume=1; shift ;;
	--no-regen-docs) regen_docs=0; shift ;;
	--push-mirror) push_mirror=1; shift ;;
	-h | --help) usage; exit 0 ;;
	*) usage >&2; die "unknown argument: $1" ;;
	esac
done

cd "$(git rev-parse --show-toplevel)"
git rev-parse -q --verify HEAD >/dev/null || die "not in a git checkout"

# --- 1. remotes and fetch ----------------------------------------------------

if ! git remote get-url "$UPSTREAM_REMOTE" >/dev/null 2>&1; then
	git remote add "$UPSTREAM_REMOTE" "$UPSTREAM_URL"
fi
# Upstream is fetch-only for us. Make an accidental push impossible.
git remote set-url --push "$UPSTREAM_REMOTE" "DISABLED--we-do-not-push-upstream"

note "fetching origin and $UPSTREAM_REMOTE (read-only)"
git fetch --quiet origin
git fetch --quiet --no-tags "$UPSTREAM_REMOTE" \
	"+refs/heads/$UPSTREAM_BRANCH:refs/remotes/$UPSTREAM_REMOTE/$UPSTREAM_BRANCH" \
	"+refs/tags/*:$TAG_NS/*"

# --- 2. resolve refs -----------------------------------------------------------

if [ -z "$target_ref" ]; then
	target_ref=$(git for-each-ref --sort=-v:refname --format='%(refname:strip=2)' "$TAG_NS/v2.*" |
		grep -E '^v2\.[0-9]+\.[0-9]+$' | head -n1 || true)
	[ -n "$target_ref" ] || die "no v2.x.y release tag found on $UPSTREAM_REMOTE"
fi
if git rev-parse -q --verify "$TAG_NS/$target_ref^{commit}" >/dev/null; then
	target_sha=$(git rev-parse "$TAG_NS/$target_ref^{commit}")
elif [ "$target_ref" = "$UPSTREAM_BRANCH" ]; then
	target_sha=$(git rev-parse "refs/remotes/$UPSTREAM_REMOTE/$UPSTREAM_BRANCH")
else
	die "unknown upstream ref: $target_ref (use a tpretz tag or \"$UPSTREAM_BRANCH\")"
fi

git rev-parse -q --verify "refs/remotes/origin/$MIRROR_BRANCH" >/dev/null ||
	die "origin/$MIRROR_BRANCH does not exist; see TT-FORK.md"
old_base=$(git rev-parse "refs/remotes/origin/$MIRROR_BRANCH")

if [ -z "$from_ref" ]; then
	if git rev-parse -q --verify refs/remotes/origin/main >/dev/null; then
		from_ref=origin/main
	else
		from_ref=origin/master
	fi
fi
git merge-base --is-ancestor "$old_base" "$from_ref" ||
	die "$from_ref is not based on origin/$MIRROR_BRANCH; is it the tt patch line?"

# upstream version label: newest v2.x.y tag that is an ancestor of the target
describe_upstream() {
	local sha=$1 tag n
	for tag in $(git for-each-ref --sort=-v:refname --format='%(refname:strip=2)' "$TAG_NS/v2.*"); do
		if git merge-base --is-ancestor "$TAG_NS/$tag" "$sha"; then
			n=$(git rev-list --count "$TAG_NS/$tag..$sha")
			if [ "$n" -eq 0 ]; then echo "$tag"; else echo "$tag+$n"; fi
			return
		fi
	done
	echo "untagged-$(git rev-parse --short "$sha")"
}
old_label=$(describe_upstream "$old_base")
new_label=$(describe_upstream "$target_sha")
sync_branch="sync/upstream-${new_label//+/-plus-}"

note "current base:  origin/$MIRROR_BRANCH = $(git rev-parse --short "$old_base") ($old_label)"
note "sync target:   $target_ref = $(git rev-parse --short "$target_sha") ($new_label)"
note "patch source:  $from_ref"

if [ "$old_base" = "$target_sha" ]; then
	note "already based on $new_label; nothing to sync"
	exit 0
fi
git merge-base --is-ancestor "$old_base" "$target_sha" ||
	die "upstream-v2 cannot fast-forward to $target_ref (upstream rewrote history?). Stop and read SKILL.md 'Upstream force-pushed'."

# --- 3. patch series vs manifest ------------------------------------------------

# Read the manifest from the patch line, not the working tree: mid-sync the
# tree is plain upstream plus the patches applied so far, and PATCHES.md
# arrives only with TT-900.
manifest=$(git show "$from_ref:PATCHES.md" 2>/dev/null) || die "$from_ref has no PATCHES.md; is it the tt patch line?"
manifest_ids=$(grep -oE '^\| *[0-9]+ *\| *TT-[0-9]+ *\|' <<<"$manifest" | grep -oE 'TT-[0-9]+')
[ -n "$manifest_ids" ] || die "could not read the patch order table from $from_ref:PATCHES.md"

# --ancestry-path keeps only commits that descend from the current mirror
# commit. Earlier generations of the patches (built on an older upstream and
# kept in main's history by the "adopt" merge, see TT-FORK.md) do not, so they
# are skipped; merges are skipped by --no-merges.
declare -A patch_sha=()
declare -A fixups=()   # id -> space-separated "fixup! tt: <id> ..." commits, folded in on apply
stray=0
while read -r sha subject; do
	[ -n "$sha" ] || continue
	if [[ $subject =~ ^(fixup!\ )+tt:\ (TT-[0-9]+)\  ]]; then
		fixups[${BASH_REMATCH[2]}]+="$sha "
	elif [[ $subject =~ ^tt:\ (TT-[0-9]+)\  ]]; then
		id=${BASH_REMATCH[1]}
		[ -z "${patch_sha[$id]:-}" ] || die "two commits claim $id: ${patch_sha[$id]} and $sha (squash them)"
		patch_sha[$id]=$sha
	else
		echo "warning: commit $sha is not a tt: patch: $subject" >&2
		stray=1
	fi
done < <(git log --reverse --no-merges --ancestry-path --format='%H %s' "$old_base..$from_ref")

for id in $manifest_ids; do
	[ -n "${patch_sha[$id]:-}" ] || die "PATCHES.md lists $id but $from_ref has no 'tt: $id' commit"
done
for id in "${!patch_sha[@]}"; do
	grep -qx "$id" <<<"$manifest_ids" || die "$from_ref has a commit for $id that PATCHES.md does not list"
done
[ "$stray" -eq 0 ] || die "non-tt commits on $from_ref; every change must be a tt: patch (see PATCHES.md)"

note "upstream commits being taken: $(git rev-list --count "$old_base..$target_sha")"
git --no-pager log --oneline "$old_base..$target_sha" | sed 's/^/      /' | head -n 50

note "patch series (manifest order) and upstream overlap:"
upstream_files=$(git diff --name-only "$old_base" "$target_sha")
for id in $manifest_ids; do
	sha=${patch_sha[$id]}
	overlap=$(comm -12 <(git diff-tree --no-commit-id --name-only -r "$sha" | sort) <(sort <<<"$upstream_files") | tr '\n' ' ')
	printf '      %-7s %s  %s\n' "$id" "$(git log -1 --format='%h %s' "$sha")" "${overlap:+[upstream also touched: $overlap]}"
	[ -z "${fixups[$id]:-}" ] || printf '              + fixups to fold in: %s\n' "${fixups[$id]}"
done

if [ "$dry_run" -eq 1 ]; then
	note "dry run: would create $sync_branch from $(git rev-parse --short "$target_sha") and apply the series above"
	exit 0
fi

# --- 4. apply ------------------------------------------------------------------

regen_and_continue() {
	# Generated docs are never merged by hand: take the sync branch's version,
	# regenerate from the code as it now stands, and carry on.
	local conflicted others
	conflicted=$(git diff --name-only --diff-filter=U)
	others=$(grep -v '^docs/' <<<"$conflicted" || true)
	if [ "$regen_docs" -eq 1 ] && [ -n "$conflicted" ] && [ -z "$others" ]; then
		note "docs/-only conflict: regenerating docs"
		# shellcheck disable=SC2086
		git checkout --ours -- $conflicted
		env -u ZABBIX_URL -u ZABBIX_USER -u ZABBIX_PASS make docs >/dev/null
		git add -A docs
		GIT_EDITOR=true git cherry-pick --continue
		return 0
	fi
	return 1
}

if [ "$resume" -eq 1 ]; then
	current=$(git rev-parse --abbrev-ref HEAD)
	[[ $current == sync/upstream-* ]] || die "--resume must run on a sync/upstream-* branch (on $current)"
	if git rev-parse -q --verify CHERRY_PICK_HEAD >/dev/null; then
		if [ -n "$(git diff --name-only --diff-filter=U)" ]; then
			regen_and_continue || die "unresolved conflicts remain: $(git diff --name-only --diff-filter=U | tr '\n' ' ')"
		else
			GIT_EDITOR=true git cherry-pick --continue
		fi
	fi
	sync_branch=$current
else
	git rev-parse -q --verify "refs/heads/$sync_branch" >/dev/null &&
		die "$sync_branch already exists; use --resume on it, or delete it to start over"
	[ -z "$(git status --porcelain --untracked-files=no)" ] || die "working tree not clean"
	git switch --quiet -c "$sync_branch" "$target_sha"
	note "created $sync_branch at $(git rev-parse --short "$target_sha")"
fi

applied=$(git log --format='%s' "$target_sha..HEAD" | grep -oE '^tt: TT-[0-9]+' | cut -c5- || true)
for id in $manifest_ids; do
	if grep -qx "$id" <<<"$applied"; then
		continue
	fi
	sha=${patch_sha[$id]}
	note "applying $id ($(git log -1 --format='%h %s' "$sha"))"
	if ! git cherry-pick -x "$sha" >/dev/null 2>&1; then
		if regen_and_continue; then
			continue
		fi
		echo
		echo "STOPPED: $id does not apply cleanly onto $new_label."
		echo "Conflicted files:"
		git diff --name-only --diff-filter=U | sed 's/^/    /'
		echo
		if git diff --name-only --diff-filter=U | grep -qE '^(go\.mod|go\.sum|\.tool-versions)$'; then
			echo "go.mod/go.sum: do NOT hand-merge. Take upstream's (git checkout --ours ...), re-apply"
			echo "the version intent with go mod edit / go get / go mod tidy, then govulncheck."
			echo "If upstream already meets TT-005's pins: git cherry-pick --skip and drop TT-005."
		fi
		echo "Resolve using PATCHES.md section '$id' (intent, anchors, 'conflict likelihood')."
		echo "Keep every TT-PATCH tag. Then: git add <files> && scripts/sync-upstream.sh --resume"
		exit 2
	fi
	for fx in ${fixups[$id]:-}; do
		note "  folding in $(git log -1 --format='%h %s' "$fx")"
		if ! git cherry-pick --no-commit "$fx" >/dev/null 2>&1; then
			echo "STOPPED: fixup $fx for $id does not apply. Resolve, then 'git commit --amend --no-edit'" \
				"and rerun with --resume." >&2
			exit 2
		fi
		git commit --quiet --amend --no-edit
	done
done

note "all patches applied on $sync_branch"
echo "Next (see SKILL.md): make docs && git status (commit doc drift into the owning patch),"
echo "build/vet/unit, acceptance on 7.4 (+ 6.0/7.0), update PATCHES.md base line,"
echo "write the consumer note delta, then record the adoption merge:"
echo "    git merge -s ours --no-ff $from_ref -m 'sync: adopt upstream $new_label (tree = upstream + tt patches)'"
echo "push $sync_branch and open a draft PR against main."

# --- 5. mirror ------------------------------------------------------------------

if [ "$push_mirror" -eq 1 ]; then
	note "fast-forwarding origin/$MIRROR_BRANCH to $(git rev-parse --short "$target_sha")"
	git push origin "$target_sha:refs/heads/$MIRROR_BRANCH"
fi
