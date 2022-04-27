package provider_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/magodo/terraform-provider-restful/internal/acceptance"
	"github.com/magodo/terraform-provider-restful/internal/client"
)

const RESTFUL_AZURE_TENANT_ID = "RESTFUL_AZURE_TENANT_ID"
const RESTFUL_AZURE_SUBSCRIPTION_ID = "RESTFUL_AZURE_SUBSCRIPTION_ID"
const RESTFUL_AZURE_CLIENT_ID = "RESTFUL_AZURE_CLIENT_ID"
const RESTFUL_AZURE_CLIENT_SECRET = "RESTFUL_AZURE_CLIENT_SECRET"

type azureData struct {
	url            string
	tenantId       string
	subscriptionId string
	clientId       string
	clientSecret   string

	rd acceptance.Rd
}

func (d azureData) precheck(t *testing.T) {
	if d.tenantId == "" {
		t.Skipf("%q is not specified", RESTFUL_AZURE_TENANT_ID)
	}
	if d.subscriptionId == "" {
		t.Skipf("%q is not specified", RESTFUL_AZURE_SUBSCRIPTION_ID)
	}
	if d.clientId == "" {
		t.Skipf("%q is not specified", RESTFUL_AZURE_CLIENT_ID)
	}
	if d.clientSecret == "" {
		t.Skipf("%q is not specified", RESTFUL_AZURE_CLIENT_SECRET)
	}
	return
}

func newAzureData() azureData {
	return azureData{
		url:            "https://management.azure.com",
		tenantId:       os.Getenv(RESTFUL_AZURE_TENANT_ID),
		subscriptionId: os.Getenv(RESTFUL_AZURE_SUBSCRIPTION_ID),
		clientId:       os.Getenv(RESTFUL_AZURE_CLIENT_ID),
		clientSecret:   os.Getenv(RESTFUL_AZURE_CLIENT_SECRET),

		rd: acceptance.NewRd(),
	}
}

func TestResource_Azure_ResourceGroup(t *testing.T) {
	addr := "restful_resource.test"
	d := newAzureData()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.resourceGroup(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(addr, "output"),
				),
			},
			// {
			// 	ResourceName:      addr,
			// 	ImportState:       true,
			// 	ImportStateIdFunc: func(s *terraform.State) (string, error) { return "", nil }, //TODO
			// },
		},
	})
}

func (d azureData) CheckDestroy(addr string) func(*terraform.State) error {
	return func(s *terraform.State) error {
		c, err := client.New(d.url, &client.BuildOption{
			Security: client.OAuth2ClientCredentialOption{
				ClientID:     d.clientId,
				ClientSecret: d.clientSecret,
				TokenURL:     fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", d.tenantId),
				Scopes: []string{
					"https://management.azure.com/.default",
				},
			},
		})
		if err != nil {
			return err
		}
		for key, resource := range s.RootModule().Resources {
			if key != addr {
				continue
			}
			ver := resource.Primary.Attributes["query.api-version"]
			_, err := c.Read(context.TODO(), resource.Primary.ID, client.ReadOption{Query: map[string]string{"api-version": ver}})
			if err == nil {
				return fmt.Errorf("%s: still exists", addr)
			}
			if err == client.ErrNotFound {
				return nil
			}
			return fmt.Errorf("reading %s: %v", addr, err)
		}
		panic("unreachable")
	}
}

func (d azureData) resourceGroup() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
  security = {
    oauth2 = {
      client_id     = %q
      client_secret = %q
      token_url     = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
      scopes        = ["https://management.azure.com/.default"]
    }
  }
  create_method = "PUT"
}

resource "restful_resource" "test" {
  path = "/subscriptions/%s/resourceGroups/restful-test-%d"
  query = {
    api-version = "2020-06-01"
  }
  body = jsonencode({
    location = "westeurope"
  })
}
`, d.url, d.clientId, d.clientSecret, d.tenantId, d.subscriptionId, d.rd.RandomIntOfLength(8))
}