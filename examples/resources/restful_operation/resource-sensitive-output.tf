resource "restful_operation" "get_token" {
  path               = "/auth/token"
  method             = "POST"
  use_sensitive_output = true
  body = {
    grant_type = "client_credentials"
  }
}

output "access_token" {
  value     = restful_operation.get_token.sensitive_output.access_token
  sensitive = true
}
