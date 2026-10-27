// TT-PATCH: TT-004 -- whole file is a tenstorrent addition (import by name for
// zabbix_proxy and zabbix_proxygroup). See PATCHES.md.

package provider

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/tpretz/terraform-provider-zabbix/internal/zabbix"
)

// importByIDOrName returns a StateFunc that accepts either the Zabbix id or
// the object's unique name as the import id.
//
// Zabbix ids are always decimal integers, so an import id made only of
// digits is taken as an id and passed through unchanged, exactly as
// ImportStatePassthrough would. Anything else is looked up by name and
// replaced with the id it resolves to; the resource's Read then runs as for
// any other import. A name that happens to be all digits therefore has to be
// imported by id -- a deliberate trade, because guessing between the two
// would make the same import id mean different things on different servers.
//
// This exists for configurations written against tenstorrent/zabbix v0.18,
// whose import blocks name proxies and proxy groups rather than giving ids.
func importByIDOrName(lookup func(*zabbix.API, string) (string, error)) schema.StateFunc {
	return func(d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
		id := d.Id()
		if _, err := strconv.ParseUint(id, 10, 64); err != nil {
			resolved, err := lookup(m.(*zabbix.API), id)
			if err != nil {
				return nil, err
			}
			d.SetId(resolved)
		}
		return []*schema.ResourceData{d}, nil
	}
}

// proxyIDByName resolves a proxy's technical name to its id, for import.
func proxyIDByName(api *zabbix.API, name string) (string, error) {
	proxies, err := api.ProxiesGet(zabbix.Params{
		"filter": map[string]interface{}{api.ProxyNameProperty(): name},
	})
	if err != nil {
		return "", err
	}
	switch len(proxies) {
	case 0:
		return "", fmt.Errorf("no proxy named %q", name)
	case 1:
		return proxies[0].ProxyID, nil
	default:
		return "", fmt.Errorf("%d proxies named %q", len(proxies), name)
	}
}

// proxyGroupIDByName resolves a proxy group name to its id, for import.
func proxyGroupIDByName(api *zabbix.API, name string) (string, error) {
	groups, err := api.ProxyGroupsGet(zabbix.Params{
		"output": []string{"proxy_groupid"},
		"filter": map[string]interface{}{"name": name},
	})
	if err != nil {
		return "", err
	}
	switch len(groups) {
	case 0:
		return "", fmt.Errorf("no proxy group named %q", name)
	case 1:
		return groups[0].ProxyGroupID, nil
	default:
		return "", fmt.Errorf("%d proxy groups named %q", len(groups), name)
	}
}
