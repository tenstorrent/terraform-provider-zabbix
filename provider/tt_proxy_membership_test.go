// TT-PATCH: TT-002 -- whole file is a tenstorrent addition (proxy group
// membership on zabbix_proxy). See PATCHES.md.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/tpretz/terraform-provider-zabbix/internal/zabbix"
)

func init() {
	for _, attr := range []string{"proxy_groupid", "local_address", "local_port"} {
		updateOwner["zabbix_proxy."+attr] = "TestAccResourceProxyGroupMembership"
	}
	removalOwner["zabbix_proxy.proxy_groupid"] = "TestAccResourceProxyGroupMembership"
	removalOwner["zabbix_proxy.local_port"] = "TestAccResourceProxyGroupMembership"
}

// serverProxyMembership re-reads a proxy's group membership through the typed
// client, as upstream's serverProxy does for the rest of the object.
func serverProxyMembership(api *zabbix.API, id string) (map[string]interface{}, error) {
	res, err := api.ProxiesGet(zabbix.Params{"proxyids": id})
	if err != nil {
		return nil, err
	}
	if len(res) != 1 {
		return nil, fmt.Errorf("proxy.get returned %d proxies for id %s, want 1", len(res), id)
	}
	p := res[0]
	return map[string]interface{}{
		"operating_mode": fmt.Sprint(int(p.Mode)),
		"tls_accept":     string(p.TLSAccept),
		"tls_issuer":     p.TLSIssuer,
		"tls_subject":    p.TLSSubject,
		"proxy_groupid":  p.ProxyGroupID,
		"local_address":  p.LocalAddress,
		"local_port":     p.LocalPort,
	}, nil
}

// TestAccResourceProxyGroupMembership is the shape TT-Public-Cloud's
// zabbix-configuration/proxies.tf runs: an active, certificate-encrypted
// proxy in a proxy group with its agent-redirect endpoint set. Then the
// endpoint is changed in place (U1/U2), the proxy is imported, and it leaves
// the group by deleting the three lines (R1, asserted on a server re-read).
func TestAccResourceProxyGroupMembership(t *testing.T) {
	const (
		pg    = "zabbix_proxygroup.testpg"
		proxy = "zabbix_proxy.testproxy"
	)

	group := `
resource "zabbix_proxygroup" "testpg" {
	name           = "test-proxygroup"
	failover_delay = "1m"
	min_online     = "1"
	description    = "test site"
}
`
	inGroup := func(addr, port string) string {
		return group + `
resource "zabbix_proxy" "testproxy" {
	name           = "test-zp-1.example.com"
	description    = "test proxy 1"
	operating_mode = "active"
	local_address  = "` + addr + `"
	local_port     = "` + port + `"
	tls_connect    = "unencrypted"
	tls_accept     = "cert"
	tls_issuer     = "CN=test-zabbix,O=Test"
	tls_subject    = "CN=test-zp-1.example.com,O=Zabbix Proxy"
	proxy_groupid  = zabbix_proxygroup.testpg.id
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
			{ // the TT-Public-Cloud shape
				Config: inGroup("test-zp-1.example.com", "10051"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(proxy, "proxy_groupid", pg, "id"),
					resource.TestCheckResourceAttr(proxy, "local_address", "test-zp-1.example.com"),
					resource.TestCheckResourceAttr(proxy, "local_port", "10051"),
					resource.TestCheckResourceAttr(proxy, "tls_accept", "cert"),
					testAccCheckServerAttrs(proxy, serverProxyMembership, map[string]string{
						"operating_mode": "0",
						"local_address":  "test-zp-1.example.com",
						"local_port":     "10051",
						"tls_accept":     "4",
						"tls_issuer":     "CN=test-zabbix,O=Test",
						"tls_subject":    "CN=test-zp-1.example.com,O=Zabbix Proxy",
					}),
					testAccCheckServerAttrPair(proxy, serverProxyMembership, "proxy_groupid", pg),
				),
			},
			{ // U1/U2 on the agent-redirect endpoint
				Config:           inGroup("10.0.0.5", "10052"),
				ConfigPlanChecks: expectUpdate(proxy),
				Check: testAccCheckServerAttrs(proxy, serverProxyMembership, map[string]string{
					"local_address": "10.0.0.5",
					"local_port":    "10052",
				}),
			},
			{ // import round trip at full size
				ResourceName:            proxy,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"tls_psk_identity", "tls_psk"},
			},
			{ // R1: leave the group by deleting all three lines. local_port's
				// default has to reach the server, and so does the empty
				// local_address Zabbix requires outside a group.
				Config: group + `
resource "zabbix_proxy" "testproxy" {
	name           = "test-zp-1.example.com"
	description    = "test proxy 1"
	operating_mode = "active"
	tls_accept     = "cert"
	tls_issuer     = "CN=test-zabbix,O=Test"
	tls_subject    = "CN=test-zp-1.example.com,O=Zabbix Proxy"
}
`,
				ConfigPlanChecks: expectUpdate(proxy),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(proxy, "proxy_groupid", "0"),
					resource.TestCheckResourceAttr(proxy, "local_address", ""),
					resource.TestCheckResourceAttr(proxy, "local_port", "10051"),
					testAccCheckServerAttrs(proxy, serverProxyMembership, map[string]string{
						"proxy_groupid": "0",
						"local_address": "",
						"local_port":    "10051",
					}),
				),
			},
			{ // the data source reports membership too
				Config: group + `
resource "zabbix_proxy" "testproxy" {
	name          = "test-zp-1.example.com"
	local_address = "10.0.0.6"
	proxy_groupid = zabbix_proxygroup.testpg.id
}

data "zabbix_proxy" "found" {
	name       = zabbix_proxy.testproxy.name
	depends_on = [zabbix_proxy.testproxy]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.zabbix_proxy.found", "proxy_groupid", pg, "id"),
					resource.TestCheckResourceAttr("data.zabbix_proxy.found", "local_address", "10.0.0.6"),
					resource.TestCheckResourceAttr("data.zabbix_proxy.found", "local_port", "10051"),
				),
			},
		},
	})
}

// TestAccResourceProxyGroupMembershipNegative covers the combinations Zabbix
// rejects, which the provider rejects first with a message naming the
// attribute.
func TestAccResourceProxyGroupMembershipNegative(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipWithoutProxyGroups(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      ttCheckAllDestroyed,
		Steps: []resource.TestStep{
			{ // a proxy in a group without local_address
				Config: `
resource "zabbix_proxygroup" "testpg" {
	name = "test-proxygroup-neg"
}

resource "zabbix_proxy" "testproxy" {
	name          = "test-proxy-neg"
	proxy_groupid = zabbix_proxygroup.testpg.id
}
`,
				ExpectError: regexp.MustCompile(`local_address is required for a proxy in a proxy group`),
			},
			{ // local_address without a group
				Config: `
resource "zabbix_proxy" "testproxy" {
	name          = "test-proxy-neg"
	local_address = "10.0.0.1"
}
`,
				ExpectError: regexp.MustCompile(`local_address applies to a proxy in a proxy group only`),
			},
			{ // a non-default local_port without a group
				Config: `
resource "zabbix_proxy" "testproxy" {
	name       = "test-proxy-neg"
	local_port = "10052"
}
`,
				ExpectError: regexp.MustCompile(`local_port applies to a proxy in a proxy group only`),
			},
		},
	})
}
