package jsonset_test

import (
	"testing"

	"github.com/magodo/terraform-provider-restful/internal/jsonset"
	"github.com/stretchr/testify/require"
)

func TestDisjointed(t *testing.T) {
	cases := []struct {
		name       string
		lhs        string
		rhs        string
		disjointed bool
	}{
		{
			name:       "Primary vs null are disjointed",
			lhs:        "1",
			rhs:        "null",
			disjointed: true,
		},
		{
			name:       "Primary vs null are disjointed (swap)",
			lhs:        "null",
			rhs:        "1",
			disjointed: true,
		},
		{
			name:       "Same typed primaries are jointed",
			lhs:        "1",
			rhs:        "2",
			disjointed: false,
		},
		{
			name:       "Different typed primaries are jointed",
			lhs:        "1",
			rhs:        "true",
			disjointed: false,
		},
		{
			name:       "Object of common keys whose values are jointed, are jointed",
			lhs:        `{"a": 1, "b": 2}`,
			rhs:        `{"c": 1, "b": 2}`,
			disjointed: false,
		},
		{
			name:       "Object of common keys whose values are disjointed, are disjointed",
			lhs:        `{"a": 1, "b": {"x": 1}}`,
			rhs:        `{"c": 1, "b": {"y": 2}}`,
			disjointed: true,
		},
		{
			name:       "Object of no common keys are disjointed",
			lhs:        `{"a": 1, "b": 2}`,
			rhs:        `{"c": 1, "d": 2}`,
			disjointed: true,
		},
		{
			name:       "Arrays of jointed elements at the same index, are jointed",
			lhs:        "[1]",
			rhs:        "[2]",
			disjointed: false,
		},
		{
			name:       "Arrays of disjointed elements at the same index, are disjointed",
			lhs:        `[{"a": 1}, 2]`,
			rhs:        `[{"b": 1}]`,
			disjointed: true,
		},
		{
			name:       "Arrays of no same indexed elements are disjointed",
			lhs:        "[]",
			rhs:        "[1]",
			disjointed: true,
		},
		{
			name:       "Mixed object and array that are disjointed",
			lhs:        `{"array": [{"x": 1}]}`,
			rhs:        `{"array": [{"y": 1}]}`,
			disjointed: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			disjointed, err := jsonset.Disjointed([]byte(tt.lhs), []byte(tt.rhs))
			require.NoError(t, err)
			require.Equal(t, tt.disjointed, disjointed)
		})
	}
}
