package restapi

import (
	"context"

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
				Description:         "The ID of the Resource, i.e. The path of the data source, relative to the `base_url` of the provider",
				MarkdownDescription: "The ID of the Resource, i.e. The path of the data source, relative to the `base_url` of the provider",
				Type:                types.StringType,
				Required:            true,
			},
			"query": {
				Description:         "The query used to read the data source",
				MarkdownDescription: "The query used to read the data source",
				Type:                types.MapType{ElemType: types.StringType},
				Optional:            true,
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
	ID    types.String `tfsdk:"id"`
	Query types.Map    `tfsdk:"query"`
	Body  types.String `tfsdk:"body"`
}

func (d dataSource) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var config dataSourceData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	c := d.p.client
	if len(config.Query.Elems) != 0 {
		m := map[string]string{}
		for k, v := range config.Query.Elems {
			m[k] = v.(types.String).Value
		}
		c.SetQueryParams(m)
	}

	b, err := c.Read(ctx, config.ID.Value)
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
		ID:    config.ID,
		Query: config.Query,
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
