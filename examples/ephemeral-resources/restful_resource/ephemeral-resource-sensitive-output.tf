ephemeral "restful_resource" "token" {
  path                 = "/api/temporary-token"
  method               = "POST"
  use_sensitive_output = true

  expiry_ahead   = "5m"
  expiry_type    = "duration"
  expiry_locator = "body.expires_in"

  close_path   = "/api/revoke-token"
  close_method = "POST"
}

output "ephemeral_token" {
  value     = ephemeral.restful_resource.token.sensitive_output.token
  sensitive = true
  ephemeral = true
}
