data "zabbix_proxygroup" "site" {
  name = "eu-west"
}

resource "zabbix_hostgroup" "example" {
  name = "Sites/EU West"
}

resource "zabbix_host" "example" {
  host          = "server-2.eu-west.example.com"
  groups        = [zabbix_hostgroup.example.id]
  proxy_groupid = data.zabbix_proxygroup.site.id
}
