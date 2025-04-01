package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/magodo/terraform-provider-restful/internal/dynamic"
)

type ephemeralResourcePrivateData struct {
	Method        types.String
	Path          types.String
	Body          types.Dynamic
	DefaultHeader types.Map
	Header        types.Map
	DefaultQuery  types.Map
	Query         types.Map

	ExpiryAhead   types.String
	ExpiryType    types.String
	ExpiryLocator types.String
	ExpiryUnit    types.String
}

type ephemeralResourcePrivateDataGo struct {
	Method        string              `json:"method,omitempty"`
	Path          string              `json:"path,omitempty"`
	Body          []byte              `json:"body,omitempty"`
	DefaultHeader map[string]string   `json:"default_header,omitempty"`
	Header        map[string]string   `json:"header,omitempty"`
	DefaultQuery  map[string][]string `json:"default_query,omitempty"`
	Query         map[string][]string `json:"query,omitempty"`

	ExpiryAhead   string `json:"expiry_ahead,omitempty"`
	ExpiryType    string `json:"expiry_type,omitempty"`
	ExpiryLocator string `json:"expiry_locator,omitempty"`
	ExpiryUnit    string `json:"expiry_unit,omitempty"`
}

func (d ephemeralResourcePrivateData) MarshalJSON() ([]byte, error) {
	dg := ephemeralResourcePrivateDataGo{
		Method:        d.Method.ValueString(),
		Path:          d.Path.ValueString(),
		ExpiryAhead:   d.ExpiryAhead.ValueString(),
		ExpiryType:    d.ExpiryType.ValueString(),
		ExpiryLocator: d.ExpiryLocator.ValueString(),
		ExpiryUnit:    d.ExpiryUnit.ValueString(),
	}

	var err error

	dg.Body, err = dynamic.ToJSON(d.Body)
	if err != nil {
		return nil, fmt.Errorf("convert dynamic body to json: %v", err)
	}

	dg.DefaultHeader = headerToGo(d.DefaultHeader)
	dg.Header = headerToGo(d.Header)

	dg.DefaultQuery, err = queryToGo(d.DefaultQuery)
	if err != nil {
		return nil, err
	}
	dg.Query, err = queryToGo(d.Query)
	if err != nil {
		return nil, err
	}

	return json.Marshal(dg)
}

func (d *ephemeralResourcePrivateData) UnmarshalJSON(b []byte) error {
	var dg ephemeralResourcePrivateDataGo
	if err := json.Unmarshal(b, &dg); err != nil {
		return err
	}

	method := types.StringNull()
	if dg.Method != "" {
		method = types.StringValue(dg.Method)
	}
	d.Method = method

	path := types.StringNull()
	if dg.Path != "" {
		path = types.StringValue(dg.Path)
	}
	d.Path = path

	body := types.DynamicNull()
	if len(dg.Body) != 0 {
		var err error
		body, err = dynamic.FromJSONImplied(dg.Body)
		if err != nil {
			return fmt.Errorf("convert dynamic body from json failed: %v", err)
		}
	}
	d.Body = body

	d.Header = headerFromGo(dg.Header)
	d.DefaultHeader = headerFromGo(dg.DefaultHeader)

	d.Query = queryFromGo(dg.Query)
	d.DefaultQuery = queryFromGo(dg.DefaultQuery)

	expiryAhead := types.StringNull()
	if dg.ExpiryAhead != "" {
		expiryAhead = types.StringValue(dg.ExpiryAhead)
	}
	d.ExpiryAhead = expiryAhead

	expiryType := types.StringNull()
	if dg.ExpiryType != "" {
		expiryType = types.StringValue(dg.ExpiryType)
	}
	d.ExpiryType = expiryType

	expiryLocator := types.StringNull()
	if dg.ExpiryLocator != "" {
		expiryLocator = types.StringValue(dg.ExpiryLocator)
	}
	d.ExpiryLocator = expiryLocator

	expiryUnit := types.StringNull()
	if dg.ExpiryUnit != "" {
		expiryUnit = types.StringValue(dg.ExpiryUnit)
	}
	d.ExpiryUnit = expiryUnit

	return nil
}

func headerToGo(v basetypes.MapValue) map[string]string {
	m := map[string]string{}
	for k, v := range v.Elements() {
		m[k] = v.(types.String).ValueString()
	}
	return m
}

func queryToGo(v basetypes.MapValue) (map[string][]string, error) {
	m := map[string][]string{}
	for k, v := range v.Elements() {
		vs := []string{}
		diags := v.(types.List).ElementsAs(context.Background(), &vs, false)
		if diags.HasError() {
			return nil, DiagToError(diags)
		}
		m[k] = vs
	}
	return m, nil
}

func headerFromGo(input map[string]string) basetypes.MapValue {
	if len(input) == 0 {
		return types.MapNull(types.StringType)
	}
	m := map[string]attr.Value{}
	for k, v := range input {
		m[k] = types.StringValue(v)
	}
	return types.MapValueMust(types.StringType, m)
}

func queryFromGo(input map[string][]string) basetypes.MapValue {
	if len(input) == 0 {
		return types.MapNull(types.ListType{ElemType: types.StringType})
	}
	m := map[string]attr.Value{}
	for k, vs := range input {
		var el []attr.Value
		for _, e := range vs {
			el = append(el, types.StringValue(e))
		}
		m[k] = types.ListValueMust(types.StringType, el)
	}
	return types.MapValueMust(types.ListType{ElemType: types.StringType}, m)
}
