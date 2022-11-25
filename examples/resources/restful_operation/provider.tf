provider "restful" {
  base_url = "https://management.azure.com"
  security = {
    oauth2 = {
      client_credentials = {
        client_id     = var.client_id
        client_secret = var.client_secret
        token_url     = format("https://login.microsoftonline.com/%s/oauth2/v2.0/token", var.tenant_id)
        scopes        = ["https://management.azure.com/.default"]
      }
    }
  }
}

