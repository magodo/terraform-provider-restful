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
	"github.com/magodo/terraform-provider-restful/internal/defaults"
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

func (opt apiOption) forRetry(ctx context.Context, retryObj basetypes.ObjectValue) (*client.RetryOption, diag.Diagnostics) {
	var retry retryData
	if diags := retryObj.As(ctx, &retry, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	var status statusDataGo
	if diags := retry.Status.As(ctx, &status, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}
	statusLocator, err := parseLocator(retry.StatusLocator.ValueString())
	if err != nil {
		return nil, diag.Diagnostics{diag.NewErrorDiagnostic("Failed to parse status locator", err.Error())}
	}

	count := defaults.RetryCount
	if !retry.Count.IsNull() && !retry.Count.IsUnknown() {
		count = int(retry.Count.ValueInt64())
	}

	waitTime := defaults.RetryWaitTime
	if !retry.WaitInSec.IsNull() && !retry.WaitInSec.IsUnknown() {
		waitTime = time.Duration(int(retry.WaitInSec.ValueInt64())) * time.Second
	}

	maxWaitTime := defaults.RetryMaxWaitTime
	if !retry.MaxWaitInSec.IsNull() && !retry.MaxWaitInSec.IsUnknown() {
		waitTime = time.Duration(int(retry.MaxWaitInSec.ValueInt64())) * time.Second
	}

	return &client.RetryOption{
		StatusLocator: statusLocator,
		Status: client.PollingStatus{
			Pending: status.Pending,
			Success: status.Success,
		},
		Count:       count,
		WaitTime:    waitTime,
		MaxWaitTime: maxWaitTime,
	}, nil
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

	if !d.RetryCreate.IsNull() && !d.RetryCreate.IsUnknown() {
		retryOpt, diags := opt.forRetry(ctx, d.RetryCreate)
		if diags.HasError() {
			return nil, diags
		}
		out.Retry = retryOpt
	}

	return &out, nil
}

func (opt apiOption) ForResourceRead(ctx context.Context, d resourceData) (*client.ReadOption, diag.Diagnostics) {
	out := client.ReadOption{
		Query:  opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header: opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}

	if !d.RetryRead.IsNull() && !d.RetryRead.IsUnknown() {
		retryOpt, diags := opt.forRetry(ctx, d.RetryRead)
		if diags.HasError() {
			return nil, diags
		}
		out.Retry = retryOpt
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

	if !d.RetryUpdate.IsNull() && !d.RetryUpdate.IsUnknown() {
		retryOpt, diags := opt.forRetry(ctx, d.RetryUpdate)
		if diags.HasError() {
			return nil, diags
		}
		out.Retry = retryOpt
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

	if !d.RetryDelete.IsNull() && !d.RetryDelete.IsUnknown() {
		retryOpt, diags := opt.forRetry(ctx, d.RetryDelete)
		if diags.HasError() {
			return nil, diags
		}
		out.Retry = retryOpt
	}

	return &out, nil
}

func (opt apiOption) ForDataSourceRead(ctx context.Context, d dataSourceData) (*client.ReadOptionDS, diag.Diagnostics) {
	out := client.ReadOptionDS{
		Method: d.Method.ValueString(),
		Query:  opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header: opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}

	if !d.Retry.IsNull() && !d.Retry.IsUnknown() {
		retryOpt, diags := opt.forRetry(ctx, d.Retry)
		if diags.HasError() {
			return nil, diags
		}
		out.Retry = retryOpt
	}
	return &out, nil
}

func (opt apiOption) ForResourceOperation(ctx context.Context, d operationResourceData) (*client.OperationOption, diag.Diagnostics) {
	out := client.OperationOption{
		Method: d.Method.ValueString(),
		Query:  opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header: opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}

	if !d.Retry.IsNull() && !d.Retry.IsUnknown() {
		retryOpt, diags := opt.forRetry(ctx, d.Retry)
		if diags.HasError() {
			return nil, diags
		}
		out.Retry = retryOpt
	}

	return &out, nil
}

func (opt apiOption) ForPoll(ctx context.Context, defaultHeader client.Header, defaultQuery client.Query, d pollData) (*client.PollOption, diag.Diagnostics) {
	var diags diag.Diagnostics

	var status statusDataGo
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

	var status statusDataGo
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
