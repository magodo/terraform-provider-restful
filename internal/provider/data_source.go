package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/magodo/terraform-plugin-framework-helper/dynamic"
	"github.com/magodo/terraform-provider-restful/internal/client"
	"github.com/magodo/terraform-provider-restful/internal/defaults"
	myvalidator "github.com/magodo/terraform-provider-restful/internal/validator"
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

func dataSourcePrecheckAttribute(s string, pathIsRequired bool, suffixDesc string, statusLocatorSupportParam bool) schema.ListNestedAttribute {
	pathDesc := "The path used to query readiness, relative to the `base_url` of the provider."
	if suffixDesc != "" {
		pathDesc += " " + suffixDesc
	}

	var statusLocatorSuffixDesc string
	if statusLocatorSupportParam {
		statusLocatorSuffixDesc = " The `path` can contain `$(body.x.y.z)` parameter that reference property from the `state.output`."
	}

	return schema.ListNestedAttribute{
		Description:         fmt.Sprintf("An array of prechecks that need to pass prior to the %q operation.", s),
		MarkdownDescription: fmt.Sprintf("An array of prechecks that need to pass prior to the %q operation.", s),
		Optional:            true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"mutex": schema.StringAttribute{
					Description:         "The name of the mutex, which implies the resource will keep waiting until this mutex is held",
					MarkdownDescription: "The name of the mutex, which implies the resource will keep waiting until this mutex is held",
					Optional:            true,
					Validators: []validator.String{
						stringvalidator.ExactlyOneOf(
							path.MatchRelative().AtParent().AtName("api"),
							path.MatchRelative().AtParent().AtName("mutex"),
						),
					},
				},
				"api": schema.SingleNestedAttribute{
					Description:         "Keeps waiting until the specified API meets the success status",
					MarkdownDescription: "Keeps waiting until the specified API meets the success status",
					Optional:            true,
					Attributes: map[string]schema.Attribute{
						"status_locator": schema.StringAttribute{
							Description:         "Specifies how to discover the status property. The format is either `code` or `scope.path`, where `scope` can be either `header` or `body`, and the `path` is using the gjson syntax." + statusLocatorSuffixDesc,
							MarkdownDescription: "Specifies how to discover the status property. The format is either `code` or `scope.path`, where `scope` can be either `header` or `body`, and the `path` is using the [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)." + statusLocatorSuffixDesc,
							Required:            true,
							Validators: []validator.String{
								myvalidator.StringIsParsable("", func(s string) error {
									return validateLocator(s)
								}),
							},
						},
						"status": schema.SingleNestedAttribute{
							Description:         "The expected status sentinels for each polling state.",
							MarkdownDescription: "The expected status sentinels for each polling state.",
							Required:            true,
							Attributes: map[string]schema.Attribute{
								"success": schema.StringAttribute{
									Description:         "The expected status sentinel for suceess status.",
									MarkdownDescription: "The expected status sentinel for suceess status.",
									Required:            true,
								},
								"pending": schema.ListAttribute{
									Description:         "The expected status sentinels for pending status.",
									MarkdownDescription: "The expected status sentinels for pending status.",
									Optional:            true,
									ElementType:         types.StringType,
								},
							},
						},
						"path": schema.StringAttribute{
							Description:         pathDesc,
							MarkdownDescription: pathDesc,
							Required:            pathIsRequired,
							Optional:            !pathIsRequired,
						},
						"query": schema.MapAttribute{
							Description:         "The query parameters. This overrides the `query` set in the resource block.",
							MarkdownDescription: "The query parameters. This overrides the `query` set in the resource block.",
							ElementType:         types.ListType{ElemType: types.StringType},
							Optional:            true,
						},
						"header": schema.MapAttribute{
							Description:         "The header parameters. This overrides the `header` set in the resource block.",
							MarkdownDescription: "The header parameters. This overrides the `header` set in the resource block.",
							ElementType:         types.StringType,
							Optional:            true,
						},
						"default_delay_sec": schema.Int64Attribute{
							Description:         fmt.Sprintf("The interval between two pollings if there is no `Retry-After` in the response header, in second. Defaults to `%d`.", defaults.PrecheckDefaultDelayInSec),
							MarkdownDescription: fmt.Sprintf("The interval between two pollings if there is no `Retry-After` in the response header, in second. Defaults to `%d`.", defaults.PrecheckDefaultDelayInSec),
							Optional:            true,
						},
					},
					Validators: []validator.Object{
						objectvalidator.ExactlyOneOf(
							path.MatchRelative().AtParent().AtName("mutex"),
							path.MatchRelative().AtParent().AtName("api"),
						),
					},
				},
			},
		},
	}
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
				MarkdownDescription: "The HTTP Method for the request. `POST` support is only intended for read-only URLs, such as submitting a search. Defaults to `GET`.",
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
			"precheck": dataSourcePrecheckAttribute("Read", true, "", false),
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
	if config.UseSensitiveOutput.ValueBool() {
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
