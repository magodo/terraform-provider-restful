package main

import (
	"context"
	"flag"
	"github.com/magodo/terraform-provider-restful/internal/provider"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	ctx := context.Background()
	serveOpts := tfsdk.ServeOpts{
		Debug: debug,
		Name:  "registry.terraform.io/magodo/restful",
	}

	err := tfsdk.Serve(ctx, provider.New, serveOpts)

	if err != nil {
		log.Fatalf("Error serving provider: %s", err)
	}
}
