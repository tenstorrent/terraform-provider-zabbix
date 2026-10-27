# By numeric id, which the frontend shows in the URL of the proxy group's
# configuration form...
terraform import zabbix_proxygroup.site 3

# ...or by name. An import id made only of digits is always taken as an id.
terraform import zabbix_proxygroup.site eu-west
