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
      client_credentials = {
        client_id     = var.client_id
        client_secret = var.client_secret
        token_url     = format("https://login.microsoftonline.com/%s/oauth2/v2.0/token", var.tenant_id)
        scopes        = ["https://management.azure.com/.default"]
      }
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
  poll = {
    status_locator = "body.status"
    status = {
      success = "Succeeded"
      failure = "Failed"
      pending = ["Pending"]
    }
    url_locator = "header.azure-asyncoperation"
  }
  route_precheck = [
    {
      mutex = restful_resource.table.id
    }
  ]
}

resource "restful_resource" "table" {
  path          = format("%s/providers/Microsoft.Network/routeTables/%s", restful_resource.rg.id, "example")
  update_method = "PATCH" # This avoids update to clean up the routes in this table
  query = {
    api-version = ["2022-07-01"]
  }
  body = jsonencode({
    location = "westus"
  })
  poll_create = local.poll
  poll_delete = local.poll
}

resource "restful_resource" "route1" {
  path = format("%s/routes/%s", restful_resource.table.id, "route1")
  query = {
    api-version = ["2022-07-01"]
  }

  precheck_create = local.route_precheck
  precheck_update = local.route_precheck
  precheck_delete = local.route_precheck

  poll_create = local.poll
  poll_update = local.poll
  poll_delete = local.poll

  body = jsonencode({
    properties = {
      nextHopType   = "VnetLocal"
      addressPrefix = "10.1.0.0/16"
    }
  })
}

resource "restful_resource" "route2" {
  path = format("%s/routes/%s", restful_resource.table.id, "route2")
  query = {
    api-version = ["2022-07-01"]
  }

  precheck_create = local.route_precheck
  precheck_update = local.route_precheck
  precheck_delete = local.route_precheck

  poll_create = local.poll
  poll_update = local.poll
  poll_delete = local.poll

  body = jsonencode({
    properties = {
      nextHopType   = "VnetLocal"
      addressPrefix = "10.2.0.0/16"
    }
  })
}
