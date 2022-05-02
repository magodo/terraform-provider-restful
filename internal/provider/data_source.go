package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
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
				Description:         "The query parameters that are applied to each request. This won't clean up the `query` set in the provider block, expcet the value with the same key.",
				MarkdownDescription: "The query parameters that are applied to each request. This won't clean up the `query` set in the provider block, expcet the value with the same key.",
				Type:                types.MapType{ElemType: types.ListType{ElemType: types.StringType}},
				Optional:            true,
				Computed:            true,
			},
			"header": {
				Description:         "The header parameters that are applied to each request. This won't clean up the `header` set in the provider block, except the value with the same key.",
				MarkdownDescription: "The header parameters that are applied to each request. This won't clean up the `header` set in the provider block, except the value with the same key.",
				Type:                types.MapType{ElemType: types.StringType},
				Optional:            true,
				Computed:            true,
			},
			"output": {
				Description:         "The response body after reading the resource",
				MarkdownDescription: "The response body after reading the resource",
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
	ID     types.String `tfsdk:"id"`
	Query  types.Map    `tfsdk:"query"`
	Header types.Map    `tfsdk:"header"`
	Output types.String `tfsdk:"output"`
}

func (d dataSource) Read(ctx context.Context, req tfsdk.ReadDataSourceRequest, resp *tfsdk.ReadDataSourceResponse) {
	var config dataSourceData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	c := d.p.client

	opt, diags := d.p.apiOpt.ForDataSourceRead(ctx, config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	response, err := c.Read(ctx, config.ID.Value, *opt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error to call Read",
			err.Error(),
		)
		return
	}
	if !response.IsSuccess() {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Create API returns %d", response.StatusCode()),
			string(response.Body()),
		)
		return
	}

	b := response.Body()

	state := dataSourceData{
		ID:     config.ID,
		Query:  opt.Query.ToTFValue(),
		Header: opt.Header.ToTFValue(),
		Output: types.String{
			Value: string(b),
		},
	}
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
}
