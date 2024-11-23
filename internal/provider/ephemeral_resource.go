package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/magodo/terraform-provider-restful/internal/dynamic"
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
	Path       types.String  `tfsdk:"path"`
	Method     types.String  `tfsdk:"method"`
	Body       types.Dynamic `tfsdk:"body"`
	OpenQuery  types.Map     `tfsdk:"open_query"`
	OpenHeader types.Map     `tfsdk:"open_header"`

	Query  types.Map `tfsdk:"query"`
	Header types.Map `tfsdk:"header"`

	RenewMethod types.String  `tfsdk:"renew_method"`
	RenewBody   types.Dynamic `tfsdk:"renew_body"`
	RenewPath   types.String  `tfsdk:"renew_path"`
	RenewQuery  types.Map     `tfsdk:"renew_query"`
	RenewHeader types.Map     `tfsdk:"renew_header"`

	ExpiryAhead   types.String `tfsdk:"expiry_ahead"`
	ExpiryType    types.String `tfsdk:"expiry_type"`
	ExpiryLocator types.String `tfsdk:"expiry_locator"`

	CloseMethod types.String  `tfsdk:"close_method"`
	CloseBody   types.Dynamic `tfsdk:"close_body"`
	ClosePath   types.String  `tfsdk:"close_path"`
	CloseQuery  types.Map     `tfsdk:"close_query"`
	CloseHeader types.Map     `tfsdk:"close_header"`

	OutputAttrs types.Set     `tfsdk:"output_attrs"`
	Output      types.Dynamic `tfsdk:"output"`
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

func (e *EphemeralResource) Schema(context.Context, ephemeral.SchemaRequest, *ephemeral.SchemaResponse) {
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

	opt, diags := e.p.apiOpt.ForOperation(ctx, config.Method, config.Query, config.Header, config.OpenQuery, config.OpenHeader)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	response, err := c.Operation(ctx, config.Path.ValueString(), config.Body, *opt)
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
		t, err := GetExpiryTime(config.ExpiryType.ValueString(), config.ExpiryLocator.ValueString(), config.ExpiryAhead.ValueString(), *response)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to parse expiry time",
				err.Error(),
			)
			return
		}
		resp.RenewAt = t
	}

	// Set Renew and Close, if any
	if !config.RenewMethod.IsNull() {
		ed := ephemeralResourcePrivateData{
			Method: config.RenewMethod,
			Path:   config.RenewPath,
			Body:   config.RenewBody,
			Header: config.RenewHeader,
			Query:  config.RenewQuery,
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
		ed := ephemeralResourcePrivateData{
			Method: config.CloseMethod,
			Path:   config.ClosePath,
			Body:   config.CloseBody,
			Header: config.CloseHeader,
			Query:  config.CloseQuery,
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
	config.Output = output

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

	var pd ephemeralResourcePrivateData
	if err := json.Unmarshal(b, &pd); err != nil {
		resp.Diagnostics.AddError(
			"Unmarshal private data",
			err.Error(),
		)
		return
	}

	opt, diags := e.p.apiOpt.ForOperation(ctx, pd.Method, pd.DefaultQuery, pd.DefaultHeader, pd.Query, pd.Header)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	response, err := c.Operation(ctx, pd.Path.ValueString(), pd.Body, *opt)
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

	t, err := GetExpiryTime(pd.ExpiryType.ValueString(), pd.ExpiryLocator.ValueString(), pd.ExpiryAhead.ValueString(), *response)
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

	var pd ephemeralResourcePrivateData
	if err := json.Unmarshal(b, &pd); err != nil {
		resp.Diagnostics.AddError(
			"Unmarshal private data",
			err.Error(),
		)
		return
	}

	opt, diags := e.p.apiOpt.ForOperation(ctx, pd.Method, pd.DefaultQuery, pd.DefaultHeader, pd.Query, pd.Header)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	response, err := c.Operation(ctx, pd.Path.ValueString(), pd.Body, *opt)
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
