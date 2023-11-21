package provider

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/magodo/terraform-provider-restful/internal/client"
)

type apiOption struct {
	BaseURL            url.URL
	CreateMethod       string
	UpdateMethod       string
	DeleteMethod       string
	MergePatchDisabled bool
	Query              client.Query
	Header             client.Header
}

func parseLocator(locator string) (client.ValueLocator, error) {
	if locator == "code" {
		return client.CodeLocator{}, nil
	}
	p := regexp.MustCompile(`^(\w+)\.(.+)$`)
	matches := p.FindAllStringSubmatch(locator, 1)
	if len(matches) != 1 {
		return nil, fmt.Errorf("invalid locator: %s", locator)
	}
	submatches := matches[0]
	k, v := submatches[1], submatches[2]
	switch k {
	case "exact":
		return client.ExactLocator(v), nil
	case "header":
		return client.HeaderLocator(v), nil
	case "body":
		return client.BodyLocator(v), nil
	default:
		return nil, fmt.Errorf("unknown locator key: %s", k)
	}
}

func (opt apiOption) ForResourceCreate(ctx context.Context, d resourceData) (*client.CreateOption, diag.Diagnostics) {
	out := client.CreateOption{
		Method: opt.CreateMethod,
		Query:  opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header: opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}
	if !d.CreateMethod.IsUnknown() && !d.CreateMethod.IsNull() {
		out.Method = d.CreateMethod.ValueString()
	}
	return &out, nil
}

func (opt apiOption) ForResourceRead(ctx context.Context, d resourceData) (*client.ReadOption, diag.Diagnostics) {
	out := client.ReadOption{
		Query:  opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header: opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}
	return &out, nil
}

func (opt apiOption) ForResourceUpdate(ctx context.Context, d resourceData) (*client.UpdateOption, diag.Diagnostics) {
	out := client.UpdateOption{
		Method:             opt.UpdateMethod,
		MergePatchDisabled: opt.MergePatchDisabled,
		Query:              opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header:             opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}
	if !d.UpdateMethod.IsUnknown() && !d.UpdateMethod.IsNull() {
		out.Method = d.UpdateMethod.ValueString()
	}
	if !d.MergePatchDisabled.IsUnknown() && !d.MergePatchDisabled.IsNull() {
		out.MergePatchDisabled = d.MergePatchDisabled.ValueBool()
	}

	return &out, nil
}

func (opt apiOption) ForResourceDelete(ctx context.Context, d resourceData) (*client.DeleteOption, diag.Diagnostics) {
	out := client.DeleteOption{
		Method: opt.DeleteMethod,
		Query:  opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header: opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}

	if !d.DeleteMethod.IsUnknown() && !d.DeleteMethod.IsNull() {
		out.Method = d.DeleteMethod.ValueString()
	}
	return &out, nil
}

func (opt apiOption) ForDataSourceRead(ctx context.Context, d dataSourceData) (*client.ReadOption, diag.Diagnostics) {
	out := client.ReadOption{
		Method: d.Method.ValueString(),
		Query:  opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header: opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}
	return &out, nil
}

func (opt apiOption) ForResourceOperation(ctx context.Context, d operationResourceData) (*client.OperationOption, diag.Diagnostics) {
	out := client.OperationOption{
		Method: d.Method.ValueString(),
		Query:  opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header: opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}
	return &out, nil
}

func (opt apiOption) ForPoll(ctx context.Context, defaultHeader client.Header, defaultQuery client.Query, d pollData) (*client.PollOption, diag.Diagnostics) {
	var diags diag.Diagnostics

	var status pollStatusGo
	if d := d.Status.As(ctx, &status, basetypes.ObjectAsOptions{}); d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	statusLocator, err := parseLocator(d.StatusLocator.ValueString())
	if err != nil {
		diags.AddError("Failed to parse status locator", err.Error())
		return nil, diags
	}

	var urlLocator client.ValueLocator
	if !d.UrlLocator.IsNull() {
		loc, err := parseLocator(d.UrlLocator.ValueString())
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

		DefaultDelay: time.Duration(d.DefaultDelay.ValueInt64()) * time.Second,
	}, nil
}

func (opt apiOption) ForPrecheck(ctx context.Context, defaultPath string, defaultHeader client.Header, defaultQuery client.Query, d precheckDataApi) (*client.PollOption, diag.Diagnostics) {
	var diags diag.Diagnostics

	var status pollStatusGo
	if d := d.Status.As(ctx, &status, basetypes.ObjectAsOptions{}); d.HasError() {
		diags.Append(d...)
		return nil, diags
	}

	statusLocator, err := parseLocator(d.StatusLocator.ValueString())
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

	uRL := opt.BaseURL
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

	return &client.PollOption{
		StatusLocator: statusLocator,
		Status: client.PollingStatus{
			Success: status.Success,
			Pending: status.Pending,
		},
		UrlLocator:   urlLocator,
		Header:       header,
		DefaultDelay: time.Duration(d.DefaultDelay.ValueInt64()) * time.Second,
	}, nil
}
