terraform {
  required_providers {
    restful = {
      source = "magodo/restful"
    }
  }
}

provider "restful" {
  base_url = "http://localhost:3000"
  alias    = "no_auth"
}

provider "restful" {
  base_url = "http://localhost:3000"
  security = {
    http = {
      basic = {
        username = "foo"
        password = "bar"
      }
    }
  }
  alias = "http_basic"
}

provider "restful" {
  base_url = "http://localhost:3000"
  security = {
    http = {
      token = {
        token = "MYTOKEN"
      }
    }
  }
  alias = "http_token"
}

provider "restful" {
  base_url = "http://localhost:3000"
  security = {
    apikey = [
      {
        in    = "header"
        name  = "Fastly-Key"
        value = "AAAAAAABBBBBBCCCCCC"
      },
    ]
  }
  alias = "api_key"
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
  alias = "oauth2_client_credentials"
}

provider "restful" {
  base_url = var.base_url
  security = {
    oauth2 = {
      password = {
        token_url = format("%s/auth/login", var.base_url)
        username  = var.username
        password  = var.password
      }
    }
  }
  alias = "oauth2_password"
}

provider "restful" {
  base_url = "https://management.azure.com"
  security = {
    oauth2 = {
      refresh_token = {
        token_url     = format("https://login.microsoftonline.com/%s/oauth2/v2.0/token", var.tenant_id)
        refresh_token = var.refresh_token
      }
    }
  }
  alias = "oauth2_refresh_token"
}
