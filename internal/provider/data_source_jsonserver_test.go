package provider_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/magodo/terraform-provider-restful/internal/acceptance"
)

func TestDataSource_JSONServer_Basic(t *testing.T) {
	addr := "restful_resource.test"
	dsaddr := "data.restful_resource.test"
	d := newJsonServerData()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.dsBasic(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(dsaddr, "output.%"),
				),
			},
		},
	})
}

func TestDataSource_JSONServer_WithSelector(t *testing.T) {
	addr := "restful_resource.test"
	dsaddr := "data.restful_resource.test"
	d := newJsonServerData()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.dsWithSelector(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(dsaddr, "output.%"),
				),
			},
		},
	})
}

func TestDataSource_JSONServer_WithOutputAttrs(t *testing.T) {
	addr := "restful_resource.test"
	dsaddr := "data.restful_resource.test"
	d := newJsonServerData()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.dsWithOutputAttrs(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(dsaddr, "output.foo", "bar"),
					resource.TestCheckResourceAttr(dsaddr, "output.obj.a", "1"),
				),
			},
		},
	})
}

func TestDataSource_JSONServer_NotExists(t *testing.T) {
	dsaddr := "data.restful_resource.test"
	d := newJsonServerData()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.dsNotExist(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(dsaddr, "id"),
					resource.TestCheckNoResourceAttr(dsaddr, "output.%"),
				),
			},
		},
	})
}

func (d jsonServerData) dsBasic() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "/posts"
  body = {
  	foo = "bar"
  }
  read_path = "$(path)/$(body.id)"
}

data "restful_resource" "test" {
  id = restful_resource.test.id
}
`, d.url)

}

func (d jsonServerData) dsWithSelector() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "/posts"
  body = {
  	foo = "bar"
  }
  read_path = "$(path)/$(body.id)"
}

data "restful_resource" "test" {
  id       = "/posts"
  selector = "#(foo==\"bar\")"
  depends_on = [restful_resource.test]
}
`, d.url)

}

func (d jsonServerData) dsWithOutputAttrs() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "/posts"
  body = {
  	foo = "bar"
	obj = {
	  a = 1
	  b = 2
	}
  }
  read_path = "$(path)/$(body.id)"
}

data "restful_resource" "test" {
  id = restful_resource.test.id
  output_attrs = ["foo", "obj.a"]
}
`, d.url)

}

func (d jsonServerData) dsNotExist() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

data "restful_resource" "test" {
  id = "/notexist"
  allow_not_exist = true
}
`, d.url)

}
