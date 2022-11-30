package validator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type stringIsJSON struct{}

func (v stringIsJSON) Description(ctx context.Context) string {
	return "validate this in json format"
}

func (v stringIsJSON) MarkdownDescription(ctx context.Context) string {
	return "validate this in json format"
}

func (_ stringIsJSON) Validate(ctx context.Context, req tfsdk.ValidateAttributeRequest, resp *tfsdk.ValidateAttributeResponse) {
	var str types.String
	diags := tfsdk.ValueAs(ctx, req.AttributeConfig, &str)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if str.IsUnknown() || str.IsNull() {
		return
	}

	var v interface{}
	if err := json.Unmarshal([]byte(str.ValueString()), &v); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.AttributePath,
			"Invalid String",
			fmt.Sprintf("String can't be unmarshaled to json: %v", err),
		)
	}
}

func StringIsJSON() stringIsJSON {
	return stringIsJSON{}
}
