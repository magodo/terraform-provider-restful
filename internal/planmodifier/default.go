package planmodifier

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

type DefaultAttributePlanModifier struct {
	Default attr.Value
}

func DefaultAttribute(value attr.Value) DefaultAttributePlanModifier {
	return DefaultAttributePlanModifier{Default: value}
}

func (m DefaultAttributePlanModifier) Modify(ctx context.Context, req tfsdk.ModifyAttributePlanRequest, resp *tfsdk.ModifyAttributePlanResponse) {
	if resp.AttributePlan == nil || req.AttributeConfig == nil {
		return
	}

	val, err := req.AttributeConfig.ToTerraformValue(ctx)
	if err != nil {
		resp.Diagnostics.AddAttributeError(req.AttributePath,
			"Error converting config value",
			fmt.Sprintf("An unexpected error was encountered converting a %s to its equivalent Terraform representation. This is always a bug in the provilder.\n\nError: %s", req.AttributePlan.Type(ctx), err),
		)
		return
	}

	// if configuration was provided, then don't use the default
	if !val.IsNull() {
		return
	}

	val, err = resp.AttributePlan.ToTerraformValue(ctx)
	if err != nil {
		resp.Diagnostics.AddAttributeError(req.AttributePath,
			"Error converting plan value",
			fmt.Sprintf("An unexpected error was encountered converting a %s to its equivalent Terraform representation. This is always a bug in the provilder.\n\nError: %s", req.AttributePlan.Type(ctx), err),
		)
		return
	}

	// If the plan is known and not null (for example due to another plan modifier),
	// don't set the default value
	if val.IsKnown() && !val.IsNull() {
		return
	}

	resp.AttributePlan = m.Default
}

func (m DefaultAttributePlanModifier) Description(ctx context.Context) string {
	return "Use a static default value for an attribute"
}

func (m DefaultAttributePlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}
