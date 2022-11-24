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

type apiOption struct{}

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

func convertPollObject(ctx context.Context, obj types.Object) (*client.PollOption, diag.Diagnostics) {
	if obj.IsNull() {
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
	out := client.ReadOption{}

	if diags := d.Query.ElementsAs(ctx, &out.Header, false); diags.HasError() {
		return nil, diags
	}

	if diags := d.Query.ElementsAs(ctx, &out.Query, false); diags.HasError() {
		return nil, diags
	}

	return &out, nil
}

func (opt apiOption) ForResourceCreate(ctx context.Context, d resourceData) (*client.CreateOption, diag.Diagnostics) {
	out := client.CreateOption{
		Method: d.CreateMethod.ValueString(),
	}

	if diags := d.Query.ElementsAs(ctx, &out.Query, false); diags.HasError() {
		return nil, diags
	}

	if diags := d.Header.ElementsAs(ctx, &out.Header, false); diags.HasError() {
		return nil, diags
	}

	var diags diag.Diagnostics
	out.PollOpt, diags = convertPollObject(ctx, d.PollCreate)
	if diags.HasError() {
		return nil, diags
	}
	return &out, nil
}

func (opt apiOption) ForResourceRead(ctx context.Context, d resourceData) (*client.ReadOption, diag.Diagnostics) {
	out := client.ReadOption{}

	if diags := d.Query.ElementsAs(ctx, &out.Query, false); diags.HasError() {
		return nil, diags
	}

	if diags := d.Header.ElementsAs(ctx, &out.Header, false); diags.HasError() {
		return nil, diags
	}

	return &out, nil
}

func (opt apiOption) ForResourceUpdate(ctx context.Context, d resourceData) (*client.UpdateOption, diag.Diagnostics) {
	out := client.UpdateOption{
		Method:             d.UpdateMethod.ValueString(),
		MergePatchDisabled: d.MergePatchDisabled.ValueBool(),
	}

	if diags := d.Query.ElementsAs(ctx, &out.Query, false); diags.HasError() {
		return nil, diags
	}

	if diags := d.Header.ElementsAs(ctx, &out.Header, false); diags.HasError() {
		return nil, diags
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
		Method: d.DeleteMethod.ValueString(),
	}

	if diags := d.Query.ElementsAs(ctx, &out.Query, false); diags.HasError() {
		return nil, diags
	}

	if diags := d.Header.ElementsAs(ctx, &out.Header, false); diags.HasError() {
		return nil, diags
	}

	var diags diag.Diagnostics
	out.PollOpt, diags = convertPollObject(ctx, d.PollDelete)
	if diags.HasError() {
		return nil, diags
	}
	return &out, nil
}

func (opt apiOption) ForResourceOperation(ctx context.Context, d operationResourceData) (*client.OperationOption, diag.Diagnostics) {
	out := client.OperationOption{
		Method: d.Method.ValueString(),
	}

	if diags := d.Query.ElementsAs(ctx, &out.Header, false); diags.HasError() {
		return nil, diags
	}

	if diags := d.Query.ElementsAs(ctx, &out.Query, false); diags.HasError() {
		return nil, diags
	}

	var diags diag.Diagnostics
	out.PollOpt, diags = convertPollObject(ctx, d.Poll)
	if diags.HasError() {
		return nil, diags
	}
	return &out, nil
}
