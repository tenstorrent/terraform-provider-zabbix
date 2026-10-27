# Tenstorrent patch manifest

This repository is **tpretz/terraform-provider-zabbix `v2` plus the patches listed
here, and nothing else**. Each patch is exactly one commit on our line. The subject
of that commit starts with `tt: <ID> `. Patches are re-applied in the order of this
table on every upstream sync. See [TT-FORK.md](./TT-FORK.md) for the overall model
and `.claude/skills/sync-upstream/SKILL.md` for the procedure.

Every line we add to or change in an upstream file carries a `TT-PATCH: <ID>`
comment. You can list our whole footprint in upstream files with:

```bash
git grep -n 'TT-PATCH' -- ':!*_tt.go' ':!*tt_*_test.go' ':!PATCHES.md' ':!TT-*.md' ':!AGENTS.md' \
  ':!.claude' ':!scripts' ':!.github/workflows/tt-*' \
  ':!internal/zabbix/proxygroup.go' ':!provider/resource_proxygroup.go'
```

Every file that is entirely ours is either named `*_tt.go` / `tt_*_test.go` or
starts with a `TT-PATCH: <ID> -- whole file` header. The command above prints
about fifteen lines today: one example in `examples/`, its generated copy in
`docs/`, and the Go hooks. The exceptions to the file naming are
`internal/zabbix/proxygroup.go` and `provider/resource_proxygroup.go`, which are
named after the resource they add.

**Generated files (`docs/`).** `make docs` regenerates them, so they are never
hand-merged. On a conflict, take upstream's version and regenerate after all
patches are applied (see the skill).

| Order | ID | Commit subject (stable prefix) | Depends on |
|---|---|---|---|
| 1 | TT-001 | `tt: TT-001 proxy groups: ...` | none |
| 2 | TT-002 | `tt: TT-002 proxy group membership on zabbix_proxy` | TT-001 (tests) |
| 3 | TT-003 | `tt: TT-003 proxy_groupid and monitored_by on zabbix_host` | TT-001 (tests), TT-002 (`ttChainCustomizeDiff`) |
| 4 | TT-004 | `tt: TT-004 import zabbix_proxy and zabbix_proxygroup by name` | TT-001, TT-002 |
| 5 | TT-005 | `tt: TT-005 toolchain and dependency advisories` | none |
| 6 | TT-900 | `tt: TT-900 fork docs: ...` | none |
| 7 | TT-901 | `tt: TT-901 upstream sync tooling: ...` | none |
| 8 | TT-902 | `tt: TT-902 CI: ...` | none |

Base: `upstream-v2` at tpretz `v2` commit `f6077b7`, which is upstream `v2.0.0`
plus two post-release commits on `v2`: a release-manifest checksum fix and dependency/stdlib security bumps.

---

## TT-001: proxy groups (`zabbix_proxygroup` resource, data source, API client)

- **Intent.** Manage Zabbix 7.0+ proxy groups: `name`, `failover_delay`,
  `min_online`, `description`.
- **Why we need it.** TT-Public-Cloud
  `aws/accounts/tenstorrent-it/monitoring/zabbix-configuration/proxies.tf`
  manages one proxy group per site. tpretz v2 has no proxy group support at all.
- **Files.**
  - New: `internal/zabbix/proxygroup.go`, `provider/resource_proxygroup.go`,
    `provider/tt_proxygroup_test.go`, `examples/{resources,data-sources}/zabbix_proxygroup/*`.
  - Upstream edits: `provider/provider.go`, two registration lines.
  - Generated: `docs/{resources,data-sources}/proxygroup.md`.
- **Conflict likelihood: low.** The only upstream edits are two lines in
  `provider.go`'s resource/data maps. They conflict only if upstream edits the
  neighbouring map lines (for example, adds a resource next to `zabbix_proxy`).
  The fix is to keep both lines. `tt_proxygroup_test.go` adds entries to
  upstream's `updateOwner`/`removalOwner` maps and the sweeper registry from
  `init()`. If upstream renames those maps or helpers (`serverObject`,
  `testAccCheckServerAttrs`, `expectUpdate`, `testAccCheckAllDestroyed`,
  `testSweepAPI`, `sweepTestPrefix`), this file fails to *compile*. It does not
  conflict, so the break is loud, and the fix is to rename the references.
- **Tests that prove it.** `TestAccResourceProxyGroup` covers S9a minimum,
  U1/U2 in-place updates against a server re-read, import, user macros, and the
  R1/S9d defaults reverting and description clearing. Also
  `TestAccDataSourceProxyGroup`, and `TestAccResourceProxyGroupUnsupported`
  (pre-7.0 only). The U1/R1 coverage guards (`TestUpdateCoverageComplete`,
  `TestRemovalCoverageComplete`) fail if an attribute loses its owner.
- **Drop when** upstream ships a proxy group resource. Then migrate state with
  `terraform state rm` + `import`, or with `moved` blocks if upstream keeps the
  name `zabbix_proxygroup`, and delete this patch. TT-002 to TT-004 are
  reassessed at the same time.

## TT-002: proxy group membership on `zabbix_proxy`

- **Intent.** Add `proxy_groupid` (default `"0"`), `local_address`, and
  `local_port` (default `"10051"`) to `zabbix_proxy` and `data.zabbix_proxy`.
- **Why we need it.** Every proxy in TPC `proxies.tf` sets all three. Zabbix
  (verified on 7.4.16) requires `local_address` inside a group, and requires it
  empty, with `local_port` = 10051, outside one.
- **Files.**
  - New: `internal/zabbix/proxy_tt.go`, `provider/resource_proxy_tt.go`,
    `provider/tt_proxy_membership_test.go`.
  - Upstream edits (6 tagged lines):
    - `internal/zabbix/proxy.go`: an embedded `proxyGroupMembership` in `Proxy`
      and in `proxyResponse`; one call each in `proxyFromResponse` and
      `proxyParams`.
    - `provider/resource_proxy.go`: one call in `buildProxyObject`, one in
      `proxyRead`.
    - `provider/provider.go`: `ttResourceProxy()` replaces `resourceProxy()`.
  - Also extends our own `examples/resources/zabbix_proxygroup/resource.tf`.
- **Conflict likelihood: medium.** `internal/zabbix/proxy.go` and
  `provider/resource_proxy.go` are areas upstream actively maintains. Each hook
  is one line next to a stable anchor:
  - `p.AllowedAddresses = w.AllowedAddresses` in the 7.0 branch of `proxyFromResponse`;
  - `out["allowed_addresses"]` in the 7.0 branch of `proxyParams`;
  - the `address/port/allowed` locals in `buildProxyObject`;
  - `d.Set("port", port)` in `proxyRead`.

  If an anchor moves, put the hook back at the semantically equivalent place.
  Schema attributes are added from `init()` to `proxySchemaBase`; if upstream
  renames that map, the build fails.
- **Tests that prove it.** `TestAccResourceProxyGroupMembership` covers the
  TPC-shaped active cert proxy in a group, U1/U2 on the endpoint, import, R1
  leave-the-group via a server re-read, and the data source.
  `TestAccResourceProxyGroupMembershipNegative` covers the rejected combinations.
- **Drop when** upstream models proxy group membership on proxies (implies TT-001).

## TT-003: `proxy_groupid` and `monitored_by` on `zabbix_host`

- **Intent.**
  - `proxy_groupid` (default `"0"`, mutually exclusive with `proxyid`).
  - `monitored_by`, Optional+Computed and derived from
    `proxyid`/`proxy_groupid` at plan time. Setting it is only a cross-check:
    a value that disagrees is a plan-time error. Zabbix's numeric codes
    `"0"/"1"/"2"` are accepted as aliases and kept as written.
  - Both attributes are also reported by `data.zabbix_host`.
- **Why we need it.**
  - it/terraform `modules/proxmox-host-config/{bmc,proxmox_http}_zabbix_hosts.tf`
    write `monitored_by = "1"`.
  - v0.18 wrote `monitored_by = "1"` into the state of every host.
  - Hosts monitored by proxy groups are the reason the groups exist.
  - Upstream's client already sends `monitored_by=2` for a group (`prepHosts`),
    but nothing exposes it.
- **Files.**
  - New: `provider/resource_host_tt.go`, `provider/tt_host_proxygroup_test.go`.
  - Upstream edits (4 tagged lines):
    - `provider/resource_host.go`: one call in `buildHostObject`, one in `hostRead`.
    - `provider/provider.go`: `ttResourceHost()` and `ttDataHost()`.
  - Also extends our proxygroup examples.
- **Conflict likelihood: medium.** `resource_host.go` is upstream's most-edited
  file. The anchors are `item.GroupIds = buildHostGroupIds(...)` in
  `buildHostObject` and `d.Set("proxyid", host.ProxyID)` in `hostRead`. The
  CustomizeDiff is chained onto upstream's, so a new upstream CustomizeDiff
  composes rather than conflicts. **Watch for** upstream adding its own
  `monitored_by` or `proxy_groupid` to `hostSchemaBase`. Our `init()` would then
  silently overwrite theirs, so compare the two and drop ours.
- **Tests that prove it.**
  - `TestAccResourceHostProxyGroup`:
    - group- and proxy-monitored hosts, including `monitored_by = "1"`;
    - import;
    - swap;
    - contradiction rejected at plan;
    - R1 back to the server via a re-read;
    - data source.
  - `TestAccResourceHostProxyGroupNegative`.
  - The R2 decision is recorded in `removalComputed`.
- **Drop when** upstream exposes `proxy_groupid` on hosts. Keep the
  `monitored_by` alias part only until it/terraform has removed its
  `monitored_by = "1"` lines and all state has been refreshed once.

## TT-004: import `zabbix_proxy` and `zabbix_proxygroup` by name

- **Intent.** An import id made only of digits is an id, as upstream treats it.
  Anything else is resolved by technical name or group name, and zero or several
  matches is an error.
- **Why we need it.** TPC `zabbix-configuration/imports.tf` imports proxies and
  proxy groups by name, which v0.18 supported.
- **Files.**
  - New: `provider/import_by_name_tt.go`, `provider/tt_import_by_name_test.go`.
  - Edits to our own TT-001/TT-002 files (one importer line each).
  - Upstream edit: `examples/resources/zabbix_proxy/import.sh` (+4 tagged lines).
- **Conflict likelihood: low.** The only upstream-file edit is an appended
  example.
- **Tests that prove it.** `TestAccImportByName`.
- **Drop when** TPC's `imports.tf` blocks are deleted or rewritten with numeric
  ids. They are no-ops once the resources are in state, so this is mainly about
  re-imports.

## TT-005: toolchain and dependency advisories

- **Intent.** Raise the Go toolchain and `golang.org/x/net` past published
  vulnerabilities that upstream's pins still carry.
- **Why we need it.** govulncheck v1.1.4 on upstream's pins (`go 1.25.13`,
  x/net `v0.58.0`) reports 10 reachable vulnerabilities, plus 4 in imported
  packages and 2 in required modules:
  - stdlib, fixed in Go 1.26.9/1.26.10: GO-2026-6809, 6613, 6605, 6609, 6608,
    6607, 6604, 6600, 6599;
  - x/net, fixed in 0.60.0: GO-2026-6617, 6612, 6611, 6610, 6603;
  - x/net, fixed in 0.62.0: GO-2026-6866, 6860.

  Our pins decide what the released binaries are built with, because every
  workflow uses `go-version-file: go.mod`. After this patch, govulncheck
  v1.1.4 and v1.9.0 both report no vulnerabilities.
- **Files.** Upstream: `go.mod` (`go 1.26.10`; x/net `v0.62.0`, plus the
  x/crypto, mod, sync, sys, text and tools minimums that x/net requires),
  `go.sum`, `.tool-versions`. Workflows are unchanged because they derive the
  version from `go.mod`. The `1.25.13` mentions in Makefile and
  `.github/dependabot.yml` comments are left as upstream wrote them.
- **Conflict likelihood: high** whenever upstream bumps dependencies, which is
  routine. **Never hand-merge `go.mod`/`go.sum`.** Take upstream's versions,
  then re-apply the intent:
  1. `go mod edit -go=<max(upstream, 1.26.10)>`
  2. set `.tool-versions` to the same;
  3. `go get golang.org/x/net@<max(upstream, v0.62.0)>`
  4. `go mod tidy`
  5. `govulncheck ./...`

  If upstream's pins are already at or above ours, the patch is empty: drop it.
  The skill describes this.
- **Tests that prove it.** `govulncheck ./...` reports nothing. Build, vet,
  unit tests and the full acceptance suite pass on the new toolchain, and
  `make check-toolchain` passes.
- **Drop when** upstream's `go.mod` reaches `go >= 1.26.10` and x/net
  `>= v0.62.0`, or later versions that govulncheck reports clean. Re-run
  govulncheck on every sync and extend this patch only for new findings. Do not
  churn other dependencies.

## TT-900: fork documentation

- **Intent.** Explain the fork and carry the consumer migration guide.
- **Files.** New: `PATCHES.md` (this file), `TT-FORK.md`, `TT-MIGRATING-v0.18.md`.
- **Conflict likelihood: none.** These are new files at paths upstream does not use.
- **Tests that prove it.** None; this patch is documentation only.
- **Drop when** never, while the fork exists.

## TT-901: upstream sync tooling

- **Intent.** Make each upstream sync a repeatable, reviewable procedure.
- **Files.** New:
  - `scripts/sync-upstream.sh`;
  - `.claude/skills/sync-upstream/SKILL.md`;
  - `.claude/CLAUDE.md`, a pointer to the skill;
  - `AGENTS.md`, the same pointer for non-Claude agents.

  Upstream's root `CLAUDE.md` is deliberately left untouched.
- **Conflict likelihood: none.**
- **Tests that prove it.**
  - The `tt-ci` job runs `shellcheck scripts/*.sh`.
  - Use `scripts/sync-upstream.sh --dry-run` to see what the next sync would do.
- **Drop when** never, while the fork exists.

## TT-902: CI

- **Intent.**
  - `.github/workflows/tt-ci.yml` runs on every PR and on pushes to
    `main`/`upstream-v2`. It runs build, vet, gofmt, unit tests, docs-check,
    shellcheck, and acceptance tests on Zabbix 6.0, 7.0 and 7.4 using upstream's
    `docker-compose.test.yml` and Makefile targets.
  - `.github/workflows/tt-upstream-watch.yml` runs daily. It reads tpretz's
    releases and tags (read-only) and opens or updates one tracking issue in
    this repository when a newer upstream release exists.
- **Files.** New: those two workflow files.
- **Conflict likelihood: none.** Upstream's own `ci.yml`, `nightly.yml` and
  `release.yml` are untouched and keep working. See TT-FORK.md for their
  behaviour on our branch names.
- **Tests that prove it.** The workflows run on this PR.
- **Drop when** never, while the fork exists.
