package provider

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModifyJSON(t *testing.T) {
	cases := []struct {
		name           string
		base           string
		body           string
		writeOnlyAttrs []string
		expect         interface{}
	}{
		{
			name:   "invalid base",
			base:   "",
			expect: errors.New(`unmarshal the base "": unexpected end of JSON input`),
		},
		{
			name:   "invalid body",
			base:   "{}",
			body:   "",
			expect: errors.New(`unmarshal the body "": unexpected end of JSON input`),
		},
		{
			name:           "with write_only_attrs",
			base:           `{"obj":{"a":1,"b":2}, "z":2}`,
			body:           `{"obj":{"a":3,"d":"5"}, "new":4}`,
			writeOnlyAttrs: []string{"obj.b"},
			expect:         `{"obj":{"a":3,"b":2}}`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := ModifyBody(tt.base, tt.body, tt.writeOnlyAttrs)
			switch expect := tt.expect.(type) {
			case error:
				require.EqualError(t, err, expect.Error())
			default:
				require.NoError(t, err)
				require.Equal(t, expect, actual)
			}
		})
	}
}

func TestGetUpdatedJSON(t *testing.T) {
	cases := []struct {
		name    string
		oldJSON interface{}
		newJSON interface{}
		expect  interface{}
	}{
		{
			name:    "simple object",
			oldJSON: map[string]interface{}{"a": 1, "b": 2},
			newJSON: map[string]interface{}{"a": 3, "c": 4},
			expect:  map[string]interface{}{"a": 3},
		},
		{
			name:    "nested object",
			oldJSON: map[string]interface{}{"obj": map[string]interface{}{"a": 1, "b": 2, "c": 3}, "z": 2},
			newJSON: map[string]interface{}{"obj": map[string]interface{}{"a": 3, "b": 4, "d": 5}, "new": 4},
			expect:  map[string]interface{}{"obj": map[string]interface{}{"a": 3, "b": 4}},
		},
		{
			name:    "simple array",
			oldJSON: []interface{}{1, 2, 3},
			newJSON: []interface{}{3, 4, 5},
			expect:  []interface{}{3, 4, 5},
		},
		{
			name:    "simple array with different size",
			oldJSON: []interface{}{1, 2, 3},
			newJSON: []interface{}{3},
			expect:  []interface{}{3},
		},
		{
			name: "complex array",
			oldJSON: []interface{}{
				map[string]interface{}{
					"a": 1,
					"b": 2,
				},
				map[string]interface{}{
					"a": 1,
					"b": 2,
				},
			},
			newJSON: []interface{}{
				map[string]interface{}{
					"a": 1,
					"c": 3,
				},
				map[string]interface{}{
					"b": 2,
					"c": 3,
				},
			},
			expect: []interface{}{
				map[string]interface{}{
					"a": 1,
				},
				map[string]interface{}{
					"b": 2,
				},
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			actual := getUpdatedJSON(tt.oldJSON, tt.newJSON)
			require.Equal(t, tt.expect, actual)
		})
	}
}

func TestGetUpdatedJSONForImport(t *testing.T) {
	cases := []struct {
		name    string
		oldJSON interface{}
		newJSON interface{}
		expect  interface{}
	}{
		{
			name:    "nil",
			oldJSON: nil,
			newJSON: map[string]interface{}{"a": 1},
			expect:  map[string]interface{}{"a": 1},
		},
		{
			name:    "simple object",
			oldJSON: map[string]interface{}{"a": nil, "b": nil},
			newJSON: map[string]interface{}{"a": 3, "c": 4},
			expect:  map[string]interface{}{"a": 3},
		},
		{
			name:    "nested object",
			oldJSON: map[string]interface{}{"obj": map[string]interface{}{"a": nil, "b": nil, "c": nil}, "z": nil},
			newJSON: map[string]interface{}{"obj": map[string]interface{}{"a": 3, "b": 4, "d": 5}, "new": 4},
			expect:  map[string]interface{}{"obj": map[string]interface{}{"a": 3, "b": 4}},
		},
		{
			name:    "nested object with no child detail",
			oldJSON: map[string]interface{}{"obj": nil, "z": nil},
			newJSON: map[string]interface{}{"obj": map[string]interface{}{"a": 3, "b": 4, "d": 5}, "new": 4},
			expect:  map[string]interface{}{"obj": map[string]interface{}{"a": 3, "b": 4, "d": 5}},
		},
		{
			name:    "0 sized array is the same as nil",
			oldJSON: []interface{}{},
			newJSON: []interface{}{1, 2, 3},
			expect:  []interface{}{1, 2, 3},
		},
		{
			name:    "0 sized array is also the same as of a single nil element",
			oldJSON: []interface{}{nil},
			newJSON: []interface{}{1, 2, 3},
			expect:  []interface{}{1, 2, 3},
		},
		// TODO
		{
			name:    "more than one element in array",
			oldJSON: []interface{}{nil, nil},
			newJSON: []interface{}{1, 2, 3},
			expect:  errors.New("the length of array should be 1"),
		},
		{
			name: "complex array",
			oldJSON: []interface{}{
				map[string]interface{}{
					"a": 1,
					"b": 2,
				},
			},
			newJSON: []interface{}{
				map[string]interface{}{
					"a": 1,
					"c": 3,
				},
				map[string]interface{}{
					"b": 2,
					"c": 3,
				},
			},
			expect: []interface{}{
				map[string]interface{}{
					"a": 1,
				},
				map[string]interface{}{
					"b": 2,
				},
			},
		},
		{
			name: "object nesting complex array",
			oldJSON: map[string]interface{}{
				"prop": map[string]interface{}{
					"foos": []interface{}{
						map[string]interface{}{
							"a": nil,
						},
					},
				},
			},
			newJSON: map[string]interface{}{
				"prop": map[string]interface{}{
					"foos": []interface{}{
						map[string]interface{}{
							"a": 0,
							"b": 0,
						},
						map[string]interface{}{
							"a": 0,
							"b": 0,
						},
					},
					"bar": 0,
				},
			},
			expect: map[string]interface{}{
				"prop": map[string]interface{}{
					"foos": []interface{}{
						map[string]interface{}{
							"a": 0,
						},
						map[string]interface{}{
							"a": 0,
						},
					},
				},
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := getUpdatedJSONForImport(tt.oldJSON, tt.newJSON)
			switch expect := tt.expect.(type) {
			case error:
				require.EqualError(t, err, expect.Error())
			default:
				require.NoError(t, err)
				require.Equal(t, expect, actual)
			}
		})
	}
}
