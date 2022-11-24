terraform {
  required_providers {
    restful = {
      source = "magodo/restful"
    }
  }
}

variable "client_id" {
  type = string
}

variable "client_secret" {
  type = string
}

variable "tenant_id" {
  type = string
}

provider "restful" {
  base_url = "https://graph.microsoft.com/v1.0"
  security = {
    oauth2 = {
      client_id     = var.client_id
      client_secret = var.client_secret
      token_url     = format("https://login.microsoftonline.com/%s/oauth2/v2.0/token", var.tenant_id)
      scopes        = ["https://graph.microsoft.com/.default"]
    }
  }
}

resource "restful_resource" "group" {
  path          = "/groups"
  update_method = "PATCH"
  read_path     = "$(path)/$(body.id)"
  body = jsonencode({
    description = "Self help community for library"
    displayName = "Library Assist"
    groupTypes = [
      "Unified"
    ]
    mailEnabled     = true
    mailNickname    = "library"
    securityEnabled = false
  })
}

resource "restful_resource" "user" {
  path          = "/users"
  update_method = "PATCH"
  read_path     = "$(path)/$(body.id)"
  body = jsonencode({
    accountEnabled    = true
    mailNickname      = "AdeleV"
    displayName       = "J.Doe"
    userPrincipalName = "jdoe@wztwcygmail.onmicrosoft.com"
    passwordProfile = {
      password = "SecretP@sswd99!"
    }
  })
  write_only_attrs = [
    "mailNickname",
    "accountEnabled",
    "passwordProfile",
  ]
}
