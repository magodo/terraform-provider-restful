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
					HCL: `
provider "restful" {
  base_url = "http://localhost:3000"
  securty  = {} # optional
}
`,
				},
				{
					Header: "HTTP Basic",
					HCL: `
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
`,
				},
				{
					Header: "HTTP Token",
					HCL: `
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
`,
				},
				{
					Header: "API Key",
					HCL: `
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
`,
				},
				{
					Header: "OAuth2 Client Credential",
					HCL: `
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
`,
				},
				{
					Header: "OAuth2 Password",
					HCL: `
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
`,
				},
				{
					Header: "OAuth2 Refresh Token",
					HCL: `
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
`,
				},
			},
		},
		Resources: map[string]tffwdocs.ResourceRenderOption{
			"restful_resource": {
				Examples: []tffwdocs.Example{
					{
						Header: "Azure Resource Group",
						HCL: `
resource "restful_resource" "rg" {
  path = format("/subscriptions/%s/resourceGroups/%s", var.subscription_id, "example")
  query = {
    api-version = ["2020-06-01"]
  }
  create_method = "PUT"
  poll_delete = {
    status_locator = "code"
    status = {
      success = "404"
      pending = ["202", "200"]
    }
  }
  body = {
    location = "westus"
    tags = {
      foo = "bar"
    }
  }
}
					`,
					},
				},
				ImportId: &tffwdocs.ImportId{
					Format: `
- id (Required)                        : The resource id.
- path (Required)                      : The path used to create the resource (as this is force new)
- query (Optional)                     : The query parameters.
- header (Optional)                    : The header.
- body (Optional)                      : The interested properties in the response body that you want to manage via this resource.
                                         If you omit this, then all the properties will be keeping track, which in most cases is 
                                         not what you want (e.g. the read only attributes shouldn't be managed).
                                         The value of each property is not important here, hence leave them as "null".
- read_selector (Optional)             : The read_selector used to specify the resource from a collection of resources.
- read_response_template (Optional)    : The read_response_template used to transform the structure of the read response.
`,
					ExampleCmdArg: `{
  "id": "/subscriptions/0-0-0-0/resourceGroups/example",
  "path": "/subscriptions/0-0-0-0/resourceGroups/example",
  "query": {"api-version": ["2020-06-01"]},
  "body": {
    "location": null,
    "tags": null
  }
}`,
					ExampleBlk: `import {
  to = restful_resource.test
  id = jsonencode({
    id = "/posts/1"
    path = "/posts"
    body = {
      foo = null
    }
    header = {
      key = "val"
    }
    query = {
      x = ["y"]
    }
  })
}`,
				},
				IdentityExamples: []tffwdocs.Example{
					{
						HCL: `
import {
  to = restful_resource.test
  identity = {
    id = jsonencode({
      id = "/posts/1"
      path = "/posts"
      body = {
        foo = null
      }
      header = {
        key = "val"
      }
      query = {
        x = ["y"]
      }
    })
  }
}
`,
					},
				},
			},
			"restful_operation": {
				Examples: []tffwdocs.Example{
					{
						Header: "Azure Register RP",
						HCL: `
resource "restful_operation" "register_rp" {
  path = format("/subscriptions/%s/providers/Microsoft.ProviderHub/register", var.subscription_id)
  query = {
    api-version = ["2014-04-01-preview"]
  }
  method = "POST"
  poll = {
    url_locator    = format("exact./subscriptions/%s/providers/Microsoft.ProviderHub?api-version=2014-04-01-preview", var.subscription_id)
    status_locator = "body.registrationState"
    status = {
      success = "Registered"
      pending = ["Registering"]
    }
  }
}
`,
					},
				},
			},
		},
		DataSources: map[string]tffwdocs.DataSourceRenderOption{
			"restful_resource": {
				Examples: []tffwdocs.Example{
					{
						HCL: `
data "restful_resource" "test" {
  id = "/posts/1"
}
`,
					},
				},
			},
		},
		EphemeralResources: map[string]tffwdocs.EphemeralResourceRenderOption{
			"restful_resource": {
				Examples: []tffwdocs.Example{
					{
						HCL: `
ephemeral "restful_resource" "test" {
  path = "/lease"
  method = "POST"

  renew_path = "/updateLease"
  renew_method = "POST"

  expiry_ahead = "0.5s"
  expiry_type = "duration"
  expiry_locator = "header.expiry"

  close_path = "/unlease"
  close_method = "POST"
}
`,
					},
				},
			},
		},
	}); err != nil {
		log.Fatal(err)
	}
}
