package validator

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Parsable interface {
	Format() string
	Parse(string) error
}

type stringIsParsable struct {
	parsable Parsable
}

func (v stringIsParsable) Description(ctx context.Context) string {
	return v.parsable.Format()
}

func (v stringIsParsable) MarkdownDescription(ctx context.Context) string {
	return v.parsable.Format()
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

	if err := v.parsable.Parse(str.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.AttributePath,
			"Invalid String",
			fmt.Sprintf("String can't be parsed: %v. Expected format: %s", err, v.parsable.Format()),
		)
	}
}

func StringIsParsable(parsable Parsable) stringIsParsable {
	return stringIsParsable{parsable: parsable}
}
