package main

import (
	"context"
	"log"

	tffwdocs "github.com/magodo/terraform-plugin-framework-docs"
	"github.com/magodo/terraform-provider-restful/internal/provider"
)

func main() {
	ctx := context.Background()
	gen, err := tffwdocs.NewGenerator(ctx, &provider.Provider{})
	if err != nil {
		log.Fatal(err)
	}
	if err := gen.WriteAll(ctx, "./docs", &tffwdocs.RenderOptions{
		Provider: &tffwdocs.ProviderRenderOption{
			Examples: []tffwdocs.Example{
				{
					Header: "No Authentication",
					HCL: []byte(`
provider "restful" {
  base_url = "http://localhost:3000"
  securty  = {} # optional
}
`),
				},
				{
					Header: "HTTP Basic",
					HCL: []byte(`
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
}
`),
				},
				{
					Header: "HTTP Token",
					HCL: []byte(`
provider "restful" {
  base_url = "http://localhost:3000"
  security = {
    http = {
      token = {
        token = "MYTOKEN"
      }
    }
  }
}
`),
				},
				{
					Header: "API Key",
					HCL: []byte(`
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
}
`),
				},
				{
					Header: "OAuth2 Client Credential",
					HCL: []byte(`
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
`),
				},
				{
					Header: "OAuth2 Password",
					HCL: []byte(`
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
}
`),
				},
				{
					Header: "OAuth2 Refresh Token",
					HCL: []byte(`
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
}
`),
				},
			},
		},
	}); err != nil {
		log.Fatal(err)
	}
}
