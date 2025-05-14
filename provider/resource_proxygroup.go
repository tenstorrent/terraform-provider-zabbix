package provider

import (
	"fmt"
	"regexp"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/tenstorrent/go-zabbix-api"
)

// Regular expression for Zabbix time unit validation (e.g., 30s, 5m, 2h, 1d)
var zabbixTimeUnitRegex = regexp.MustCompile(`^([1-9]\d*[smhd])+$`)

// proxyGroupSchema defines the schema for a Zabbix proxy group.
var proxyGroupSchema = map[string]*schema.Schema{
	"name": {
		Type:         schema.TypeString,
		Description:  "Name of the proxy group.",
		Required:     true,
		ValidateFunc: validation.StringIsNotWhiteSpace,
	},
	"description": {
		Type:        schema.TypeString,
		Description: "Description of the proxy group.",
		Optional:    true,
		Default:     "",
	},
	"failover_delay": {
		Type:         schema.TypeString,
		Description:  "Proxy group failover delay. Accepts seconds or time unit with suffix (e.g., 30s, 5m, 1h).",
		Optional:     true,
		Default:      "1m", // Default as per Zabbix API documentation
		ValidateFunc: validation.StringMatch(zabbixTimeUnitRegex, "must be a valid Zabbix time unit (e.g., '30s', '5m', '1h')"),
	},
	"min_online": {
		Type:         schema.TypeInt,
		Description:  "Minimum number of online proxies needed to consider the group available.",
		Optional:     true,
		Default:      1, // Default as per Zabbix API documentation
		ValidateFunc: validation.IntAtLeast(0),
	},
}

// dataProxyGroup defines the data source for a Zabbix proxy group.
func dataProxyGroup() *schema.Resource {
	// Use the same schema but mark fields as Computed for data source
	dataSchema := make(map[string]*schema.Schema)
	for k, v := range proxyGroupSchema {
		// Need to create a new schema object to avoid modifying the original resource schema
		newSchema := *v // Copy the schema definition
		newSchema.Computed = true
		newSchema.Required = false   // Ensure not required for data source
		newSchema.Optional = false   // Ensure not optional for data source
		newSchema.Default = nil      // Don't use defaults for data source
		newSchema.ValidateFunc = nil // Remove validation for computed fields
		dataSchema[k] = &newSchema
	}
	// Override 'name' to be Required for lookup and restore its validation
	dataSchema["name"].Required = true
	dataSchema["name"].Computed = false
	dataSchema["name"].ValidateFunc = proxyGroupSchema["name"].ValidateFunc // Restore validation for name

	return &schema.Resource{
		Read:   dataProxyGroupRead,
		Schema: dataSchema,
	}
}

// dataProxyGroupRead handles reading the proxy group data source.
func dataProxyGroupRead(d *schema.ResourceData, m interface{}) error {
	api := m.(*zabbix.API)

	groupName := d.Get("name").(string)
	log.Debug(fmt.Sprintf("dataProxyGroupRead: Reading proxy group with name %s", groupName))

	params := zabbix.Params{
		"filter": map[string]string{
			"name": groupName,
		},
		"output": "extend", // Ensure all fields are returned
	}

	groups, err := api.ProxyGroupsGet(params)
	if err != nil {
		return fmt.Errorf("Failed to get proxy group '%s': %w", groupName, err)
	}

	if len(groups) == 0 {
		log.Debug(fmt.Sprintf("dataProxyGroupRead: Proxy group '%s' not found", groupName))
		d.SetId("") // Mark resource as not found
		return nil
	}

	if len(groups) > 1 {
		return fmt.Errorf("Multiple proxy groups found with name '%s'", groupName)
	}

	group := groups[0]
	log.Debug(fmt.Sprintf("dataProxyGroupRead: Found proxy group: %+v", group))
	d.SetId(group.ProxyGroupID)
	d.Set("name", group.Name)
	d.Set("description", group.Description)
	d.Set("failover_delay", group.FailoverDelay)
	d.Set("min_online", group.GetMinOnline())

	return nil
}

// resourceProxyGroup defines the resource for a Zabbix proxy group.
func resourceProxyGroup() *schema.Resource {
	return &schema.Resource{
		Create: resourceProxyGroupCreate,
		Read:   resourceProxyGroupRead,
		Update: resourceProxyGroupUpdate,
		Delete: resourceProxyGroupDelete,
		Importer: &schema.ResourceImporter{
			State: func(d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
				api := m.(*zabbix.API)
				var group *zabbix.ProxyGroup
				var err error

				log.Debug(fmt.Sprintf("Importing proxy group with ID/name: %s", d.Id()))

				// Try to get the proxy group by name first
				params := zabbix.Params{
					"filter": map[string]string{
						"name": d.Id(),
					},
					"output": "extend",
				}

				groups, err := api.ProxyGroupsGet(params)
				if err != nil {
					return nil, fmt.Errorf("failed to get proxy group '%s': %w", d.Id(), err)
				}

				log.Debug(fmt.Sprintf("Found %d groups by name", len(groups)))

				if len(groups) == 0 {
					// If not found by name, try using the ID directly
					log.Debug(fmt.Sprintf("No groups found by name, trying ID lookup"))
					group, err = api.ProxyGroupGetByID(d.Id())
					if err != nil {
						return nil, fmt.Errorf("failed to get proxy group with ID '%s': %w", d.Id(), err)
					}
					log.Debug(fmt.Sprintf("Found group by ID: %+v", group))
				} else if len(groups) > 1 {
					return nil, fmt.Errorf("multiple proxy groups found with name '%s'", d.Id())
				} else {
					group = &groups[0]
					log.Debug(fmt.Sprintf("Found group by name: %+v", group))
				}

				if group == nil {
					return nil, fmt.Errorf("no proxy group found with name or ID '%s'", d.Id())
				}

				// Set all fields
				d.SetId(group.ProxyGroupID)
				log.Debug(fmt.Sprintf("Set ID to: %s", group.ProxyGroupID))

				if err := d.Set("name", group.Name); err != nil {
					return nil, fmt.Errorf("failed to set name: %w", err)
				}
				log.Debug(fmt.Sprintf("Set name to: %s", group.Name))

				if err := d.Set("description", group.Description); err != nil {
					return nil, fmt.Errorf("failed to set description: %w", err)
				}
				log.Debug(fmt.Sprintf("Set description to: %s", group.Description))

				if err := d.Set("failover_delay", group.FailoverDelay); err != nil {
					return nil, fmt.Errorf("failed to set failover_delay: %w", err)
				}
				log.Debug(fmt.Sprintf("Set failover_delay to: %s", group.FailoverDelay))

				if err := d.Set("min_online", group.GetMinOnline()); err != nil {
					return nil, fmt.Errorf("failed to set min_online: %w", err)
				}
				log.Debug(fmt.Sprintf("Set min_online to: %d", group.GetMinOnline()))

				log.Debug("Import completed successfully")
				return []*schema.ResourceData{d}, nil
			},
		},
		Schema: proxyGroupSchema,
	}
}

// resourceProxyGroupCreate handles creation of a Zabbix proxy group.
func resourceProxyGroupCreate(d *schema.ResourceData, m interface{}) error {
	api := m.(*zabbix.API)

	group := zabbix.ProxyGroup{
		Name:          d.Get("name").(string),
		Description:   d.Get("description").(string),
		FailoverDelay: d.Get("failover_delay").(string),
	}
	group.SetMinOnline(d.Get("min_online").(int))

	groupsToCreate := zabbix.ProxyGroups{group}

	log.Debug(fmt.Sprintf("resourceProxyGroupCreate: Creating proxy group: %+v", group))
	err := api.ProxyGroupsCreate(groupsToCreate)
	if err != nil {
		return fmt.Errorf("Failed to create proxy group: %w", err)
	}

	createdGroup := groupsToCreate[0]
	d.SetId(createdGroup.ProxyGroupID)
	log.Info(fmt.Sprintf("resourceProxyGroupCreate: Created proxy group '%s' with ID %s", createdGroup.Name, createdGroup.ProxyGroupID))

	// Read back the state, including potentially computed values or defaults from Zabbix
	return resourceProxyGroupRead(d, m)
}

// resourceProxyGroupRead handles reading a Zabbix proxy group resource.
func resourceProxyGroupRead(d *schema.ResourceData, m interface{}) error {
	api := m.(*zabbix.API)
	groupID := d.Id()

	log.Debug(fmt.Sprintf("resourceProxyGroupRead: Reading proxy group with ID %s", groupID))

	// Convert string ID to number
	idNum, err := strconv.Atoi(groupID)
	if err != nil {
		return fmt.Errorf("invalid proxy group ID '%s': %w", groupID, err)
	}

	// Always use proxy_groupids for reading after creation
	params := zabbix.Params{
		"proxy_groupids": []int{idNum},
		"output":         "extend",
	}

	groups, err := api.ProxyGroupsGet(params)
	if err != nil {
		// If the group doesn't exist, handle it gracefully (e.g., for deletes)
		log.Debug(fmt.Sprintf("resourceProxyGroupRead: Proxy group with ID %s not found: %v", groupID, err))
		d.SetId("")
		return nil // Terraform handles removing state if ID is unset
	}

	if len(groups) == 0 {
		log.Debug(fmt.Sprintf("resourceProxyGroupRead: No proxy group found with ID %s", groupID))
		d.SetId("")
		return nil
	}

	if len(groups) > 1 {
		return fmt.Errorf("multiple proxy groups found with ID %s", groupID)
	}

	group := groups[0]
	log.Debug(fmt.Sprintf("resourceProxyGroupRead: Found proxy group: %+v", group))
	d.SetId(group.ProxyGroupID)
	d.Set("name", group.Name)
	d.Set("description", group.Description)
	d.Set("failover_delay", group.FailoverDelay)
	d.Set("min_online", group.GetMinOnline())

	return nil
}

// resourceProxyGroupUpdate handles updating a Zabbix proxy group resource.
func resourceProxyGroupUpdate(d *schema.ResourceData, m interface{}) error {
	api := m.(*zabbix.API)

	group := zabbix.ProxyGroup{
		ProxyGroupID: d.Id(),
		Name:         d.Get("name").(string),
		// Note: Zabbix API update typically requires sending all desired fields.
		Description:   d.Get("description").(string),
		FailoverDelay: d.Get("failover_delay").(string),
	}
	group.SetMinOnline(d.Get("min_online").(int))

	groupsToUpdate := zabbix.ProxyGroups{group}

	log.Debug(fmt.Sprintf("resourceProxyGroupUpdate: Updating proxy group ID %s to: %+v", d.Id(), group))
	err := api.ProxyGroupsUpdate(groupsToUpdate)
	if err != nil {
		return fmt.Errorf("Failed to update proxy group ID %s: %w", d.Id(), err)
	}

	log.Info(fmt.Sprintf("resourceProxyGroupUpdate: Updated proxy group ID %s", d.Id()))
	return resourceProxyGroupRead(d, m)
}

// resourceProxyGroupDelete handles deletion of a Zabbix proxy group resource.
func resourceProxyGroupDelete(d *schema.ResourceData, m interface{}) error {
	api := m.(*zabbix.API)
	groupID := d.Id()

	log.Debug(fmt.Sprintf("resourceProxyGroupDelete: Deleting proxy group with ID %s", groupID))
	err := api.ProxyGroupsDelete([]string{groupID})
	if err != nil {
		// Consider if we need to handle 'already deleted' errors specifically
		return fmt.Errorf("Failed to delete proxy group ID %s: %w", groupID, err)
	}

	log.Info(fmt.Sprintf("resourceProxyGroupDelete: Deleted proxy group ID %s", groupID))
	d.SetId("") // Mark resource as deleted
	return nil
}
