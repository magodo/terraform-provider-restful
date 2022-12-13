package buildpath

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildPath(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
		baseURL string
		path    string
		body    string
		expect  string
		err     string
	}{
		{
			name:    "Path",
			pattern: "$(path)/abc",
			path:    "collections",
			expect:  "collections/abc",
		},
		{
			name:    "Multiple paths",
			pattern: "$(path)/abc",
			path:    "collections",
			expect:  "collections/abc",
		},
		{
			name:    "Path and body value",
			pattern: "$(path)/$(body.name)",
			path:    "collections",
			body:    `{"name": "abc"}`,
			expect:  "collections/abc",
		},
		{
			name:    "Body value contains special chars",
			pattern: "$(body.name)",
			body:    `{"name": "a/b/c"}`,
			expect:  `a%2Fb%2Fc`,
		},
		{
			name:    "Body value contains special chars, but explictly plain",
			pattern: "$plain(body.name)",
			body:    `{"name": "a/b/c"}`,
			expect:  `a/b/c`,
		},
		{
			name:    "Body doesn't contain the expected property",
			pattern: "$(body.prop)",
			body:    `{"name": "abc"}`,
			err:     `no property found at path "prop" in the body`,
		},
		{
			name:    "Unknown match",
			pattern: "$(foo)",
			err:     `invalid match`,
		},
		{
			name:    "Body id",
			pattern: "#(body.id)",
			body:    `{"id": "https://base/collections/abc"}`,
			baseURL: "https://base/",
			expect:  "collections/abc",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := BuildPath(tt.pattern, tt.baseURL, tt.path, []byte(tt.body))
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.Equal(t, tt.expect, actual)
		})
	}
}
