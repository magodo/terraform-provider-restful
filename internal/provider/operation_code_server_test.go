package provider_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/lfventura/terraform-provider-restful/internal/acceptance"
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
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.Null()),
				},
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
			},
		},
	})
}

func TestOperation_CodeServer_HeaderQuery(t *testing.T) {
	addr := "restful_operation.test"

	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)
	mux.HandleFunc("POST /tests/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("type") != "operation" {
			w.WriteHeader(http.StatusBadRequest)
		}
		if r.URL.Query().Get("type") != "operation" {
			w.WriteHeader(http.StatusBadRequest)
		}
		return
	})
	mux.HandleFunc("DELETE /tests/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("type") != "delete" {
			w.WriteHeader(http.StatusBadRequest)
		}
		if r.URL.Query().Get("type") != "delete" {
			w.WriteHeader(http.StatusBadRequest)
		}
		return
	})
	srv.Start()
	d := newCodeServerOperation(srv.URL)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.headerquery(srv.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.Null()),
				},
			},
		},
	})
}

func TestOperation_CodeServer_HeaderQueryFromBody(t *testing.T) {
	addr := "restful_operation.test"

	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)
	var body []byte
	mux.HandleFunc("POST /tests/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("type") != "operation" {
			w.WriteHeader(http.StatusBadRequest)
		}
		if r.URL.Query().Get("type") != "operation" {
			w.WriteHeader(http.StatusBadRequest)
		}
		body, _ = io.ReadAll(r.Body)
		w.Write(body)
		return
	})
	mux.HandleFunc("DELETE /tests/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("type") != "delete_b" {
			w.WriteHeader(http.StatusBadRequest)
		}
		if r.URL.Query().Get("type") != "delete_b" {
			w.WriteHeader(http.StatusBadRequest)
		}
		return
	})
	srv.Start()
	d := newCodeServerOperation(srv.URL)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.headerqueryFromBody(srv.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.NotNull()),
				},
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

func (d codeServerOperation) headerquery(url string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_operation" "test" {
  path = "/tests/1"
  method = "POST"
  operation_header = {
  	type = "operation"
  }
  operation_query = {
  	type = ["operation"]
  }

  delete_path = "/tests/1"
  delete_method = "DELETE"
  delete_header = {
  	type = "delete"
  }
  delete_query = {
  	type = ["delete"]
  }

  body = {}
}
`, url)
}

func (d codeServerOperation) headerqueryFromBody(url string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_operation" "test" {
  path = "/tests/1"
  method = "POST"
  delete_method = "DELETE"
  operation_header = {
  	type = "operation"
  }
  operation_query = {
  	type = ["operation"]
  }
  delete_header = {
  	type = "$(body.delete)"
  }
  delete_query = {
  	type = ["$(body.delete)"]
  }
  body = {
    delete = "delete_b"
  }
}
`, url)
}
