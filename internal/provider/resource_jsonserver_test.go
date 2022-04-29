package provider_test

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/magodo/terraform-provider-restful/internal/acceptance"
	"github.com/magodo/terraform-provider-restful/internal/client"
)

const RESTFUL_JSON_SERVER_URL = "RESTFUL_JSON_SERVER_URL"

type jsonServerData struct {
	url string
}

func (d jsonServerData) precheck(t *testing.T) {
	if d.url == "" {
		t.Skipf("%q is not specified", RESTFUL_JSON_SERVER_URL)
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
				Config: d.basic(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(addr, "output"),
				),
			},
			{
				ResourceName: addr,
				ImportState:  true,
			},
		},
	})
}

func (d jsonServerData) CheckDestroy(addr string) func(*terraform.State) error {
	return func(s *terraform.State) error {
		c, err := client.New(d.url, nil)
		if err != nil {
			return err
		}
		for key, resource := range s.RootModule().Resources {
			if key != addr {
				continue
			}
			resp, err := c.Read(context.TODO(), resource.Primary.ID, client.ReadOption{})
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

func (d jsonServerData) basic() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "posts"
  body = jsonencode({
  	foo = "bar"
})
}
`, d.url)

}
