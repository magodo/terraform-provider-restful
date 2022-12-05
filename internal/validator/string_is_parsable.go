package validator

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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

func (v stringIsParsable) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	str := req.ConfigValue

	if str.IsUnknown() || str.IsNull() {
		return
	}

	if err := v.parse(str.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid String",
			fmt.Sprintf("String can't be parsed: %v", err),
		)
	}
}

func StringIsParsable(description string, parseFunc ParseFunc) stringIsParsable {
	return stringIsParsable{desc: description, parse: parseFunc}
}
