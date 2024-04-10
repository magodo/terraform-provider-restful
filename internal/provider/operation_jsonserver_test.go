package provider_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/magodo/terraform-provider-restful/internal/acceptance"
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
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(addr, "output.%"),
				),
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
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(addr, "output.%"),
					resource.TestCheckResourceAttr(addr, "output.enabled", `true`),
				),
			},
			{
				// We need to check the resource's state after another refresh, after the operation resource is deleted, hence exists this step.
				Config: d.withDelete(false),
				Check:  resource.ComposeTestCheckFunc(),
			},
			{
				Config: d.withDelete(false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resaddr, "output.enabled", `false`),
				),
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
