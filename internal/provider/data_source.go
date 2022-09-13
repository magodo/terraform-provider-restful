package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tidwall/gjson"
)

type DataSource struct {
	p *Provider
}

var _ datasource.DataSource = &DataSource{}

type dataSourceData struct {
	ID       types.String `tfsdk:"id"`
	Query    types.Map    `tfsdk:"query"`
	Header   types.Map    `tfsdk:"header"`
	Selector types.String `tfsdk:"selector"`
	Output   types.String `tfsdk:"output"`
}

func (d *DataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource"
}

func (d *DataSource) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description:         "`restful_resource` data source can be used to retrieve the model of a restful resource by ID.",
		MarkdownDescription: "`restful_resource` data source can be used to retrieve the model of a restful resource by ID.",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Description:         "The ID of the Resource, i.e. The path of the data source, relative to the `base_url` of the provider.",
				MarkdownDescription: "The ID of the Resource, i.e. The path of the data source, relative to the `base_url` of the provider.",
				Type:                types.StringType,
				Required:            true,
			},
			"query": {
				Description:         "The query parameters that are applied to each request. This overrides the `query` set in the provider block.",
				MarkdownDescription: "The query parameters that are applied to each request. This overrides the `query` set in the provider block.",
				Type:                types.MapType{ElemType: types.ListType{ElemType: types.StringType}},
				Optional:            true,
				Computed:            true,
			},
			"header": {
				Description:         "The header parameters that are applied to each request. This overrides the `header` set in the provider block.",
				MarkdownDescription: "The header parameters that are applied to each request. This overrides the `header` set in the provider block.",
				Type:                types.MapType{ElemType: types.StringType},
				Optional:            true,
				Computed:            true,
			},
			"selector": {
				Description:         "A selector in gjson query syntax, that is used when `id` represents a collection of resources, to select exactly one member resource of from it",
				MarkdownDescription: "A selector in [gjson query syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md#queries), that is used when `id` represents a collection of resources, to select exactly one member resource of from it",
				Type:                types.StringType,
				Optional:            true,
				Computed:            true,
			},
			"output": {
				Description:         "The response body after reading the resource.",
				MarkdownDescription: "The response body after reading the resource.",
				Type:                types.StringType,
				Computed:            true,
			},
		},
	}, nil
}

func (d *DataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.p = &Provider{}
	if req.ProviderData != nil {
		d.p = req.ProviderData.(*Provider)
	}
	return
}

func (d *DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
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

	if config.Selector.Value != "" {
		result := gjson.GetBytes(b, config.Selector.Value)
		if !result.Exists() {
			resp.Diagnostics.AddError(
				fmt.Sprintf("Failed to select resource from response"),
				fmt.Sprintf("Can't find resource with query %q", config.Selector.Value),
			)
			return
		}
		if len(result.Array()) > 1 {
			resp.Diagnostics.AddError(
				fmt.Sprintf("Failed to select resource from response"),
				fmt.Sprintf("Multiple resources with query %q found (%d)", config.Selector.Value, len(result.Array())),
			)
			return
		}
		b = []byte(result.Array()[0].Raw)
	}

	state := dataSourceData{
		ID:       config.ID,
		Query:    opt.Query.ToTFValue(),
		Header:   opt.Header.ToTFValue(),
		Selector: config.Selector,
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
