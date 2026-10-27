// TT-PATCH: TT-001 -- whole file is a tenstorrent addition (proxy groups). See PATCHES.md.
//
// Everything a tenstorrent patch needs from the shared test machinery is
// registered from here (and the other tt_*_test.go files) rather than by
// editing upstream's tables: the U1/R1/R2 coverage maps are package-level
// maps, so an init() can add to them, and the sweeper registry is the same.
// That keeps acc_update_test.go, acc_removal_test.go, acc_destroy_test.go
// and sweeper_test.go byte-identical to upstream.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/tpretz/terraform-provider-zabbix/internal/zabbix"
)

func init() {
	for _, attr := range []string{"name", "description", "failover_delay", "min_online"} {
		updateOwner["zabbix_proxygroup."+attr] = "TestAccResourceProxyGroup"
	}
	removalOwner["zabbix_proxygroup.failover_delay"] = "TestAccResourceProxyGroup"
	removalOwner["zabbix_proxygroup.min_online"] = "TestAccResourceProxyGroup"

	// A proxy group cannot be deleted while a proxy or a host still uses it.
	resource.AddTestSweepers("zabbix_proxygroup", &resource.Sweeper{
		Name:         "zabbix_proxygroup",
		Dependencies: []string{"zabbix_host", "zabbix_proxy"},
		F:            sweepProxyGroups,
	})
}

// skipWithoutProxyGroups aborts the calling test below Zabbix 7.0.
func skipWithoutProxyGroups(t *testing.T) {
	if testAccVersion(t) < zabbix.V70 {
		t.Skipf("proxy groups require Zabbix 7.0 or later (server reports %d)", testAccVersion(t))
	}
}

// serverProxyGroup re-reads a proxy group (Zabbix 7.0+).
var serverProxyGroup = serverObject("proxygroup.get", "proxy_groupids", nil)

// testAccProxyGroupExists is a no-op below 7.0, where no test can have
// created a proxy group.
func testAccProxyGroupExists(api *zabbix.API, id string) (bool, error) {
	if api.Config.Version < zabbix.V70 {
		return false, nil
	}
	res, err := api.ProxyGroupsGet(zabbix.Params{"proxy_groupids": []string{id}})
	return len(res) > 0, err
}

// ttCheckAllDestroyed is upstream's testAccCheckAllDestroyed plus the object
// types tenstorrent patches add. Every tt_* acceptance test uses it.
func ttCheckAllDestroyed(s *terraform.State) error {
	return resource.ComposeAggregateTestCheckFunc(
		testAccCheckAllDestroyed,
		testAccCheckDestroyed("zabbix_proxygroup", testAccProxyGroupExists),
	)(s)
}

// testAccCheckServerAttrPair re-reads addr's object from Zabbix and requires
// API property prop to equal the id of the resource at otherAddr -- U1 for a
// reference to another object, whose value no fixture can know in advance.
func testAccCheckServerAttrPair(addr string, get serverGetter, prop, otherAddr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		want, err := testAccStateID(s, otherAddr)
		if err != nil {
			return err
		}
		return testAccCheckServerAttrs(addr, get, map[string]string{prop: want})(s)
	}
}

func sweepProxyGroups(_ string) error {
	api, err := testSweepAPI()
	if err != nil {
		return err
	}
	if api.Config.Version < zabbix.V70 {
		return nil // no proxy groups before 7.0
	}

	groups, err := api.ProxyGroupsGet(zabbix.Params{
		"output":      []string{"proxy_groupid"},
		"search":      map[string]interface{}{"name": sweepTestPrefix},
		"startSearch": true,
	})
	if err != nil {
		return fmt.Errorf("listing proxy groups to sweep: %w", err)
	}
	if len(groups) == 0 {
		return nil
	}

	ids := make([]string, len(groups))
	for i, g := range groups {
		ids[i] = g.ProxyGroupID
	}
	if err := api.ProxyGroupsDeleteByIds(ids); err != nil {
		return fmt.Errorf("deleting %d proxy group(s): %w", len(ids), err)
	}
	return nil
}

// TestAccResourceProxyGroup covers the resource on its own: the documented
// minimum (S9a/S9c), every attribute changed in life against a server re-read
// (U1/U2), import, the two defaults reverting when their lines are deleted
// and the description clearing (R1/S9d), and the user-macro form Zabbix
// accepts for both numeric settings.
func TestAccResourceProxyGroup(t *testing.T) {
	const addr = "zabbix_proxygroup.testpg"

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			skipWithoutProxyGroups(t)
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      ttCheckAllDestroyed,
		Steps: []resource.TestStep{
			{ // S9a: a proxy group is a name and nothing else
				Config: `
resource "zabbix_proxygroup" "testpg" {
	name = "test-proxygroup"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", "test-proxygroup"),
					resource.TestCheckResourceAttr(addr, "failover_delay", "1m"),
					resource.TestCheckResourceAttr(addr, "min_online", "1"),
					resource.TestCheckResourceAttr(addr, "description", ""),
					testAccCheckServerAttrs(addr, serverProxyGroup, map[string]string{
						"name":           "test-proxygroup",
						"failover_delay": "1m",
						"min_online":     "1",
						"description":    "",
					}),
				),
			},
			{ // U1/U2: every attribute changed in place
				Config: `
resource "zabbix_proxygroup" "testpg" {
	name           = "test-proxygroup-renamed"
	failover_delay = "2m"
	min_online     = "2"
	description    = "managed by terraform"
}
`,
				ConfigPlanChecks: expectUpdate(addr),
				Check: testAccCheckServerAttrs(addr, serverProxyGroup, map[string]string{
					"name":           "test-proxygroup-renamed",
					"failover_delay": "2m",
					"min_online":     "2",
					"description":    "managed by terraform",
				}),
			},
			{ // import by id
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: true,
			},
			{ // user macros are accepted for both numeric settings
				Config: `
resource "zabbix_proxygroup" "testpg" {
	name           = "test-proxygroup-renamed"
	failover_delay = "{$PG.FAILOVER}"
	min_online     = "{$PG.MIN.ONLINE}"
	description    = "managed by terraform"
}
`,
				ConfigPlanChecks: expectUpdate(addr),
				Check: testAccCheckServerAttrs(addr, serverProxyGroup, map[string]string{
					"failover_delay": "{$PG.FAILOVER}",
					"min_online":     "{$PG.MIN.ONLINE}",
				}),
			},
			{ // R1/S9d: delete every optional line; the defaults must reach
				// the server and the description must clear
				Config: `
resource "zabbix_proxygroup" "testpg" {
	name = "test-proxygroup-renamed"
}
`,
				ConfigPlanChecks: expectUpdate(addr),
				Check: testAccCheckServerAttrs(addr, serverProxyGroup, map[string]string{
					"failover_delay": "1m",
					"min_online":     "1",
					"description":    "",
				}),
			},
		},
	})
}

func TestAccDataSourceProxyGroup(t *testing.T) {
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
	name           = "test-proxygroup-data"
	failover_delay = "30s"
	min_online     = "2"
	description    = "looked up"
}

data "zabbix_proxygroup" "found" {
	name       = "test-proxygroup-data"
	depends_on = [zabbix_proxygroup.testpg]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.zabbix_proxygroup.found", "id", "zabbix_proxygroup.testpg", "id"),
					resource.TestCheckResourceAttr("data.zabbix_proxygroup.found", "failover_delay", "30s"),
					resource.TestCheckResourceAttr("data.zabbix_proxygroup.found", "min_online", "2"),
					resource.TestCheckResourceAttr("data.zabbix_proxygroup.found", "description", "looked up"),
				),
			},
			{ // a name that matches nothing is an error, not an empty result
				Config: `
data "zabbix_proxygroup" "missing" {
	name = "test-proxygroup-does-not-exist"
}
`,
				ExpectError: regexp.MustCompile(`no proxy group found matching name = "test-proxygroup-does-not-exist"`),
			},
		},
	})
}

// TestAccResourceProxyGroupUnsupported runs only below 7.0, where it checks
// that a proxy group is refused with a message naming the requirement.
func TestAccResourceProxyGroupUnsupported(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if testAccVersion(t) >= zabbix.V70 {
				t.Skip("server has proxy groups; nothing to reject")
			}
		},
		ProviderFactories: testAccProviderFactories,
		CheckDestroy:      ttCheckAllDestroyed,
		Steps: []resource.TestStep{
			{
				Config: `
resource "zabbix_proxygroup" "testpg" {
	name = "test-proxygroup-old"
}
`,
				ExpectError: regexp.MustCompile(`proxy groups require Zabbix 7.0 or later`),
			},
		},
	})
}
