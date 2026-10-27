// TT-PATCH: TT-001 -- whole file is a tenstorrent addition (proxy groups). See PATCHES.md.

package provider

import (
	"errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/tpretz/terraform-provider-zabbix/internal/zabbix"
)

// Zabbix's own defaults for a new proxy group. The schema repeats them so
// that a group created without them plans clean, and so that deleting the
// line reverts the server to them (R1).
const (
	proxyGroupDefaultFailoverDelay = "1m"
	proxyGroupDefaultMinOnline     = "1"
)

// proxyGroupSchemaBase describes every attribute a proxy group has.
// resourceProxyGroup and dataProxyGroup each take a copy and adjust
// required/optional/computed.
var proxyGroupSchemaBase = map[string]*schema.Schema{
	"name": &schema.Schema{
		Type:         schema.TypeString,
		Description:  "Name of the proxy group, unique across the Zabbix server",
		ValidateFunc: validation.StringIsNotWhiteSpace,
	},
	"failover_delay": &schema.Schema{
		Type: schema.TypeString,
		Description: "How long a proxy must be offline (or back online) before the server moves its hosts " +
			"to another proxy in the group. A time unit between 10s and 15m, such as `30s` or `1m`, or a user macro",
	},
	"min_online": &schema.Schema{
		Type: schema.TypeString,
		Description: "Minimum number of online proxies for the group to be considered online, 1 to 1000, " +
			"or a user macro. A string because Zabbix accepts a macro here",
	},
	"description": &schema.Schema{
		Type:        schema.TypeString,
		Description: "Description of the proxy group",
	},
}

// resourceProxyGroup terraform proxy group resource entrypoint
func resourceProxyGroup() *schema.Resource {
	s := map[string]*schema.Schema{}
	for k, v := range proxyGroupSchemaBase {
		c := *v
		switch k {
		case "name":
			c.Required = true
		default:
			c.Optional = true
		}
		s[k] = &c
	}
	s["failover_delay"].Default = proxyGroupDefaultFailoverDelay
	s["min_online"].Default = proxyGroupDefaultMinOnline

	return &schema.Resource{
		Description: "Manages a Zabbix proxy group: a set of proxies that share the monitoring of the hosts " +
			"assigned to the group, with automatic failover between them. Requires Zabbix 7.0 or later.",
		Create: resourceProxyGroupCreate,
		Read:   resourceProxyGroupRead,
		Update: resourceProxyGroupUpdate,
		Delete: resourceProxyGroupDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: s,
	}
}

// dataProxyGroup terraform proxy group data source entrypoint
func dataProxyGroup() *schema.Resource {
	s := map[string]*schema.Schema{}
	for k, v := range proxyGroupSchemaBase {
		c := *v
		switch k {
		case "name":
			c.Required = true
		default:
			c.Computed = true
			c.ValidateFunc = nil
		}
		s[k] = &c
	}

	return &schema.Resource{
		Description: "Looks up an existing Zabbix proxy group by name. Requires Zabbix 7.0 or later.",
		Read:        dataProxyGroupRead,
		Schema:      s,
	}
}

func buildProxyGroupObject(d *schema.ResourceData) zabbix.ProxyGroup {
	return zabbix.ProxyGroup{
		Name:          d.Get("name").(string),
		FailoverDelay: d.Get("failover_delay").(string),
		MinOnline:     d.Get("min_online").(string),
		Description:   d.Get("description").(string),
	}
}

// resourceProxyGroupCreate terraform create handler
func resourceProxyGroupCreate(d *schema.ResourceData, m interface{}) error {
	api := m.(*zabbix.API)

	items := zabbix.ProxyGroups{buildProxyGroupObject(d)}
	if err := api.ProxyGroupsCreate(items); err != nil {
		return err
	}

	log.Trace("created proxy group: %+v", items[0])
	d.SetId(items[0].ProxyGroupID)

	return resourceProxyGroupRead(d, m)
}

// proxyGroupRead shared read implementation
func proxyGroupRead(d *schema.ResourceData, m interface{}, params zabbix.Params) error {
	api := m.(*zabbix.API)

	groups, err := api.ProxyGroupsGet(params)
	if err != nil {
		return err
	}

	if len(groups) < 1 {
		d.SetId("")
		return nil
	}
	if len(groups) > 1 {
		return errors.New("multiple proxy groups found")
	}
	g := groups[0]

	log.Debug("Got proxy group: %+v", g)

	d.SetId(g.ProxyGroupID)
	d.Set("name", g.Name)
	d.Set("failover_delay", g.FailoverDelay)
	d.Set("min_online", g.MinOnline)
	d.Set("description", g.Description)

	return nil
}

// resourceProxyGroupRead terraform read handler
func resourceProxyGroupRead(d *schema.ResourceData, m interface{}) error {
	log.Debug("Lookup of proxy group with id %s", d.Id())

	return proxyGroupRead(d, m, zabbix.Params{
		"proxy_groupids": d.Id(),
	})
}

// dataProxyGroupRead read handler for the data source
func dataProxyGroupRead(d *schema.ResourceData, m interface{}) error {
	err := proxyGroupRead(d, m, zabbix.Params{
		"filter": map[string]interface{}{
			"name": d.Get("name"),
		},
	})
	if err != nil {
		return err
	}
	return dataSourceFound(d, "proxy group", "name")
}

// resourceProxyGroupUpdate terraform update handler
func resourceProxyGroupUpdate(d *schema.ResourceData, m interface{}) error {
	api := m.(*zabbix.API)

	g := buildProxyGroupObject(d)
	g.ProxyGroupID = d.Id()

	if err := api.ProxyGroupsUpdate(zabbix.ProxyGroups{g}); err != nil {
		return err
	}

	return resourceProxyGroupRead(d, m)
}

// resourceProxyGroupDelete terraform delete handler
func resourceProxyGroupDelete(d *schema.ResourceData, m interface{}) error {
	api := m.(*zabbix.API)
	return api.ProxyGroupsDeleteByIds([]string{d.Id()})
}
