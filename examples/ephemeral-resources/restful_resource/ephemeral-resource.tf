ephemeral "restful_resource" "test" {
  path = "/lease"
  method = "POST"

  renew_path = "/updateLease"
  renew_method = "POST"

  expiry_ahead = "0.5s"
  expiry_type = "duration"
  expiry_locator = "header.expiry"

  close_path = "/unlease"
  close_method = "POST"
}
