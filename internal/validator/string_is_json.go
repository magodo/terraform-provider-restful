package validator

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type stringIsJSON struct{}

func (v stringIsJSON) Description(ctx context.Context) string {
	return "validate this in json format"
}

func (v stringIsJSON) MarkdownDescription(ctx context.Context) string {
	return "validate this in json format"
}

func (_ stringIsJSON) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	str := req.ConfigValue

	if str.IsUnknown() || str.IsNull() {
		return
	}

	var v interface{}
	if err := json.Unmarshal([]byte(str.ValueString()), &v); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid String",
			fmt.Sprintf("String can't be unmarshaled to json: %v", err),
		)
	}
}

func StringIsJSON() stringIsJSON {
	return stringIsJSON{}
}
