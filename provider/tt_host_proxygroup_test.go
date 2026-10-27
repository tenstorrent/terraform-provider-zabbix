// TT-PATCH: TT-003 -- whole file is a tenstorrent addition (proxy_groupid and
// monitored_by on zabbix_host). See PATCHES.md.

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func init() {
	updateOwner["zabbix_host.proxy_groupid"] = "TestAccResourceHostProxyGroup"
	updateOwner["zabbix_host.monitored_by"] = "TestAccResourceHostProxyGroup"
	removalOwner["zabbix_host.proxy_groupid"] = "TestAccResourceHostProxyGroup"
	removalComputed["zabbix_host.monitored_by"] = "intended: not an independent setting -- Zabbix's monitored_by is " +
		"fully determined by proxyid and proxy_groupid, the client derives it from them on every write, and " +
		"hostMonitoringCustomizeDiff plans the derived value whenever the configuration is silent. Deleting the " +
		"line therefore leaves the correct value, not a stale one; a configured value is only a cross-check. " +
		"Optional at all only so that v0.18 configurations writing monitored_by = \"1\" keep working; " +
		"TestAccResourceHostProxyGroup"
}

// TestAccResourceHostProxyGroup covers hosts monitored by a proxy group and by
// a proxy -- the latter with the `monitored_by = "1"` it/terraform's
// proxmox-host-config writes -- then swaps them, rejects a contradictory
// monitored_by at plan time, and returns both to the server by deleting the
// lines (R1, asserted on a server re-read).
func TestAccResourceHostProxyGroup(t *testing.T) {
	const (
		pg     = "zabbix_proxygroup.testpg"
		proxy  = "zabbix_proxy.testproxy"
		hostPG = "zabbix_host.testhost_pg"
		hostPX = "zabbix_host.testhost_px"
	)

	common := `
resource "zabbix_hostgroup" "testgrp" {
	name = "test-group"
}

resource "zabbix_proxygroup" "testpg" {
	name = "test-proxygroup"
}

resource "zabbix_proxy" "testproxy" {
	name = "test-proxy-hostpg"
}
`
	hosts := func(pgHost, pxHost string) string {
		return common + `
resource "zabbix_host" "testhost_pg" {
	host   = "testhost-pg"
	groups = [zabbix_hostgroup.testgrp.id]
` + pgHost + `
}

resource "zabbix_host" "testhost_px" {
	host   = "testhost-px"
	groups = [zabbix_hostgroup.testgrp.id]
` + pxHost + `
}
`
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipWithoutProxyGroups(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      ttCheckAllDestroyed,
		Steps: []resource.TestStep{
			{ // one host per kind, the proxy one in it/terraform's spelling
				Config: hosts(
					`proxy_groupid = zabbix_proxygroup.testpg.id`,
					`proxyid      = zabbix_proxy.testproxy.id
	monitored_by = "1"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(hostPG, "proxy_groupid", pg, "id"),
					resource.TestCheckResourceAttr(hostPG, "proxyid", "0"),
					resource.TestCheckResourceAttr(hostPG, "monitored_by", "proxy_group"),
					testAccCheckServerAttrs(hostPG, serverHost, map[string]string{
						"monitored_by": "2",
						"proxyid":      "0",
					}),
					testAccCheckServerAttrPair(hostPG, serverHost, "proxy_groupid", pg),

					resource.TestCheckResourceAttrPair(hostPX, "proxyid", proxy, "id"),
					resource.TestCheckResourceAttr(hostPX, "proxy_groupid", "0"),
					// the v0.18 alias is kept as written, not rewritten
					resource.TestCheckResourceAttr(hostPX, "monitored_by", "1"),
					testAccCheckServerAttrs(hostPX, serverHost, map[string]string{
						"monitored_by":  "1",
						"proxy_groupid": "0",
					}),
				),
			},
			{ // import reports the group membership
				ResourceName:            hostPG,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"tls_psk_identity", "tls_psk"},
			},
			{ // swap them. testhost_pg leaves the group for the proxy with its
				// proxy_groupid line deleted (R1: the "0" default must reach the
				// server); testhost_px joins the group with monitored_by given
				// by name.
				Config: hosts(
					`proxyid = zabbix_proxy.testproxy.id`,
					`proxy_groupid = zabbix_proxygroup.testpg.id
	monitored_by  = "proxy_group"`),
				ConfigPlanChecks: expectUpdate(hostPG, hostPX),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(hostPG, "monitored_by", "proxy"),
					resource.TestCheckResourceAttr(hostPG, "proxy_groupid", "0"),
					testAccCheckServerAttrs(hostPG, serverHost, map[string]string{
						"monitored_by":  "1",
						"proxy_groupid": "0",
					}),
					testAccCheckServerAttrPair(hostPG, serverHost, "proxyid", proxy),

					resource.TestCheckResourceAttr(hostPX, "monitored_by", "proxy_group"),
					resource.TestCheckResourceAttr(hostPX, "proxyid", "0"),
					testAccCheckServerAttrs(hostPX, serverHost, map[string]string{
						"monitored_by": "2",
						"proxyid":      "0",
					}),
				),
			},
			{ // a contradictory monitored_by is refused at plan time
				Config: hosts(
					`proxyid      = zabbix_proxy.testproxy.id
	monitored_by = "server"`,
					`proxy_groupid = zabbix_proxygroup.testpg.id`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`monitored_by = "server" contradicts`),
			},
			{ // R1: delete every line; both go back to the server, and
				// monitored_by re-derives without being written
				Config:           hosts(``, ``),
				ConfigPlanChecks: expectUpdate(hostPG, hostPX),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(hostPG, "monitored_by", "server"),
					resource.TestCheckResourceAttr(hostPX, "monitored_by", "server"),
					testAccCheckServerAttrs(hostPX, serverHost, map[string]string{
						"monitored_by":  "0",
						"proxy_groupid": "0",
						"proxyid":       "0",
					}),
				),
			},
			{ // the data source reports both
				Config: hosts(``, `proxy_groupid = zabbix_proxygroup.testpg.id`) + `
data "zabbix_host" "found" {
	host       = "testhost-px"
	depends_on = [zabbix_host.testhost_px]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.zabbix_host.found", "proxy_groupid", pg, "id"),
					resource.TestCheckResourceAttr("data.zabbix_host.found", "monitored_by", "proxy_group"),
				),
			},
		},
	})
}

// TestAccResourceHostProxyGroupNegative: a host cannot name both.
func TestAccResourceHostProxyGroupNegative(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipWithoutProxyGroups(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      ttCheckAllDestroyed,
		Steps: []resource.TestStep{
			{
				Config: `
resource "zabbix_hostgroup" "testgrp" {
	name = "test-group"
}

resource "zabbix_proxygroup" "testpg" {
	name = "test-proxygroup-neg"
}

resource "zabbix_host" "testhost" {
	host          = "testhost-neg"
	groups        = [zabbix_hostgroup.testgrp.id]
	proxyid       = "12345"
	proxy_groupid = zabbix_proxygroup.testpg.id
}
`,
				ExpectError: regexp.MustCompile(`proxyid \(12345\) and proxy_groupid \(\d+\) are mutually exclusive`),
			},
		},
	})
}
