# Publishing to Terraform Registry

This guide explains how to publish this fork to the Terraform Registry under the `lfventura` namespace.

## Prerequisites

1. **GitHub Repository**: Ensure your fork is pushed to GitHub at `github.com/lfventura/terraform-provider-restful`

2. **GPG Key**: You need a GPG key to sign releases
   ```bash
   # Generate a GPG key if you don't have one
   gpg --full-generate-key
   
   # Export your public key
   gpg --armor --export YOUR_KEY_ID
   ```

3. **GitHub Secrets**: Add the following secrets to your GitHub repository:
   - `GPG_PRIVATE_KEY`: Your GPG private key
   - `PASSPHRASE`: Your GPG key passphrase

4. **Terraform Registry Account**: Sign in to https://registry.terraform.io with your GitHub account

## Publishing Steps

### 1. Register the Provider

1. Go to https://registry.terraform.io
2. Click "Publish" → "Provider"
3. Select your repository: `lfventura/terraform-provider-restful`
4. Follow the registration process

### 2. Create a Release

Create and push a version tag to trigger the release workflow:

```bash
# Tag the release (use semantic versioning)
git tag v1.0.0

# Push the tag to GitHub
git push origin v1.0.0
```

The GitHub Actions workflow (`.github/workflows/release.yml`) will automatically:
- Build binaries for multiple platforms
- Sign the release with your GPG key
- Create a GitHub release
- Upload artifacts that Terraform Registry will index

### 3. Verify Publication

After the release is complete:
1. Check the GitHub release page
2. The Terraform Registry will automatically detect and index the new version
3. Users can then use your provider:

```hcl
terraform {
  required_providers {
    restful = {
      source  = "lfventura/restful"
      version = "~> 1.0"
    }
  }
}
```

## Version Guidelines

- Use semantic versioning: `MAJOR.MINOR.PATCH`
- `MAJOR`: Breaking changes
- `MINOR`: New features (backward compatible)
- `PATCH`: Bug fixes

## Troubleshooting

### GPG Signing Issues
If release signing fails, verify:
- Your GPG secrets are correctly set in GitHub
- The key is not expired
- The passphrase is correct

### Registry Not Updating
The registry indexes new versions every few minutes. If it takes too long:
- Check that the release artifacts are properly generated
- Ensure the `terraform-registry-manifest.json` is included in the release
- Verify the GPG signature is valid

## Maintaining the Fork

When upstream updates are available:

```bash
# Add upstream remote (one time)
git remote add upstream https://github.com/magodo/terraform-provider-restful.git

# Fetch and merge updates
git fetch upstream
git merge upstream/main

# Resolve conflicts if any
# Test thoroughly
# Create a new release
```

## Support

For issues specific to this fork:
- Open an issue at: https://github.com/lfventura/terraform-provider-restful/issues

For upstream features or bugs:
- Refer to the original repository: https://github.com/magodo/terraform-provider-restful
