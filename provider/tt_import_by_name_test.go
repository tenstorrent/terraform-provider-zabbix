// TT-PATCH: TT-004 -- whole file is a tenstorrent addition (import by name for
// zabbix_proxy and zabbix_proxygroup). See PATCHES.md.

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccImportByName imports a proxy group and a proxy by name, the form
// TT-Public-Cloud's zabbix-configuration/imports.tf uses, and checks that a
// name matching nothing is an error rather than an empty import.
func TestAccImportByName(t *testing.T) {
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
resource "zabbix_proxygroup" "testpg" {
	name        = "test-import-pg"
	description = "imported by name"
}

resource "zabbix_proxy" "testproxy" {
	name          = "test-import-zp.example.com"
	proxy_groupid = zabbix_proxygroup.testpg.id
	local_address = "test-import-zp.example.com"
}
`,
			},
			{
				ResourceName:      "zabbix_proxygroup.testpg",
				ImportState:       true,
				ImportStateId:     "test-import-pg",
				ImportStateVerify: true,
			},
			{
				ResourceName:            "zabbix_proxy.testproxy",
				ImportState:             true,
				ImportStateId:           "test-import-zp.example.com",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"tls_psk_identity", "tls_psk"},
			},
			{ // by id still works on both
				ResourceName:      "zabbix_proxygroup.testpg",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:            "zabbix_proxy.testproxy",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"tls_psk_identity", "tls_psk"},
			},
			{
				ResourceName:  "zabbix_proxygroup.testpg",
				ImportState:   true,
				ImportStateId: "test-import-no-such-group",
				ExpectError:   regexp.MustCompile(`no proxy group named "test-import-no-such-group"`),
			},
			{
				ResourceName:  "zabbix_proxy.testproxy",
				ImportState:   true,
				ImportStateId: "test-import-no-such-proxy",
				ExpectError:   regexp.MustCompile(`no proxy named "test-import-no-such-proxy"`),
			},
		},
	})
}
