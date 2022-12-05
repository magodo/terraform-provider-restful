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
					resource.TestCheckResourceAttrSet(dsaddr, "output"),
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
					resource.TestCheckResourceAttrSet(dsaddr, "output"),
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
  body = jsonencode({
  	foo = "bar"
})
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
  body = jsonencode({
  	foo = "bar"
})
  read_path = "$(path)/$(body.id)"
}

data "restful_resource" "test" {
  id       = "/posts"
  selector = "#(foo==\"bar\")"
  depends_on = [restful_resource.test]
}
`, d.url)

}
