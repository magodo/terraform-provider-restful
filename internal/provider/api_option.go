package provider

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/magodo/terraform-provider-restful/internal/client"
)

type apiOption struct {
	CreateMethod       string
	UpdateMethod       string
	MergePatchDisabled bool
	Query              client.Query
	Header             client.Header
}

func parseLocator(locator string) (client.ValueLocator, error) {
	if locator == "code" {
		return client.CodeLocator{}, nil
	}
	p := regexp.MustCompile(`^(.+)\[(.+)\]$`)
	matches := p.FindAllStringSubmatch(locator, 1)
	if len(matches) != 1 {
		return nil, fmt.Errorf("invalid locator: %s", locator)
	}
	submatches := matches[0]
	scope, path := submatches[1], submatches[2]
	switch scope {
	case "header":
		return client.HeaderLocator(path), nil
	case "body":
		return client.BodyLocator(path), nil
	default:
		return nil, fmt.Errorf("unknown locator scope: %s", scope)
	}
}

func convertPollObject(ctx context.Context, obj types.Object) (*client.PollOption, diag.Diagnostics) {
	if obj.Null {
		return nil, nil
	}
	popt := &client.PollOption{}

	var pd pollDataGo
	diags := obj.As(ctx, &pd, types.ObjectAsOptions{})
	if diags != nil {
		return nil, diags
	}

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

func (opt apiOption) ForDataSourceRead(ctx context.Context, d dataSourceData) (*client.ReadOption, diag.Diagnostics) {
	out := client.ReadOption{
		Query:  opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header: opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}
	return &out, nil
}

func (opt apiOption) ForResourceCreate(ctx context.Context, d resourceData) (*client.CreateOption, diag.Diagnostics) {
	out := client.CreateOption{
		CreateMethod: opt.CreateMethod,
		Query:        opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header:       opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}
	if !d.CreateMethod.Unknown && !d.CreateMethod.Null {
		out.CreateMethod = d.CreateMethod.Value
	}
	var diags diag.Diagnostics
	out.PollOpt, diags = convertPollObject(ctx, d.PollCreate)
	if diags.HasError() {
		return nil, diags
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
		UpdateMethod:       opt.UpdateMethod,
		MergePatchDisabled: opt.MergePatchDisabled,
		Query:              opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header:             opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}
	if !d.UpdateMethod.Unknown && !d.UpdateMethod.Null {
		out.UpdateMethod = d.UpdateMethod.Value
	}
	if !d.MergePatchDisabled.Unknown && !d.MergePatchDisabled.Null {
		out.MergePatchDisabled = d.MergePatchDisabled.Value
	}

	var diags diag.Diagnostics
	out.PollOpt, diags = convertPollObject(ctx, d.PollUpdate)
	if diags.HasError() {
		return nil, diags
	}
	return &out, nil
}

func (opt apiOption) ForResourceDelete(ctx context.Context, d resourceData) (*client.DeleteOption, diag.Diagnostics) {
	out := client.DeleteOption{
		Query:  opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header: opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}
	var diags diag.Diagnostics
	out.PollOpt, diags = convertPollObject(ctx, d.PollDelete)
	if diags.HasError() {
		return nil, diags
	}
	return &out, nil
}

func (opt apiOption) ForResourceAction(ctx context.Context, d actionResourceData) (*client.ActionOption, diag.Diagnostics) {
	out := client.ActionOption{
		Method: d.Method.Value,
		Query:  opt.Query.Clone().TakeOrSelf(ctx, d.Query),
		Header: opt.Header.Clone().TakeOrSelf(ctx, d.Header),
	}
	var diags diag.Diagnostics
	out.PollOpt, diags = convertPollObject(ctx, d.Poll)
	if diags.HasError() {
		return nil, diags
	}
	return &out, nil
}
