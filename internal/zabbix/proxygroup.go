// TT-PATCH: TT-001 -- whole file is a tenstorrent addition (proxy groups). See PATCHES.md.

package zabbix

import "fmt"

// ProxyGroup represents a Zabbix proxy group object.
//
// Proxy groups were introduced in Zabbix 7.0 (V70): a host monitored by a
// group is assigned to one of its online proxies by the server, and moved to
// another when that proxy goes offline. There is no equivalent before 7.0.
// https://www.zabbix.com/documentation/current/en/manual/api/reference/proxygroup/object
type ProxyGroup struct {
	ProxyGroupID string `json:"proxy_groupid,omitempty"`
	Name         string `json:"name"`
	// FailoverDelay and MinOnline are strings on the wire and accept user
	// macros as well as plain values, so they are kept as strings here.
	FailoverDelay string `json:"failover_delay"`
	MinOnline     string `json:"min_online"`
	// Description has no omitempty: proxygroup.update leaves an absent
	// property as it is, so a cleared description must be sent as "".
	Description string `json:"description"`
	// State is read-only (0 unknown, 1 offline, 2 recovering, 3 online,
	// 4 degrading). proxygroup.create/update reject it as an unexpected
	// parameter, so prepProxyGroups strips it before writing.
	State string `json:"state,omitempty"`
}

// ProxyGroups is an array of ProxyGroup
type ProxyGroups []ProxyGroup

// ProxyGroupsGet Wrapper for proxygroup.get
// https://www.zabbix.com/documentation/current/en/manual/api/reference/proxygroup/get
func (api *API) ProxyGroupsGet(params Params) (res ProxyGroups, err error) {
	if err = api.requireProxyGroups(); err != nil {
		return
	}
	if _, present := params["output"]; !present {
		params["output"] = "extend"
	}
	err = api.CallWithErrorParse("proxygroup.get", params, &res)
	return
}

// ProxyGroupGetByID Gets a proxy group by id only if there is exactly one match.
func (api *API) ProxyGroupGetByID(id string) (res *ProxyGroup, err error) {
	groups, err := api.ProxyGroupsGet(Params{"proxy_groupids": id})
	if err != nil {
		return
	}

	if len(groups) == 1 {
		res = &groups[0]
	} else {
		e := ExpectedOneResult(len(groups))
		err = &e
	}
	return
}

// prepProxyGroups strips read-only properties, which the write methods reject
// as unexpected parameters.
func prepProxyGroups(groups ProxyGroups) {
	for i := range groups {
		groups[i].State = ""
	}
}

// ProxyGroupsCreate Wrapper for proxygroup.create
// https://www.zabbix.com/documentation/current/en/manual/api/reference/proxygroup/create
func (api *API) ProxyGroupsCreate(groups ProxyGroups) (err error) {
	if err = api.requireProxyGroups(); err != nil {
		return
	}
	prepProxyGroups(groups)
	response, err := api.CallWithError("proxygroup.create", groups)
	if err != nil {
		return
	}

	result := response.Result.(map[string]interface{})
	ids, ok := result["proxy_groupids"].([]interface{})
	if !ok || len(ids) != len(groups) {
		return fmt.Errorf("proxygroup.create: unexpected result %v", result)
	}
	for i, id := range ids {
		groups[i].ProxyGroupID = id.(string)
	}
	return
}

// ProxyGroupsUpdate Wrapper for proxygroup.update
// https://www.zabbix.com/documentation/current/en/manual/api/reference/proxygroup/update
func (api *API) ProxyGroupsUpdate(groups ProxyGroups) (err error) {
	if err = api.requireProxyGroups(); err != nil {
		return
	}
	prepProxyGroups(groups)
	_, err = api.CallWithError("proxygroup.update", groups)
	return
}

// ProxyGroupsDeleteByIds Wrapper for proxygroup.delete
// https://www.zabbix.com/documentation/current/en/manual/api/reference/proxygroup/delete
func (api *API) ProxyGroupsDeleteByIds(ids []string) (err error) {
	if err = api.requireProxyGroups(); err != nil {
		return
	}
	response, err := api.CallWithError("proxygroup.delete", ids)
	if err != nil {
		return
	}

	result := response.Result.(map[string]interface{})
	deleted, _ := result["proxy_groupids"].([]interface{})
	if len(ids) != len(deleted) {
		err = &ExpectedMore{len(ids), len(deleted)}
	}
	return
}

// requireProxyGroups fails fast, with a message that names the cause, when
// the server predates proxy groups. Calling a method the server does not have
// would otherwise surface as a bare "Incorrect API" error.
func (api *API) requireProxyGroups() error {
	if api.Config.Version < V70 {
		return fmt.Errorf("proxy groups require Zabbix 7.0 or later (this server reports %d)", api.Config.Version)
	}
	return nil
}
