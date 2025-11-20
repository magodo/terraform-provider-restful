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

const RESTFUL_ADO_PAT = "RESTFUL_ADO_PAT"
const RESTFUL_ADO_URL = "RESTFUL_ADO_URL"

type adoData struct {
	pat     string
	url     string
	version string
	rd      acceptance.Rd
}

func (d adoData) precheck(t *testing.T) {
	if d.pat == "" {
		t.Skipf("%q is not specified", RESTFUL_ADO_PAT)
	}
	if d.url == "" {
		t.Skipf("%q is not specified", RESTFUL_ADO_URL)
	}
}

func newAdoData() adoData {
	return adoData{
		pat:     os.Getenv(RESTFUL_ADO_PAT),
		url:     os.Getenv(RESTFUL_ADO_URL),
		version: "7.2-preview",
		rd:      acceptance.NewRd(),
	}
}

func TestResource_ADO_Project(t *testing.T) {
	addr := "restful_resource.test"
	d := newAdoData()
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.project("foo"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: false,
				ImportStateIdFunc: d.projectImportStateIdFunc(addr),
			},
			{
				Config: d.project("bar"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:      addr,
				ImportState:       true,
				ImportStateVerify: false,
				ImportStateIdFunc: d.projectImportStateIdFunc(addr),
			},
		},
	})
}

func (d adoData) CheckDestroy(addr string) func(*terraform.State) error {
	return func(s *terraform.State) error {
		ctx := context.TODO()
		c, err := client.New(ctx, d.url, &client.BuildOption{
			Security: client.HTTPBasicOption{
				Username: "",
				Password: d.pat,
			},
		})
		if err != nil {
			return err
		}
		c.SetLoggerContext(ctx)
		resource := s.RootModule().Resources[addr]
		if resource != nil {
			query := client.Query{"api-version": []string{d.version}}
			resp, err := c.Read(ctx, resource.Primary.ID, client.ReadOption{Query: query})
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

func (d adoData) projectTemplate() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
  security = {
	http = {
	  basic = {
		username = ""
		password = %q
	  }
	}
  }
}

data "restful_resource" "process" {
  id = "_apis/process/processes"
  method = "GET"
  header = {
    "api-version": %q
  }
  selector = "value.#(isDefault==true)"
}
`, d.url, d.pat, d.version)
}

func (d adoData) project(name string) string {
	return fmt.Sprintf(`
%s

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

resource "restful_resource" "test" {
  path = "_apis/projects"
  read_path = "_apis/projects/$(body.id)"
  update_method = "PATCH"
  query = {
    api-version = ["7.2-preview"]
  }
  post_create_read = {
    path = "_apis/projects/%s"
  }
  poll_create = local.poll
  poll_update = local.poll
  poll_delete = local.poll
  body = {
    name = "%s"
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
`, d.projectTemplate(), name, name)
}

func (d adoData) projectImportStateIdFunc(addr string) func(s *terraform.State) (string, error) {
	return func(s *terraform.State) (string, error) {
		return fmt.Sprintf(`{
"id": %q,
"path": "/_apis/projects",
"body": {
  "name": null,
  "description": null,
  "visibility": null
}
}`, s.RootModule().Resources[addr].Primary.Attributes["id"]), nil
	}
}
