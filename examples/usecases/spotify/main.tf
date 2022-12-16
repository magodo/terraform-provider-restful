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
  base_url = "https://api.spotify.com/v1"
  security = {
    http = {
      token = {
        token = var.token
      }
    }
  }
}

data "restful_resource" "me" {
  id = "/me"
}

resource "restful_resource" "playlist" {
  path        = "/users/${jsondecode(data.restful_resource.me.output).id}/playlists"
  read_path   = "/playlists/$(body.id)"
  delete_path = "/playlists/$(body.id)/followers"
  body = jsonencode({
    name = "World Cup (by Terraform)"
  })
}

locals {
  my_favorite_tracks = {
    "The Cup of Life" : "Ricky Martin",
    "Wavin' Flag" : "K'NAAN",
    "Waka Waka" : "Shakira",
  }
}

data "restful_resource" "track" {
  for_each = local.my_favorite_tracks
  id       = "/search"
  query = {
    q     = ["${each.key}", "artist:${each.value}"]
    type  = ["track"]
    limit = [1]
  }
}

resource "restful_operation" "add_tracks_to_playlist" {
  path   = "${restful_resource.playlist.id}/tracks"
  method = "PUT"
  body = jsonencode({
    uris = [for d in data.restful_resource.track : jsondecode(d.output).tracks.items[0].uri]
  })
}
