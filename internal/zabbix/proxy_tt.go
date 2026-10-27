// TT-PATCH: TT-002 -- whole file is a tenstorrent addition (proxy group
// membership on proxies). See PATCHES.md. proxy.go carries three tagged
// hooks into this file; everything else lives here.

package zabbix

// ProxyNoGroup is the proxy_groupid of a proxy that is not in a group.
const ProxyNoGroup = "0"

// proxyGroupMembership is embedded in Proxy (and the wire form in
// proxyResponse) so that proxy.go needs one line per struct, not three.
//
// ProxyGroupID is the group the proxy belongs to, ProxyNoGroup for none.
// LocalAddress and LocalPort are where active agents of hosts monitored by
// the group are redirected to reach this proxy. Zabbix (probed on 7.4.16)
// requires LocalAddress for a proxy in a group, and requires it empty -- with
// LocalPort at its default -- for one that is not. All three exist from 7.0
// only; the provider refuses them on older servers before calling the API.
type proxyGroupMembership struct {
	ProxyGroupID string `json:"proxy_groupid"`
	LocalAddress string `json:"local_address"`
	LocalPort    string `json:"local_port"`
}

// proxyGroupFromResponse copies the 7.0+ membership properties into p.
func proxyGroupFromResponse(p *Proxy, w proxyResponse) {
	p.proxyGroupMembership = w.proxyGroupMembership
	if p.ProxyGroupID == "" {
		p.ProxyGroupID = ProxyNoGroup
	}
}

// proxyGroupParams adds the 7.0+ membership properties to a create/update
// request. They are always sent, so leaving a group (or clearing the address)
// reaches the server rather than being read as "leave as is".
func proxyGroupParams(out Params, p Proxy) {
	out["proxy_groupid"] = p.ProxyGroupID
	if p.ProxyGroupID == "" {
		out["proxy_groupid"] = ProxyNoGroup
	}
	out["local_address"] = p.LocalAddress
	out["local_port"] = proxyPortOrDefault(p.LocalPort)
}
