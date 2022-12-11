package buildpath

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
)

var (
	// IdPattern matches against a full URL, which is meant to be prefix trimed by the server's base URL.
	// This can be "#{body.x.y.z}".
	IdPattern = regexp.MustCompile(`\#(\w*)\(([\w.]+)\)`)

	// ValuePattern matches against a normal string. This can be either "${path}", or "${body.x.y.z}"
	ValuePattern = regexp.MustCompile(`\$(\w*)\(([\w.]+)\)`)
)

type PathFunc func(string) string

var PathFuncs = map[string]PathFunc{
	"urlencode": url.QueryEscape,
}

func BuildPath(pattern string, baseURL, path string, body []byte) (string, error) {
	out := pattern

	matches := ValuePattern.FindAllStringSubmatch(out, -1)
	for _, match := range matches {
		f := func(s string) string {
			return s
		}
		if fname := match[1]; fname != "" {
			f = PathFuncs[fname]
		}
		if match[2] == "path" {
			out = strings.ReplaceAll(out, match[0], f(path))
			continue
		}
		if strings.HasPrefix(match[2], "body.") {
			jsonPath := strings.TrimPrefix(match[2], "body.")
			prop := gjson.GetBytes(body, jsonPath)
			if !prop.Exists() {
				return "", fmt.Errorf("no property found at path %q in the body", jsonPath)
			}
			out = strings.ReplaceAll(out, match[0], f(prop.String()))
			continue
		}
		return "", fmt.Errorf("invalid match: %s", match[0])
	}

	matches = IdPattern.FindAllStringSubmatch(out, -1)
	for _, match := range matches {
		f := func(s string) string {
			return s
		}
		if fname := match[1]; fname != "" {
			f = PathFuncs[fname]
		}
		if strings.HasPrefix(match[2], "body.") {
			jsonPath := strings.TrimPrefix(match[2], "body.")
			prop := gjson.GetBytes(body, jsonPath)
			if !prop.Exists() {
				return "", fmt.Errorf("no property found at path %q in the body", jsonPath)
			}
			out = strings.ReplaceAll(out, match[0], f(strings.TrimPrefix(prop.String(), baseURL)))
			continue
		}
		return "", fmt.Errorf("invalid match: %s", match[0])
	}
	return out, nil
}
