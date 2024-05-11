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

const RESTFUL_DEAD_SIMPLE_SERVER_URL = "RESTFUL_DEAD_SIMPLE_SERVER_URL"

type deadSimpleServerData struct {
	url string
}

func (d deadSimpleServerData) precheck(t *testing.T) {
	if d.url == "" {
		t.Skipf("%q is not specified", RESTFUL_DEAD_SIMPLE_SERVER_URL)
	}
	return
}

func newDeadSimpleServerData() deadSimpleServerData {
	return deadSimpleServerData{
		url: os.Getenv(RESTFUL_DEAD_SIMPLE_SERVER_URL),
	}
}

func TestResource_DeadSimpleServer_Basic(t *testing.T) {
	addr := "restful_resource.test"
	d := newDeadSimpleServerData()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { d.precheck(t) },
		CheckDestroy:             d.CheckDestroy(addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.basic("foo"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(addr, "output"),
				),
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"create_method"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return `{"id": "test", "path": "test", "body": [{"foo": null}]}`, nil
				},
			},
			{
				Config: d.basic("bar"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(addr, "output"),
				),
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"create_method"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return `{"id": "test", "path": "test", "body": [{"foo": null}]}`, nil
				},
			},
		},
	})
}

func (d deadSimpleServerData) CheckDestroy(addr string) func(*terraform.State) error {
	return func(s *terraform.State) error {
		c, err := client.New(context.TODO(), d.url, nil)
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

func (d deadSimpleServerData) basic(v string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "test"
  create_method = "PUT"
  body = jsonencode([
  {
  	foo = %q
  }
])
}
`, d.url, v)
}
