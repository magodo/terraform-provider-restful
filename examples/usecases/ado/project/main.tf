terraform {
  required_providers {
    restful = {
      source = "magodo/restful"
    }
  }
}

variable "pat" {
  type = string
}

variable "host" {
  type = string
}

provider "restful" {
  base_url = var.host
  security = {
    http = {
      basic = {
        username = ""
        password = var.pat
      }
    }
  }
}

data "restful_resource" "process" {
  id = "_apis/process/processes"
  method = "GET"
  header = {
    "api-version": "7.2-preview"
  }
  selector = "value.#(isDefault==true)"
}

locals {
  poll = {
    url_locator = "body.url"
    status_locator = "body.status"
    status = {
      success = "succeeded"
      pending = ["inProgress", "queued"]
    }
    default_delay_sec = "3"
  }
}

resource "restful_resource" "project" {
  path = "_apis/projects"
  read_path = "_apis/projects/$(body.id)"
  update_method = "PATCH"
  query = {
    api-version = ["7.2-preview"]
  }
  post_create_read = {
    path = "_apis/projects/restful"
  }
  poll_create = local.poll
  poll_update = local.poll
  poll_delete = local.poll
  body = {
    name = "restful"
    description = "Created by the restful provider"
    visibility = "private"
    capabilities = {
      processTemplate = {
        templateTypeId = data.restful_resource.process.output.id
      }
      versioncontrol = {
        sourceControlType = "git"
      }
    }
  }

  lifecycle {
    ignore_changes = [body.capabilities]
  }
}
