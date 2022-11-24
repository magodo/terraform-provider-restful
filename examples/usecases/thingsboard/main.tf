# This example is a simplified version of the getting started example at: https://thingsboard.io/docs/getting-started-guides/helloworld
# The tested thingsboard API version is v3.3.3.

terraform {
  required_providers {
    restful = {
      source = "magodo/restful"
    }
  }
}

// This should be the URL of the thingsbaord-api-proxy (see: https://github.com/magodo/thingsboard-api-proxy), e.g. http://localhost:12345/api
variable "base_url" {
  type = string
}

variable "username" {
  type    = string
  default = "tenant@thingsboard.org"
}

variable "password" {
  type    = string
  default = "tenant"
}

provider "restful" {
  base_url = var.base_url
  security = {
    oauth2 = {
      token_url = format("%s/auth/login", var.base_url)
      username  = var.username
      password  = var.password
    }
  }
}

data "restful_resource" "user" {
  id = "/users"
  query = {
    pageSize = [10]
    page     = [0]
  }
  selector = format("data.#(name==%s)", var.username)
}

resource "restful_resource" "customer" {
  path      = "/customer"
  read_path = "$(path)/$(body.id.id)"
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
  read_path = "$(path)/$(body.id.id)"
  body = jsonencode({
    tenantId = {
      id         = jsondecode(data.restful_resource.user.output).tenantId.id
      entityType = "TENANT"
    }
    name               = "My Profile"
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
  read_path = "$(path)/$(body.id.id)"
  body = jsonencode({
    tenantId = {
      id         = jsondecode(data.restful_resource.user.output).tenantId.id
      entityType = "TENANT"
    }
    customerId = {
      id         = jsondecode(restful_resource.customer.output).id.id
      entityType = "CUSTOMER"
    }
    name  = "My Device"
    label = "Room 123 Sensor"
    deviceProfileId : {
      id         = jsondecode(restful_resource.device_profile.output).id.id
      entityType = "DEVICE_PROFILE"
    }
  })
}

data "restful_resource" "device_credential" {
  id = format("%s/credentials", restful_resource.device.id)
}

resource "random_uuid" "my_device_entity" {}
resource "random_uuid" "my_device_widget" {}

locals {
  my_device_entity = {
    alias = "MyDevice"
    filter = {
      resolveMultiple = false
      singleEntity = {
        entityType = "DEVICE"
        id         = jsondecode(restful_resource.device.output).id.id
      }
      type = "singleEntity"
    }
    id = random_uuid.my_device_entity.id
  }

  my_device_widget = {
    bundleAlias = "cards"
    col         = 0
    config = {
      actions         = {}
      backgroundColor = "#ff5722"
      color           = "rgba(255, 255, 255, 0.87)"
      datasources = [
        {
          dataKeys = [
            {
              _hash    = 0.009193323503694284
              color    = "#2196f3"
              label    = "temperature"
              name     = "temperature"
              settings = {}
              type     = "timeseries"
            },
          ]
          entityAliasId = local.my_device_entity.id
          filterId      = null
          name          = null
          type          = "entity"
        },
      ]
      decimals         = 0
      dropShadow       = true
      enableFullscreen = true
      padding          = "16px"
      settings = {
        labelPosition = "top"
      }
      showLegend = false
      showTitle  = false
      timewindow = {
        realtime = {
          timewindowMs = 60000
        }
      }
      title = "New Simple card"
      titleStyle = {
        fontSize   = "16px"
        fontWeight = 400
      }
      units                  = "°C"
      useDashboardTimewindow = true
      widgetStyle            = {}
    }
    description  = null
    id           = random_uuid.my_device_widget.id
    image        = null
    isSystemType = true
    row          = 0
    sizeX        = 5
    sizeY        = 3
    title        = "New widget"
    type         = "latest"
    typeAlias    = "simple_card"
  }
}

resource "restful_resource" "dashboard" {
  path      = "/dashboard"
  read_path = "$(path)/$(body.id.id)"
  body = jsonencode({
    tenantId = {
      id         = jsondecode(data.restful_resource.user.output).tenantId.id
      entityType = "TENANT"
    }
    title = "My Dashboard"
    configuration = {
      entityAliases = {
        (local.my_device_entity.id) = local.my_device_entity
      }
      filters = {}
      settings = {
        showDashboardExport     = true
        showDashboardTimewindow = true
        showDashboardsSelect    = true
        showEntitiesSelect      = true
        showTitle               = false
        stateControllerId       = "entity"
        toolbarAlwaysOpen       = true
      }
      states = {
        default = {
          layouts = {
            main = {
              gridSettings = {
                backgroundColor    = "#eeeeee"
                backgroundSizeMode = "100%"
                columns            = 24
                margin             = 10
              }
              widgets = {
                (local.my_device_widget.id) = {
                  col   = 0
                  row   = 0
                  sizeX = 5
                  sizeY = 3
                }
              }
            }
          }
          name = "My Dashboard"
          root = true
        }
      }
      widgets = {
        (local.my_device_widget.id) = local.my_device_widget
      }
    }
  })
}

output "device_token" {
  value     = jsondecode(data.restful_resource.device_credential.output).credentialsId
  sensitive = true
}
