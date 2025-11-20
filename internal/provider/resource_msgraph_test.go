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

const RESTFUL_MSGRAPH_TENANT_ID = "RESTFUL_MSGRAPH_TENANT_ID"
const RESTFUL_MSGRAPH_CLIENT_ID = "RESTFUL_MSGRAPH_CLIENT_ID"
const RESTFUL_MSGRAPH_CLIENT_SECRET = "RESTFUL_MSGRAPH_CLIENT_SECRET"
const RESTFUL_MSGRAPH_ORG_DOMAIN = "RESTFUL_MSGRAPH_ORG_DOMAIN"

type msgraphData struct {
	url          string
	tenantId     string
	clientId     string
	clientSecret string
	orgDomain    string

	rd acceptance.Rd
}

func (d msgraphData) precheck(t *testing.T) {
	if d.tenantId == "" {
		t.Skipf("%q is not specified", RESTFUL_MSGRAPH_TENANT_ID)
	}
	if d.clientId == "" {
		t.Skipf("%q is not specified", RESTFUL_MSGRAPH_CLIENT_ID)
	}
	if d.clientSecret == "" {
		t.Skipf("%q is not specified", RESTFUL_MSGRAPH_CLIENT_SECRET)
	}
	if d.orgDomain == "" {
		t.Skipf("%q is not specified", RESTFUL_MSGRAPH_ORG_DOMAIN)
	}
	return
}

func newMsGraphData() msgraphData {
	return msgraphData{
		url:          "https://graph.microsoft.com/v1.0",
		tenantId:     os.Getenv(RESTFUL_MSGRAPH_TENANT_ID),
		clientId:     os.Getenv(RESTFUL_MSGRAPH_CLIENT_ID),
		clientSecret: os.Getenv(RESTFUL_MSGRAPH_CLIENT_SECRET),
		orgDomain:    os.Getenv(RESTFUL_MSGRAPH_ORG_DOMAIN),

		rd: acceptance.NewRd(),
	}
}

func TestResource_MsGraph_User(t *testing.T) {
	addr := "restful_resource.test"
	d := newMsGraphData()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.user(false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: false,
				ImportStateIdFunc: d.userImportStateIdFunc(addr),
			},
			{
				Config: d.userUpdate(false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: false,
				ImportStateIdFunc: d.userImportStateIdFunc(addr),
			},
			{
				Config: d.user(true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: false,
				ImportStateIdFunc: d.userImportStateIdFunc(addr),
			},
		},
	})
}

func (d msgraphData) CheckDestroy(addr string) func(*terraform.State) error {
	return func(s *terraform.State) error {
		ctx := context.TODO()
		c, err := client.New(ctx, d.url, &client.BuildOption{
			Security: client.OAuth2ClientCredentialOption{
				ClientId:     d.clientId,
				ClientSecret: d.clientSecret,
				TokenURL:     fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/v2.0/token", d.tenantId),
				Scopes: []string{
					"https://graph.microsoft.com/.default",
				},
			},
		})
		if err != nil {
			return err
		}
		c.SetLoggerContext(ctx)
		resource := s.RootModule().Resources[addr]
		if resource != nil {
			resp, err := c.Read(ctx, resource.Primary.ID, client.ReadOption{})
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

func (d msgraphData) user(mpDisabled bool) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
  security = {
    oauth2 = {
	  client_credentials = {
        client_id     = %q
        client_secret = %q
        token_url     = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
        scopes        = ["https://graph.microsoft.com/.default"]
	  }
    }
  }
  update_method = "PATCH"
}

resource "restful_resource" "test" {
  path = "/users"
  read_path = "$(path)/$(body.id)"
  merge_patch_disabled 	= %t
  body = {
    accountEnabled    = true
	mailNickname 	  = "AdeleV"
    passwordProfile = {
      password = "SecretP@sswd99!"
    }

    displayName       = "J.Doe"
    userPrincipalName = "%d@%s"
  }
  write_only_attrs = [
    "mailNickname",
    "accountEnabled",
    "passwordProfile",
  ]
}
`, d.url, d.clientId, d.clientSecret, d.tenantId, mpDisabled, d.rd, d.orgDomain)
}

func (d msgraphData) userUpdate(mpDisabled bool) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
  security = {
    oauth2 = {
	  client_credentials = {
        client_id     = %q
        client_secret = %q
        token_url     = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"
        scopes        = ["https://graph.microsoft.com/.default"]
	  }
    }
  }
  update_method = "PATCH"
}

resource "restful_resource" "test" {
  path = "/users"
  read_path = "$(path)/$(body.id)"
  merge_patch_disabled 	= %t
  body = {
    accountEnabled    = false
	mailNickname 	  = "AdeleV"
    passwordProfile = {
      password = "SecretP@sswd99!"
    }
    displayName       = "J.Doe2"
    userPrincipalName = "%d@%s"
  }
  write_only_attrs = [
    "mailNickname",
    "accountEnabled",
    "passwordProfile",
  ]
}
`, d.url, d.clientId, d.clientSecret, d.tenantId, mpDisabled, d.rd, d.orgDomain)
}

func (d msgraphData) userImportStateIdFunc(addr string) func(s *terraform.State) (string, error) {
	return func(s *terraform.State) (string, error) {
		return fmt.Sprintf(`{
"id": %q,
"path": "/users",
"body": {
  "displayName": null,
  "userPrincipalName": null
}
}`, s.RootModule().Resources[addr].Primary.Attributes["id"]), nil
	}
}
