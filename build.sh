#!/bin/bash
set -e  # Exit on any error

# Read version from bumpver.toml
VERSION=$(grep 'current_version' bumpver.toml | cut -d'"' -f2 | sed 's/v//')
if [ -z "$VERSION" ]; then
    echo "Error: Could not read version from bumpver.toml"
    exit 1
fi

# Detect architecture
ARCH=$(uname -m)
if [ "$ARCH" = "arm64" ]; then
    ARCH="darwin_arm64"
elif [ "$ARCH" = "x86_64" ]; then
    ARCH="darwin_amd64"
else
    echo "Error: Unsupported architecture: $ARCH"
    exit 1
fi

echo "Building version: $VERSION for $ARCH"
echo "Cleaning previous build..."
go clean

echo "Downloading dependencies..."
go mod download

echo "Building provider..."
go build -o terraform-provider-zabbix

echo "Generating documentation..."
go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest
"$(go env GOPATH)/bin/tfplugindocs" generate 

# Use local path instead of registry path
PLUGIN_DIR="$HOME/.terraform.d/plugins/local/tenstorrent/zabbix/$VERSION/$ARCH"

# Create plugin directory if it doesn't exist
mkdir -p "$PLUGIN_DIR"

echo "Installing to plugin directory: $PLUGIN_DIR"
cp -v terraform-provider-zabbix "$PLUGIN_DIR/"

echo "Build and installation complete!"
echo "Source binary SHA256: $(shasum -a 256 terraform-provider-zabbix | cut -d' ' -f1)"
echo "Installed binary SHA256: $(shasum -a 256 $PLUGIN_DIR/terraform-provider-zabbix | cut -d' ' -f1)" 