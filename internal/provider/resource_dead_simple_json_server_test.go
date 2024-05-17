package provider_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/magodo/terraform-provider-restful/internal/acceptance"
	"github.com/magodo/terraform-provider-restful/internal/client"
)

type deadSimpleServerData struct{}

func TestResource_DeadSimpleServer_ObjectArray(t *testing.T) {
	addr := "restful_resource.test"

	type object struct {
		b  []byte
		id string
	}
	var obj *object
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PUT":
			b, _ := io.ReadAll(r.Body)
			r.Body.Close()
			obj = &object{b: b, id: r.URL.String()}
			return
		case "GET":
			if obj == nil || r.URL.String() != obj.id {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Write(obj.b)
			return
		case "DELETE":
			obj = nil
			return
		}
	}))
	d := deadSimpleServerData{}
	resource.Test(t, resource.TestCase{
		CheckDestroy:             d.CheckDestroy(srv.URL, addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.object_array(srv.URL, "foo"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(addr, "output.#"),
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
				Config: d.object_array(srv.URL, "bar"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(addr, "output.#"),
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

func TestResource_DeadSimpleServer_CreateRetString(t *testing.T) {
	addr := "restful_resource.test"

	type object struct {
		b  []byte
		id string
	}
	const id = "foo"
	var obj *object
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PUT":
			b, _ := io.ReadAll(r.Body)
			r.Body.Close()
			r.URL.Path, _ = url.JoinPath(r.URL.Path, id)
			obj = &object{b: b, id: r.URL.String()}
			w.Write([]byte(id))
			return
		case "GET":
			if obj == nil || r.URL.String() != obj.id {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Write(obj.b)
			return
		case "DELETE":
			obj = nil
			return
		}
	}))
	d := deadSimpleServerData{}
	resource.Test(t, resource.TestCase{
		CheckDestroy:             d.CheckDestroy(srv.URL, addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.create_ret_string(srv.URL),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(addr, "output"),
				),
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"create_method", "read_path"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return fmt.Sprintf(`{"path": "/test", "id": "/test/%s", "body": {}}`, id), nil
				},
			},
		},
	})
}

func (d deadSimpleServerData) CheckDestroy(url, addr string) func(*terraform.State) error {
	return func(s *terraform.State) error {
		c, err := client.New(context.TODO(), url, nil)
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

func (d deadSimpleServerData) object_array(url, v string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "test"
  create_method = "PUT"
  body = [
    {
  	  foo = %q
    }
  ]
}
`, url, v)
}

func (d deadSimpleServerData) create_ret_string(url string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "/test"
  create_method = "PUT"
  read_path = "$(path)/$(body)"
  body = "{}"
}
`, url)
}
