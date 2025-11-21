package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/lfventura/terraform-provider-restful/internal/client"
	"github.com/magodo/terraform-plugin-framework-helper/dynamic"
)

type DataSource struct {
	p *Provider
}

var _ datasource.DataSource = &DataSource{}

type dataSourceData struct {
	ID                 types.String  `tfsdk:"id"`
	Method             types.String  `tfsdk:"method"`
	Query              types.Map     `tfsdk:"query"`
	Header             types.Map     `tfsdk:"header"`
	Body               types.Dynamic `tfsdk:"body"`
	Selector           types.String  `tfsdk:"selector"`
	OutputAttrs        types.Set     `tfsdk:"output_attrs"`
	AllowNotExist      types.Bool    `tfsdk:"allow_not_exist"`
	Precheck           types.List    `tfsdk:"precheck"`
	UseSensitiveOutput types.Bool    `tfsdk:"use_sensitive_output"`
	Output             types.Dynamic `tfsdk:"output"`
	SensitiveOutput    types.Dynamic `tfsdk:"sensitive_output"`
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
			"method": schema.StringAttribute{
				Description:         "The HTTP Method for the request. Allowed methods are a subset of methods defined in RFC7231 namely, GET, HEAD, and POST. POST support is only intended for read-only URLs, such as submitting a search. Defaults to `GET`.",
				MarkdownDescription: "The HTTP Method for the request. Allowed methods are a subset of methods defined in [RFC7231](https://datatracker.ietf.org/doc/html/rfc7231#section-4.3) namely, `GET`, `HEAD`, and `POST`. `POST` support is only intended for read-only URLs, such as submitting a search. Defaults to `GET`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("GET", "POST", "HEAD"),
				},
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
			"body": schema.DynamicAttribute{
				Description:         "The request body that is sent when using `POST` method.",
				MarkdownDescription: "The request body that is sent when using `POST` method.",
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
			"precheck": precheckAttribute("Read", true, "", false),
			"use_sensitive_output": schema.BoolAttribute{
				Description:         "Whether to use `sensitive_output` instead of `output`. When true, the response will be stored in `sensitive_output` (which is marked as sensitive). Defaults to `false`.",
				MarkdownDescription: "Whether to use `sensitive_output` instead of `output`. When true, the response will be stored in `sensitive_output` (which is marked as sensitive). Defaults to `false`.",
				Optional:            true,
			},
			"output": schema.DynamicAttribute{
				Description:         "The response body after reading the resource. This is only populated when `use_sensitive_output` is false.",
				MarkdownDescription: "The response body after reading the resource. This is only populated when `use_sensitive_output` is false.",
				Computed:            true,
			},
			"sensitive_output": schema.DynamicAttribute{
				Description:         "The response body after reading the resource (sensitive). This is only populated when `use_sensitive_output` is true.",
				MarkdownDescription: "The response body after reading the resource (sensitive). This is only populated when `use_sensitive_output` is true.",
				Computed:            true,
				Sensitive:           true,
			},
		},
	}
}

func (d *DataSource) ValidateConfig(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	var config dataSourceData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if !config.Body.IsUnknown() && !config.Body.IsNull() {
		if !config.Method.IsUnknown() {
			if config.Method.ValueString() != "POST" {
				resp.Diagnostics.AddError(
					"Invalid configuration",
					"`body` is only applicable when `method` is set to `POST`",
				)
			}
		}
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
	c := d.p.client
	c.SetLoggerContext(ctx)

	var config dataSourceData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	opt, diags := d.p.apiOpt.ForDataSourceRead(ctx, config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if !config.Precheck.IsNull() {
		unlockFunc, diags := precheck(ctx, c, d.p.apiOpt, "", opt.Header, opt.Query, config.Precheck, basetypes.NewDynamicNull())
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		defer unlockFunc()
	}

	state := dataSourceData{
		ID:            config.ID,
		Method:        config.Method,
		Query:         config.Query,
		Header:        config.Header,
		Body:          config.Body,
		Selector:      config.Selector,
		OutputAttrs:   config.OutputAttrs,
		AllowNotExist: config.AllowNotExist,
		Precheck:      config.Precheck,
	}

	// Set body if provided
	var body []byte
	if !config.Body.IsNull() && config.Method.ValueString() == "POST" {
		var err error
		body, err = dynamic.ToJSON(config.Body)
		if err != nil {
			resp.Diagnostics.AddError(
				"Error to convert body",
				err.Error(),
			)
			return
		}
	}

	response, err := c.ReadDS(ctx, config.ID.ValueString(), body, *opt)
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

	if sel := config.Selector.ValueString(); sel != "" {
		bodyLocator := client.BodyLocator(sel)
		sb, ok := bodyLocator.LocateValueInResp(*response)
		if !ok {
			if config.AllowNotExist.ValueBool() {
				// Setting the input attributes to the state anyway
				diags = resp.State.Set(ctx, state)
				resp.Diagnostics.Append(diags...)
				return
			}
			resp.Diagnostics.AddError(
				fmt.Sprintf("`selector` failed to select from the response"),
				string(response.Body()),
			)
			return
		}
		b = []byte(sb)
	}

	// Set output
	if !config.OutputAttrs.IsNull() {
		// Update the output to only contain the specified attributes.
		var outputAttrs []string
		diags = config.OutputAttrs.ElementsAs(ctx, &outputAttrs, false)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}
		fb, err := FilterAttrsInJSON(string(b), outputAttrs)
		if err != nil {
			resp.Diagnostics.AddError(
				"Filter `output` during Read",
				err.Error(),
			)
			return
		}
		b = []byte(fb)
	}

	output, err := dynamic.FromJSONImplied(b)
	if err != nil {
		resp.Diagnostics.AddError(
			"Evaluating `output` during Read",
			err.Error(),
		)
		return
	}
	// Populate the appropriate output based on use_sensitive_output
	if state.UseSensitiveOutput.ValueBool() {
		state.SensitiveOutput = output
		state.Output = types.DynamicNull()
	} else {
		state.Output = output
		state.SensitiveOutput = types.DynamicNull()
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
}
