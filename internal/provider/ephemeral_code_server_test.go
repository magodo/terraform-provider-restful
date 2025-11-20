package provider_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/lfventura/terraform-provider-restful/internal/acceptance"
	"github.com/stretchr/testify/require"
)

type codeServerEphemeral struct{}

func TestEphemeral_CodeServer_basic(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)

	mux.HandleFunc("POST /lease", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"foo": "bar"}`))
	})

	srv.Start()
	d := codeServerEphemeral{}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.basic(srv.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("foo"), knownvalue.StringExact("bar")),
				},
			},
		},
	})
}

func TestEphemeral_CodeServer_complete(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)

	mux.HandleFunc("POST /sleep", func(w http.ResponseWriter, r *http.Request) {
		n, _ := strconv.Atoi(r.Header.Get("time"))
		time.Sleep(time.Duration(n) * time.Second)
	})

	var leaseCnt int
	mux.HandleFunc("POST /lease", func(w http.ResponseWriter, r *http.Request) {
		leaseCnt++
		w.Header().Add("expiry", "1")
		w.Write([]byte(`{"foo": "bar"}`))
	})

	var updateLeaseCnt int
	mux.HandleFunc("POST /updateLease", func(w http.ResponseWriter, r *http.Request) {
		updateLeaseCnt++
		w.Header().Add("expiry", "1")
	})

	var unleaseCnt int
	mux.HandleFunc("POST /unlease", func(w http.ResponseWriter, r *http.Request) {
		unleaseCnt++
	})

	srv.Start()
	d := codeServerEphemeral{}
	resource.Test(t, resource.TestCase{
		ExternalProviders: map[string]resource.ExternalProvider{
			"time": {
				VersionConstraint: "=0.9.1",
				Source:            "registry.terraform.io/hashicorp/time",
			},
		},
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config: d.complete(srv.URL),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("foo"), knownvalue.StringExact("bar")),
				},
			},
		},
	})

	//t.Log(leaseCnt, updateLeaseCnt, unleaseCnt)

	require.Equal(t, 6, leaseCnt, "open")
	require.Equal(t, 3, updateLeaseCnt, "renew") // 2 (sleep time) / (1-0.4) = 3
	require.Equal(t, 6, unleaseCnt, "close")
}

func TestEphemeral_CodeServer_HeaderQueryFromBody(t *testing.T) {
	mux := http.NewServeMux()
	srv := httptest.NewUnstartedServer(mux)
	var body []byte
	mux.HandleFunc("POST /open", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("type") != "open" {
			w.WriteHeader(http.StatusBadRequest)
		}
		if r.URL.Query().Get("type") != "open" {
			w.WriteHeader(http.StatusBadRequest)
		}
		body, _ = io.ReadAll(r.Body)
		w.Write(body)
		return
	})
	mux.HandleFunc("POST /close", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("type") != "close_b" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(``))
		}
		if r.URL.Query().Get("type") != "close_b" {
			w.WriteHeader(http.StatusBadRequest)
		}
		return
	})

	srv.Start()
	d := codeServerEphemeral{}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acceptance.ProviderFactory(),
		Steps: []resource.TestStep{
			{
				Config:            d.headerqueryFromBody(srv.URL),
				ConfigStateChecks: []statecheck.StateCheck{},
			},
		},
	})
}

func (d codeServerEphemeral) basic(url string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

ephemeral "restful_resource" "test" {
  path = "/lease"
  method = "POST"
}

provider "echo" {
  data = ephemeral.restful_resource.test.output
}

resource "echo" "test" {}
`, url)
}

func (d codeServerEphemeral) complete(url string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

ephemeral "restful_resource" "test" {
  path = "/lease"
  method = "POST"

  renew_path = "/updateLease"
  renew_method = "POST"

  expiry_ahead = "0.4s"
  expiry_type = "duration"
  expiry_locator = "header.expiry"
  expiry_unit = "s"

  close_path = "/unlease"
  close_method = "POST"
}

provider "restful" {
  base_url = %q
  header = {
	dep = ephemeral.restful_resource.test.output.foo
  }
  alias = "sleep" 
}

resource "restful_operation" "test" {
  path = "sleep"
  method = "POST"
  header = {
    time = "2"
  }
  provider = restful.sleep
}

provider "echo" {
  data = ephemeral.restful_resource.test.output
}

resource "echo" "test" {}
`, url, url)
}

func (d codeServerEphemeral) headerqueryFromBody(url string) string {
	return fmt.Sprintf(`
provider "restful" {
  base_url = %q
}

ephemeral "restful_resource" "test" {
  path = "/open"
  method = "POST"
  open_header = {
  	type = "open"
  }
  open_query = {
  	type = ["open"]
  }
  body = {
    close = "close_b"
  }

  close_path = "/close"
  close_method = "POST"
  close_header = {
  	type = "$(body.close)"
  }
  close_query = {
  	type = ["$(body.close)"]
  }
}
`, url)
}
