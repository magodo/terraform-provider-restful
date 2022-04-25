package restapi

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/magodo/terraform-provider-restapi/client"
)

type dataSourceType struct{}

func (d dataSourceType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description:         "Restful data source",
		MarkdownDescription: "Restful data source",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Description:         "The ID of the Resource",
				MarkdownDescription: "The ID of the Resource",
				Type:                types.StringType,
				Required:            true,
			},
			"body": {
				Description:         "The properties of the resource",
				MarkdownDescription: "The properties of the resource",
				Type:                types.StringType,
				Computed:            true,
			},
		},
	}, nil
}

func (d dataSourceType) NewDataSource(_ context.Context, p tfsdk.Provider) (tfsdk.DataSource, diag.Diagnostics) {
	return dataSource{p: *p.(*provider)}, nil
}

type dataSource struct {
	p provider
}

var _ tfsdk.DataSource = dataSource{}

type dataSourceData struct {
	ID   types.String `tfsdk:"id"`
	Body types.String `tfsdk:"body"`
}

func (d dataSource) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var config dataSourceData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	c, err := d.p.ClientBuilder.Build(ctx)
	if err != nil {
		resp.Diagnostics.AddError(
			"Read failure",
			fmt.Sprintf("failed to build client: %v", err.Error()),
		)
	}

	b, err := c.Read(config.ID.Value)
	if err != nil {
		if err == client.ErrNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Read failure",
			err.Error(),
		)
		return
	}

	state := dataSourceData{
		ID: config.ID,
		Body: types.String{
			Value: string(b),
		},
	}
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
}
