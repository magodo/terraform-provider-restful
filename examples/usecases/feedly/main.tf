# This needs to be run with "-parallelism=1", otherwise the "/feeds" might miss to add the feed to the collection. 
variable "token" {
  type = string
}

terraform {
  required_providers {
    restful = {
      source = "magodo/restful"
    }
  }
}

provider "restful" {
  base_url = "https://cloud.feedly.com/v3"
  security = {
    http = {
      token = {
        token = var.token
      }
    }
  }
}

locals {
  feeds = toset([
    "feed/https://arslan.io/index.xml",
    "feed/http://dave.cheney.net/category/golang/feed",
    "feed/https://jbd.dev/index.xml",
  ])
}

resource "restful_resource" "collection_go" {
  path          = "collections"
  update_method = "POST"
  read_path     = "$(path)/$(body.0.id)"
  read_selector = "0"
  body = jsonencode({
    label = "Go"
  })
}

resource "restful_resource" "feeds" {
  for_each        = local.feeds
  path            = "${restful_resource.collection_go.id}/feeds"
  create_method   = "PUT"
  create_selector = "#[feedId == \"${each.value}\"]"
  read_path       = "feeds/$(body.id)"
  delete_path     = "${restful_resource.collection_go.id}/feeds/$(body.id)"
  body = jsonencode({
    id = each.value
  })
}
