package provider_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"testing"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/lfventura/terraform-provider-restful/internal/acceptance"
	"github.com/lfventura/terraform-provider-restful/internal/client"
)

type codeServerData struct{}

func TestResource_CodeServer_ObjectArray(t *testing.T) {
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
	d := codeServerData{}
	resource.Test(t, resource.TestCase{
		CheckDestroy:             d.CheckDestroy(srv.URL, addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.object_array(srv.URL, "foo"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.NotNull()),
				},
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
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.NotNull()),
				},
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

func TestResource_CodeServer_CreateRetString(t *testing.T) {
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
			ret, _ := json.Marshal(id)
			w.Write([]byte(ret))
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
	d := codeServerData{}
	resource.Test(t, resource.TestCase{
		CheckDestroy:             d.CheckDestroy(srv.URL, addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.create_ret_string(srv.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"create_method", "read_path"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return fmt.Sprintf(`{"path": "test", "id": "test/%s", "body": {}}`, id), nil
				},
			},
		},
	})
}

func TestResource_CodeServer_RetFullURL(t *testing.T) {
	addr := "restful_resource.test"

	type object struct {
		b []byte
	}
	objs := map[int]object{}
	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)
	idx := 0
	mux.HandleFunc("POST /tests", func(w http.ResponseWriter, r *http.Request) {
		resp := []byte(fmt.Sprintf(`{"self": "%s/tests/%d"}`, srv.URL, idx))
		objs[idx] = object{b: resp}
		idx++
		w.Write(resp)
		return
	})
	mux.HandleFunc("GET /{id}", func(w http.ResponseWriter, r *http.Request) {
		id, _ := strconv.Atoi(r.PathValue("id"))
		obj, ok := objs[id]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Write(obj.b)
		return
	})
	mux.HandleFunc("DELETE /{id}", func(w http.ResponseWriter, r *http.Request) {
		idStr := filepath.Base(r.URL.Path)
		id, _ := strconv.Atoi(idStr)
		delete(objs, id)
		return
	})
	srv.Start()
	d := codeServerData{}
	resource.Test(t, resource.TestCase{
		CheckDestroy:             d.CheckDestroy(srv.URL, addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.create_ret_url(srv.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.NotNull()),
				},
			},
		},
	})
}

func TestResource_CodeServer_HeaderQuery(t *testing.T) {
	addr := "restful_resource.test"

	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)
	mux.HandleFunc("POST /tests/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("type") != "create" {
			w.WriteHeader(http.StatusBadRequest)
		}
		if r.URL.Query().Get("type") != "create" {
			w.WriteHeader(http.StatusBadRequest)
		}
		return
	})
	mux.HandleFunc("PUT /tests/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("type") != "update" {
			w.WriteHeader(http.StatusBadRequest)
		}
		if r.URL.Query().Get("type") != "update" {
			w.WriteHeader(http.StatusBadRequest)
		}
		return
	})
	mux.HandleFunc("GET /tests/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("type") != "read" {
			w.WriteHeader(http.StatusBadRequest)
		}
		if r.URL.Query().Get("type") != "read" {
			w.WriteHeader(http.StatusBadRequest)
		}
		w.Write([]byte(`{}`))
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
	d := codeServerData{}
	resource.Test(t, resource.TestCase{
		//CheckDestroy:             d.CheckDestroy(srv.URL, addr),
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.headerquery(srv.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.NotNull()),
				},
			},
		},
	})
}

func TestResource_CodeServer_HeaderQueryFromBody(t *testing.T) {
	addr := "restful_resource.test"

	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)
	var body []byte
	mux.HandleFunc("POST /tests/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("type") != "create" {
			w.WriteHeader(http.StatusBadRequest)
		}
		if r.URL.Query().Get("type") != "create" {
			w.WriteHeader(http.StatusBadRequest)
		}
		body, _ = io.ReadAll(r.Body)
		w.Write(body)
		return
	})
	mux.HandleFunc("PUT /tests/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("type") != "update_b" {
			w.WriteHeader(http.StatusBadRequest)
		}
		if r.URL.Query().Get("type") != "update_b" {
			w.WriteHeader(http.StatusBadRequest)
		}
		return
	})
	mux.HandleFunc("GET /tests/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("type") != "read_b" {
			w.WriteHeader(http.StatusBadRequest)
		}
		if r.URL.Query().Get("type") != "read_b" {
			w.WriteHeader(http.StatusBadRequest)
		}
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
	d := codeServerData{}
	resource.Test(t, resource.TestCase{
		//CheckDestroy:             d.CheckDestroy(srv.URL, addr),
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

func TestResource_CodeServer_ReadResponseTemplate(t *testing.T) {
	addr := "restful_resource.test"

	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)
	mux.HandleFunc("PUT /tests/1", func(w http.ResponseWriter, r *http.Request) {
		return
	})
	mux.HandleFunc("GET /tests/1", func(w http.ResponseWriter, r *http.Request) {
		// From https://github.com/lfventura/terraform-provider-restful/issues/130
		b := []byte(`[
   {
      "property_name": "system",
      "value": "testing-system"
   }
]`)
		w.Write(b)
		return
	})
	mux.HandleFunc("DELETE /tests/1", func(w http.ResponseWriter, r *http.Request) {
		return
	})
	srv.Start()
	d := codeServerData{}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.readResponseTemplate(srv.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.NotNull()),
				},
			},
			{
				ResourceName:            addr,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"create_method"},
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return `{"id": "/tests/1", "path": "/tests/1", "body": [{"properties": [{"property_name": null, "value": null}]}], "read_response_template": "{\"properties\": $(body)}"}`, nil
				},
			},
		},
	})
}

func TestResource_CodeServer_DeleteMethodBody(t *testing.T) {
	addr := "restful_resource.test"

	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)

	var state []byte
	mux.HandleFunc("PUT /tests/1", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		state = b
	})
	mux.HandleFunc("GET /tests/1", func(w http.ResponseWriter, r *http.Request) {
		w.Write(state)
		return
	})
	mux.HandleFunc("PATCH /tests/1", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		state, err = jsonpatch.MergePatch(state, b)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
		}
	})
	mux.HandleFunc("DELETE /tests/1", func(w http.ResponseWriter, r *http.Request) {
		state = nil
	})
	srv.Start()
	d := codeServerData{}
	resource.Test(t, resource.TestCase{
		CheckDestroy: func(s *terraform.State) error {
			ctx := context.TODO()
			c, err := client.New(context.TODO(), srv.URL, nil)
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
				if resp.StatusCode() != http.StatusOK {
					return fmt.Errorf("%s: failed to read", addr)
				}
				var m map[string]interface{}
				if err := json.Unmarshal(resp.Body(), &m); err != nil {
					return err
				}
				l, ok := m["properties"]
				if !ok {
					return fmt.Errorf("expected `properties`, got nil")
				}
				if ll := len(l.([]interface{})); ll != 0 {
					return fmt.Errorf("expected zero length of `properties`, got %d", ll)
				}
				return nil
			}
			panic("unreachable")
		},
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.deleteMethodBody(srv.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.NotNull()),
				},
			},
		},
	})
}

func TestResource_CodeServer_DeleteMethodBodyRaw(t *testing.T) {
	addr := "restful_resource.test"

	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)

	const myid = 123
	var state []byte
	mux.HandleFunc("PUT /tests/1", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}
		var m map[string]interface{}
		if err := json.Unmarshal(b, &m); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}
		m["id"] = myid
		state, _ = json.Marshal(m)
	})
	mux.HandleFunc("GET /tests/1", func(w http.ResponseWriter, r *http.Request) {
		w.Write(state)
		return
	})
	mux.HandleFunc("PATCH /tests/1", func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(err.Error()))
			return
		}
		state, err = jsonpatch.MergePatch(state, b)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(fmt.Sprintf("%s: %s", string(b), []byte(err.Error()))))
			return
		}
	})
	mux.HandleFunc("DELETE /tests/1", func(w http.ResponseWriter, r *http.Request) {
		state = nil
	})
	srv.Start()
	d := codeServerData{}
	resource.Test(t, resource.TestCase{
		CheckDestroy: func(s *terraform.State) error {
			ctx := context.TODO()
			c, err := client.New(context.TODO(), srv.URL, nil)
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
				if resp.StatusCode() != http.StatusOK {
					return fmt.Errorf("%s: failed to read", addr)
				}

				dec := json.NewDecoder(bytes.NewBuffer(resp.Body()))
				dec.UseNumber()
				var m map[string]interface{}
				if err := dec.Decode(&m); err != nil {
					return err
				}
				l, ok := m["properties"]
				if !ok {
					return fmt.Errorf("expected `properties`, got nil")
				}
				if ll := len(l.([]interface{})); ll != 0 {
					return fmt.Errorf("expected zero length of `properties`, got %d", ll)
				}
				id, ok := m["id"]
				if !ok {
					return fmt.Errorf("expected `id`, got nil")
				}
				if id.(json.Number).String() != strconv.Itoa(myid) {
					return fmt.Errorf("expect id=%d, got=%s", myid, id)
				}
				return nil
			}
			panic("unreachable")
		},
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.deleteMethodBodyRaw(srv.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue(addr, tfjsonpath.New("output"), knownvalue.NotNull()),
				},
			},
		},
	})
}

func (d codeServerData) CheckDestroy(url, addr string) func(*terraform.State) error {
	return func(s *terraform.State) error {
		ctx := context.TODO()
		c, err := client.New(context.TODO(), url, nil)
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

func (d codeServerData) object_array(url, v string) string {
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

func (d codeServerData) create_ret_string(url string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "test"
  create_method = "PUT"
  read_path = "$(path)/$(body)"
  body = {}
}
`, url)
}

func (d codeServerData) create_ret_url(url string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "/tests"
  read_path = "$url_path.trim_path(body.self)"
  body = {}
}
`, url)
}

func (d codeServerData) headerquery(url string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "/tests/1"
  create_header = {
  	type = "create"
  }
  create_query = {
  	type = ["create"]
  }
  update_header = {
  	type = "update"
  }
  update_query = {
  	type = ["update"]
  }
  read_header = {
  	type = "read"
  }
  read_query = {
  	type = ["read"]
  }
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

func (d codeServerData) headerqueryFromBody(url string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "/tests/1"
  create_header = {
  	type = "create"
  }
  create_query = {
  	type = ["create"]
  }
  update_header = {
  	type = "$(body.update)"
  }
  update_query = {
  	type = ["$(body_update)"]
  }
  read_header = {
  	type = "$(body.read)"
  }
  read_query = {
  	type = ["$(body.read)"]
  }
  delete_header = {
  	type = "$(body.delete)"
  }
  delete_query = {
  	type = ["$(body.delete)"]
  }
  body = {
    read = "read_b"
    update = "update_b"
    delete = "delete_b"
  }
}
`, url)
}

func (d codeServerData) readResponseTemplate(url string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "/tests/1"
  create_method = "PUT"
  body = {
    properties = [
      {
        property_name = "system"
        value         = "testing-system"
      }
    ]
  }
  read_response_template = "{\"properties\": $(body)}"
}
`, url)
}

func (d codeServerData) deleteMethodBody(url string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "/tests/1"
  create_method = "PUT"
  delete_method = "PATCH"
  body = {
    properties = [
      {
        property_name = "system"
        value         = "testing-system"
      }
    ]
  }
  delete_body = {
    properties = []
  }
}
`, url)
}

func (d codeServerData) deleteMethodBodyRaw(url string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

resource "restful_resource" "test" {
  path = "/tests/1"
  create_method = "PUT"
  delete_method = "PATCH"
  body = {
    properties = [
      {
        property_name = "system"
        value         = "testing-system"
      }
    ]
  }
  delete_body_raw = <<EOF
{
	"id":  $(body.id),
	"properties": []
}
EOF
}
`, url)
}
