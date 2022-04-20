package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/magodo/terraform-provider-restapi/restapi"
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	ctx := context.Background()
	serveOpts := tfsdk.ServeOpts{
		Debug: debug,
		Name:  "registry.terraform.io/magodo/restapi",
	}

	err := tfsdk.Serve(ctx, restapi.New, serveOpts)

	if err != nil {
		log.Fatalf("Error serving provider: %s", err)
	}
}
