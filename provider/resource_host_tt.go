// TT-PATCH: TT-003 -- whole file is a tenstorrent addition (proxy_groupid and
// monitored_by on zabbix_host). See PATCHES.md.
//
// Hooks into upstream code, all tagged TT-PATCH: TT-003:
//   - provider/provider.go registers ttResourceHost() / ttDataHost()
//   - provider/resource_host.go calls ttHostMonitoring from buildHostObject
//     and ttHostSetMonitoring from hostRead
// The attributes are added to hostSchemaBase by init() below. The API client
// already sends proxy_groupid / monitored_by (prepHosts); no client change.

package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/tpretz/terraform-provider-zabbix/internal/zabbix"
)

// HOST_MONITORED_BY_LOOKUP maps the Terraform "monitored_by" value onto the
// Zabbix >= 7.0 host property of the same name.
//
// monitored_by is not an independent setting: it is fully determined by
// proxyid and proxy_groupid, and the API client derives it from them on every
// write (see prepHosts). The attribute exists so that a configuration can say
// what it means, and so that configurations written against
// tenstorrent/zabbix v0.18 -- which required it, as Zabbix's numeric code --
// keep working. Those numeric codes are therefore accepted as aliases.
var HOST_MONITORED_BY_LOOKUP = map[string]zabbix.MonitoredByType{
	"server":      zabbix.MonitoredByServer,
	"proxy":       zabbix.MonitoredByProxy,
	"proxy_group": zabbix.MonitoredByProxyGroup,
}
var HOST_MONITORED_BY_LOOKUP_REV = map[zabbix.MonitoredByType]string{}
var HOST_MONITORED_BY_LOOKUP_ARR = []string{}

// HOST_MONITORED_BY_LEGACY_ARR are Zabbix's own codes, accepted on input for
// compatibility and normalised to the names above.
var HOST_MONITORED_BY_LEGACY_ARR = []string{}

var _ = func() bool {
	for k, v := range HOST_MONITORED_BY_LOOKUP {
		HOST_MONITORED_BY_LOOKUP_REV[v] = k
		HOST_MONITORED_BY_LOOKUP_ARR = append(HOST_MONITORED_BY_LOOKUP_ARR, k)
		HOST_MONITORED_BY_LEGACY_ARR = append(HOST_MONITORED_BY_LEGACY_ARR, string(v))
	}
	sort.Strings(HOST_MONITORED_BY_LOOKUP_ARR)
	sort.Strings(HOST_MONITORED_BY_LEGACY_ARR)
	return true
}()

func init() {
	hostSchemaBase["proxy_groupid"] = &schema.Schema{
		Type: schema.TypeString,
		Description: "ID of the proxy group (`zabbix_proxygroup`) monitoring this host, or \"0\" for none. " +
			"Mutually exclusive with `proxyid`. Zabbix 7.0 and later",
	}
	hostSchemaBase["monitored_by"] = &schema.Schema{
		Type:        schema.TypeString,
		Description: hostMonitoredByDescription,
	}
}

// ttResourceHost is upstream's zabbix_host plus proxy_groupid / monitored_by.
func ttResourceHost() *schema.Resource {
	r := resourceHost()
	hostMonitoringResourceSchema(r.Schema)
	r.CustomizeDiff = ttChainCustomizeDiff(r.CustomizeDiff, hostMonitoringCustomizeDiff)
	return r
}

// ttDataHost is upstream's data.zabbix_host plus the same two, read-only.
func ttDataHost() *schema.Resource {
	r := dataHost()
	r.Schema["proxy_groupid"].Computed = true
	r.Schema["monitored_by"].Computed = true
	return r
}

// ttHostMonitoring adds proxy_groupid to the host being written, refusing a
// host that names both a proxy and a group, or a group on a pre-7.0 server.
func ttHostMonitoring(d *schema.ResourceData, m interface{}, host *zabbix.Host) error {
	group := d.Get("proxy_groupid").(string)
	if err := hostCheckMonitoring(d.Get("proxyid").(string), group); err != nil {
		return err
	}
	if v := m.(*zabbix.API).Config.Version; group != zabbix.ProxyNoGroup && v < zabbix.V70 {
		return fmt.Errorf("proxy_groupid requires Zabbix 7.0 or later (this server reports %d)", v)
	}
	host.ProxyGroupID = group
	return nil
}

// hostMonitoredByDescription is shared by the resource and the data source.
var hostMonitoredByDescription = "What monitors the host, one of: " +
	strings.Join(HOST_MONITORED_BY_LOOKUP_ARR, ", ") +
	". Always derived from `proxyid` and `proxy_groupid` -- leave it out and the plan shows the derived value. " +
	"Setting it is only a cross-check: a value that disagrees with them is an error. Zabbix's numeric codes " +
	strings.Join(HOST_MONITORED_BY_LEGACY_ARR, ", ") +
	" are accepted as aliases, for configurations written against tenstorrent/zabbix v0.18"

// canonicalMonitoredBy turns a configured value, name or numeric code, into
// the name. Unknown values pass through unchanged for the validator to reject.
func canonicalMonitoredBy(v string) string {
	if name, ok := HOST_MONITORED_BY_LOOKUP_REV[zabbix.MonitoredByType(v)]; ok {
		return name
	}
	return v
}

// hostMonitoredByFor derives monitored_by from the two ids that determine it.
func hostMonitoredByFor(proxyID, proxyGroupID string) string {
	switch {
	case proxyGroupID != "" && proxyGroupID != zabbix.ProxyNoGroup:
		return HOST_MONITORED_BY_LOOKUP_REV[zabbix.MonitoredByProxyGroup]
	case proxyID != "" && proxyID != "0":
		return HOST_MONITORED_BY_LOOKUP_REV[zabbix.MonitoredByProxy]
	default:
		return HOST_MONITORED_BY_LOOKUP_REV[zabbix.MonitoredByServer]
	}
}

// hostMonitoringResourceSchema adjusts the proxy-related host attributes for
// resource use.
func hostMonitoringResourceSchema(o map[string]*schema.Schema) {
	o["proxy_groupid"].Optional = true
	o["proxy_groupid"].Default = zabbix.ProxyNoGroup
	o["proxy_groupid"].ValidateFunc = validation.StringIsNotWhiteSpace

	o["monitored_by"].Optional = true
	o["monitored_by"].Computed = true
	o["monitored_by"].ValidateFunc = validation.StringInSlice(
		append(append([]string{}, HOST_MONITORED_BY_LOOKUP_ARR...), HOST_MONITORED_BY_LEGACY_ARR...), false)
	o["monitored_by"].DiffSuppressFunc = func(_, old, new string, _ *schema.ResourceData) bool {
		return new != "" && canonicalMonitoredBy(old) == canonicalMonitoredBy(new)
	}
}

// hostMonitoringCustomizeDiff checks proxyid / proxy_groupid / monitored_by
// against each other at plan time, and plans the derived monitored_by when
// the configuration does not give one.
func hostMonitoringCustomizeDiff(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	if !d.NewValueKnown("proxyid") || !d.NewValueKnown("proxy_groupid") {
		// derived at apply time; say so rather than plan a stale value
		if _, given := configuredString(d, "monitored_by"); !given {
			return d.SetNewComputed("monitored_by")
		}
		return nil
	}

	proxyID := d.Get("proxyid").(string)
	groupID := d.Get("proxy_groupid").(string)
	if err := hostCheckMonitoring(proxyID, groupID); err != nil {
		return err
	}
	derived := hostMonitoredByFor(proxyID, groupID)

	if v, given := configuredString(d, "monitored_by"); given {
		if d.NewValueKnown("monitored_by") && canonicalMonitoredBy(v) != derived {
			return fmt.Errorf("monitored_by = %q contradicts proxyid = %q / proxy_groupid = %q, which mean %q. "+
				"monitored_by is derived from those two; remove it, or correct it", v, proxyID, groupID, derived)
		}
		return nil
	}
	if canonicalMonitoredBy(d.Get("monitored_by").(string)) != derived {
		return d.SetNew("monitored_by", derived)
	}
	return nil
}

// hostCheckMonitoring rejects a host that names both a proxy and a proxy group.
func hostCheckMonitoring(proxyID, groupID string) error {
	if proxyID != "" && proxyID != "0" && groupID != "" && groupID != zabbix.ProxyNoGroup {
		return fmt.Errorf("proxyid (%s) and proxy_groupid (%s) are mutually exclusive: a host is monitored by a proxy or by a proxy group, not both", proxyID, groupID)
	}
	return nil
}

// ttHostSetMonitoring writes proxy_groupid and monitored_by from a host read.
// A configured monitored_by alias ("1") that means the same as what the
// server reports is left as written, so that the value in state matches the
// configuration and a create does not report a spurious change.
func ttHostSetMonitoring(d *schema.ResourceData, api *zabbix.API, host zabbix.Host) {
	group := host.ProxyGroupID
	if group == "" {
		group = zabbix.ProxyNoGroup
	}
	d.Set("proxy_groupid", group)

	var actual string
	if name, ok := HOST_MONITORED_BY_LOOKUP_REV[host.MonitoredBy]; ok && api.Config.Version >= zabbix.V70 {
		actual = name
	} else {
		actual = hostMonitoredByFor(host.ProxyID, group)
	}
	if prior, ok := d.Get("monitored_by").(string); ok && canonicalMonitoredBy(prior) == actual {
		return
	}
	d.Set("monitored_by", actual)
}
