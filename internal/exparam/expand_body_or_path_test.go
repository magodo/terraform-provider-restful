package exparam

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExpandBodyOrPath(t *testing.T) {
	cases := []struct {
		name    string
		pattern string
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
			pattern: "$(path)/$(path)",
			path:    "a",
			expect:  "a/a",
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
			name:    "Body value contains special chars with explicit escpae",
			pattern: "$escape(body.name)",
			body:    `{"name": "a/b/c"}`,
			expect:  `a%2Fb%2Fc`,
		},
		{
			name:    "Body value contains escaped path, and want to unescape",
			pattern: "$unescape(body.path)",
			body:    `{"path": "a%2Fb%2Fc"}`,
			expect:  `a/b/c`,
		},
		{
			name:    "Body value has a full path, and want to trim the current call path",
			pattern: "$trim_path(body.path)",
			path:    "/a/b",
			body:    `{"path": "/a/b/c"}`,
			expect:  `c`,
		},
		{
			name:    "Body value has a full path, keep the base",
			pattern: "$base(body.path)",
			body:    `{"path": "/a/b/c"}`,
			expect:  `c`,
		},
		{
			name:    "Still trim path, but with ending slash",
			pattern: "$trim_path(body.path)",
			path:    "/a/b/",
			body:    `{"path": "/a/b/c"}`,
			expect:  `c`,
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
			name:    "Body returns URL, and wants to only keep the path segment",
			pattern: "$url_path(body.id)",
			body:    `{"id": "https://base/collections/abc"}`,
			expect:  "/collections/abc",
		},
		{
			name:    "Body returns URL, and wants to only keep the path segment, and then trim the call path",
			pattern: "$url_path.trim_path(body.id)",
			path:    "/foo",
			body:    `{"id": "https://base/foo/bar/abc"}`,
			expect:  "bar/abc",
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ExpandBodyOrPath(tt.pattern, tt.path, []byte(tt.body))
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}
			require.Equal(t, tt.expect, actual)
		})
	}
}
