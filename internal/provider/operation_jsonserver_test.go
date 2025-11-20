package provider_test

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/lfventura/terraform-provider-restful/internal/acceptance"
)

type jsonServerOperation struct {
	url string
}

func (d jsonServerOperation) precheck(t *testing.T) {
	if d.url == "" {
		t.Skipf("%q is not specified", RESTFUL_JSON_SERVER_URL)
	}
	return
}

func (d jsonServerOperation) precheckMigrate(t *testing.T) {
	d.precheck(t)
	if _, ok := os.LookupEnv(RESTFUL_MIGRATE_TEST); !ok {
		t.Skipf("%q is not specified", RESTFUL_MIGRATE_TEST)
	}
	return
}

func newJsonServerOperation() jsonServerOperation {
	return jsonServerOperation{
		url: os.Getenv(RESTFUL_JSON_SERVER_URL),
	}
}

func TestOperation_JSONServer_Basic(t *testing.T) {
	addr := "restful_operation.test"
	d := newJsonServerOperation()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.basic(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
		},
	})
}

func TestOperation_JSONServer_withDelete(t *testing.T) {
	addr := "restful_operation.test"
	resaddr := "restful_resource.test"
	d := newJsonServerOperation()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.withDelete(true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("enabled"), knownvalue.Bool(true)),
				},
			},
			{
				// We need to check the resource's state after another refresh, after the operation resource is deleted, hence exists this step.
				Config: d.withDelete(false),
			},
			{
				Config: d.withDelete(false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(resaddr, tfjsonpath.New("output").AtMapKey("enabled"), knownvalue.Bool(false)),
				},
			},
		},
	})
}

func TestOperation_JSONServer_statusLocatorParam(t *testing.T) {
	addr := "restful_operation.test"
	d := newJsonServerOperation()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.statusLocatorParam(),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output").AtMapKey("id"), knownvalue.NotNull()),
				},
			},
		},
	})
}

func TestOperation_JSONServer_EphemeralBodyOverlap(t *testing.T) {
	d := newJsonServerOperation()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config:      d.ephemeralBodyOverlap(),
				ExpectError: regexp.MustCompile(`the body and the ephemeral body are not disjointed`),
			},
		},
	})
}

func TestOperation_JSONServer_EphemeralBody(t *testing.T) {
	addr := "restful_operation.test"
	d := newJsonServerOperation()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.ephemeralBody(`foo`),
				ConfigStateChecks: []statecheck.StateCheck{
					// Should only contain foo and id.
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.MapSizeExact(2)),
				},
			},
			{
				Config: d.ephemeralBody(`bar`),
				ConfigStateChecks: []statecheck.StateCheck{
					// Should only contain foo and id.
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.MapSizeExact(2)),
				},
			},
			{
				Config: d.ephemeralBodyNull(),
				ConfigStateChecks: []statecheck.StateCheck{
					// Should only contain foo and id.
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.MapSizeExact(2)),
				},
			},
			{
				Config: d.ephemeralBody(`foo`),
				ConfigStateChecks: []statecheck.StateCheck{
					// Should only contain foo and id.
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.MapSizeExact(2)),
				},
			},
		},
	})
}

func TestOperation_JSONServer_MigrateV0ToV1(t *testing.T) {
	d := newJsonServerOperation()
	resource.Test(t, resource.TestCase{
		PreCheck: func() { d.precheckMigrate(t) },
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

func (d jsonServerOperation) basic() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_operation" "test" {
  path = "posts"
  method = "POST"
  body = {
  	foo = "bar"
  }
}
`, d.url)
}

func (d jsonServerOperation) withDelete(create bool) string {
	tpl := fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

# This resource is used to check the state of the posts after the operation resource is deleted
resource "restful_resource" "test" {
  path = "posts"
  body = {}
  read_path = "$(path)/$(body.id)"
  output_attrs = ["enabled"]
}
`, d.url)

	if create {
		tpl += `
resource "restful_operation" "test" {
  path = restful_resource.test.id
  method = "PUT"
  body = {
    enabled = true
  }
  delete_method = "PUT"
  delete_path = restful_resource.test.id
  delete_body = {
    enabled = false
  }
  output_attrs = ["enabled"]
}`
	}
	return tpl
}

func (d jsonServerOperation) statusLocatorParam() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_operation" "test" {
  path = "posts"
  method = "POST"
  delete_method = "DELETE"
  delete_path = "$(path)/$(body.id)"
  body = {
  	foo = "bar"
  }
  poll = {
	status_locator = "body.#(id == $(body.id)).foo"
	status = {
		success = "bar"
	}
  }
  precheck_delete = [{
  	api = {
		status_locator = "body.#(id == $(body.id)).foo"
		status = {
			success = "bar"
		}
	}
  }]
}
`, d.url)
}

func (d jsonServerOperation) migrate_v0() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_operation" "test" {
  path = "posts"
  method = "POST"
  body = jsonencode({
  	foo = "bar"
  })
}
`, d.url)
}

func (d jsonServerOperation) migrate_v1() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_operation" "test" {
  path = "posts"
  method = "POST"
  body = {
  	foo = "bar"
  }
}
`, d.url)
}

func (d jsonServerOperation) ephemeralBodyOverlap() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_operation" "test" {
  path = "posts"
  method = "POST"
  body = {
  	foo = "foo"
  }
  ephemeral_body = {
    foo = "bar"
  }
}
`, d.url)
}

func (d jsonServerOperation) ephemeralBody(v string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

variable "v" {
  type = string
  ephemeral = true
  default = %q
}

resource "restful_operation" "test" {
  path = "posts"
  method = "POST"
  body = {
  	foo = "foo"
  }
  ephemeral_body = {
    secret = var.v
  }
}
`, d.url, v)
}

func (d jsonServerOperation) ephemeralBodyNull() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_operation" "test" {
  path = "posts"
  method = "POST"
  body = {
  	foo = "foo"
  }
  ephemeral_body = null
}
`, d.url)
}

func TestOperation_JSONServer_UseSensitiveOutput(t *testing.T) {
	addr := "restful_operation.test"
	d := newJsonServerOperation()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.useSensitiveOutput("foo"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("sensitive_output").AtMapKey("id"), knownvalue.NotNull()),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("sensitive_output").AtMapKey("foo"), knownvalue.StringExact("foo")),
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.Null()),
				},
			},
		},
	})
}

func (d jsonServerOperation) useSensitiveOutput(v string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_operation" "test" {
  path = "posts"
  method = "POST"
  use_sensitive_output = true
  body = {
  	foo = %q
  }
}
`, d.url, v)
}
