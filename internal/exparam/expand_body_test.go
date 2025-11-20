package exparam_test

import (
	"testing"

	"github.com/lfventura/terraform-provider-restful/internal/exparam"
	"github.com/stretchr/testify/require"
)

func TestExpand(t *testing.T) {
	cases := []struct {
		name    string
		expr    string
		body    string
		expect  string
		isError bool
	}{
		{
			name:   "$(body) is a string",
			expr:   "$(body)",
			body:   `"abc"`,
			expect: "abc",
		},
		{
			name:   "$(body) is a number",
			expr:   "$(body)",
			body:   "123",
			expect: "123",
		},
		{
			name:   "$(body) is an object",
			expr:   "$(body)",
			body:   `{"foo": 123}`,
			expect: `{"foo": 123}`,
		},
		{
			name:   "$(body.a) is a string",
			expr:   "$(body.a)",
			body:   `{"a": "abc"}`,
			expect: "abc",
		},
		{
			name:   "$(body.a) is a number",
			expr:   "$(body.a)",
			body:   `{"a": 123}`,
			expect: "123",
		},
		{
			name:   "$(body.a) is an object",
			expr:   "$(body.a)",
			body:   `{"a": {"foo": 123}}`,
			expect: `{"foo": 123}`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := exparam.ExpandBody(tt.expr, []byte(tt.body))
			if tt.isError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expect, actual)
		})
	}
}
