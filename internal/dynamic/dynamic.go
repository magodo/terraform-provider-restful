package dynamic

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func ToJSON(d types.Dynamic) ([]byte, error) {
	return attrValueToJSON(d.UnderlyingValue())
}

func attrListToJSON(in []attr.Value) ([]json.RawMessage, error) {
	var l []json.RawMessage
	for _, v := range in {
		vv, err := attrValueToJSON(v)
		if err != nil {
			return nil, err
		}
		l = append(l, json.RawMessage(vv))
	}
	return l, nil
}

func attrMapToJSON(in map[string]attr.Value) (map[string]json.RawMessage, error) {
	m := map[string]json.RawMessage{}
	for k, v := range in {
		vv, err := attrValueToJSON(v)
		if err != nil {
			return nil, err
		}
		m[k] = json.RawMessage(vv)
	}
	return m, nil
}

func attrValueToJSON(val attr.Value) ([]byte, error) {
	switch value := val.(type) {
	case types.Bool:
		return json.Marshal(value.ValueBool())
	case types.String:
		return json.Marshal(value.ValueString())
	case types.Int64:
		return json.Marshal(value.ValueInt64())
	case types.Float64:
		return json.Marshal(value.ValueFloat64())
	case types.Number:
		v, _ := value.ValueBigFloat().Float64()
		return json.Marshal(v)
	case types.List:
		l, err := attrListToJSON(value.Elements())
		if err != nil {
			return nil, err
		}
		return json.Marshal(l)
	case types.Set:
		l, err := attrListToJSON(value.Elements())
		if err != nil {
			return nil, err
		}
		return json.Marshal(l)
	case types.Tuple:
		l, err := attrListToJSON(value.Elements())
		if err != nil {
			return nil, err
		}
		return json.Marshal(l)
	case types.Map:
		m, err := attrMapToJSON(value.Elements())
		if err != nil {
			return nil, err
		}
		return json.Marshal(m)
	case types.Object:
		m, err := attrMapToJSON(value.Attributes())
		if err != nil {
			return nil, err
		}
		return json.Marshal(m)
	default:
		return nil, fmt.Errorf("Unhandled type: %T", value)
	}
}

func FromJSON(b []byte, typ attr.Type) (types.Dynamic, error) {
	v, err := attrValueFromJSON(b, typ)
	if err != nil {
		return types.Dynamic{}, err
	}
	return types.DynamicValue(v), nil
}

func attrListFromJSON(b []byte, etyp attr.Type) ([]attr.Value, error) {
	var l []json.RawMessage
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, err
	}
	var vals []attr.Value
	for _, b := range l {
		val, err := attrValueFromJSON(b, etyp)
		if err != nil {
			return nil, err
		}
		vals = append(vals, val)
	}
	return vals, nil
}

func attrValueFromJSON(b []byte, typ attr.Type) (attr.Value, error) {
	switch typ := typ.(type) {
	case basetypes.BoolType:
		var v bool
		if err := json.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		return types.BoolValue(v), nil
	case basetypes.StringType:
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		return types.StringValue(v), nil
	case basetypes.Int64Type:
		var v int64
		if err := json.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		return types.Int64Value(v), nil
	case basetypes.Float64Type:
		var v float64
		if err := json.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		return types.Float64Value(v), nil
	case basetypes.NumberType:
		var v float64
		if err := json.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		return types.NumberValue(big.NewFloat(v)), nil
	case basetypes.ListType:
		vals, err := attrListFromJSON(b, typ.ElemType)
		if err != nil {
			return nil, err
		}
		vv, diags := types.ListValue(typ.ElemType, vals)
		if diags.HasError() {
			diag := diags.Errors()[0]
			return nil, fmt.Errorf("%s: %s", diag.Summary(), diag.Detail())
		}
		return vv, nil
	case basetypes.SetType:
		vals, err := attrListFromJSON(b, typ.ElemType)
		if err != nil {
			return nil, err
		}
		vv, diags := types.SetValue(typ.ElemType, vals)
		if diags.HasError() {
			diag := diags.Errors()[0]
			return nil, fmt.Errorf("%s: %s", diag.Summary(), diag.Detail())
		}
		return vv, nil
	case basetypes.TupleType:
		var l []json.RawMessage
		if err := json.Unmarshal(b, &l); err != nil {
			return nil, err
		}
		if len(l) != len(typ.ElemTypes) {
			return nil, fmt.Errorf("tuple element size not match")
		}
		var vals []attr.Value
		for i, b := range l {
			val, err := attrValueFromJSON(b, typ.ElemTypes[i])
			if err != nil {
				return nil, err
			}
			vals = append(vals, val)
		}
		vv, diags := types.TupleValue(typ.ElemTypes, vals)
		if diags.HasError() {
			diag := diags.Errors()[0]
			return nil, fmt.Errorf("%s: %s", diag.Summary(), diag.Detail())
		}
		return vv, nil
	case basetypes.MapType:
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		vals := map[string]attr.Value{}
		for k, v := range m {
			val, err := attrValueFromJSON(v, typ.ElemType)
			if err != nil {
				return nil, err
			}
			vals[k] = val
		}
		vv, diags := types.MapValue(typ.ElemType, vals)
		if diags.HasError() {
			diag := diags.Errors()[0]
			return nil, fmt.Errorf("%s: %s", diag.Summary(), diag.Detail())
		}
		return vv, nil
	case basetypes.ObjectType:
		var m map[string]json.RawMessage
		if err := json.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		vals := map[string]attr.Value{}
		attrTypes := typ.AttributeTypes()
		for k, v := range m {
			val, err := attrValueFromJSON(v, attrTypes[k])
			if err != nil {
				return nil, err
			}
			vals[k] = val
		}
		vv, diags := types.ObjectValue(attrTypes, vals)
		if diags.HasError() {
			diag := diags.Errors()[0]
			return nil, fmt.Errorf("%s: %s", diag.Summary(), diag.Detail())
		}
		return vv, nil
	default:
		return nil, fmt.Errorf("Unhandled type: %T", typ)
	}
}
