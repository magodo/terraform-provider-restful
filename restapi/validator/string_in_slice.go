package validator

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type stringInSliceValidator struct {
	valids []string
}

func (v stringInSliceValidator) Description(ctx context.Context) string {
	return fmt.Sprintf("string must be one of [%s]", strings.Join(v.valids, ","))
}

func (v stringInSliceValidator) MarkdownDescription(ctx context.Context) string {
	return fmt.Sprintf("string must be one of [%s]", strings.Join(v.valids, ","))
}

func (v stringInSliceValidator) Validate(ctx context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
	var str types.String
	diags := tfsdk.ValueAs(ctx, req.AttributeConfig, &str)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if str.Unknown || str.Null {
		return
	}

	for _, valid := range v.valids {
		if str.Value == valid {
			return
		}
	}
	resp.Diagnostics.AddAttributeError(
		req.AttributePath,
		"Invalid String",
		fmt.Sprintf("String must be one of [%s], got: %s.", strings.Join(v.valids, ","), str.Value),
	)
}

func StringInSlice(valids ...string) stringInSliceValidator {
	return stringInSliceValidator{valids: valids}
}
