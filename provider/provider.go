package provider

import (
	"errors"
	"fmt"
	logger "log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/tenstorrent/go-zabbix-api"
)

// Provider definition
func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"username": &schema.Schema{
				Type:         schema.TypeString,
				Optional:     true,
				Description:  "Zabbix API username",
				ValidateFunc: validation.StringIsNotWhiteSpace,
				DefaultFunc:  schema.MultiEnvDefaultFunc([]string{"ZABBIX_USER", "ZABBIX_USERNAME"}, nil),
			},
			"password": &schema.Schema{
				Type:         schema.TypeString,
				Optional:     true,
				Description:  "Zabbix API password",
				ValidateFunc: validation.StringIsNotWhiteSpace,
				DefaultFunc:  schema.MultiEnvDefaultFunc([]string{"ZABBIX_PASS", "ZABBIX_PASSWORD"}, nil),
			},
			"token": &schema.Schema{
				Type:         schema.TypeString,
				Optional:     true,
				Description:  "Zabbix Api Token",
				ValidateFunc: validation.StringIsNotWhiteSpace,
				DefaultFunc:  schema.MultiEnvDefaultFunc([]string{"ZABBIX_TOKEN"}, nil),
			},
			"url": &schema.Schema{
				Type:         schema.TypeString,
				Required:     true,
				Description:  "Zabbix API url",
				DefaultFunc:  schema.MultiEnvDefaultFunc([]string{"ZABBIX_URL", "ZABBIX_SERVER_URL"}, nil),
				ValidateFunc: validation.IsURLWithHTTPorHTTPS,
			},
			"tls_insecure": &schema.Schema{
				Type:        schema.TypeBool,
				Description: "Disable TLS certificate checking (for testing use only)",
				Optional:    true,
				Default:     false,
			},
			"serialize": &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Serialize API requests, if required due to API race conditions",
			},
		},
		DataSourcesMap: map[string]*schema.Resource{
			"zabbix_host":        dataHost(),
			"zabbix_application": dataApplication(),
			"zabbix_proxy":       dataProxy(),
			"zabbix_hostgroup":   dataHostgroup(),
			"zabbix_template":    dataTemplate(),
			"zabbix_proxygroup":  dataProxyGroup(),
		},
		ResourcesMap: map[string]*schema.Resource{
			"zabbix_trigger":       resourceTrigger(),
			"zabbix_proto_trigger": resourceProtoTrigger(),
			"zabbix_template":      resourceTemplate(),
			"zabbix_hostgroup":     resourceHostgroup(),
			"zabbix_host":          resourceHost(),
			"zabbix_application":   resourceApplication(),
			"zabbix_proxy":         resourceProxy(),
			"zabbix_proxygroup":    resourceProxyGroup(),

			"zabbix_graph":       resourceGraph(),
			"zabbix_proto_graph": resourceProtoGraph(),

			"zabbix_item_trapper":       resourceItemTrapper(),
			"zabbix_proto_item_trapper": resourceProtoItemTrapper(),
			"zabbix_lld_trapper":        resourceLLDTrapper(),

			"zabbix_item_http":       resourceItemHttp(),
			"zabbix_proto_item_http": resourceProtoItemHttp(),
			"zabbix_lld_http":        resourceLLDHttp(),

			"zabbix_item_simple":       resourceItemSimple(),
			"zabbix_proto_item_simple": resourceProtoItemSimple(),
			"zabbix_lld_simple":        resourceLLDSimple(),

			"zabbix_item_external":       resourceItemExternal(),
			"zabbix_proto_item_external": resourceProtoItemExternal(),
			"zabbix_lld_external":        resourceLLDExternal(),

			"zabbix_item_internal":       resourceItemInternal(),
			"zabbix_proto_item_internal": resourceProtoItemInternal(),
			"zabbix_lld_internal":        resourceLLDInternal(),

			"zabbix_item_snmp":       resourceItemSnmp(),
			"zabbix_proto_item_snmp": resourceProtoItemSnmp(),
			"zabbix_lld_snmp":        resourceLLDSnmp(),

			"zabbix_item_snmptrap":       resourceItemSnmpTrap(),
			"zabbix_proto_item_snmptrap": resourceProtoItemSnmpTrap(),

			"zabbix_item_agent":       resourceItemAgent(),
			"zabbix_proto_item_agent": resourceProtoItemAgent(),
			"zabbix_lld_agent":        resourceLLDAgent(),

			"zabbix_item_aggregate":       resourceItemAggregate(),
			"zabbix_proto_item_aggregate": resourceProtoItemAggregate(),

			"zabbix_item_calculated":       resourceItemCalculated(),
			"zabbix_proto_item_calculated": resourceProtoItemCalculated(),

			"zabbix_item_dependent":       resourceItemDependent(),
			"zabbix_proto_item_dependent": resourceProtoItemDependent(),
			"zabbix_lld_dependent":        resourceLLDDependent(),
		},
		ConfigureFunc: providerConfigure,
	}
}

// providerConfigure configure this provider
func providerConfigure(d *schema.ResourceData) (meta interface{}, err error) {
	fmt.Println("hola")
	log.Trace("Started zabbix provider init")
	log.Trace(fmt.Sprintf("token: %s", d.Get("token").(string)))
	l := logger.New(stderr, "[DEBUG] ", logger.LstdFlags)

	api, apierr := zabbix.NewAPI(zabbix.Config{
		Url:         d.Get("url").(string),
		TlsNoVerify: d.Get("tls_insecure").(bool),
		Log:         l,
		Serialize:   d.Get("serialize").(bool),
	})
	if apierr != nil {
		return nil, apierr
	}

	// Add debug logging for version
	log.Trace(fmt.Sprintf("Zabbix version: %d", api.Config.Version))
	log.Trace(fmt.Sprintf("Using token: %s", d.Get("token").(string)))

	// For Zabbix 7.2+, only API key authentication is supported
	if api.Config.Version >= 70200 {
		if d.Get("token").(string) == "" {
			return nil, errors.New("API key is required for Zabbix 7.2+")
		}
		log.Trace("Using API key authentication for Zabbix 7.2+")
		api.SetAPIKey(d.Get("token").(string))
	} else {
		// For older versions, use the existing authentication methods
		if d.Get("token").(string) != "" {
			log.Trace("Using token authentication for older version")
			api.Auth = d.Get("token").(string)
		} else if d.Get("username").(string) != "" && d.Get("password").(string) != "" {
			log.Trace("Using username/password authentication for older version")
			_, err = api.Login(d.Get("username").(string), d.Get("password").(string))
		} else {
			return nil, errors.New("either token or username/password is required")
		}
	}

	meta = api
	log.Trace("Started zabbix provider got error: %+v", err)

	log.Trace("--- providerConfigure() COMPLETED successfully ---")
	return
}

// tagGenerate build tag structs from terraform inputs
func tagGenerate(d *schema.ResourceData) (tags zabbix.Tags) {
	set := d.Get("tag").(*schema.Set).List()
	tags = make(zabbix.Tags, len(set))

	for i := 0; i < len(set); i++ {
		current := set[i].(map[string]interface{})
		tags[i] = zabbix.Tag{
			Tag:   current["key"].(string),
			Value: current["value"].(string),
		}
	}

	return
}

// flattenTags convert response to terraform input
func flattenTags(list zabbix.Tags) *schema.Set {
	set := schema.NewSet(func(i interface{}) int {
		m := i.(map[string]interface{})
		return hashString(m["key"].(string) + "V" + m["value"].(string))
	}, []interface{}{})
	for i := 0; i < len(list); i++ {
		set.Add(map[string]interface{}{
			"key":   list[i].Tag,
			"value": list[i].Value,
		})
	}
	return set
}
