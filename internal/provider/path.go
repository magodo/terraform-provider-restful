package provider

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
)

var (
	// pId matches against a full URL, which is meant to be prefix trimed by the server's base URL.
	// This can be "#{body.x.y.z}".
	pId = regexp.MustCompile(`\#\(([\w.]+)\)`)

	// pValue matches against a normal string. This can be either "${path}", or "${body.x.y.z}"
	pValue = regexp.MustCompile(`\$\(([\w.]+)\)`)
)

func BuildPath(pattern string, baseURL, path string, body []byte) (string, error) {
	out := pattern

	matches := pValue.FindAllStringSubmatch(out, -1)
	for _, match := range matches {
		if match[1] == "path" {
			out = strings.ReplaceAll(out, match[0], path)
			continue
		}
		if strings.HasPrefix(match[1], "body.") {
			jsonPath := strings.TrimPrefix(match[1], "body.")
			prop := gjson.GetBytes(body, jsonPath)
			if !prop.Exists() {
				return "", fmt.Errorf("no property found at path %q in the body", jsonPath)
			}
			out = strings.ReplaceAll(out, match[0], prop.String())
			continue
		}
		return "", fmt.Errorf("invalid match: %s", match[0])
	}

	matches = pId.FindAllStringSubmatch(out, -1)
	for _, match := range matches {
		if strings.HasPrefix(match[1], "body.") {
			jsonPath := strings.TrimPrefix(match[1], "body.")
			prop := gjson.GetBytes(body, jsonPath)
			if !prop.Exists() {
				return "", fmt.Errorf("no property found at path %q in the body", jsonPath)
			}
			out = strings.ReplaceAll(out, match[0], strings.TrimPrefix(prop.String(), baseURL))
			continue
		}
		return "", fmt.Errorf("invalid match: %s", match[0])
	}
	return out, nil
}
