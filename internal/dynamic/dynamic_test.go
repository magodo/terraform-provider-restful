package dynamic

import (
	"context"
	"math/big"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func TestToJSON(t *testing.T) {
	input := types.DynamicValue(
		types.ObjectValueMust(
			map[string]attr.Type{
				"bool":    types.BoolType,
				"string":  types.StringType,
				"int64":   types.Int64Type,
				"float64": types.Float64Type,
				"number":  types.NumberType,
				"list": types.ListType{
					ElemType: types.BoolType,
				},
				"set": types.SetType{
					ElemType: types.BoolType,
				},
				"tuple": types.TupleType{
					ElemTypes: []attr.Type{
						types.BoolType,
						types.StringType,
					},
				},
				"map": types.MapType{
					ElemType: types.BoolType,
				},
				"object": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"bool":   types.BoolType,
						"string": types.StringType,
					},
				},
			},
			map[string]attr.Value{
				"bool":    types.BoolValue(true),
				"string":  types.StringValue("a"),
				"int64":   types.Int64Value(123),
				"float64": types.Float64Value(1.23),
				"number":  types.NumberValue(big.NewFloat(1.23)),
				"list": types.ListValueMust(
					types.BoolType,
					[]attr.Value{
						types.BoolValue(true),
						types.BoolValue(false),
					},
				),
				"set": types.SetValueMust(
					types.BoolType,
					[]attr.Value{
						types.BoolValue(true),
						types.BoolValue(false),
					},
				),
				"tuple": types.TupleValueMust(
					[]attr.Type{
						types.BoolType,
						types.StringType,
					},
					[]attr.Value{
						types.BoolValue(true),
						types.StringValue("a"),
					},
				),
				"map": types.MapValueMust(
					types.BoolType,
					map[string]attr.Value{
						"a": types.BoolValue(true),
					},
				),
				"object": types.ObjectValueMust(
					map[string]attr.Type{
						"bool":   types.BoolType,
						"string": types.StringType,
					},
					map[string]attr.Value{
						"bool":   types.BoolValue(true),
						"string": types.StringValue("a"),
					},
				),
			},
		),
	)

	expect := `
{
	"bool": true,
	"string": "a",
	"int64": 123,
	"float64": 1.23,
	"number": 1.23,
	"list": [true, false],
	"set": [true, false],
	"tuple": [true, "a"],
	"map": {
		"a": true
	},
	"object": {
		"bool": true,
		"string": "a"
	}
}`

	b, err := ToJSON(input)
	require.NoError(t, err)
	require.JSONEq(t, expect, string(b))
}

func TestFromJSON(t *testing.T) {
	input := `
{
	"bool": true,
	"string": "a",
	"int64": 123,
	"float64": 1.23,
	"number": 1.23,
	"list": [true, false],
	"set": [true, false],
	"tuple": [true, "a"],
	"map": {
		"a": true
	},
	"object": {
		"bool": true,
		"string": "a"
	}
}`
	expect := types.DynamicValue(
		types.ObjectValueMust(
			map[string]attr.Type{
				"bool":    types.BoolType,
				"string":  types.StringType,
				"int64":   types.Int64Type,
				"float64": types.Float64Type,
				"number":  types.NumberType,
				"list": types.ListType{
					ElemType: types.BoolType,
				},
				"set": types.SetType{
					ElemType: types.BoolType,
				},
				"tuple": types.TupleType{
					ElemTypes: []attr.Type{
						types.BoolType,
						types.StringType,
					},
				},
				"map": types.MapType{
					ElemType: types.BoolType,
				},
				"object": types.ObjectType{
					AttrTypes: map[string]attr.Type{
						"bool":   types.BoolType,
						"string": types.StringType,
					},
				},
			},
			map[string]attr.Value{
				"bool":    types.BoolValue(true),
				"string":  types.StringValue("a"),
				"int64":   types.Int64Value(123),
				"float64": types.Float64Value(1.23),
				"number":  types.NumberValue(big.NewFloat(1.23)),
				"list": types.ListValueMust(
					types.BoolType,
					[]attr.Value{
						types.BoolValue(true),
						types.BoolValue(false),
					},
				),
				"set": types.SetValueMust(
					types.BoolType,
					[]attr.Value{
						types.BoolValue(true),
						types.BoolValue(false),
					},
				),
				"tuple": types.TupleValueMust(
					[]attr.Type{
						types.BoolType,
						types.StringType,
					},
					[]attr.Value{
						types.BoolValue(true),
						types.StringValue("a"),
					},
				),
				"map": types.MapValueMust(
					types.BoolType,
					map[string]attr.Value{
						"a": types.BoolValue(true),
					},
				),
				"object": types.ObjectValueMust(
					map[string]attr.Type{
						"bool":   types.BoolType,
						"string": types.StringType,
					},
					map[string]attr.Value{
						"bool":   types.BoolValue(true),
						"string": types.StringValue("a"),
					},
				),
			},
		),
	)

	actual, err := FromJSON([]byte(input), expect.UnderlyingValue().Type(context.TODO()))
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}
