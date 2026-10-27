# Zabbix 7.0 and later. Hosts monitored by a proxy group are spread across
# its online proxies by the server, and moved when a proxy goes offline.
resource "zabbix_proxygroup" "site" {
  name           = "eu-west"
  description    = "EU West site proxies"
  failover_delay = "1m"
  min_online     = "1"
}
