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
			name:       "Primary and null are jointed",
			lhs:        []byte("1"),
			rhs:        []byte("null"),
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
			name:       "Arrays of disjointed elements at the same index, are jointed",
			lhs:        []byte(`[{"a": 1}, 2]`),
			rhs:        []byte(`[{"b": 1}]`),
			disjointed: false,
		},
		{
			name:       "Arrays of no same indexed elements are jointed",
			lhs:        []byte("[]"),
			rhs:        []byte("[1]"),
			disjointed: false,
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

func TestDifference(t *testing.T) {
	cases := []struct {
		name   string
		lhs    []byte
		rhs    []byte
		result string
		err    bool
	}{
		{
			name: "Invalid json",
			lhs:  []byte("1"),
			rhs:  nil,
			err:  true,
		},
		{
			name:   "Not both maps: primary vs null",
			lhs:    []byte("1"),
			rhs:    []byte("null"),
			result: "1",
		},
		{
			name:   "Not both maps: null vs primary",
			lhs:    []byte("null"),
			rhs:    []byte("1"),
			result: "null",
		},
		{
			name:   "Not both maps: primary vs map",
			lhs:    []byte(`1`),
			rhs:    []byte(`{"a": 1}`),
			result: "1",
		},
		{
			name:   "Not both maps: map vs primary",
			lhs:    []byte(`{"a": 1}`),
			rhs:    []byte(`1`),
			result: `{"a": 1}`,
		},
		{
			name:   "Empty map",
			lhs:    []byte(`{"a": 1, "b": 2, "c": 3}`),
			rhs:    []byte(`{}`),
			result: `{"a": 1, "b": 2, "c": 3}`,
		},
		{
			name:   "Empty map (swap)",
			lhs:    []byte(`{}`),
			rhs:    []byte(`{"a": 1, "b": 2, "c": 3}`),
			result: `{}`,
		},
		{
			name:   "Simple map",
			lhs:    []byte(`{"a": 1, "b": 2, "c": 3}`),
			rhs:    []byte(`{"a": 2, "b": 3}`),
			result: `{"c": 3}`,
		},
		{
			name:   "Simple map (swap)",
			lhs:    []byte(`{"a": 2, "b": 3}`),
			rhs:    []byte(`{"a": 1, "b": 2, "c": 3}`),
			result: `{}`,
		},
		{
			name:   "Nested map",
			lhs:    []byte(`{"m": {"a": 1, "b": 2, "c": 3}, "x": 1, "y": 2, "z": 3}`),
			rhs:    []byte(`{"m": {"a": 2, "b": 3}, "x": 2, "y": 3}`),
			result: `{"m": { "c": 3}, "z": 3}`,
		},
		{
			name:   "Nested map swap",
			lhs:    []byte(`{"m": {"a": 2, "b": 3}, "x": 2, "y": 3}`),
			rhs:    []byte(`{"m": {"a": 1, "b": 2, "c": 3}, "x": 1, "y": 2, "z": 3}`),
			result: `{}`,
		},
		{
			name:   "Nested map with empty map",
			lhs:    []byte(`{"m": {}, "x": 1, "y": 2}`),
			rhs:    []byte(`{"m": {"a": 1, "b": 2, "c": 3}, "x": 2, "y": 3}`),
			result: `{"m": {}}`,
		},
		{
			name:   "More nested map 1",
			lhs:    []byte(`{"m": {"a": 2, "b": 3}, "x": 1, "y": 2, "z": 3}`),
			rhs:    []byte(`{"m": {"a": 1, "b": 2, "c": 3}, "x": 2, "y": 3}`),
			result: `{"z": 3}`,
		},
		{
			name:   "More nested map 2",
			lhs:    []byte(`{"m": {"a": 2, "b": 3, "c": {"x": 1, "y": 2, "z": 3}}}`),
			rhs:    []byte(`{"m": {"a": 1, "b": 2, "c": {"x": 2, "y": 3}}}`),
			result: `{"m": {"c": {"z": 3}}}`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := jsonset.Difference(tt.lhs, tt.rhs)
			if tt.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.JSONEq(t, tt.result, string(result))
		})
	}
}

func TestNullifyObject(t *testing.T) {
	cases := []struct {
		name   string
		input  []byte
		result string
		err    bool
	}{
		{
			name:  "null",
			input: nil,
			err:   true,
		},
		{
			name:   "JSON null",
			input:  []byte("null"),
			result: "null",
		},
		{
			name:   "Primary",
			input:  []byte("123"),
			result: "null",
		},
		{
			name:   "Array",
			input:  []byte("[1,2,3]"),
			result: "null",
		},
		{
			name:   "Simple map",
			input:  []byte(`{"a": 1, "b": 2, "c": 3}`),
			result: `{"a": null, "b": null, "c": null}`,
		},
		{
			name:   "Complex map",
			input:  []byte(`{"m": {"a": 1, "b": 2}, "array": [1,2,3], "p": 1}`),
			result: `{"m": {"a": null, "b": null}, "array": null, "p": null}`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := jsonset.NullifyObject(tt.input)
			if tt.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.JSONEq(t, tt.result, string(result))
		})
	}
}
