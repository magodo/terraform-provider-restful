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

variable "subscription_id" {
  type = string
}

provider "restful" {
  base_url = "https://management.azure.com"
  security = {
    oauth2 = {
      client_id     = var.client_id
      client_secret = var.client_secret
      token_url     = format("https://login.microsoftonline.com/%s/oauth2/v2.0/token", var.tenant_id)
      scopes        = ["https://management.azure.com/.default"]
    }
  }
  create_method = "PUT"
}

resource "restful_resource" "rg" {
  path = format("/subscriptions/%s/resourceGroups/%s", var.subscription_id, "example")
  query = {
    api-version = ["2020-06-01"]
  }
  poll_delete = {
    status_locator = "code"
    status = {
      success = "404"
      pending = ["202", "200"]
    }
  }
  body = jsonencode({
    location = "westus"
    tags = {
      foo = "bar"
    }
  })
}

locals {
  vnet_poll = {
    status_locator = "body[status]"
    status = {
      success = "Succeeded"
      failure = "Failed"
      pending = ["Pending"]
    }
    url_locator = "header[azure-asyncoperation]"
  }
}

resource "restful_resource" "vnet" {
  path = format("%s/providers/Microsoft.Network/virtualNetworks/%s", restful_resource.rg.id, "example")
  query = {
    api-version = ["2021-05-01"]
  }
  poll_create = local.vnet_poll
  poll_update = local.vnet_poll
  poll_delete = local.vnet_poll
  body = jsonencode({
    location = "westus"
    properties = {
      addressSpace = {
        addressPrefixes = ["10.0.0.0/16"]
      }
      subnets = [
        {
          name = "subnet1"
          properties = {
            addressPrefix = "10.0.1.0/24"
          }
        }
      ]
    }
  })
}
