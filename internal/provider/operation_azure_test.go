package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/lfventura/terraform-provider-restful/internal/acceptance"
)

func TestOperationResource_Azure_Register_RP(t *testing.T) {
	addr := "restful_operation.test"
	d := newAzureData()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.unregisterRP("Microsoft.ProviderHub"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				Config: d.registerRP("Microsoft.ProviderHub"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
		},
	})
}

func TestOperationResource_Azure_GetToken(t *testing.T) {
	addr := "restful_operation.test"
	d := newAzureData()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.getToken(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("access_token"), knownvalue.NotNull()),
				},
			},
		},
	})
}

func (d azureData) registerRP(rp string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %[1]q
  security = {
    oauth2 = {
	  client_credentials = {
		  client_id     = %[2]q
		  client_secret = %[3]q
		  token_url     = "https://login.microsoftonline.com/%[4]s/oauth2/v2.0/token"
		  scopes        = ["https://management.azure.com/.default"]
	  }
    }
  }
}

resource "restful_operation" "test" {
  path = "/subscriptions/%[5]s/providers/%[6]s/register"
  query = {
    api-version = ["2014-04-01-preview"]
  }
  method = "POST"
  poll = {
	url_locator = "exact./subscriptions/%[5]s/providers/%[6]s?api-version=2014-04-01-preview"
    status_locator = "body.registrationState"
    status = {
      success = "Registered"
      pending = ["Registering"]
    }
  }
}
`, d.url, d.clientId, d.clientSecret, d.tenantId, d.subscriptionId, rp)
}

func (d azureData) unregisterRP(rp string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %[1]q
  security = {
    oauth2 = {
	  client_credentials = {
        client_id     = %[2]q
        client_secret = %[3]q
        token_url     = "https://login.microsoftonline.com/%[4]s/oauth2/v2.0/token"
        scopes        = ["https://management.azure.com/.default"]
	  }
    }
  }
}

resource "restful_operation" "test" {
  path = "/subscriptions/%[5]s/providers/%[6]s/unregister"
  query = {
    api-version = ["2014-04-01-preview"]
  }
  method = "POST"
  poll = {
	url_locator = "exact./subscriptions/%[5]s/providers/%[6]s?api-version=2014-04-01-preview"
    status_locator = "body.registrationState"
    status = {
      success = "Unregistered"
      pending = ["Unregistering"]
    }
  }
}
`, d.url, d.clientId, d.clientSecret, d.tenantId, d.subscriptionId, rp)
}

func (d azureData) getToken() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = "https://login.microsoftonline.com"
}

resource "restful_operation" "test" {
  path   = "/%s/oauth2/v2.0/token"
  method = "POST"
  header = {
    Accept : "application/json",
    Content-Type : "application/x-www-form-urlencoded",
  }
  body = {
    client_id = "%s"
    client_secret = "%s"
    grant_type = "client_credentials"
    scope = "https://management.azure.com/.default"
  }
}
`, d.tenantId, d.clientId, d.clientSecret)
}
