# Migrating TT-Public-Cloud and it/terraform from tenstorrent/zabbix v0.18.1 to the v2 build

<!-- TT-PATCH: TT-900 -- consumer-facing; kept in sync with the copy handed to the consumer repos. -->

Target build: tpretz `v2` (v2.0.0 + 2 post-release commits, `f6077b7`) plus the
tenstorrent patches TT-001 to TT-005. See `PATCHES.md`: proxy groups, proxy-group
membership on proxies and hosts, `monitored_by`, import by name, and a
Go 1.26.10 / x/net security bump. That bump changes nothing for consumers.
**Nothing below has been applied** to either consumer repo.

**Version.** The proposed first release is **`2.0.0-tt.1`**. The tag scheme is
`v<upstream>-tt.<n>`, explained in `TT-FORK.md`. It is a semver pre-release, so
consumers must pin it **exactly**: `version = "2.0.0-tt.1"`. `~> 2.0` will not
select it. Both repos already pin exactly. Nothing has been released yet. This
document writes the version as `2.0.0-tt.1`.

## TL;DR

| Repo | Required edits | Optional cleanup |
|---|---|---|
| TT-Public-Cloud `aws/accounts/tenstorrent-it/monitoring/zabbix-configuration` | version pin; `name` on `zabbix_proxy`; TLS value renames (`none`→`unencrypted`, `certificate`→`cert`); refresh `.terraform.lock.hcl` | none |
| it/terraform | version pin in **one** mixin, then regenerate `common/common-providers-*.tf` | delete the two `monitored_by = "1"` lines |

Neither repo uses any removed resource. `zabbix_application`, `zabbix_item_aggregate`,
`zabbix_template.groups`, item `applications`, and the legacy SNMP item attributes are
all unused. Neither repo indexes `interface[N]`, `macro[N]` or `item[N]` anywhere,
so the change from lists to sets breaks nothing.

### Rehearsed, not just reasoned

I built a reduced version of both repos' Zabbix configuration. It was a local
rehearsal and is not kept in this repo. It has the TPC hostgroups, proxy group,
proxy, ping host, simple item and trigger, plus the it/terraform agent, BMC, Proxmox-HTTP
hosts and the tt-inferencex HTTP item and trigger. I applied it with the **published
v0.18.1** against Zabbix 7.4.16. The proxy group and proxy were pre-created and imported
**by name**, as `imports.tf` does.

I then switched to the v2 build with the edits below. The first plan against the
v0.18.1 state was **"No changes"**. Applying it upgraded the state silently: host state
went from schema v0 to v1, and `min_online` changed from a number to a string. After that:

- in-place updates worked: proxy description, and moving a host from a proxy to a proxy group;
- a further plan was empty;
- `destroy` was clean.

The unmodified v0.18.1 config fails at plan time, before anything is touched, with the
errors quoted in section 1.

---

## 1. TT-Public-Cloud: `aws/accounts/tenstorrent-it/monitoring/zabbix-configuration`

### 1a. `terraform.tf`: version pin

```diff
     zabbix = {
       source  = "tenstorrent/zabbix"
-      version = "0.18.1"
+      version = "2.0.0-tt.1"
     }
```

Then run `terraform init -upgrade` and commit the updated `.terraform.lock.hcl`.
It is tracked in git here.

### 1b. `proxies.tf`: `zabbix_proxy` needs `name`, and the TLS vocabulary changed

v0.18.1's `zabbix_proxy` had no `name`. The `host` line is commented out because it only
applied to passive proxies. v2 requires `name`. Without it:

```
Error: Missing required argument
  on proxies.tf line 18, in resource "zabbix_proxy" "proxies":
The argument "name" is required, but no definition was found.
```

The v2 TLS values match `zabbix_host`'s: `unencrypted` / `psk` / `cert`. v0.18 used
`none` / `psk` / `certificate`. With the old values:

```
Error: expected tls_connect to be one of ["cert" "psk" "unencrypted"], got none
Error: expected tls_accept to be one of ["cert" "psk" "unencrypted"], got certificate
```

`proxies.yml` uses `tls_connect: none` / `tls_accept: certificate` on all 10 proxies. The
smallest change maps the values in HCL and leaves the YAML alone:

```diff
 locals {
   proxy_groups = yamldecode(file("${path.module}/proxies.yml"))["proxy_groups"]
   proxies      = yamldecode(file("${path.module}/proxies.yml"))["proxies"]
+  # proxies.yml keeps the v0.18 TLS vocabulary; v2 uses zabbix_host's.
+  tls_mode = { none = "unencrypted", psk = "psk", certificate = "cert" }
 }
@@
 resource "zabbix_proxy" "proxies" {
   for_each = { for proxy in local.proxies : proxy.name => proxy }

-  # Required attributes
-  # host           = each.value.name
-  # allowed_addresses = each.value.name
+  name           = each.value.name
   description    = each.value.description
   operating_mode = each.value.operating_mode
   local_address  = each.value.name
   local_port     = "10051"

   # Optional TLS attributes
-  tls_connect = try(each.value.tls_connect, "none")
-  tls_accept  = try(each.value.tls_accept, null)
+  tls_connect = local.tls_mode[try(each.value.tls_connect, "none")]
+  tls_accept  = local.tls_mode[try(each.value.tls_accept, "none")]
   tls_issuer  = try(each.value.tls_issuer, null)
   tls_subject = try(each.value.tls_subject, null)
```

You could edit `proxies.yml` instead (`none`→`unencrypted`, `certificate`→`cert`).

`zabbix_proxygroup` needs **no change**. `min_online` is now a string, because Zabbix
accepts a user macro there. The YAML already supplies `"1"`, and the old numeric state
converts on its own.

**State.** Nothing to do by hand. v0.18.1's proxy state has `host = ""`, no `name`, and
`tls_accept = "certificate"`. The first refresh under v2 rewrites all three from the
server, and the plan is empty when the config matches the server. **Read the first plan
carefully anyway.** v0.18.1's proxy resource sent the *pre-7.0* property names (`host`,
`status`) to a 7.x server, so whether `operating_mode` and the other settings ever
reached Zabbix was not reliable. v2 reads the real values, so any difference you see is
genuine drift that v0.18 hid, not something the upgrade introduced.

### 1c. `imports.tf`: no change

The import blocks use **names** as ids (`id = "tyo1"`, `id = "yyz2-zp-1.yyz2.tenstorrent.com"`).
The v2 build imports `zabbix_proxygroup` and `zabbix_proxy` by name or by numeric id; an
all-digit id is always treated as an id. Upstream v2 alone would **reject** these blocks.
Import blocks for addresses already in state do nothing, so they are harmless either way.

### 1d. `proxy-mesh-ping.tf`, `hostgroups.tf`: no change

- `zabbix_host.proxy_ping_hosts`: `proxyid`, `groups`, `enabled`, no interface. v2 allows
  a host without an interface. v0.18 wrote `monitored_by = "1"` into state by default; v2
  derives the same value and reports no diff (rehearsed).
- `zabbix_item_simple`: every attribute is unchanged in v2.
- `zabbix_trigger`: `manual_close = "0"` is still accepted, because Terraform converts
  `"0"` to `false`. `dependencies = []` and `priority = "warn"` are unchanged. v2 rebuilds
  the expression from the server rather than using `expandExpression`. In the rehearsal,
  `icmppingloss[ip, 40, 400]` (note the spaces) came back byte-identical.

### 1e. Disabled files (`websites.tf.disabled`, `zabbix-aws-ping.tf.disabled`)

These are compatible as written. They use `monitored_by = "1"` (accepted as an alias),
`interface.port = "10050"` (now a number; the string converts), and item `description`
(present in v2). Apply the same TLS and `name` changes if any proxy resources are added there.

---

## 2. it/terraform

### 2a. Version pin: one source file, then regenerate

All 20 `common/common-providers-*.tf` files are generated by `common/Makefile` from
`common/mixins/terraform-provider-versions.tf.in`. About 346 deployments symlink one of
those files, so the pin affects **every** deployment, whether it uses Zabbix or not.

```diff
--- a/common/mixins/terraform-provider-versions.tf.in
+++ b/common/mixins/terraform-provider-versions.tf.in
@@ -45,4 +45,4 @@
     zabbix = {
       source  = "tenstorrent/zabbix"
-      version = "0.18.1"
+      version = "2.0.0-tt.1"
     }
```

Then run `make -C common` and commit the regenerated `common/common-providers-*.tf`.
No `.terraform.lock.hcl` is committed in this repo, but every Atlantis or operator
workspace needs `terraform init -upgrade` once.
`modules/{ansibilize-v2,proxmox-host-config,kea-dhcp-proxmox}/terraform.tf` give
`source` without a `version`, so they need no change.

### 2b. `modules/ansibilize-v2/zabbix_registration.tf`: no change

`zabbix_host.agent` uses `proxyid`, `groups`, `templates`, `tag`, one `interface`
(`main`, `dns`, `type = "agent"`), and a dynamic `macro`. v2 supports all of these.
`interface` and `macro` are now sets, but nothing indexes them. On the first plan, the
host state upgrader (v0→v1) converts the interface list to a set automatically.
The `data.zabbix_proxy` lookup uses `name` (not the deprecated `host`), and the
`data.zabbix_hostgroup` and `data.zabbix_template` lookups are unchanged.

### 2c. `modules/proxmox-host-config/{bmc,proxmox_http}_zabbix_hosts.tf`: optional cleanup

`monitored_by = "1"` still works. The v2 build accepts Zabbix's codes `"0"`/`"1"`/`"2"`
as aliases and keeps them as written, so the plan is clean. v2 derives `monitored_by`
from `proxyid`/`proxy_groupid`, so the line adds nothing. Remove it at leisure:

```diff
--- a/modules/proxmox-host-config/bmc_zabbix_hosts.tf
+++ b/modules/proxmox-host-config/bmc_zabbix_hosts.tf
@@ -33,5 +33,4 @@
   proxyid      = data.zabbix_proxy.bmc_site_proxy[0].id
-  monitored_by = "1"
--- a/modules/proxmox-host-config/proxmox_http_zabbix_hosts.tf
+++ b/modules/proxmox-host-config/proxmox_http_zabbix_hosts.tf
@@ -52,5 +52,4 @@
   proxyid      = data.zabbix_proxy.proxmox_http_site_proxy[0].id
-  monitored_by = "1"
```

A value that **contradicts** `proxyid`, such as `monitored_by = "0"` with a proxy set,
is now a plan-time error rather than something sent to Zabbix.

The BMC host's `interface { type = "snmp" }` gets the same SNMP defaults as before:
`snmp_version = "2"` and `{$SNMP_COMMUNITY}`.

`modules/proxmox-host-config/cluster_zabbix_hostgroup.tf` (`zabbix_hostgroup`) needs no change.

### 2d. `deployments/tt-inferencex-aws1/monitoring.tf`: no change

`zabbix_item_http` uses `request_method`, `retrieve_mode`, `valuetype = "text"`,
`follow_redirects`, `verify_peer`, `verify_host`, `delay`, `timeout` and `history`.
All are unchanged. One v0.18 permanent diff goes away under v2: the rehearsal
showed v0.18.1 proposing `post_type = "raw" -> "body"` on every plan, and v2 planning
clean.

v2 also rebuilds trigger expressions in their source form instead of reading them
through `expandExpression`, which turns user macros into their values. So
`find(... "{$CERT.WEBSITE.RESPONSE}")` will keep matching even once that macro has a
value at a level the API resolves.

**A risk that was already there and that v2 does not change:** `data "zabbix_host"
"ttinferencex"` has `depends_on = [module.ansible]`. Any pending change in
`module.ansible` delays reading that data source until apply, which makes `hostid`
unknown. Item `hostid` is `ForceNew`, so the plan **replaces** `zabbix_item_http.site_response`.
Zabbix then **cascade-deletes `zabbix_trigger.site_response`**, which depends on the item,
and the trigger only comes back on the next apply. The rehearsal reproduced this.
Consider referencing `module.ansible`'s `zabbix_host.agent` id through a module output
instead of `depends_on`.

---

## 3. Behaviour changes to know about (no edits needed)

- **`serialize = true` is the default.** Mutating API calls run one at a time, to avoid
  real Zabbix race conditions; see upstream MIGRATING.md §9. The TPC ping mesh has
  hundreds of items and triggers, so a full re-create will be slower. Plan and refresh
  speed is unchanged.
- **Display name follows `host` renames** while `name` is unset or still equal to the
  old `host`. Both repos set `name` explicitly almost everywhere.
- **Back up state first** (`terraform state pull > backup.tfstate`). After a v2 apply,
  v0.18.1 can no longer read the state, because the host schema version moves to 1.
- **The minimum Zabbix version is 6.0**, and proxy groups need 7.0. Production is 7.x,
  and 7.4 is the target.

## 4. Things in our usage that upstream v2 does *not* support

All of these are tenstorrent patches (`PATCHES.md`). None is in tpretz `v2`. They are
the reason the fork still exists:

| Feature | Used by | Options |
|---|---|---|
| `zabbix_proxygroup` resource and data source | TPC `proxies.tf` | (a) keep it in our fork. (b) Upstreaming is not planned (policy). (c) Manage proxy groups outside Terraform (not recommended). |
| `zabbix_proxy.proxy_groupid` / `local_address` / `local_port` | TPC `proxies.tf` | same as above |
| `zabbix_host.proxy_groupid` | not used yet; needed when hosts move to proxy-group monitoring | same as above |
| `zabbix_host.monitored_by` | it/terraform proxmox-host-config (`"1"`) | Keep it (TT-003); or delete the two lines so TT-003 can shrink later. |
| Import by name (`zabbix_proxy`, `zabbix_proxygroup`) | TPC `imports.tf` | Keep it (TT-004); or switch the import ids to numeric so TT-004 can be dropped. |

The v0.18 additions that are *not* carried over, because v2 already does them better:

- item `description`;
- the `binary` value type;
- token auth;
- `selectHostGroups` on 7.2+;
- proxy `operating_mode` and TLS handling.

Nothing in our usage depends on v0.18's own versions of these.

## 5. Suggested rollout order

1. TPC `zabbix-configuration`, a single state. Back up state, apply 1a–1b, run
   `init -upgrade`, plan, and check the first plan line by line (see 1b).
2. One it/terraform deployment that uses Zabbix, for example a proxmox-host-config cluster.
   Change the mixin, but plan just that deployment first.
3. Roll the regenerated `common-providers-*.tf` to the rest of the deployments.

## Before any of this can start

- **A release must exist.** Someone has to tag `v2.0.0-tt.1` on the new `main`,
  once the fork's default branch has been switched. That needs the GPG signing
  secrets described in `TT-FORK.md`.
- **`go-zabbix-api` is no longer used.** v2 has its own internal client
  (`internal/zabbix`), so nothing in either consumer repo changes because of it.
