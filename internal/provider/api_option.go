package provider

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

func newPollOptionFromPrecheckObject(ctx context.Context, base url.URL, id string, obj types.Object) (*client.PollOption, diag.Diagnostics) {
	if obj.IsNull() {
		return nil, nil
	}

	// We are converting the precheckDataGo to pollDataGo here to reuse the newPollOptionFromPollDataGo().
	var pck precheckDataGo
	diags := obj.As(ctx, &pck, types.ObjectAsOptions{})
	if diags != nil {
		return nil, diags
	}

	pd := pollDataGo{
		StatusLocator: pck.StatusLocator,
		Status:        pck.Status,
		DefaultDelay:  pck.DefaultDelay,
	}

	uRL := base

	path := id
	if pck.Path != nil {
		path = *pck.Path
	}

	var err error
	uRL.Path, err = url.JoinPath(uRL.Path, path)
	if err != nil {
		diags.Append(diag.NewErrorDiagnostic("failed to convert precheck object", fmt.Sprintf("joining url: %v", err)))
		return nil, diags
	}

	if !pck.Query.IsNull() {
		var q url.Values
		pck.Query.ElementsAs(ctx, &q, false)
		uRL.RawQuery = q.Encode()
	}

	urlLocator := "exact." + uRL.String()
	pd.UrlLocator = &urlLocator

	return newPollOptionFromPollDataGo(pd)
}

func newPollOptionFromPollObject(ctx context.Context, obj types.Object) (*client.PollOption, diag.Diagnostics) {
	if obj.IsNull() {
		return nil, nil
	}

	var pd pollDataGo
	diags := obj.As(ctx, &pd, types.ObjectAsOptions{})
	if diags != nil {
		return nil, diags
	}

	return newPollOptionFromPollDataGo(pd)
}

func newPollOptionFromPollDataGo(pd pollDataGo) (*client.PollOption, diag.Diagnostics) {
	popt := &client.PollOption{}
	var diags diag.Diagnostics
	loc, err := parseLocator(pd.StatusLocator)
	if err != nil {
		diags.AddError(
			"Failed to parse status locator",
			err.Error(),
		)
		return nil, diags
	}
	popt.StatusLocator = loc

	popt.Status = client.PollingStatus{
		Success: pd.Status.Success,
		Pending: pd.Status.Pending,
	}

	if pd.UrlLocator != nil {
		loc, err := parseLocator(*pd.UrlLocator)
		if err != nil {
			diags.AddError(
				"Failed to parse url locator",
				err.Error(),
			)
			return nil, diags
		}
		popt.UrlLocator = loc
	}

	if pd.DefaultDelay != nil {
		popt.DefaultDelay = time.Duration(*pd.DefaultDelay) * time.Second
	}

	return popt, nil
}

func (opt apiOption) ForResourceCreate(ctx context.Context, d resourceData) (*client.CreateOption, *client.PollOption, *client.PollOption, diag.Diagnostics) {
	out := client.CreateOption{
		Method: opt.CreateMethod,
		Query:  opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header: opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}
	if !d.CreateMethod.IsUnknown() && !d.CreateMethod.IsNull() {
		out.Method = d.CreateMethod.ValueString()
	}
	precheckOpt, diags := newPollOptionFromPrecheckObject(ctx, opt.BaseURL, d.ID.String(), d.PrecheckCreate)
	if diags.HasError() {
		return nil, nil, nil, diags
	}
	pollOpt, diags := newPollOptionFromPollObject(ctx, d.PollCreate)
	if diags.HasError() {
		return nil, nil, nil, diags
	}
	return &out, precheckOpt, pollOpt, nil
}

func (opt apiOption) ForResourceRead(ctx context.Context, d resourceData) (*client.ReadOption, *client.PollOption, diag.Diagnostics) {
	out := client.ReadOption{
		Query:  opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header: opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}
	precheckOpt, diags := newPollOptionFromPrecheckObject(ctx, opt.BaseURL, d.ID.String(), d.PrecheckRead)
	if diags.HasError() {
		return nil, nil, diags
	}
	return &out, precheckOpt, nil
}

func (opt apiOption) ForResourceUpdate(ctx context.Context, d resourceData) (*client.UpdateOption, *client.PollOption, *client.PollOption, diag.Diagnostics) {
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
	precheckOpt, diags := newPollOptionFromPrecheckObject(ctx, opt.BaseURL, d.ID.String(), d.PrecheckUpdate)
	if diags.HasError() {
		return nil, nil, nil, diags
	}
	pollOpt, diags := newPollOptionFromPollObject(ctx, d.PollUpdate)
	if diags.HasError() {
		return nil, nil, nil, diags
	}

	return &out, precheckOpt, pollOpt, nil
}

func (opt apiOption) ForResourceDelete(ctx context.Context, d resourceData) (*client.DeleteOption, *client.PollOption, *client.PollOption, diag.Diagnostics) {
	out := client.DeleteOption{
		Method: opt.DeleteMethod,
		Query:  opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header: opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}

	if !d.DeleteMethod.IsUnknown() && !d.DeleteMethod.IsNull() {
		out.Method = d.DeleteMethod.ValueString()
	}
	precheckOpt, diags := newPollOptionFromPrecheckObject(ctx, opt.BaseURL, d.ID.String(), d.PrecheckDelete)
	if diags.HasError() {
		return nil, nil, nil, diags
	}
	pollOpt, diags := newPollOptionFromPollObject(ctx, d.PollDelete)
	if diags.HasError() {
		return nil, nil, nil, diags
	}
	return &out, precheckOpt, pollOpt, nil
}

func (opt apiOption) ForDataSourceRead(ctx context.Context, d dataSourceData) (*client.ReadOption, *client.PollOption, diag.Diagnostics) {
	out := client.ReadOption{
		Query:  opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header: opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}
	precheckOpt, diags := newPollOptionFromPrecheckObject(ctx, opt.BaseURL, d.ID.String(), d.Precheck)
	if diags.HasError() {
		return nil, nil, diags
	}
	return &out, precheckOpt, nil
}

func (opt apiOption) ForResourceOperation(ctx context.Context, d operationResourceData) (*client.OperationOption, *client.PollOption, *client.PollOption, diag.Diagnostics) {
	out := client.OperationOption{
		Method: d.Method.ValueString(),
		Query:  opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header: opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}
	precheckOpt, diags := newPollOptionFromPrecheckObject(ctx, opt.BaseURL, d.Path.String(), d.Precheck)
	if diags.HasError() {
		return nil, nil, nil, diags
	}
	pollOpt, diags := newPollOptionFromPollObject(ctx, d.Poll)
	if diags.HasError() {
		return nil, nil, nil, diags
	}
	return &out, precheckOpt, pollOpt, nil
}
