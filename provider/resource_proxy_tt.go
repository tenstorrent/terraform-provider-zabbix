// TT-PATCH: TT-002 -- whole file is a tenstorrent addition (proxy group
// membership on zabbix_proxy). See PATCHES.md.
//
// Hooks into upstream code, all tagged TT-PATCH: TT-002:
//   - provider/provider.go registers ttResourceProxy() instead of resourceProxy()
//   - provider/resource_proxy.go calls ttProxyGroupMembership from
//     buildProxyObject and ttSetProxyGroupMembership from proxyRead
// The attributes themselves are added to proxySchemaBase by init() below, so
// the data source picks them up (as computed) through upstream's own
// proxyDataSchema without any edit there.

package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/customdiff"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"

	"github.com/tpretz/terraform-provider-zabbix/internal/zabbix"
)

func init() {
	proxySchemaBase["proxy_groupid"] = &schema.Schema{
		Type: schema.TypeString,
		Description: "ID of the proxy group (`zabbix_proxygroup`) the proxy belongs to, or \"0\" for none. " +
			"A proxy in a group needs `local_address`. Zabbix 7.0 and later",
	}
	proxySchemaBase["local_address"] = &schema.Schema{
		Type: schema.TypeString,
		Description: "Address (IP or DNS name) that active agents of hosts monitored by the proxy group are " +
			"redirected to in order to reach this proxy. Required when `proxy_groupid` is set, and must be " +
			"empty when it is not. Zabbix 7.0 and later",
	}
	proxySchemaBase["local_port"] = &schema.Schema{
		Type: schema.TypeString,
		Description: "Port that goes with `local_address`. Only a proxy in a proxy group can use anything " +
			"other than the default. Zabbix 7.0 and later",
	}
}

// ttResourceProxy is upstream's zabbix_proxy plus proxy group membership.
func ttResourceProxy() *schema.Resource {
	r := resourceProxy()

	// Zabbix reports both of these for every proxy, in or out of a group, on
	// 7.0 and later; the provider reports the same for older servers.
	r.Schema["proxy_groupid"].Default = zabbix.ProxyNoGroup
	r.Schema["proxy_groupid"].ValidateFunc = validation.StringIsNotWhiteSpace
	r.Schema["local_port"].Default = zabbix.ProxyDefaultPort

	r.CustomizeDiff = ttChainCustomizeDiff(r.CustomizeDiff, ttProxyGroupCustomizeDiff)
	r.Importer = &schema.ResourceImporter{State: importByIDOrName(proxyIDByName)} // TT-004: by id or by name
	return r
}

// ttChainCustomizeDiff runs an upstream CustomizeDiff (which may be nil) and
// then a tenstorrent one, so a patch can add plan-time checks to a resource
// without editing its constructor.
func ttChainCustomizeDiff(upstream, ours schema.CustomizeDiffFunc) schema.CustomizeDiffFunc {
	if upstream == nil {
		return ours
	}
	return customdiff.All(upstream, ours)
}

// ttProxyGroupCustomizeDiff rejects proxy group membership on a pre-7.0 server
// at plan time. (Upstream's proxy params silently drop properties an older
// server does not have; for these that would be a silent wrong answer.)
func ttProxyGroupCustomizeDiff(_ context.Context, d *schema.ResourceDiff, m interface{}) error {
	api, ok := m.(*zabbix.API)
	if !ok || api == nil || api.Config.Version >= zabbix.V70 {
		return nil
	}
	group := d.Get("proxy_groupid").(string)
	port := d.Get("local_port").(string)
	if (group != "" && group != zabbix.ProxyNoGroup) || d.Get("local_address").(string) != "" ||
		(port != "" && port != zabbix.ProxyDefaultPort) {
		return fmt.Errorf("proxy groups (proxy_groupid, local_address, local_port) require Zabbix 7.0 or later (this server reports %d)", api.Config.Version)
	}
	return nil
}

// ttProxyGroupMembership fills in the proxy group attributes, rejecting the
// combinations Zabbix rejects with a message that names the attribute: a proxy
// in a group must have a local_address, and one outside a group must have
// neither a local_address nor a non-default local_port.
func ttProxyGroupMembership(d *schema.ResourceData, proxy *zabbix.Proxy) error {
	group := d.Get("proxy_groupid").(string)
	localAddress := d.Get("local_address").(string)
	localPort := d.Get("local_port").(string)

	if group == "" || group == zabbix.ProxyNoGroup {
		if localAddress != "" {
			return errors.New("local_address applies to a proxy in a proxy group only (set proxy_groupid, or remove local_address)")
		}
		if localPort != "" && localPort != zabbix.ProxyDefaultPort {
			return fmt.Errorf("local_port applies to a proxy in a proxy group only (outside a group Zabbix requires the default %q)", zabbix.ProxyDefaultPort)
		}
	} else if localAddress == "" {
		return errors.New("local_address is required for a proxy in a proxy group: it is where the group's active agents are sent to reach this proxy")
	}

	proxy.ProxyGroupID = group
	proxy.LocalAddress = localAddress
	proxy.LocalPort = localPort
	return nil
}

// ttSetProxyGroupMembership writes the membership attributes from a read.
// Before 7.0 there are no proxy groups; the "not in a group" values are
// reported so the attributes mean the same on every version.
func ttSetProxyGroupMembership(d *schema.ResourceData, proxy zabbix.Proxy) {
	group, localPort := proxy.ProxyGroupID, proxy.LocalPort
	if group == "" {
		group = zabbix.ProxyNoGroup
	}
	if localPort == "" {
		localPort = zabbix.ProxyDefaultPort
	}
	d.Set("proxy_groupid", group)
	d.Set("local_address", proxy.LocalAddress)
	d.Set("local_port", localPort)
}
