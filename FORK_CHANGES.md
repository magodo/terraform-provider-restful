# Fork Changes

This is a fork of [magodo/terraform-provider-restful](https://github.com/magodo/terraform-provider-restful) maintained by [@lfventura](https://github.com/lfventura).

## Upstream Pull Request

The changes in this fork have been submitted to the upstream repository:
- PR: https://github.com/magodo/terraform-provider-restful/pull/164

## Additional Features

### Sensitive Output Support

Added support for marking resource outputs as sensitive in Terraform:

- **New Attribute: `use_sensitive_output`** (Boolean, Optional)
  - When set to `true`, the response will be stored in `sensitive_output` instead of `output`
  - Defaults to `false`
  - Available in: `restful_resource`, `restful_operation`, `data.restful_resource`, `ephemeral.restful_resource`

- **New Output: `sensitive_output`** (Dynamic, Read-Only, Sensitive)
  - Contains the API response when `use_sensitive_output = true`
  - Automatically marked as sensitive by Terraform
  - Available in all resource types

### Usage Example

```hcl
resource "restful_resource" "secret" {
  path = "/api/secrets"
  use_sensitive_output = true
  body = {
    key   = "api_key"
    value = "secret_value"
  }
}

output "secret_data" {
  value     = restful_resource.secret.sensitive_output
  sensitive = true
}
```

## Publishing

This fork is published to the Terraform Registry as:
```hcl
terraform {
  required_providers {
    restful = {
      source = "lfventura/restful"
    }
  }
}
```

## Compatibility

This fork maintains full compatibility with the upstream version. Existing configurations will continue to work without modifications.
