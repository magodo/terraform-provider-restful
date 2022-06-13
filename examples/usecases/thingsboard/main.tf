terraform {
  required_providers {
    restful = {
      source = "magodo/restful"
    }
  }
}

variable "base_url" {
  type = string
}

variable "bearer" {
  type = string
}

variable "user_id" {
  type = string
}

provider "restful" {
  base_url = var.base_url
  security = {
    http = {
      type  = "Bearer"
      token = var.bearer
    }
  }
}

data "restful_resource" "user" {
  id = format("/user/%s", var.user_id)
}

resource "restful_resource" "customer" {
  path      = "/customer"
  name_path = "id.id"
  body = jsonencode({
    title = "Example Company"
    tenantId = {
      id         = jsondecode(data.restful_resource.user.output).tenantId.id
      entityType = "TENANT"
    }
    country  = "US"
    state    = "NY"
    city     = "New York"
    address  = "addr1"
    address2 = "addr2"
    zip      = "10004"
    phone    = "+1(415)777-7777"
    email    = "example@company.com"
  })
}

resource "restful_resource" "device_profile" {
  path      = "/deviceProfile"
  name_path = "id.id"
  body = jsonencode({
    tenantId = {
      id         = jsondecode(data.restful_resource.user.output).tenantId.id
      entityType = "TENANT"
    }
    name               = "example"
    description        = "Example device profile"
    type               = "DEFAULT"
    transportType      = "DEFAULT"
    defaultRuleChainId = null
    defaultDashboardId = null
    defaultQueueName   = null
    profileData = {
      configuration = {
        type = "DEFAULT"
      }
      transportConfiguration = {
        type = "DEFAULT"
      }
      provisionConfiguration = {
        type                  = "DISABLED"
        provisionDeviceSecret = null
      }
      alarms = null
    }
    provisionDeviceKey = null
    firmwareId         = null
    softwareId         = null
    default            = false
  })
}

resource "restful_resource" "device" {
  path      = "/device"
  name_path = "id.id"
  body = jsonencode({
    tenantId = {
      id         = jsondecode(data.restful_resource.user.output).tenantId.id
      entityType = "TENANT"
    }
    customerId = {
      id         = jsondecode(restful_resource.customer.output).id.id
      entityType = "CUSTOMER"
    }
    name  = "example"
    type  = "example"
    label = "Room 234 Sensor"
    deviceProfileId : {
      id         = jsondecode(restful_resource.device_profile.output).id.id
      entityType = "DEVICE_PROFILE"
    }
  })
}
