package buildpath

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/tidwall/gjson"
)

func BuildQuery(pattern string, body []byte) (string, error) {
	out := pattern
	ff := FuncFactory{}.Build()

	// Matches "$(body.x.y.z)"
	//
	// Especially, when pattern matches to "body", the pattern can prefix with a chain of functions.
	// The form is like: $f1.f2(body.x.y.z)
	// By defaults, the "escape" is applied. Otherwise, if explicitly defined a function,
	// the "escape" won't be applied automatically, and need manually define if needed.
	matches := ValuePattern.FindAllStringSubmatch(out, -1)
	for _, match := range matches {
		var ts string
		if match[2] == "body" {
			if err := json.Unmarshal(body, &ts); err != nil {
				return "", fmt.Errorf(`"body" expects type of string, but failed to unmarshal as a string: %v`, err)
			}
		} else if strings.HasPrefix(match[2], "body.") {
			jsonPath := strings.TrimPrefix(match[2], "body.")
			prop := gjson.GetBytes(body, jsonPath)
			if !prop.Exists() {
				return "", fmt.Errorf("no property found at path %q in the body", jsonPath)
			}
			ts = prop.String()
		} else {
			return "", fmt.Errorf("invalid match: %s", match[0])
		}

		// Apply path functions if any
		fs := []Func{ff[FuncEscape]}
		if fnames := match[1]; fnames != "" {
			// If specified any function, remove the default escape function
			fs = []Func{}
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
				return "", fmt.Errorf("failed to apply %d-th path functions: %v", i, err)
			}
		}
		out = strings.ReplaceAll(out, match[0], ts)
	}
	return out, nil
}
