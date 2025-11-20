data "restful_resource" "credentials" {
  id                   = "/api/credentials/1"
  use_sensitive_output = true
}

output "credential_value" {
  value     = data.restful_resource.credentials.sensitive_output
  sensitive = true
}
