package provider_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/lfventura/terraform-provider-restful/internal/acceptance"
	"github.com/lfventura/terraform-provider-restful/internal/client"
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
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.resourceGroup(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"poll_delete", "create_method"},
				ImportStateIdFunc:       d.resourceGroupImportStateIdFunc(addr),
			},
			{
				Config: d.resourceGroup_complete(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"poll_delete", "create_method"},
				ImportStateIdFunc:       d.resourceGroupCompleteImportStateIdFunc(addr),
			},
		},
	})
}

func TestResource_Azure_ResourceGroup_updatePath(t *testing.T) {
	addr := "restful_resource.test"
	d := newAzureData()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.resourceGroup_updatePath(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"poll_delete", "create_method", "update_path"},
				ImportStateIdFunc:       d.resourceGroupUpdatePathImportStateIdFunc(addr),
			},
			{
				Config: d.resourceGroup_updatePath_complete(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"poll_delete", "create_method", "update_path"},
				ImportStateIdFunc:       d.resourceGroupUpdatePathCompleteImportStateIdFunc(addr),
			},
		},
	})
}

func TestResource_Azure_VirtualNetwork(t *testing.T) {
	addr := "restful_resource.test"
	d := newAzureData()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.vnet("foo"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"poll_create", "poll_update", "poll_delete", "create_method"},
				ImportStateIdFunc:       d.vnetImportStateIdFunc(addr),
			},
			{
				Config: d.vnet("bar"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"poll_create", "poll_update", "poll_delete", "create_method"},
				ImportStateIdFunc:       d.vnetImportStateIdFunc(addr),
			},
		},
	})
}

func TestResource_Azure_VirtualNetwork_Precheck(t *testing.T) {
	addr := "restful_resource.test"
	d := newAzureData()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.vnet_precheck("foo"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"poll_create", "poll_update", "poll_delete", "precheck_create", "precheck_update", "precheck_delete", "create_method"},
				ImportStateIdFunc:       d.vnetImportStateIdFunc(addr),
			},
			{
				Config: d.vnet_precheck("bar"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"poll_create", "poll_update", "poll_delete", "precheck_create", "precheck_update", "precheck_delete", "create_method"},
				ImportStateIdFunc:       d.vnetImportStateIdFunc(addr),
			},
		},
	})
}

func TestResource_Azure_VirtualNetwork_SimplePoll(t *testing.T) {
	addr := "restful_resource.test"
	d := newAzureData()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.vnet_simple_poll("foo"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"poll_create", "poll_update", "poll_delete", "create_method"},
				ImportStateIdFunc:       d.vnetImportStateIdFunc(addr),
			},
			{
				Config: d.vnet_simple_poll("bar"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"poll_create", "poll_update", "poll_delete", "create_method"},
				ImportStateIdFunc:       d.vnetImportStateIdFunc(addr),
			},
		},
	})
}

func TestResource_Azure_RouteTable_Precheck(t *testing.T) {
	addr := "restful_resource.route1"
	d := newAzureData()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.routetable_precheck("foo"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"poll_create", "poll_update", "poll_delete", "precheck_create", "precheck_update", "precheck_delete", "create_method", "output.etag"},
				ImportStateIdFunc:       d.routeImportStateIdFunc(addr),
			},
			{
				Config: d.routetable_precheck("bar"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"poll_create", "poll_update", "poll_delete", "precheck_create", "precheck_update", "precheck_delete", "create_method", "output.etag"},
				ImportStateIdFunc:       d.routeImportStateIdFunc(addr),
			},
		},
	})
}

func (d azureData) CheckDestroy(addr string) func(*terraform.State) error {
	return func(s *terraform.State) error {
		ctx := context.TODO()
		c, err := client.New(ctx, d.url, &client.BuildOption{
			Security: client.OAuth2ClientCredentialOption{
				ClientId:     d.clientId,
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
		c.SetLoggerContext(ctx)

		resource := s.RootModule().Resources[addr]
		if resource != nil {
			ver := resource.Primary.Attributes["query.api-version.0"]
			resp, err := c.Read(ctx, resource.Primary.ID, client.ReadOption{Query: map[string][]string{"api-version": {ver}}})
			if err != nil {
				return fmt.Errorf("reading %s: %v", addr, err)
			}
			if resp.StatusCode() != http.StatusNotFound {
				return fmt.Errorf("%s: still exists", addr)
			}
		}
		return nil
	}
}

func (d azureData) resourceGroup() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
  security = {
    oauth2 = {
	  client_credentials = {
		client_id     = %q
		client_secret = %q
		token_url     = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
		scopes        = ["https://management.azure.com/.default"]
	  }
    }
  }
}

resource "restful_resource" "test" {
  path = "/subscriptions/%s/resourceGroups/acctest-%d"
  query = {
    api-version = ["2020-06-01"]
  }
  body = {
    location = "westeurope"
  }

  create_method = "PUT"

  poll_delete = {
    status_locator = "code"
    status = {
      success = "200"
      pending = ["202"]
    }
    url_locator = "header.location"
  }
}
`, d.url, d.clientId, d.clientSecret, d.tenantId, d.subscriptionId, d.rd)
}
func (d azureData) resourceGroup_complete() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
  security = {
    oauth2 = {
	  client_credentials = {
		client_id     = %q
		client_secret = %q
		token_url     = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
		scopes        = ["https://management.azure.com/.default"]
	  }
    }
  }
}

resource "restful_resource" "test" {
  path = "/subscriptions/%s/resourceGroups/acctest-%d"
  query = {
    api-version = ["2020-06-01"]
  }
  body = {
    location = "westeurope"
	tags = {
	  foo = "bar"
	}
  }

  create_method = "PUT"

  poll_delete = {
    status_locator = "code"
    status = {
      success = "200"
      pending = ["202"]
    }
    url_locator = "header.location"
  }
}
`, d.url, d.clientId, d.clientSecret, d.tenantId, d.subscriptionId, d.rd)
}

func (d azureData) resourceGroup_updatePath() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
  security = {
    oauth2 = {
	  client_credentials = {
		client_id     = %q
		client_secret = %q
		token_url     = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
		scopes        = ["https://management.azure.com/.default"]
	  }
    }
  }
}

resource "restful_resource" "test" {
  path = "/subscriptions/%s/resourceGroups/acctest-%d"
  query = {
    api-version = ["2020-06-01"]
  }
  body = {
    location = "westeurope"
  }

  create_method = "PUT"

  update_path = "$(path)"

  poll_delete = {
    status_locator = "code"
    status = {
      success = "200"
      pending = ["202"]
    }
    url_locator = "header.location"
  }
}
`, d.url, d.clientId, d.clientSecret, d.tenantId, d.subscriptionId, d.rd)
}

func (d azureData) resourceGroup_updatePath_complete() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
  security = {
    oauth2 = {
	  client_credentials = {
		client_id     = %q
		client_secret = %q
		token_url     = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
		scopes        = ["https://management.azure.com/.default"]
	  }
    }
  }
}

resource "restful_resource" "test" {
  path = "/subscriptions/%s/resourceGroups/acctest-%d"
  query = {
    api-version = ["2020-06-01"]
  }
  body = {
    location = "westeurope"
	tags = {
	  foo = "bar"
	}
  }

  create_method = "PUT"

  update_path = "$(path)"

  poll_delete = {
    status_locator = "code"
    status = {
      success = "200"
      pending = ["202"]
    }
    url_locator = "header.location"
  }
}
`, d.url, d.clientId, d.clientSecret, d.tenantId, d.subscriptionId, d.rd)
}

func (d azureData) resourceGroupImportStateIdFunc(addr string) func(s *terraform.State) (string, error) {
	return func(s *terraform.State) (string, error) {
		return fmt.Sprintf(`{
"id": %[1]q,
"query": {
  "api-version": ["2020-06-01"]
},
"path": %[1]q,
"body": {
  "location": null
}
}`, s.RootModule().Resources[addr].Primary.Attributes["id"]), nil
	}
}

func (d azureData) resourceGroupCompleteImportStateIdFunc(addr string) func(s *terraform.State) (string, error) {
	return func(s *terraform.State) (string, error) {
		return fmt.Sprintf(`{
"id": %[1]q,
"query": {
  "api-version": ["2020-06-01"]
},
"path": %[1]q,
"body": {
  "location": null,
  "tags": null
}
}`, s.RootModule().Resources[addr].Primary.Attributes["id"]), nil
	}
}

func (d azureData) resourceGroupUpdatePathImportStateIdFunc(addr string) func(s *terraform.State) (string, error) {
	return func(s *terraform.State) (string, error) {
		return fmt.Sprintf(`{
"id": %[1]q,
"query": {
  "api-version": ["2020-06-01"]
},
"path": %[1]q,
"update_path": %[1]q,
"body": {
  "location": null
}
}`, s.RootModule().Resources[addr].Primary.Attributes["id"]), nil
	}
}

func (d azureData) resourceGroupUpdatePathCompleteImportStateIdFunc(addr string) func(s *terraform.State) (string, error) {
	return func(s *terraform.State) (string, error) {
		return fmt.Sprintf(`{
"id": %[1]q,
"query": {
  "api-version": ["2020-06-01"]
},
"path": %[1]q,
"update_path": %[1]q,
"body": {
  "location": null,
  "tags": null
}
}`, s.RootModule().Resources[addr].Primary.Attributes["id"]), nil
	}
}

func (d azureData) vnetImportStateIdFunc(addr string) func(s *terraform.State) (string, error) {
	return func(s *terraform.State) (string, error) {
		return fmt.Sprintf(`{
  "id": %[1]q,
  "query": {
    "api-version": ["2021-05-01"]
  },
  "path": %[1]q,
  "body": {
    "location": null,
    "properties": {
      "addressSpace": {
        "addressPrefixes": null
      }
	},
	"tags": null
  }
}`, s.RootModule().Resources[addr].Primary.Attributes["id"]), nil
	}
}

func (d azureData) routeImportStateIdFunc(addr string) func(s *terraform.State) (string, error) {
	return func(s *terraform.State) (string, error) {
		return fmt.Sprintf(`{
  "id": %[1]q,
  "query": {
    "api-version": ["2022-07-01"]
  },
  "path": %[1]q,
  "body": {
    "properties": {
      "addressPrefix": null,
	  "nextHopType": null
	}
  }
}`, s.RootModule().Resources[addr].Primary.Attributes["id"]), nil
	}
}

func (d azureData) vnet_template() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
  security = {
    oauth2 = {
	  client_credentials = {
		client_id     = %q
		client_secret = %q
		token_url     = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
		scopes        = ["https://management.azure.com/.default"]
	  }
    }
  }
  create_method = "PUT"
}

resource "restful_resource" "rg" {
  path = "/subscriptions/%s/resourceGroups/acctest-%d"
  query = {
    api-version = ["2020-06-01"]
  }
  body = {
    location = "westeurope"
  }

  poll_delete = {
    status_locator = "code"
    status = {
      success = "200"
      pending = ["202"]
    }
    url_locator = "header.location"
  }
}

`, d.url, d.clientId, d.clientSecret, d.tenantId, d.subscriptionId, d.rd)
}

func (d azureData) vnet(tag string) string {
	return fmt.Sprintf(`
%s

locals {
  vnet_poll = {
    status_locator = "body.status"
    status = {
      success = "Succeeded"
      pending = ["Pending"]
    }
    url_locator = "header.azure-asyncoperation"
  }
}

resource "restful_resource" "test" {
  path = format("%%s/providers/Microsoft.Network/virtualNetworks/restful-test-%%d", restful_resource.rg.id, %d)
  query = {
    api-version = ["2021-05-01"]
  }

  poll_create = local.vnet_poll
  poll_update = local.vnet_poll
  poll_delete = local.vnet_poll

  body = {
    location = "westus"
    properties = {
      addressSpace = {
        addressPrefixes = ["10.0.0.0/16"]
      }
    }
    tags = {
      foo = "%s"
    }
  }
}
`, d.vnet_template(), d.rd, tag)
}

// Note that the precheck used here is meaningless, only meant to test this feature won't cause issue.
func (d azureData) vnet_precheck(tag string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
  security = {
    oauth2 = {
	  client_credentials = {
		client_id     = %q
		client_secret = %q
		token_url     = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
		scopes        = ["https://management.azure.com/.default"]
	  }
    }
  }
  create_method = "PUT"
}

resource "restful_resource" "rg" {
  path = "/subscriptions/%s/resourceGroups/acctest-%d"
  query = {
    api-version = ["2020-06-01"]
  }
  body = {
    location = "westeurope"
  }

  poll_delete = {
    status_locator = "code"
    status = {
      success = "200"
      pending = ["202"]
    }
    url_locator = "header.location"
  }
}

locals {
  vnet_precheck = [
	  {
	  	api = {
			path = restful_resource.rg.id
			query = {
			  api-version = ["2020-06-01"]
			}
			status_locator = "body.properties.provisioningState"
			status = {
			  success = "Succeeded"
			}
		}
	  }
  ]
  vnet_poll = {
    status_locator = "body.status"
    status = {
      success = "Succeeded"
      pending = ["Pending"]
    }
    url_locator = "header.azure-asyncoperation"
  }
}

resource "restful_resource" "test" {
  path = format("%%s/providers/Microsoft.Network/virtualNetworks/restful-test-%%d", restful_resource.rg.id, %d)
  query = {
    api-version = ["2021-05-01"]
  }

  precheck_create = local.vnet_precheck
  precheck_update = local.vnet_precheck
  precheck_delete = local.vnet_precheck

  poll_create = local.vnet_poll
  poll_update = local.vnet_poll
  poll_delete = local.vnet_poll

  body = {
    location = "westus"
    properties = {
      addressSpace = {
        addressPrefixes = ["10.0.0.0/16"]
      }
    }
    tags = {
      foo = "%s"
    }
  }
}
`, d.url, d.clientId, d.clientSecret, d.tenantId, d.subscriptionId, d.rd, d.rd, tag)
}

func (d azureData) vnet_simple_poll(tag string) string {
	return fmt.Sprintf(`
%s

resource "restful_resource" "test" {
  path = format("%%s/providers/Microsoft.Network/virtualNetworks/restful-test-%%d", restful_resource.rg.id, %d)
  query = {
    api-version = ["2021-05-01"]
  }

  poll_create = {
    status_locator = "body.properties.provisioningState"
    status = {
      success = "Succeeded"
      pending = ["Updating"]
    }
  }
  poll_update = {
    status_locator = "body.properties.provisioningState"
    status = {
      success = "Succeeded"
      pending = ["Updating"]
    }
  }
  poll_delete = {
    status_locator = "code"
    status = {
      success = "404"
      pending = ["200"]
    }
  }

  body = {
    location = "westus"
    properties = {
      addressSpace = {
        addressPrefixes = ["10.0.0.0/16"]
      }
    }
    tags = {
      foo = "%s"
    }
  }
}
`, d.vnet_template(), d.rd, tag)
}

func (d azureData) routetable_precheck(tag string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
  security = {
    oauth2 = {
	  client_credentials = {
		client_id     = %q
		client_secret = %q
		token_url     = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
		scopes        = ["https://management.azure.com/.default"]
	  }
    }
  }
  create_method = "PUT"
}

resource "restful_resource" "rg" {
  path = "/subscriptions/%s/resourceGroups/acctest-%d"
  query = {
    api-version = ["2020-06-01"]
  }
  body = {
    location = "westeurope"
  }

  poll_delete = {
    status_locator = "code"
    status = {
      success = "200"
      pending = ["202"]
    }
    url_locator = "header.location"
  }
}

locals {
  poll = {
    status_locator = "body.status"
    status = {
      success = "Succeeded"
      failure = "Failed"
      pending = ["Pending"]
    }
    url_locator = "header.azure-asyncoperation"
  }
  route_precheck = [
    {
      mutex = restful_resource.table.id
    }
  ]
}


resource "restful_resource" "table" {
  path = format("%%s/providers/Microsoft.Network/routeTables/restfultest-%d", restful_resource.rg.id)
  update_method = "PATCH"
  query = {
    api-version = ["2022-07-01"]
  }
  body = {
    location = "westus"
    tags = {
      foo = "%s"
    }
  }
  poll_create = local.poll
  poll_delete = local.poll
}

resource "restful_resource" "route1" {
  path = format("%%s/routes/route1", restful_resource.table.id)
  query = {
    api-version = ["2022-07-01"]
  }

  precheck_create = local.route_precheck
  precheck_update = local.route_precheck
  precheck_delete = local.route_precheck

  poll_create = local.poll
  poll_update = local.poll
  poll_delete = local.poll

  body = {
    properties = {
      nextHopType   = "VnetLocal"
      addressPrefix = "10.1.0.0/16"
    }
  }
}

resource "restful_resource" "route2" {
  path = format("%%s/routes/route2", restful_resource.table.id)
  query = {
    api-version = ["2022-07-01"]
  }

  precheck_create = local.route_precheck
  precheck_update = local.route_precheck
  precheck_delete = local.route_precheck

  poll_create = local.poll
  poll_update = local.poll
  poll_delete = local.poll

  body = {
    properties = {
      nextHopType   = "VnetLocal"
      addressPrefix = "10.2.0.0/16"
    }
  }
}
`, d.url, d.clientId, d.clientSecret, d.tenantId, d.subscriptionId, d.rd, d.rd, tag)
}
