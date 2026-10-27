# Tenstorrent fork notes (TT-PATCH: TT-901)

This repository is **tpretz/terraform-provider-zabbix `v2` plus a small tenstorrent
patch set**. The root `CLAUDE.md` is upstream's and is kept byte-identical. These
notes add to it:

- Read `TT-FORK.md` (fork model, branches, versioning, release) and `PATCHES.md`
  (the patch manifest) before changing anything.
- Every change we carry is one `tt: TT-NNN` commit listed in `PATCHES.md`. Put new
  logic in new files (`*_tt.go`, `tt_*_test.go`). Keep edits to upstream files to
  tagged one-line hooks: `// TT-PATCH: TT-NNN`.
- To take a new upstream release use the **`sync-upstream`** skill
  (`.claude/skills/sync-upstream/SKILL.md`, with `scripts/sync-upstream.sh`).
- Never push to `main`, never push tags or release, never contribute upstream.
