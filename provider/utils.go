package provider

import (
	"hash/crc32"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/tenstorrent/go-zabbix-api"
)

// flattenHostGroupIds converts a list of Zabbix host group identifiers into a Terraform schema.Set.
// This function is version-agnostic, working with both Zabbix 6.0 and 7.2 API responses.
func flattenHostGroupIds(list zabbix.HostGroupIDs) *schema.Set {
	s := schema.NewSet(schema.HashString, []interface{}{})
	for _, v := range list {
		s.Add(v.GroupID)
	}
	return s
}

func flattenTemplateIds(list zabbix.TemplateIDs) *schema.Set {
	s := schema.NewSet(schema.HashString, []interface{}{})
	for _, v := range list {
		s.Add(v.TemplateID)
	}
	return s
}

// buildHostGroupIds converts a Terraform schema.Set into a list of Zabbix host group identifiers.
// This function creates the minimal structure required for Zabbix API calls.
func buildHostGroupIds(s *schema.Set) zabbix.HostGroupIDs {
	list := s.List()

	groups := make(zabbix.HostGroupIDs, len(list))

	for i := 0; i < len(list); i++ {
		groups[i] = zabbix.HostGroupID{
			GroupID: list[i].(string),
		}
	}

	return groups
}

func buildTriggerIds(s *schema.Set) zabbix.TriggerIDs {
	list := s.List()

	groups := make(zabbix.TriggerIDs, len(list))

	for i := 0; i < len(list); i++ {
		groups[i] = zabbix.TriggerID{
			TriggerID: list[i].(string),
		}
	}

	return groups
}

func buildTemplateIds(s *schema.Set) zabbix.TemplateIDs {
	list := s.List()

	groups := make(zabbix.TemplateIDs, len(list))

	for i := 0; i < len(list); i++ {
		groups[i] = zabbix.TemplateID{
			TemplateID: list[i].(string),
		}
	}

	return groups
}

// mergeSchemas, take a varadic list of schemas and merge, latter overwrites former
func mergeSchemas(schemas ...map[string]*schema.Schema) map[string]*schema.Schema {
	n := map[string]*schema.Schema{}

	for _, s := range schemas {
		for k, v := range s {
			n[k] = v
		}
	}

	return n
}

// hashString returns a stable non-negative hash of s. It matches the
// former terraform helper/hashcode.String so existing state hashes are unchanged.
func hashString(s string) int {
	v := int(crc32.ChecksumIEEE([]byte(s)))
	if v >= 0 {
		return v
	}
	if -v >= 0 {
		return -v
	}
	return 0
}
