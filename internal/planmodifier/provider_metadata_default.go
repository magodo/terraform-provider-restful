package planmodifier

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
)

type ProviderMetadataDefaultAttributePlanModifier struct {
	AttrPath path.Path
	Default  attr.Value
}

func ProviderMetadataDefaultAttribute(path path.Path, defaultValue attr.Value) ProviderMetadataDefaultAttributePlanModifier {
	return ProviderMetadataDefaultAttributePlanModifier{AttrPath: path, Default: defaultValue}
}

func (m ProviderMetadataDefaultAttributePlanModifier) Modify(ctx context.Context, req tfsdk.ModifyAttributePlanRequest, resp *tfsdk.ModifyAttributePlanResponse) {
	if resp.AttributePlan == nil || req.AttributeConfig == nil {
		return
	}

	// if configuration was provided, then don't use the default
	val, err := req.AttributeConfig.ToTerraformValue(ctx)
	if err != nil {
		resp.Diagnostics.AddAttributeError(req.AttributePath,
			"Error converting config value",
			fmt.Sprintf("An unexpected error was encountered converting a %s to its equivalent Terraform representation. This is always a bug in the provilder.\n\nError: %s", req.AttributePlan.Type(ctx), err),
		)
		return
	}
	if !val.IsNull() {
		return
	}

	// If the plan is known and not null (for example due to another plan modifier),
	// don't set the default value
	val, err = resp.AttributePlan.ToTerraformValue(ctx)
	if err != nil {
		resp.Diagnostics.AddAttributeError(req.AttributePath,
			"Error converting plan value",
			fmt.Sprintf("An unexpected error was encountered converting a %s to its equivalent Terraform representation. This is always a bug in the provilder.\n\nError: %s", req.AttributePlan.Type(ctx), err),
		)
		return
	}
	if val.IsKnown() && !val.IsNull() {
		return
	}

	var v attr.Value
	if diags := req.ProviderMeta.GetAttribute(ctx, m.AttrPath, &v); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	// In case this is not specified in the provider metadata, then use the provided default value in builder.
	if v.IsNull() {
		v = m.Default
	}

	resp.AttributePlan = v
}

func (m ProviderMetadataDefaultAttributePlanModifier) Description(ctx context.Context) string {
	return fmt.Sprintf("Use the default value from provider metadata for attribute %q", m.AttrPath.String())
}

func (m ProviderMetadataDefaultAttributePlanModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}
