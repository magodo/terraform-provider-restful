package exparam

import (
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

// ExpandBody expands params of "$(body[.x.y.z])" in the expression.
//
// The param can be prefixed by a chain of functions.
// The form is like: $f1.f2(body.x.y.z)
func ExpandBody(expr string, body []byte) (string, error) {
	if len(body) == 0 {
		return expr, nil
	}

	out := expr
	ff := FuncFactory{}.Build()
	matches := Pattern.FindAllStringSubmatch(out, -1)
	for _, match := range matches {
		var jp string
		if match[2] == "body" {
			jp = "@this"
		} else if strings.HasPrefix(match[2], "body.") {
			jp = strings.TrimPrefix(match[2], "body.")
		} else {
			return "", fmt.Errorf("invalid match: %s", match[0])
		}
		prop := gjson.GetBytes(body, jp)
		if !prop.Exists() {
			return "", fmt.Errorf("no property found at path %q in the body", jp)
		}
		ts := prop.String()

		// Apply functions if any
		fs := []Func{}
		if fnames := match[1]; fnames != "" {
			for _, fname := range strings.Split(fnames, ".") {
				f, ok := ff[FuncName(fname)]
				if !ok {
					return "", fmt.Errorf("unknonw function %q", fname)
				}
				fs = append(fs, f)
			}
		}

		for i, f := range fs {
			var err error
			ts, err = f(ts)
			if err != nil {
				return "", fmt.Errorf("failed to apply %d-th functions: %v", i, err)
			}
		}
		out = strings.ReplaceAll(out, match[0], ts)
	}
	return out, nil
}
