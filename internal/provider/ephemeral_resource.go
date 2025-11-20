package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/dynamicvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/magodo/terraform-plugin-framework-helper/dynamic"
	"github.com/lfventura/terraform-provider-restful/internal/exparam"
	myvalidator "github.com/lfventura/terraform-provider-restful/internal/validator"
)

type EphemeralResource struct {
	p *Provider
}

var _ ephemeral.EphemeralResourceWithConfigure = &EphemeralResource{}
var _ ephemeral.EphemeralResourceWithClose = &EphemeralResource{}
var _ ephemeral.EphemeralResourceWithRenew = &EphemeralResource{}

const (
	pkRenew = "renew"
	pkClose = "close"
)

type ephemeralResourceData struct {
	Method     types.String  `tfsdk:"method"`
	Path       types.String  `tfsdk:"path"`
	Body       types.Dynamic `tfsdk:"body"`
	OpenQuery  types.Map     `tfsdk:"open_query"`
	OpenHeader types.Map     `tfsdk:"open_header"`

	Query  types.Map `tfsdk:"query"`
	Header types.Map `tfsdk:"header"`

	RenewMethod  types.String  `tfsdk:"renew_method"`
	RenewBodyRaw types.String  `tfsdk:"renew_body_raw"`
	RenewBody    types.Dynamic `tfsdk:"renew_body"`
	RenewPath    types.String  `tfsdk:"renew_path"`
	RenewQuery   types.Map     `tfsdk:"renew_query"`
	RenewHeader  types.Map     `tfsdk:"renew_header"`

	ExpiryAhead   types.String `tfsdk:"expiry_ahead"`
	ExpiryType    types.String `tfsdk:"expiry_type"`
	ExpiryLocator types.String `tfsdk:"expiry_locator"`
	ExpiryUnit    types.String `tfsdk:"expiry_unit"`

	CloseMethod  types.String  `tfsdk:"close_method"`
	CloseBodyRaw types.String  `tfsdk:"close_body_raw"`
	CloseBody    types.Dynamic `tfsdk:"close_body"`
	ClosePath    types.String  `tfsdk:"close_path"`
	CloseQuery   types.Map     `tfsdk:"close_query"`
	CloseHeader  types.Map     `tfsdk:"close_header"`

	OutputAttrs        types.Set     `tfsdk:"output_attrs"`
	UseSensitiveOutput types.Bool    `tfsdk:"use_sensitive_output"`
	Output             types.Dynamic `tfsdk:"output"`
	SensitiveOutput    types.Dynamic `tfsdk:"sensitive_output"`
}

func (e *EphemeralResource) Metadata(ctx context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource"
}

func (r *EphemeralResource) Configure(ctx context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, ok := req.ProviderData.(providerData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Ephemeral Resource Configure Type",
			fmt.Sprintf("got: %T.", req.ProviderData),
		)
		return
	}
	if diags := providerData.provider.Init(ctx, providerData.config); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	r.p = providerData.provider
}

func (e *EphemeralResource) Schema(ctx context.Context, req ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "`restful_resource` manages an ephemeral resource.",
		MarkdownDescription: "`restful_resource` manages an ephemeral resource.",
		Attributes: map[string]schema.Attribute{
			"method": schema.StringAttribute{
				Description:         "The HTTP method to open the ephemeral resource. Possible values are `GET`, `PUT`, `POST`, `PATCH`.",
				MarkdownDescription: "The HTTP method to open the ephemeral resource. Possible values are `GET`, `PUT`, `POST`, `PATCH`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("GET", "PUT", "POST", "PATCH"),
				},
			},
			"path": schema.StringAttribute{
				Description:         "The path used to open the ephemeral resource, relative to the `base_url` of the provider.",
				MarkdownDescription: "The path used to open the ephemeral resource, relative to the `base_url` of the provider.",
				Required:            true,
			},
			"body": schema.DynamicAttribute{
				Description:         "The payload to open the ephemeral resource.",
				MarkdownDescription: "The payload to open the ephemeral resource.",
				Optional:            true,
			},
			"open_query": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("query", "open"),
				MarkdownDescription: operationOverridableAttrDescription("query", "open"),
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
			},
			"open_header": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("header", "open"),
				MarkdownDescription: operationOverridableAttrDescription("header", "open"),
				ElementType:         types.StringType,
				Optional:            true,
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

			"renew_method": schema.StringAttribute{
				Description:         "The HTTP method to renew the ephemeral resource. Possible values are `GET`, `PUT`, `POST`, `PATCH`.",
				MarkdownDescription: "The HTTP method to renew the ephemeral resource. Possible values are `GET`, `PUT`, `POST`, `PATCH`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("GET", "PUT", "POST", "PATCH"),
					stringvalidator.AlsoRequires(
						path.MatchRoot("renew_path"),
					),
				},
			},
			"renew_path": schema.StringAttribute{
				Description:         "The path used to renew the ephemeral resource, relative to the `base_url` of the provider. " + bodyOrPathParamDescription,
				MarkdownDescription: "The path used to renew the ephemeral resource, relative to the `base_url` of the provider. " + bodyOrPathParamDescription,
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(
						path.MatchRoot("renew_method"),
					),
				},
			},
			"renew_body": schema.DynamicAttribute{
				Description:         "The payload to renew the ephemeral resource. Conflicts with `renew_body_raw`.",
				MarkdownDescription: "The payload to renew the ephemeral resource. Conflicts with `renew_body_raw`.",
				Optional:            true,
				Validators: []validator.Dynamic{
					dynamicvalidator.AlsoRequires(
						path.MatchRoot("renew_method"),
					),
					dynamicvalidator.ConflictsWith(
						path.MatchRoot("renew_body_raw"),
					),
				},
			},
			"renew_body_raw": schema.StringAttribute{
				Description:         "The raw payload for the `Renew` call. It can contain `$(body.x.y.z)` parameter that reference property from the `Open` response. Conflicts with `renew_body`.",
				MarkdownDescription: "The raw payload for the `Renew` call. It can contain `$(body.x.y.z)` parameter that reference property from the `Open` response. Conflicts with `renew_body`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(
						path.MatchRoot("renew_method"),
					),
					stringvalidator.ConflictsWith(
						path.MatchRoot("renew_body"),
					),
				},
			},
			"renew_query": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("query", "renew"),
				MarkdownDescription: operationOverridableAttrDescription("query", "renew"),
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
				Validators: []validator.Map{
					mapvalidator.AlsoRequires(
						path.MatchRoot("renew_method"),
					),
				},
			},
			"renew_header": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("header", "renew"),
				MarkdownDescription: operationOverridableAttrDescription("header", "renew"),
				ElementType:         types.StringType,
				Optional:            true,
				Validators: []validator.Map{
					mapvalidator.AlsoRequires(
						path.MatchRoot("renew_method"),
					),
				},
			},

			"expiry_type": schema.StringAttribute{
				Description:         `The type of the ephemeral resource expiry time. Possible values are: "duration", "time" and "time.[layout]". "duration" means the expiry time is a duration; "time" means the expiry time is a time, which defaults to RF3339 layout, unless the "layout" is explicitly specified (following Go's convention: https://pkg.go.dev/time).`,
				MarkdownDescription: `The type of the ephemeral resource expiry time. Possible values are: "duration", "time" and "time.[layout]". "duration" means the expiry time is a [duration](https://pkg.go.dev/time#ParseDuration); "time" means the expiry time is a time, which defaults to RF3339 layout, unless the "layout" is explicitly specified (following Go's [convention](https://pkg.go.dev/time)).`,
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(
						path.MatchRoot("renew_method"),
						path.MatchRoot("expiry_locator"),
					),
					myvalidator.StringIsParsable("expiry_type", func(s string) error {
						return validateExpiryType(s)
					}),
				},
			},

			"expiry_locator": schema.StringAttribute{
				Description:         "Specifies how to discover the expiry time. The format is `scope.path`, where `scope` can be one of `exact`, `header` and `body`, and the `path` is using the gjson syntax.",
				MarkdownDescription: "Specifies how to discover the expiry time. The format is `scope.path`, where `scope` can be one of `exact`, `header` and `body`, and the `path` is using the [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md).",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(
						path.MatchRoot("renew_method"),
						path.MatchRoot("expiry_type"),
					),
					myvalidator.StringIsParsable("expiry_locator", func(s string) error {
						return validateLocator(s)
					}),
				},
			},

			"expiry_unit": schema.StringAttribute{
				Description:         "Specifies the unit of the expiry when the `expiry_type` is `duration`. Valid units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.",
				MarkdownDescription: "Specifies the unit of the expiry when the `expiry_type` is `duration`. Valid units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(
						path.MatchRoot("renew_method"),
						path.MatchRoot("expiry_type"),
					),
				},
			},

			"expiry_ahead": schema.StringAttribute{
				Description:         "Advance the ephemeral resource expiry time by this duration. The format is same as Go's ParseDuration.",
				MarkdownDescription: "Advance the ephemeral resource expiry time by this duration. The format is same as Go's [ParseDuration](https://pkg.go.dev/time#ParseDuration).",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(
						path.MatchRoot("renew_method"),
					),
					myvalidator.StringIsParsable("expiry_ahead", func(s string) error {
						_, err := time.ParseDuration(s)
						return err
					}),
				},
			},

			"close_method": schema.StringAttribute{
				Description:         "The HTTP method to close the ephemeral resource. Possible values are `PUT`, `POST`, `PATCH`, `DELETE`.",
				MarkdownDescription: "The HTTP method to close the ephemeral resource. Possible values are `PUT`, `POST`, `PATCH`, `DELETE`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("PUT", "POST", "PATCH", "DELETE"),
					stringvalidator.AlsoRequires(
						path.MatchRoot("close_path"),
					),
				},
			},
			"close_path": schema.StringAttribute{
				Description:         "The path used to close the ephemeral resource, relative to the `base_url` of the provider. " + bodyOrPathParamDescription,
				MarkdownDescription: "The path used to close the ephemeral resource, relative to the `base_url` of the provider. " + bodyOrPathParamDescription,
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(
						path.MatchRoot("close_method"),
					),
				},
			},
			"close_body": schema.DynamicAttribute{
				Description:         "The payload to close the ephemeral resource. Conflicts with `close_body_raw`.",
				MarkdownDescription: "The payload to close the ephemeral resource. Conflicts with `close_body_raw`.",
				Optional:            true,
				Validators: []validator.Dynamic{
					dynamicvalidator.AlsoRequires(
						path.MatchRoot("close_method"),
					),
					dynamicvalidator.ConflictsWith(
						path.MatchRoot("close_body_raw"),
					),
				},
			},
			"close_body_raw": schema.StringAttribute{
				Description:         "The raw payload for the `Close` call. It can contain `$(body.x.y.z)` parameter that reference property from the `Open` response. Conflicts with `close_body`.",
				MarkdownDescription: "The raw payload for the `Close` call. It can contain `$(body.x.y.z)` parameter that reference property from the `Open` response. Conflicts with `close_body`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(
						path.MatchRoot("close_method"),
					),
					stringvalidator.ConflictsWith(
						path.MatchRoot("close_body"),
					),
				},
			},
			"close_query": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("query", "close"),
				MarkdownDescription: operationOverridableAttrDescription("query", "close"),
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
				Validators: []validator.Map{
					mapvalidator.AlsoRequires(
						path.MatchRoot("close_method"),
					),
				},
			},
			"close_header": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("header", "close"),
				MarkdownDescription: operationOverridableAttrDescription("header", "close"),
				ElementType:         types.StringType,
				Optional:            true,
				Validators: []validator.Map{
					mapvalidator.AlsoRequires(
						path.MatchRoot("close_method"),
					),
				},
			},

			"output_attrs": schema.SetAttribute{
				Description:         "A set of `output` attribute paths (in gjson syntax) that will be exported in the `output`. If this is not specified, all attributes will be exported by `output`.",
				MarkdownDescription: "A set of `output` attribute paths (in [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)) that will be exported in the `output`. If this is not specified, all attributes will be exported by `output`.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"use_sensitive_output": schema.BoolAttribute{
				Description:         "Whether to use `sensitive_output` instead of `output`. When true, the response will be stored in `sensitive_output` (which is marked as sensitive). Defaults to `false`.",
				MarkdownDescription: "Whether to use `sensitive_output` instead of `output`. When true, the response will be stored in `sensitive_output` (which is marked as sensitive). Defaults to `false`.",
				Optional:            true,
			},

			"output": schema.DynamicAttribute{
				Description:         "The response body. This is only populated when `use_sensitive_output` is false.",
				MarkdownDescription: "The response body. This is only populated when `use_sensitive_output` is false.",
				Computed:            true,
			},
			"sensitive_output": schema.DynamicAttribute{
				Description:         "The response body (sensitive). This is only populated when `use_sensitive_output` is true.",
				MarkdownDescription: "The response body (sensitive). This is only populated when `use_sensitive_output` is true.",
				Computed:            true,
				Sensitive:           true,
			},
		},
	}
	return
}

func (e *EphemeralResource) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	c := e.p.client
	c.SetLoggerContext(ctx)

	var config ephemeralResourceData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	tflog.Info(ctx, "Open an ephemeral resource", map[string]interface{}{"path": config.Path.ValueString()})

	opt, diags := e.p.apiOpt.ForOperation(ctx, config.Method, config.Query, config.Header, config.OpenQuery, config.OpenHeader, nil)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	body, err := dynamic.ToJSON(config.Body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Convert open body from dynamic to json",
			err.Error(),
		)
		return
	}

	response, err := c.Operation(ctx, config.Path.ValueString(), body, *opt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error to call open operation",
			err.Error(),
		)
		return
	}
	if !response.IsSuccess() {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Open operation API returns %d", response.StatusCode()),
			string(response.Body()),
		)
		return
	}

	// Set RenewAt, if specified
	if !config.ExpiryType.IsNull() && !config.ExpiryType.IsUnknown() {
		t, err := GetExpiryTime(config.ExpiryType.ValueString(), config.ExpiryLocator.ValueString(), config.ExpiryUnit.ValueString(), config.ExpiryAhead.ValueString(), *response)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to parse expiry time",
				err.Error(),
			)
			return
		}
		tflog.Info(ctx, fmt.Sprintf("renew_at=%v", t))
		resp.RenewAt = t
	}

	// Set Output
	rb := response.Body()
	if !config.OutputAttrs.IsNull() {
		// Update the output to only contain the specified attributes.
		var outputAttrs []string
		diags = config.OutputAttrs.ElementsAs(ctx, &outputAttrs, false)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}
		fb, err := FilterAttrsInJSON(string(rb), outputAttrs)
		if err != nil {
			resp.Diagnostics.AddError(
				"Filter `output` during operation",
				err.Error(),
			)
			return
		}
		rb = []byte(fb)
	}

	output, err := dynamic.FromJSONImplied(rb)
	if err != nil {
		resp.Diagnostics.AddError(
			"Converting `output` from JSON to dynamic",
			err.Error(),
		)
		return
	}
	// Populate the appropriate output based on use_sensitive_output
	if !config.UseSensitiveOutput.IsNull() && config.UseSensitiveOutput.ValueBool() {
		config.SensitiveOutput = output
		config.Output = types.DynamicNull()
	} else {
		config.Output = output
		config.SensitiveOutput = types.DynamicNull()
	}

	// Set Renew and Close, if any
	if !config.RenewMethod.IsNull() {
		path, err := exparam.ExpandBodyOrPath(config.RenewPath.ValueString(), config.Path.ValueString(), response.Body())
		if err != nil {
			resp.Diagnostics.AddError(
				fmt.Sprintf("Failed to build the path for renew the resource"),
				err.Error(),
			)
			return
		}

		var renewBody types.Dynamic
		switch {
		case !config.RenewBody.IsNull():
			renewBody = config.RenewBody
		case !config.RenewBodyRaw.IsNull():
			renewBodyRaw, err := exparam.ExpandBody(config.RenewBodyRaw.ValueString(), response.Body())
			if err != nil {
				resp.Diagnostics.AddError(
					fmt.Sprintf("Failed to expand the renew_body_raw"),
					err.Error(),
				)
				return
			}
			renewBody, err = dynamic.FromJSONImplied([]byte(renewBodyRaw))
			if err != nil {
				resp.Diagnostics.AddError(
					fmt.Sprintf("Failed to convert the expanded renew_body_raw to dynamic"),
					err.Error(),
				)
				return
			}
		}

		ed := ephemeralResourcePrivateData{
			Method:        config.RenewMethod,
			Path:          basetypes.NewStringValue(path),
			Body:          renewBody,
			DefaultHeader: config.Header,
			Header:        config.RenewHeader,
			DefaultQuery:  config.Query,
			Query:         config.RenewQuery,
			ExpiryType:    config.ExpiryType,
			ExpiryLocator: config.ExpiryLocator,
			ExpiryAhead:   config.ExpiryAhead,
			ExpiryUnit:    config.ExpiryUnit,
			Output:        output,
		}
		b, err := json.Marshal(ed)
		if err != nil {
			resp.Diagnostics.AddError(
				"Setting private data for renew",
				err.Error(),
			)
			return
		}
		resp.Diagnostics.Append(resp.Private.SetKey(ctx, pkRenew, b)...)
		if diags.HasError() {
			return
		}
	}

	if !config.CloseMethod.IsNull() {
		path, err := exparam.ExpandBodyOrPath(config.ClosePath.ValueString(), config.Path.ValueString(), response.Body())
		if err != nil {
			resp.Diagnostics.AddError(
				fmt.Sprintf("Failed to build the path for renew the resource"),
				err.Error(),
			)
		}

		var closeBody types.Dynamic
		switch {
		case !config.CloseBody.IsNull():
			closeBody = config.CloseBody
		case !config.CloseBodyRaw.IsNull():
			closeBodyRaw, err := exparam.ExpandBody(config.CloseBodyRaw.ValueString(), response.Body())
			if err != nil {
				resp.Diagnostics.AddError(
					fmt.Sprintf("Failed to expand the close_body_raw"),
					err.Error(),
				)
				return
			}
			closeBody, err = dynamic.FromJSONImplied([]byte(closeBodyRaw))
			if err != nil {
				resp.Diagnostics.AddError(
					fmt.Sprintf("Failed to convert the expanded close_body_raw to dynamic"),
					err.Error(),
				)
				return
			}
		}

		ed := ephemeralResourcePrivateData{
			Method:        config.CloseMethod,
			Path:          basetypes.NewStringValue(path),
			Body:          closeBody,
			DefaultHeader: config.Header,
			Header:        config.CloseHeader,
			DefaultQuery:  config.Query,
			Query:         config.CloseQuery,
			Output:        output,
		}
		b, err := json.Marshal(ed)
		if err != nil {
			resp.Diagnostics.AddError(
				"Setting private data for close",
				err.Error(),
			)
			return
		}
		resp.Diagnostics.Append(resp.Private.SetKey(ctx, pkClose, b)...)
		if diags.HasError() {
			return
		}
	}

	diags = resp.Result.Set(ctx, config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
}

func (e *EphemeralResource) Renew(ctx context.Context, req ephemeral.RenewRequest, resp *ephemeral.RenewResponse) {
	c := e.p.client
	c.SetLoggerContext(ctx)

	b, diags := req.Private.GetKey(ctx, pkRenew)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if b == nil {
		return
	}

	var pd ephemeralResourcePrivateData
	if err := json.Unmarshal(b, &pd); err != nil {
		resp.Diagnostics.AddError(
			"Unmarshal private data (renew)",
			err.Error(),
		)
		return
	}

	tflog.Info(ctx, "Renew an ephemeral resource", map[string]interface{}{"path": pd.Path.ValueString()})

	var output []byte
	if !pd.Output.IsNull() {
		var err error
		output, err = dynamic.ToJSON(pd.Output)
		if err != nil {
			resp.Diagnostics.AddError(
				"converting private output to json",
				err.Error(),
			)
			return
		}
	}
	opt, diags := e.p.apiOpt.ForOperation(ctx, pd.Method, pd.DefaultQuery, pd.DefaultHeader, pd.Query, pd.Header, output)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	response, err := c.Operation(ctx, pd.Path.ValueString(), output, *opt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error to call renew operation",
			err.Error(),
		)
		return
	}
	if !response.IsSuccess() {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Renew operation API returns %d", response.StatusCode()),
			string(response.Body()),
		)
		return
	}

	t, err := GetExpiryTime(pd.ExpiryType.ValueString(), pd.ExpiryLocator.ValueString(), pd.ExpiryUnit.ValueString(), pd.ExpiryAhead.ValueString(), *response)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to parse expiry time",
			err.Error(),
		)
		return
	}
	resp.RenewAt = t
}

func (e *EphemeralResource) Close(ctx context.Context, req ephemeral.CloseRequest, resp *ephemeral.CloseResponse) {
	c := e.p.client
	c.SetLoggerContext(ctx)

	b, diags := req.Private.GetKey(ctx, pkClose)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if b == nil {
		return
	}

	var pd ephemeralResourcePrivateData
	if err := json.Unmarshal(b, &pd); err != nil {
		resp.Diagnostics.AddError(
			"Unmarshal private data (close)",
			err.Error(),
		)
		return
	}

	tflog.Info(ctx, "Close an ephemeral resource", map[string]interface{}{"path": pd.Path.ValueString()})

	var output []byte
	if !pd.Output.IsNull() {
		var err error
		output, err = dynamic.ToJSON(pd.Output)
		if err != nil {
			resp.Diagnostics.AddError(
				"converting private output to json",
				err.Error(),
			)
			return
		}
	}
	opt, diags := e.p.apiOpt.ForOperation(ctx, pd.Method, pd.DefaultQuery, pd.DefaultHeader, pd.Query, pd.Header, output)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	response, err := c.Operation(ctx, pd.Path.ValueString(), output, *opt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error to call close operation",
			err.Error(),
		)
		return
	}
	if !response.IsSuccess() {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Close operation API returns %d", response.StatusCode()),
			string(response.Body()),
		)
		return
	}
}
