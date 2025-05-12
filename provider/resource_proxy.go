package provider

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
	"github.com/tenstorrent/go-zabbix-api"
)

// proxySchemaBase base proxy schema
var proxySchemaBase = map[string]*schema.Schema{
	"name": &schema.Schema{
		Type:         schema.TypeString,
		Description:  "FQDN of proxy",
		ValidateFunc: validation.StringIsNotWhiteSpace,
		Required:     true,
	},
}

// proxyModeMap maps between string and integer representations of proxy modes
var proxyModeMap = map[string]int{
	"active":  0,
	"passive": 1,
}

// proxyModeToString converts internal integer mode to string representation
func proxyModeToString(modeStr string) string {
	if mode, err := strconv.Atoi(modeStr); err == nil {
		for str, val := range proxyModeMap {
			if val == mode {
				return str
			}
		}
	}
	return "active" // default to active if invalid
}

// tlsModes maps between string and integer representations of TLS modes
var tlsModes = map[string]int{
	"none":        1,
	"psk":         2,
	"certificate": 4,
}

// tlsModeToString converts internal integer TLS mode to string representation
func tlsModeToString(modeStr string) string {
	if mode, err := strconv.Atoi(modeStr); err == nil {
		for str, val := range tlsModes {
			if val == mode {
				return str
			}
		}
	}
	return "none" // default to none if invalid
}

// dataProxy terraform proxy data source entrypoint
func dataProxy() *schema.Resource {
	return &schema.Resource{
		Read:   dataProxyRead,
		Schema: proxySchemaBase,
	}
}

// dataProxyRead read handler for data resource
func dataProxyRead(d *schema.ResourceData, m interface{}) error {
	api := m.(*zabbix.API)
	hostName := "name"
	if api.Config.Version < 70000 {
		hostName = "host"
	}
	log.Trace(fmt.Sprintf("version: %d", api.Config.Version))
	return proxyRead(d, m, zabbix.Params{
		"filter": map[string]interface{}{
			hostName: d.Get("name"),
		},
	})
}

// resourceProxy terraform resource handler
func resourceProxy() *schema.Resource {
	return &schema.Resource{
		Create: resourceProxyCreate,
		Read:   resourceProxyRead,
		Update: resourceProxyUpdate,
		Delete: resourceProxyDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"host": &schema.Schema{
				Type:         schema.TypeString,
				ValidateFunc: validation.StringIsNotWhiteSpace,
				Description:  "Proxy hostname (required for passive proxies)",
				Optional:     true,
			},
			"operating_mode": &schema.Schema{
				Type:         schema.TypeString,
				Description:  "Proxy operating mode (active or passive)",
				Optional:     true,
				Default:      "active",
				ValidateFunc: validation.StringInSlice([]string{"active", "passive"}, false),
			},
			"local_address": &schema.Schema{
				Type:        schema.TypeString,
				Description: "IP address or hostname used by the active proxy to connect to Zabbix server",
				Optional:    true,
			},
			"local_port": &schema.Schema{
				Type:        schema.TypeString,
				Description: "Port used by the active proxy to connect to Zabbix server",
				Optional:    true,
			},
			"allowed_addresses": &schema.Schema{
				Type:        schema.TypeString,
				Description: "Comma-separated list of allowed IP addresses (required for active proxies)",
				Optional:    true,
			},
			"description": &schema.Schema{
				Type:        schema.TypeString,
				Description: "Proxy description",
				Optional:    true,
			},
			"tls_connect": &schema.Schema{
				Type:         schema.TypeString,
				Description:  "Connections to proxy (none, psk, or certificate)",
				Optional:     true,
				Default:      "none",
				ValidateFunc: validation.StringInSlice([]string{"none", "psk", "certificate"}, false),
			},
			"tls_accept": &schema.Schema{
				Type:         schema.TypeString,
				Description:  "Connections from proxy (none, psk, or certificate)",
				Optional:     true,
				Default:      "none",
				ValidateFunc: validation.StringInSlice([]string{"none", "psk", "certificate"}, false),
			},
			"tls_psk_identity": &schema.Schema{
				Type:        schema.TypeString,
				Description: "PSK identity",
				Optional:    true,
			},
			"tls_psk": &schema.Schema{
				Type:        schema.TypeString,
				Description: "PSK value",
				Optional:    true,
				Sensitive:   true,
			},
			"tls_subject": &schema.Schema{
				Type:        schema.TypeString,
				Description: "TLS certificate subject",
				Optional:    true,
			},
			"tls_issuer": &schema.Schema{
				Type:        schema.TypeString,
				Description: "TLS certificate issuer",
				Optional:    true,
			},
			"proxy_groupid": &schema.Schema{
				Type:        schema.TypeString,
				Description: "ID of the proxy group this proxy belongs to",
				Optional:    true,
			},
		},
	}
}

// validateProxyFields checks that the fields are valid for the given operating mode
func validateProxyFields(operatingMode, host, allowedAddresses string) error {
	if operatingMode == "active" {
		if host != "" {
			return fmt.Errorf("host field cannot be set for active proxies")
		}
		if allowedAddresses == "" {
			return fmt.Errorf("allowed_addresses is required for active proxies")
		}
	} else { // passive mode
		if allowedAddresses != "" {
			return fmt.Errorf("allowed_addresses field cannot be set for passive proxies")
		}
		if host == "" {
			return fmt.Errorf("host is required for passive proxies")
		}
	}
	return nil
}

// resourceProxyCreate terraform proxy create function
func resourceProxyCreate(d *schema.ResourceData, m interface{}) error {
	api := m.(*zabbix.API)

	operatingMode := d.Get("operating_mode").(string)
	host := d.Get("host").(string)
	allowedAddresses := d.Get("allowed_addresses").(string)

	if err := validateProxyFields(operatingMode, host, allowedAddresses); err != nil {
		return err
	}

	proxy := zabbix.Proxy{
		Host:             host,
		OperatingMode:    strconv.Itoa(proxyModeMap[operatingMode]),
		Description:      d.Get("description").(string),
		TLSConnect:       strconv.Itoa(tlsModes[d.Get("tls_connect").(string)]),
		TLSAccept:        strconv.Itoa(tlsModes[d.Get("tls_accept").(string)]),
		TLSPSKIdentity:   d.Get("tls_psk_identity").(string),
		TLSPSK:           d.Get("tls_psk").(string),
		TLSSubject:       d.Get("tls_subject").(string),
		TLSIssuer:        d.Get("tls_issuer").(string),
		ProxyGroupID:     d.Get("proxy_groupid").(string),
		AllowedAddresses: allowedAddresses,
		LocalAddress:     d.Get("local_address").(string),
		LocalPort:        d.Get("local_port").(string),
	}

	proxies := []zabbix.Proxy{proxy}

	err := api.ProxiesCreate(proxies)
	if err != nil {
		return err
	}

	log.Trace("created proxy: %+v", proxies[0])

	d.SetId(proxies[0].ProxyID)

	return resourceProxyRead(d, m)
}

// proxyRead common proxy read function
func proxyRead(d *schema.ResourceData, m interface{}, params zabbix.Params) error {
	api := m.(*zabbix.API)

	proxies, err := api.ProxiesGet(params)
	if err != nil {
		return err
	}

	if len(proxies) < 1 {
		return errors.New("proxy not found")
	}
	if len(proxies) > 1 {
		return errors.New("multiple proxies found")
	}
	proxy := proxies[0]

	log.Debug("Got proxy: %+v", proxy)

	d.SetId(proxy.ProxyID)
	if d.Get("name") != nil {
		d.Set("name", proxy.Host)
	} else {
		d.Set("host", proxy.Host)
		d.Set("operating_mode", proxyModeToString(proxy.OperatingMode))
		d.Set("description", proxy.Description)
		d.Set("tls_connect", tlsModeToString(proxy.TLSConnect))
		d.Set("tls_accept", tlsModeToString(proxy.TLSAccept))
		d.Set("tls_psk_identity", proxy.TLSPSKIdentity)
		d.Set("tls_subject", proxy.TLSSubject)
		d.Set("tls_issuer", proxy.TLSIssuer)
		d.Set("proxy_groupid", proxy.ProxyGroupID)
		d.Set("allowed_addresses", proxy.AllowedAddresses)
		d.Set("local_address", proxy.LocalAddress)
		d.Set("local_port", proxy.LocalPort)
		// Don't set tls_psk as it's sensitive
	}

	return nil
}

// resourceProxyRead terraform proxy read function
func resourceProxyRead(d *schema.ResourceData, m interface{}) error {
	return proxyRead(d, m, zabbix.Params{
		"proxyids": []string{d.Id()},
	})
}

// resourceProxyUpdate terraform proxy update function
func resourceProxyUpdate(d *schema.ResourceData, m interface{}) error {
	api := m.(*zabbix.API)

	operatingMode := d.Get("operating_mode").(string)
	host := d.Get("host").(string)
	allowedAddresses := d.Get("allowed_addresses").(string)

	if err := validateProxyFields(operatingMode, host, allowedAddresses); err != nil {
		return err
	}

	proxy := zabbix.Proxy{
		ProxyID:          d.Id(),
		Host:             host,
		OperatingMode:    strconv.Itoa(proxyModeMap[operatingMode]),
		Description:      d.Get("description").(string),
		TLSConnect:       strconv.Itoa(tlsModes[d.Get("tls_connect").(string)]),
		TLSAccept:        strconv.Itoa(tlsModes[d.Get("tls_accept").(string)]),
		TLSPSKIdentity:   d.Get("tls_psk_identity").(string),
		TLSPSK:           d.Get("tls_psk").(string),
		TLSSubject:       d.Get("tls_subject").(string),
		TLSIssuer:        d.Get("tls_issuer").(string),
		ProxyGroupID:     d.Get("proxy_groupid").(string),
		AllowedAddresses: allowedAddresses,
		LocalAddress:     d.Get("local_address").(string),
		LocalPort:        d.Get("local_port").(string),
	}

	proxies := []zabbix.Proxy{proxy}

	err := api.ProxiesUpdate(proxies)
	if err != nil {
		return err
	}

	log.Trace("updated proxy: %+v", proxy)

	return resourceProxyRead(d, m)
}

// resourceProxyDelete terraform proxy delete function
func resourceProxyDelete(d *schema.ResourceData, m interface{}) error {
	api := m.(*zabbix.API)
	return api.ProxiesDelete([]string{d.Id()})
}
