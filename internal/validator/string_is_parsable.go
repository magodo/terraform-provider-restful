package validator

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ParseFunc func(string) error

type stringIsParsable struct {
	desc  string
	parse ParseFunc
}

func (v stringIsParsable) Description(ctx context.Context) string {
	return v.desc
}

func (v stringIsParsable) MarkdownDescription(ctx context.Context) string {
	return v.desc
}

func (v stringIsParsable) Validate(ctx context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
	var str types.String
	diags := tfsdk.ValueAs(ctx, req.AttributeConfig, &str)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if str.IsUnknown() || str.IsNull() {
		return
	}

	if err := v.parse(str.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.AttributePath,
			"Invalid String",
			fmt.Sprintf("String can't be parsed: %v", err),
		)
	}
}

func StringIsParsable(description string, parseFunc ParseFunc) stringIsParsable {
	return stringIsParsable{desc: description, parse: parseFunc}
}
