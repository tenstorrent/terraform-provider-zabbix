# Zabbix objects are imported by their numeric id, which the frontend shows
# in the URL of the object's configuration form.
terraform import zabbix_proxy.active 10

# TT-PATCH: TT-004 -- the technical name works too. An import id made only of
# digits is always taken as an id.
terraform import zabbix_proxy.active proxy-eu-west

# tls_psk_identity and tls_psk are write-only in the Zabbix API -- proxy.get
# never returns them -- so they import as empty. Re-add them to the
# configuration afterwards; the next apply sends them again.
