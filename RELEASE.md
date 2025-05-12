# Release Process

This document describes the process for releasing new versions of the Terraform Provider for Zabbix.

## Prerequisites

1. GPG key for signing releases
2. GitHub repository access
3. GoReleaser installed locally (optional, for testing)

## About GPG Signing

The GPG key used for signing releases serves primarily to ensure the release artifacts (binaries, checksums) haven't been tampered with. This provides a chain of trust between the build process and the end users.

The key does not need to be signed by others or part of the Web of Trust. The trust comes from:
- The code being in your organization's repository
- The release process being automated through GitHub Actions
- The consistent use of the same key for signing across releases

## Setting up GPG Signing

### 1. Generate a new GPG key

```bash
# Create a temporary directory for the key
mkdir -p ~/terraform-provider-zabbix-gpg
cd ~/terraform-provider-zabbix-gpg

# Set up a temporary GPG home directory
export GNUPGHOME=~/terraform-provider-zabbix-gpg

# Generate a new GPG key
gpg --full-generate-key

# Follow the prompts:
# 1. Select key type (RSA and RSA)
# 2. Select key size (4096 bits)
# 3. Select how long the key should be valid (0 = key does not expire)
# 4. Enter name: "[organization]/terraform-provider-zabbix GPG signing key"
# 5. Enter email: A role-based email (not personal)
# 6. Enter a secure passphrase (store this in a secure password manager)
```

### 2. Export the GPG key

```bash
# List your GPG keys to get the key ID
gpg --list-secret-keys --keyid-format LONG

# Export the private key (replace KEY_ID with your key ID)
gpg --export-secret-keys --armor KEY_ID > private.key

# Export the public key
gpg --export --armor KEY_ID > public.key

# Clean up the temporary GPG home
unset GNUPGHOME
```

### 3. Store the keys securely

1. Store the following in your secure password manager:
   - Private key (contents of private.key)
   - Public key (contents of public.key)
   - Passphrase used during key generation
   - Key ID (from the --list-secret-keys output)

2. Add GPG keys to GitHub repository secrets:
   - Go to your GitHub repository settings
   - Navigate to "Secrets and variables" → "Actions"
   - Add the following secrets:
     - `GPG_PRIVATE_KEY`: The contents of your private.key file
     - `PASSPHRASE`: The passphrase you used when creating the GPG key

### 4. Using the key for future releases

When you need to use the key again:
```bash
# Set up the temporary GPG home
export GNUPGHOME=~/terraform-provider-zabbix-gpg

# Import the private key (from your secure storage)
gpg --import private.key

# Use the key as needed
# ...

# Clean up when done
unset GNUPGHOME
```

## How the Release Process Works

The release process is handled by GitHub Actions and GoReleaser. Here's what happens:

1. When you push a tag matching `v*` (e.g., `v0.18.0`), the release workflow is triggered
2. The workflow:
   - Checks out the code
   - Sets up Go
   - Imports the GPG key directly using standard GPG commands
   - Gets the GPG fingerprint for signing
   - Runs GoReleaser to build, package, sign, and publish the release

GoReleaser handles:
- Building the provider for multiple platforms
- Creating checksum files
- Signing the checksums with your GPG key
- Creating the GitHub release with all artifacts

## Testing a Release Locally

Before creating an official release tag, you can test the release process locally to ensure everything works correctly:

1. Make sure GoReleaser is installed locally:
   ```bash
   brew install goreleaser  # macOS
   # or use other installation methods from https://goreleaser.com/install/
   ```

2. Test the release process without publishing:
   ```bash
   # Use the GORELEASER_CURRENT_TAG environment variable to specify your target version
   GORELEASER_CURRENT_TAG=vX.Y.Z goreleaser release --snapshot --clean
   ```

   This command:
   - Uses the specified version (vX.Y.Z) instead of the current Git tag
   - Creates a snapshot release (doesn't publish anything)
   - Cleans the dist directory before building

3. Inspect the output files in the `dist/` directory to ensure they're generated correctly
   
4. Fix any issues that arise before creating the actual release tag

Note: GoReleaser uses Git tags to determine the version number, not the version in bumpver.toml or other files.

## Releasing a New Version

1. Update version numbers:
   - Update `bumpver.toml` with the new version
   - Update any version references in the code

2. Commit your changes:
   ```bash
   git add .
   git commit -m "Prepare for vX.Y.Z release"
   ```

3. Create and push a new tag:
   ```bash
   git tag -a vX.Y.Z -m "Release vX.Y.Z"
   git push origin vX.Y.Z
   ```

4. The GitHub Action will automatically:
   - Build the provider for all supported platforms
   - Create checksums
   - Sign the artifacts with your GPG key
   - Create a GitHub release

## Verifying the Release

1. Check the GitHub Actions tab to ensure the release workflow completed successfully
2. Verify the release artifacts are properly signed with:
   ```bash
   gpg --verify terraform-provider-zabbix_X.Y.Z_SHA256SUMS.sig terraform-provider-zabbix_X.Y.Z_SHA256SUMS
   ```
3. Test the provider with the new version

## Troubleshooting

If the release fails:
1. Check the GitHub Actions logs for specific errors
2. Common issues include:
   - GPG key not properly imported (check key format)
   - Passphrase issues (verify the PASSPHRASE secret)
   - Permissions problems (check GitHub token scope)
3. Ensure all secrets are correctly configured in the repository
4. Check that the version numbers are consistent across the codebase 