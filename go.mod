module github.com/ppodevlabs/terraform-provider-zabbix

go 1.11

require (
	github.com/hashicorp/terraform v0.12.23
	github.com/hashicorp/terraform-plugin-sdk v1.7.0
	github.com/ppodevlabs/go-zabbix-api v0.16.4
	github.com/tpretz/go-zabbix-api v0.16.0 // indirect
)

replace github.com/ppodevlabs/go-zabbix-api => ../go-zabbix-api
