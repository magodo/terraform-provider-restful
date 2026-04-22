package provider

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/magodo/terraform-plugin-framework-helper/dynamic"
	"github.com/magodo/terraform-provider-restful/internal/client"
	"github.com/magodo/terraform-provider-restful/internal/defaults"
)

type apiOption struct {
	BaseURL            string
	CreateMethod       string
	UpdateMethod       string
	DeleteMethod       string
	MergePatchDisabled bool
	Query              client.Query
	Header             client.Header
}

// baseURL resolves the BaseURL by the order of: resource level -- fallback --> provider level
func (opt apiOption) baseURL(baseUrl types.String) (string, diag.Diagnostics) {
	if baseUrl.ValueString() != "" {
		return baseUrl.ValueString(), nil
	}
	if opt.BaseURL != "" {
		return opt.BaseURL, nil
	}
	return "", diag.Diagnostics{
		diag.NewErrorDiagnostic(
			"`base_url` is not specified",
			"set it either at the provider level, or at the resource level",
		),
	}
}

func (opt apiOption) ForResourceCreate(ctx context.Context, d resourceData) (*client.CreateOption, diag.Diagnostics) {
	baseURL, diags := opt.baseURL(d.BaseURL)
	if diags.HasError() {
		return nil, diags
	}
	out := client.CreateOption{
		BaseURL: baseURL,
		Method:  opt.CreateMethod,
		Query:   opt.Query.Clone().TakeOrSelf(ctx, d.Query).TakeOrSelf(ctx, d.CreateQuery),
		Header:  opt.Header.Clone().TakeOrSelf(ctx, d.Header).TakeOrSelf(ctx, d.CreateHeader),
	}
	if !d.CreateMethod.IsUnknown() && !d.CreateMethod.IsNull() {
		out.Method = d.CreateMethod.ValueString()
	}

	return &out, nil
}

func (opt apiOption) ForResourceRead(ctx context.Context, d resourceData, body []byte) (*client.ReadOption, diag.Diagnostics) {
	baseURL, diags := opt.baseURL(d.BaseURL)
	if diags.HasError() {
		return nil, diags
	}
	out := client.ReadOption{
		BaseURL: baseURL,
		Query:   opt.Query.Clone().TakeOrSelf(ctx, d.Query).TakeWithExparamOrSelf(ctx, d.ReadQuery, body),
		Header:  opt.Header.Clone().TakeOrSelf(ctx, d.Header).TakeWithExparamOrSelf(ctx, d.ReadHeader, body),
	}

	return &out, nil
}

func (opt apiOption) ForResourcePostCreateRead(ctx context.Context, d resourceData, pr postCreateRead, body []byte) (*client.ReadOption, diag.Diagnostics) {
	baseURL, diags := opt.baseURL(d.BaseURL)
	if diags.HasError() {
		return nil, diags
	}
	out := client.ReadOption{
		BaseURL: baseURL,
		Query:   opt.Query.Clone().TakeOrSelf(ctx, d.Query).TakeWithExparamOrSelf(ctx, pr.Query, body),
		Header:  opt.Header.Clone().TakeOrSelf(ctx, d.Header).TakeWithExparamOrSelf(ctx, pr.Header, body),
	}

	return &out, nil
}

func (opt apiOption) ForResourceUpdate(ctx context.Context, d resourceData, body []byte) (*client.UpdateOption, diag.Diagnostics) {
	baseURL, diags := opt.baseURL(d.BaseURL)
	if diags.HasError() {
		return nil, diags
	}
	out := client.UpdateOption{
		BaseURL:            baseURL,
		Method:             opt.UpdateMethod,
		MergePatchDisabled: opt.MergePatchDisabled,
		Query:              opt.Query.Clone().TakeOrSelf(ctx, d.Query).TakeWithExparamOrSelf(ctx, d.UpdateQuery, body),
		Header:             opt.Header.Clone().TakeOrSelf(ctx, d.Header).TakeWithExparamOrSelf(ctx, d.UpdateHeader, body),
	}
	if !d.UpdateMethod.IsUnknown() && !d.UpdateMethod.IsNull() {
		out.Method = d.UpdateMethod.ValueString()
	}
	if !d.MergePatchDisabled.IsUnknown() && !d.MergePatchDisabled.IsNull() {
		out.MergePatchDisabled = d.MergePatchDisabled.ValueBool()
	}

	return &out, nil
}

func (opt apiOption) ForResourceDelete(ctx context.Context, d resourceData, body []byte) (*client.DeleteOption, diag.Diagnostics) {
	baseURL, diags := opt.baseURL(d.BaseURL)
	if diags.HasError() {
		return nil, diags
	}
	out := client.DeleteOption{
		BaseURL: baseURL,
		Method:  opt.DeleteMethod,
		Query:   opt.Query.Clone().TakeOrSelf(ctx, d.Query).TakeWithExparamOrSelf(ctx, d.DeleteQuery, body),
		Header:  opt.Header.Clone().TakeOrSelf(ctx, d.Header).TakeWithExparamOrSelf(ctx, d.DeleteHeader, body),
	}

	if !d.DeleteMethod.IsUnknown() && !d.DeleteMethod.IsNull() {
		out.Method = d.DeleteMethod.ValueString()
	}

	return &out, nil
}

func (opt apiOption) ForDataSourceRead(ctx context.Context, d dataSourceData) (*client.ReadOptionDS, diag.Diagnostics) {
	baseURL, diags := opt.baseURL(d.BaseURL)
	if diags.HasError() {
		return nil, diags
	}
	out := client.ReadOptionDS{
		BaseURL: baseURL,
		Method:  d.Method.ValueString(),
		Query:   opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header:  opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}

	return &out, nil
}

func (opt apiOption) ForOperation(ctx context.Context, baseURL types.String, method basetypes.StringValue, defQuery, defHeader, ovQuery, ovHeader basetypes.MapValue, body []byte) (*client.OperationOption, diag.Diagnostics) {
	uRL, diags := opt.baseURL(baseURL)
	if diags.HasError() {
		return nil, diags
	}
	out := client.OperationOption{
		BaseURL: uRL,
		Method:  method.ValueString(),
		Query:   opt.Query.Clone().TakeOrSelf(ctx, defQuery).TakeWithExparamOrSelf(ctx, ovQuery, body),
		Header:  opt.Header.Clone().TakeOrSelf(ctx, defHeader).TakeWithExparamOrSelf(ctx, ovHeader, body),
	}

	return &out, nil
}

func (opt apiOption) ForListResourceRead(ctx context.Context, d ListResourceData) (*client.ReadOptionLR, diag.Diagnostics) {
	baseURL, diags := opt.baseURL(d.BaseURL)
	if diags.HasError() {
		return nil, diags
	}
	out := client.ReadOptionLR{
		BaseURL: baseURL,
		Method:  d.Method.ValueString(),
		Query:   opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header:  opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}

	return &out, nil
}

func (opt apiOption) ForPoll(ctx context.Context, defaultHeader client.Header, defaultQuery client.Query, d pollData, body basetypes.DynamicValue) (*client.PollOption, diag.Diagnostics) {
	var diags diag.Diagnostics

	var status statusDataGo
	if d := d.Status.As(ctx, &status, basetypes.ObjectAsOptions{}); d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	bodyJSON, err := dynamic.ToJSON(body)
	if err != nil {
		diags.AddError("Failed to convert dynamic body to json", err.Error())
		return nil, diags
	}

	statusLocator, err := expandValueLocatorWithParam(d.StatusLocator.ValueString(), bodyJSON)
	if err != nil {
		diags.AddError("Failed to parse status locator", err.Error())
		return nil, diags
	}

	var urlLocator client.ValueLocator
	if !d.UrlLocator.IsNull() {
		loc, err := expandValueLocatorWithParam(d.UrlLocator.ValueString(), bodyJSON)
		if err != nil {
			diags.AddError("Failed to parse url locator", err.Error())
			return nil, diags
		}
		urlLocator = loc
	}

	header := defaultHeader
	if !d.Header.IsNull() {
		header = header.Clone().TakeOrSelf(ctx, d.Header)
	}

	defaultSec := defaults.PollDefaultDelayInSec
	if !d.DefaultDelay.IsNull() && !d.DefaultDelay.IsUnknown() {
		defaultSec = int(d.DefaultDelay.ValueInt64())
	}

	return &client.PollOption{
		StatusLocator: statusLocator,
		Status: client.PollingStatus{
			Success: status.Success,
			Pending: status.Pending,
		},
		UrlLocator: urlLocator,
		Header:     header,

		// The poll option always use the default query, which is typically is from the original request
		Query: defaultQuery,

		DefaultDelay: time.Duration(defaultSec) * time.Second,
	}, nil
}

func (opt apiOption) ForPrecheck(ctx context.Context, defaultBaseURL string, defaultPath string, defaultHeader client.Header, defaultQuery client.Query, d precheckDataApi, body basetypes.DynamicValue) (*client.PollOption, diag.Diagnostics) {
	var diags diag.Diagnostics

	var status statusDataGo
	if d := d.Status.As(ctx, &status, basetypes.ObjectAsOptions{}); d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	bodyJSON, err := dynamic.ToJSON(body)
	if err != nil {
		diags.AddError("Failed to convert dynamic body to json", err.Error())
		return nil, diags
	}

	statusLocator, err := expandValueLocatorWithParam(d.StatusLocator.ValueString(), bodyJSON)
	if err != nil {
		diags.AddError("Failed to parse status locator", err.Error())
		return nil, diags
	}

	header := defaultHeader
	if !d.Header.IsNull() {
		if d := d.Header.ElementsAs(ctx, &header, false); d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
	}

	baseURL := defaultBaseURL
	if !d.BaseURL.IsNull() {
		baseURL = d.BaseURL.ValueString()
	}
	uRL, err := url.Parse(baseURL)
	if err != nil {
		diags.Append(diag.NewErrorDiagnostic("failed to create precheck option", fmt.Sprintf("parsing base url %q: %v", baseURL, err)))
		return nil, diags
	}

	path := defaultPath
	if !d.Path.IsNull() {
		path = d.Path.ValueString()
	}
	uRL.Path, err = url.JoinPath(uRL.Path, path)
	if err != nil {
		diags.Append(diag.NewErrorDiagnostic("failed to create precheck option", fmt.Sprintf("joining url: %v", err)))
		return nil, diags
	}

	var query url.Values = url.Values(defaultQuery)
	if !d.Query.IsNull() {
		var q url.Values
		if d := d.Query.ElementsAs(ctx, &q, false); d.HasError() {
			diags.Append(d...)
			return nil, diags
		}
		query = q
	}
	uRL.RawQuery = query.Encode()
	urlLocator := client.ExactLocator(uRL.String())

	defaultSec := defaults.PrecheckDefaultDelayInSec
	if !d.DefaultDelay.IsNull() && !d.DefaultDelay.IsUnknown() {
		defaultSec = int(d.DefaultDelay.ValueInt64())
	}

	return &client.PollOption{
		StatusLocator: statusLocator,
		Status: client.PollingStatus{
			Success: status.Success,
			Pending: status.Pending,
		},
		UrlLocator:   urlLocator,
		Header:       header,
		DefaultDelay: time.Duration(defaultSec) * time.Second,
	}, nil
}
