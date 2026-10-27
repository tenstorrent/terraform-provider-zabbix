# Agent notes (TT-PATCH: TT-901)

This repository is tpretz/terraform-provider-zabbix `v2` plus a small
tenstorrent patch set. Start with:

- `TT-FORK.md`: how the fork works (branches, versioning, releases, CI).
- `PATCHES.md`: the patch manifest. Every change we carry is one `tt: TT-NNN` commit.
- `.claude/skills/sync-upstream/SKILL.md`: the procedure for taking a new
  upstream release. `scripts/sync-upstream.sh` does the mechanical part.
- `CLAUDE.md`: upstream's own engineering notes and test bar. They apply to our
  patches too.

Never push to `main`, never push tags or cut releases, and never open PRs,
issues or comments on tpretz or pepordev.
