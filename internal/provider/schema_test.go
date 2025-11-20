package provider_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	fwprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/lfventura/terraform-provider-restful/internal/provider"
)

func TestSchemaValidation(t *testing.T) {
	p := &provider.Provider{}
	ctx := context.TODO()

	var resp fwprovider.SchemaResponse
	p.Schema(ctx, fwprovider.SchemaRequest{}, &resp)
	if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
		t.Fatal(diags.Errors())
	}

	for _, rf := range p.Resources(ctx) {
		res := rf()
		var resp resource.SchemaResponse
		res.Schema(ctx, resource.SchemaRequest{}, &resp)
		if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
			t.Fatal(diags.Errors())
		}
	}

	for _, rf := range p.DataSources(ctx) {
		res := rf()
		var resp datasource.SchemaResponse
		res.Schema(ctx, datasource.SchemaRequest{}, &resp)
		if diags := resp.Schema.ValidateImplementation(ctx); diags.HasError() {
			t.Fatal(diags.Errors())
		}
	}
}
