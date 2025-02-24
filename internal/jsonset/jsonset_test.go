package jsonset_test

import (
	"testing"

	"github.com/magodo/terraform-provider-restful/internal/jsonset"
	"github.com/stretchr/testify/require"
)

func TestDisjointed(t *testing.T) {
	cases := []struct {
		name       string
		lhs        []byte
		rhs        []byte
		disjointed bool
		err        bool
	}{
		{
			name:       "Invalid json",
			lhs:        []byte("1"),
			rhs:        nil,
			disjointed: false,
			err:        true,
		},
		{
			name:       "Primary vs null are disjointed",
			lhs:        []byte("1"),
			rhs:        []byte("null"),
			disjointed: true,
		},
		{
			name:       "Primary vs null are disjointed (swap)",
			lhs:        []byte("null"),
			rhs:        []byte("1"),
			disjointed: true,
		},
		{
			name:       "Same typed primaries are jointed",
			lhs:        []byte("1"),
			rhs:        []byte("2"),
			disjointed: false,
		},
		{
			name:       "Different typed primaries are jointed",
			lhs:        []byte("1"),
			rhs:        []byte("true"),
			disjointed: false,
		},
		{
			name:       "Object of common keys whose values are jointed, are jointed",
			lhs:        []byte(`{"a": 1, "b": 2}`),
			rhs:        []byte(`{"c": 1, "b": 2}`),
			disjointed: false,
		},
		{
			name:       "Object of common keys whose values are disjointed, are disjointed",
			lhs:        []byte(`{"a": 1, "b": {"x": 1}}`),
			rhs:        []byte(`{"c": 1, "b": {"y": 2}}`),
			disjointed: true,
		},
		{
			name:       "Object of no common keys are disjointed",
			lhs:        []byte(`{"a": 1, "b": 2}`),
			rhs:        []byte(`{"c": 1, "d": 2}`),
			disjointed: true,
		},
		{
			name:       "Arrays of jointed elements at the same index, are jointed",
			lhs:        []byte("[1]"),
			rhs:        []byte("[2]"),
			disjointed: false,
		},
		{
			name:       "Arrays of disjointed elements at the same index, are disjointed",
			lhs:        []byte(`[{"a": 1}, 2]`),
			rhs:        []byte(`[{"b": 1}]`),
			disjointed: true,
		},
		{
			name:       "Arrays of no same indexed elements are disjointed",
			lhs:        []byte("[]"),
			rhs:        []byte("[1]"),
			disjointed: true,
		},
		{
			name:       "Mixed object and array that are disjointed",
			lhs:        []byte(`{"array": [{"x": 1}]}`),
			rhs:        []byte(`{"array": [{"y": 1}]}`),
			disjointed: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			disjointed, err := jsonset.Disjointed(tt.lhs, tt.rhs)
			if tt.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.disjointed, disjointed)
		})
	}
}
