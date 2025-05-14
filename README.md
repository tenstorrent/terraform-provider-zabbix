# Terraform Provider for Zabbix

A [Terraform](https://terraform.io) provider for [Zabbix](https://www.zabbix.com).
Based on tpretz/zabbix and ppodevlabs/zabbix - our thanks for the work of many contributors to this.

## Documentation

For detailed documentation about the provider and its resources, please visit the [provider documentation](docs/index.md).

## Requirements

- Access to Zabbix API over http or https
- Terraform 0.12 or newer

## Quick Start

1. Install the provider by placing the binary in your Terraform plugins directory
2. Configure the provider:

```hcl
provider "zabbix" {
  # Required
  username = "<api_user>"
  password = "<api_password>"
  url      = "http://example.com/api_jsonrpc.php"
  
  # Optional
  tls_insecure = false  # Disable TLS verification (false by default)
  serialize    = false  # Serialize Zabbix API calls (false by default)
}
```

3. See the [examples](examples/) directory for usage examples

## Features

- Create and manage Zabbix hosts, hostgroups, and templates
- Support for various item types (agent, SNMP, HTTP, etc.)
- Low-level discovery (LLD) support
- Proxy management
- Template to Terraform conversion tool

## Development

### Building

For now, a relatively naive build script has been produced:
```bash
./build.sh
```

This will:
1. Build the provider
2. Generate documentation
3. Install the provider to your local Terraform plugins directory

### Testing

No automated testing has been implemented yet.

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## License

This project is licensed under the terms of the MIT license.