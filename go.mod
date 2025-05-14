module github.com/tenstorrent/terraform-provider-zabbix

go 1.11

require (
	github.com/hashicorp/terraform v0.12.23
	github.com/hashicorp/terraform-plugin-sdk v1.7.0
	github.com/tenstorrent/go-zabbix-api v0.18.1
	github.com/tpretz/go-zabbix-api v0.16.0 // indirect
)

replace github.com/tenstorrent/go-zabbix-api => ../go-zabbix-api
