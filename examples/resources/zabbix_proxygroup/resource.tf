# Zabbix 7.0 and later. Hosts monitored by a proxy group are spread across
# its online proxies by the server, and moved when a proxy goes offline.
resource "zabbix_proxygroup" "site" {
  name           = "eu-west"
  description    = "EU West site proxies"
  failover_delay = "1m"
  min_online     = "1"
}

# A proxy in a group needs local_address: active agents of the group's hosts
# are redirected there to reach this particular proxy.
resource "zabbix_proxy" "site" {
  for_each = toset(["zp-1.eu-west.example.com", "zp-2.eu-west.example.com"])

  name          = each.value
  proxy_groupid = zabbix_proxygroup.site.id
  local_address = each.value
  local_port    = "10051"

  tls_accept  = "cert"
  tls_issuer  = "CN=Example CA"
  tls_subject = "CN=${each.value},O=Zabbix Proxy"
}

resource "zabbix_hostgroup" "site" {
  name = "Sites/EU West"
}

# Monitored by whichever proxy in the group the server assigns.
resource "zabbix_host" "example" {
  host          = "server-1.eu-west.example.com"
  groups        = [zabbix_hostgroup.site.id]
  proxy_groupid = zabbix_proxygroup.site.id

  interface {
    type = "agent"
    dns  = "server-1.eu-west.example.com"
  }
}
