package planmodifier

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type DefaultAttributePlanModifier struct {
	Default attr.Value
}

func DefaultAttribute(value attr.Value) DefaultAttributePlanModifier {
	return DefaultAttributePlanModifier{Default: value}
}

func (m DefaultAttributePlanModifier) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	// if configuration was provided, then don't use the default
	if !req.ConfigValue.IsNull() {
		return
	}

	// If the plan is known and not null (for example due to another plan modifier),
	// don't set the default value
	if !req.PlanValue.IsUnknown() && !req.PlanValue.IsNull() {
		return
	}

	resp.PlanValue = m.Default.(types.Int64)
}

func (m DefaultAttributePlanModifier) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	// if configuration was provided, then don't use the default
	if !req.ConfigValue.IsNull() {
		return
	}

	// If the plan is known and not null (for example due to another plan modifier),
	// don't set the default value
	if !req.PlanValue.IsUnknown() && !req.PlanValue.IsNull() {
		return
	}

	resp.PlanValue = m.Default.(types.Bool)
}

func (m DefaultAttributePlanModifier) Description(ctx context.Context) string {
	return "Use a static default value for an attribute"
}

func (m DefaultAttributePlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}
