package validator

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/magodo/terraform-provider-restful/internal/buildpath"
)

type stringsIsPathBuilder struct{}

func (v stringsIsPathBuilder) Description(ctx context.Context) string {
	return "validate this is a path builder expression"
}

func (v stringsIsPathBuilder) MarkdownDescription(ctx context.Context) string {
	return "validate this is a path builder expression"
}

func (_ stringsIsPathBuilder) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	str := req.ConfigValue

	if str.IsUnknown() || str.IsNull() {
		return
	}

	pathFuncs := buildpath.PathFuncFactory{}.Build()
	check := func(matches [][]string) diag.Diagnostic {
		for _, match := range matches {
			fnames, value := match[1], match[2]
			for _, fname := range strings.Split(fnames, ".") {
				if fname != "" {
					if _, ok := pathFuncs[buildpath.FuncName(fname)]; !ok {
						return diag.NewAttributeErrorDiagnostic(
							req.Path,
							"Invalid String",
							fmt.Sprintf("unknown function: %s", fname),
						)
					}
					if !strings.HasPrefix(value, "body.") {
						return diag.NewAttributeErrorDiagnostic(
							req.Path,
							"Invalid String",
							fmt.Sprintf("value isn't a body reference"),
						)
					}
				}
			}
		}
		return nil
	}

	resp.Diagnostics.Append(check(buildpath.ValuePattern.FindAllStringSubmatch(str.ValueString(), -1)))
	if resp.Diagnostics.HasError() {
		return
	}
}

func StringIsPathBuilder() stringsIsPathBuilder {
	return stringsIsPathBuilder{}
}
