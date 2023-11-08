package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/tidwall/gjson"
)

type DataSource struct {
	p *Provider
}

var _ datasource.DataSource = &DataSource{}

type dataSourceData struct {
	ID            types.String `tfsdk:"id"`
	Query         types.Map    `tfsdk:"query"`
	Header        types.Map    `tfsdk:"header"`
	Selector      types.String `tfsdk:"selector"`
	OutputAttrs   types.Set    `tfsdk:"output_attrs"`
	AllowNotExist types.Bool   `tfsdk:"allow_not_exist"`
	Precheck      types.List   `tfsdk:"precheck"`
	Output        types.String `tfsdk:"output"`
}

func (d *DataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource"
}

func (d *DataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "`restful_resource` data source can be used to retrieve the model of a restful resource by ID.",
		MarkdownDescription: "`restful_resource` data source can be used to retrieve the model of a restful resource by ID.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:         "The ID of the Resource, i.e. The path of the data source, relative to the `base_url` of the provider.",
				MarkdownDescription: "The ID of the Resource, i.e. The path of the data source, relative to the `base_url` of the provider.",
				Required:            true,
			},
			"query": schema.MapAttribute{
				Description:         "The query parameters that are applied to each request. This overrides the `query` set in the provider block.",
				MarkdownDescription: "The query parameters that are applied to each request. This overrides the `query` set in the provider block.",
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
			},
			"header": schema.MapAttribute{
				Description:         "The header parameters that are applied to each request. This overrides the `header` set in the provider block.",
				MarkdownDescription: "The header parameters that are applied to each request. This overrides the `header` set in the provider block.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"selector": schema.StringAttribute{
				Description:         "A selector in gjson query syntax, that is used when `id` represents a collection of resources, to select exactly one member resource of from it",
				MarkdownDescription: "A selector in [gjson query syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md#queries), that is used when `id` represents a collection of resources, to select exactly one member resource of from it",
				Optional:            true,
			},
			"output_attrs": schema.SetAttribute{
				Description:         "A set of `output` attribute paths (in gjson syntax) that will be exported in the `output`. If this is not specified, all attributes will be exported by `output`.",
				MarkdownDescription: "A set of `output` attribute paths (in [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)) that will be exported in the `output`. If this is not specified, all attributes will be exported by `output`.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"allow_not_exist": schema.BoolAttribute{
				Description:         "Whether to throw error if the data source being queried doesn't exist (i.e. status code is 404). Defaults to `false`.",
				MarkdownDescription: "Whether to throw error if the data source being queried doesn't exist (i.e. status code is 404). Defaults to `false`.",
				Optional:            true,
			},
			"precheck": precheckAttribute("Read", true, ""),
			"output": schema.StringAttribute{
				Description:         "The response body after reading the resource.",
				MarkdownDescription: "The response body after reading the resource.",
				Computed:            true,
			},
		},
	}
}

func (d *DataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, ok := req.ProviderData.(providerData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("got: %T.", req.ProviderData),
		)
		return
	}
	if diags := providerData.provider.Init(ctx, providerData.config); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	d.p = providerData.provider
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

	unlockFunc, diags := precheck(ctx, c, d.p.apiOpt, "", opt.Header, opt.Query, config.Precheck)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	defer unlockFunc()

	state := dataSourceData{
		ID:            config.ID,
		Query:         config.Query,
		Header:        config.Header,
		Selector:      config.Selector,
		OutputAttrs:   config.OutputAttrs,
		AllowNotExist: config.AllowNotExist,
		Precheck:      config.Precheck,
	}

	response, err := c.Read(ctx, config.ID.ValueString(), *opt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error to call Read",
			err.Error(),
		)
		return
	}
	if !response.IsSuccess() {
		if response.StatusCode() == http.StatusNotFound && config.AllowNotExist.ValueBool() {
			// Setting the input attributes to the state anyway
			diags = resp.State.Set(ctx, state)
			resp.Diagnostics.Append(diags...)
			return
		}
		resp.Diagnostics.AddError(
			fmt.Sprintf("Read API returns %d", response.StatusCode()),
			string(response.Body()),
		)
		return
	}

	b := response.Body()

	if config.Selector.ValueString() != "" {
		result := gjson.GetBytes(b, config.Selector.ValueString())
		if !result.Exists() {
			resp.Diagnostics.AddError(
				fmt.Sprintf("Failed to select resource from response"),
				fmt.Sprintf("Can't find resource with query %q", config.Selector.ValueString()),
			)
			return
		}
		if len(result.Array()) > 1 {
			resp.Diagnostics.AddError(
				fmt.Sprintf("Failed to select resource from response"),
				fmt.Sprintf("Multiple resources with query %q found (%d)", config.Selector.ValueString(), len(result.Array())),
			)
			return
		}
		b = []byte(result.Array()[0].Raw)
	}

	// Set output
	output := string(b)
	if !config.OutputAttrs.IsNull() {
		// Update the output to only contain the specified attributes.
		var outputAttrs []string
		diags = config.OutputAttrs.ElementsAs(ctx, &outputAttrs, false)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}
		output, err = FilterAttrsInJSON(output, outputAttrs)
		if err != nil {
			resp.Diagnostics.AddError(
				"Filter `output` during Read",
				err.Error(),
			)
			return
		}
	}

	state.Output = types.StringValue(output)

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
}
