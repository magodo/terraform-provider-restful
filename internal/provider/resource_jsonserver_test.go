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
	"github.com/magodo/terraform-provider-restful/internal/acceptance"
	"github.com/magodo/terraform-provider-restful/internal/client"
)

const RESTFUL_JSON_SERVER_URL = "RESTFUL_JSON_SERVER_URL"
const RESTFUL_MIGRATE_TEST = "RESTFUL_MIGRATE_TEST"

type jsonServerData struct {
	url string
}

func (d jsonServerData) precheck(t *testing.T) {
	if d.url == "" {
		t.Skipf("%q is not specified", RESTFUL_JSON_SERVER_URL)
	}
	return
}

func (d jsonServerData) precheckMigrate(t *testing.T) {
	d.precheck(t)
	if _, ok := os.LookupEnv(RESTFUL_MIGRATE_TEST); !ok {
		t.Skipf("%q is not specified", RESTFUL_MIGRATE_TEST)
	}
	return
}

func newJsonServerData() jsonServerData {
	return jsonServerData{
		url: os.Getenv(RESTFUL_JSON_SERVER_URL),
	}
}

func TestResource_JSONServer_Basic(t *testing.T) {
	addr := "restful_resource.test"
	d := newJsonServerData()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.basic("foo"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"read_path"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return fmt.Sprintf(`{"id": %q, "path": "posts", "body": {"foo": null}}`, s.RootModule().Resources[addr].Primary.Attributes["id"]), nil
				},
			},
			{
				Config: d.basic("bar"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"read_path"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return fmt.Sprintf(`{"id": %q, "path": "posts", "body": {"foo": null}}`, s.RootModule().Resources[addr].Primary.Attributes["id"]), nil
				},
			},
		},
	})
}

func TestResource_JSONServer_PatchUpdate(t *testing.T) {
	addr := "restful_resource.test"
	d := newJsonServerData()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.patch("foo"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"read_path", "update_method"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return fmt.Sprintf(`{"id": %q, "path": "posts", "update_method": "PATCH", "body": {"foo": null}}`, s.RootModule().Resources[addr].Primary.Attributes["id"]), nil
				},
			},
			{
				Config: d.patch("bar"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"read_path", "update_method"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return fmt.Sprintf(`{"id": %q, "path": "posts", "update_method": "PATCH", "body": {"foo": null}}`, s.RootModule().Resources[addr].Primary.Attributes["id"]), nil
				},
			},
		},
	})
}

func TestResource_JSONServer_FullPath(t *testing.T) {
	addr := "restful_resource.test"
	d := newJsonServerData()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.fullPath("foo"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"read_path", "update_path", "delete_path"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return fmt.Sprintf(`{"id": %q, "path": "posts", "body": {"foo": null}}`, s.RootModule().Resources[addr].Primary.Attributes["id"]), nil
				},
			},
			{
				Config: d.fullPath("bar"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"read_path", "update_path", "delete_path"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return fmt.Sprintf(`{"id": %q, "path": "posts", "body": {"foo": null}}`, s.RootModule().Resources[addr].Primary.Attributes["id"]), nil
				},
			},
		},
	})
}

func TestResource_JSONServer_OutputAttrs(t *testing.T) {
	addr := "restful_resource.test"
	d := newJsonServerData()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.outputAttrs(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("foo"), knownvalue.StringExact("bar")),
				},
			},
		},
	})
}

func TestResource_JSONServer_ReadSelectorParam(t *testing.T) {
	addr := "restful_resource.test"
	d := newJsonServerData()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroyWithReadSelector(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.readSelectorParam("bar"),
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"read_path", "update_path", "delete_path", "read_selector"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					attrs := s.RootModule().Resources[addr].Primary.Attributes
					return fmt.Sprintf(`{"id": "%s", "path": "posts", "body": {"foo": null}, "read_selector": "#(id == %s)"}`, attrs["id"], attrs["output.id"]), nil
				},
			},
			{
				Config: d.readSelectorParam("baz"),
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"read_path", "update_path", "delete_path", "read_selector"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					attrs := s.RootModule().Resources[addr].Primary.Attributes
					return fmt.Sprintf(`{"id": "%s", "path": "posts", "body": {"foo": null}, "read_selector": "#(id == %s)"}`, attrs["id"], attrs["output.id"]), nil
				},
			},
		},
	})
}

func TestResource_JSONServer_StatusLocatorParam(t *testing.T) {
	addr := "restful_resource.test"
	d := newJsonServerData()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroyWithReadSelector(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.statusLocatorParam("bar"),
			},
			{
				Config: d.statusLocatorParam("baz"),
			},
		},
	})
}

func TestResource_JSONServer_UpdateBodyPatch(t *testing.T) {
	addr := "restful_resource.test"
	d := newJsonServerData()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.updateBodyPatch("foo"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("foo"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.MapSizeExact(2)),
				},
			},
			{
				Config: d.updateBodyPatch("bar"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("addon").AtMapKey("a"), knownvalue.StringExact("hmm")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("addon").AtMapKey("b"), knownvalue.NotNull()),
				},
			},
		},
	})
}

func TestResource_JSONServer_MigrateV0ToV1(t *testing.T) {
	addr := "restful_resource.test"
	d := newJsonServerData()
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { d.precheckMigrate(t) },
		CheckDestroy: d.CheckDestroy(addr),
		Steps: []resource.TestStep{
			{
				ProtoV6ProviderFactories: nil,
				ExternalProviders: map[string]resource.ExternalProvider{
					"restful": {
						VersionConstraint: "= 0.13.2",
						Source:            "registry.terraform.io/magodo/restful",
					},
				},
				Config: d.migrate_v0(),
			},
			{
				ProtoV6ProviderFactories: acceptance.ProviderFactory(),
				ExternalProviders:        nil,
				Config:                   d.migrate_v1(),
				PlanOnly:                 true,
			},
		},
	})
}

func (d jsonServerData) CheckDestroy(addr string) func(*terraform.State) error {
	return func(s *terraform.State) error {
		ctx := context.TODO()
		c, err := client.New(ctx, d.url, nil)
		if err != nil {
			return err
		}
		c.SetLoggerContext(ctx)
		for key, resource := range s.RootModule().Resources {
			if key != addr {
				continue
			}
			resp, err := c.Read(ctx, resource.Primary.ID, client.ReadOption{})
			if err != nil {
				return fmt.Errorf("reading %s: %v", addr, err)
			}
			if resp.StatusCode() != http.StatusNotFound {
				return fmt.Errorf("%s: still exists", addr)
			}
			return nil
		}
		panic("unreachable")
	}
}

func (d jsonServerData) CheckDestroyWithReadSelector(addr string) func(*terraform.State) error {
	return func(s *terraform.State) error {
		ctx := context.TODO()
		c, err := client.New(ctx, d.url, nil)
		if err != nil {
			return err
		}
		c.SetLoggerContext(ctx)

		for key, resource := range s.RootModule().Resources {
			if key != addr {
				continue
			}
			resp, err := c.Read(ctx, fmt.Sprintf("%s/%s", resource.Primary.ID, resource.Primary.Attributes["output.id"]), client.ReadOption{})
			if err != nil {
				return fmt.Errorf("reading %s: %v", addr, err)
			}
			if resp.StatusCode() != http.StatusNotFound {
				return fmt.Errorf("%s: still exists", addr)
			}
			return nil
		}
		panic("unreachable")
	}
}

func (d jsonServerData) basic(v string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "posts"
  body = {
  	foo = %q
  }
  read_path = "$(path)/$(body.id)"
}
`, d.url, v)
}

func (d jsonServerData) patch(v string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "posts"
  read_path = "$(path)/$(body.id)"
  update_method = "PATCH"
  body = {
  	foo = %q
  }
}
`, d.url, v)
}

func (d jsonServerData) fullPath(v string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "posts"
  read_path = "$(path)/$(body.id)"
  update_path = "$(path)/$(body.id)"
  delete_path = "$(path)/$(body.id)"
  body = {
  	foo = %q
  }
}
`, d.url, v)

}

func (d jsonServerData) outputAttrs() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "posts"
  body = {
  	foo = "bar"
	obj = {
		a = 1	
		b = 2
	}
  }
  read_path = "$(path)/$(body.id)"
  output_attrs = ["foo", "obj.a"]
}
`, d.url)
}

func (d jsonServerData) migrate_v0() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "posts"
  body = jsonencode({
  	foo = "bar"
  })
  read_path = "$(path)/$(body.id)"
}
`, d.url)
}

func (d jsonServerData) migrate_v1() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "posts"
  body = {
  	foo = "bar"
  }
  read_path = "$(path)/$(body.id)"
}
`, d.url)
}

func (d jsonServerData) readSelectorParam(v string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "posts"
  body = {
  	foo = "%s"
  }
  update_path = "$(path)/$(body.id)"
  delete_path = "$(path)/$(body.id)"
  read_selector = "#(id == $(body.id))"
}
`, d.url, v)
}

func (d jsonServerData) statusLocatorParam(v string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %[1]q
}

resource "restful_resource" "test" {
  path = "posts"
  read_path = "$(path)/$(body.id)"
  body = {
  	foo = "%[2]s"
  }

  poll_create = {
  	url_locator = "exact.posts"
	status_locator = "body.#(id == $(body.id)).foo"
	status = {
		success = "bar"
	}
  }
  poll_update = {
  	url_locator = "exact.posts"
	status_locator = "body.#(id == $(body.id)).foo"
	status = {
		success = "baz"
	}
  }
  precheck_update = [{
  	api = {
  		path = "posts"
		status_locator = "body.#(id == $(body.id)).foo"
		status = {
			success = "bar"
		}
	}
  }]
  precheck_delete = [{
  	api = {
  		path = "posts"
		status_locator = "body.#(id == $(body.id)).foo"
		status = {
			success = "baz"
		}
	}
  }]
}
`, d.url, v)
}

func (d jsonServerData) updateBodyPatch(v string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "posts"
  body = {
  	foo = %q
  }
  update_body_patches = [
    {
	  path = "addon.a"
	  raw_json = "\"hmm\""
	},
    {
	  path = "addon.b"
	  raw_json = "$(body.id)"
	},
  ]
  read_path = "$(path)/$(body.id)"
}
`, d.url, v)
}
