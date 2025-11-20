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
