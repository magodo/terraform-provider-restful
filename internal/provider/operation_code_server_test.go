package provider_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/magodo/terraform-provider-restful/internal/acceptance"
)

type codeServerOperation struct {
	url string
}

func newCodeServerOperation(url string) codeServerOperation {
	return codeServerOperation{url: url}
}

func TestOperation_CodeServer_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		return
	}))

	addr := "restful_operation.test"
	d := newCodeServerOperation(srv.URL)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.empty(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr(addr, "output"),
				),
			},
		},
	})
}

func TestOperation_CodeServer_idBuilder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "POST":
			w.Write([]byte(`{"id": 1}`))
			return
		case "GET":
			if !strings.HasSuffix(r.URL.Path, "/1") {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Write([]byte(`{"status": "OK"}`))
		case "DELETE":
			return
		}
	}))

	d := newCodeServerOperation(srv.URL)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.idBilder(),
				Check:  resource.ComposeTestCheckFunc(),
			},
		},
	})
}

func (d codeServerOperation) empty() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_operation" "test" {
  path = "posts"
  method = "POST"
  body = null
}
`, d.url)
}

func (d codeServerOperation) idBilder() string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_operation" "test" {
  path 			= "foo"
  id_builder	= "bar/$(body.id)"
  method 		= "POST"
  poll = {
	status_locator = "body.status"
	status = {
		success = "OK"
	}
  }
}
`, d.url)
}
