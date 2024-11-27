package provider

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

func TestMarshalEphemeralResourcePrivateData(t *testing.T) {
	cases := []struct {
		name   string
		in     ephemeralResourcePrivateData
		expect string
	}{
		{
			name: "all set",
			in: ephemeralResourcePrivateData{
				Method:        types.StringValue("POST"),
				Path:          types.StringValue("path"),
				Body:          types.DynamicValue(types.MapValueMust(types.StringType, map[string]attr.Value{"foo": types.StringValue("bar")})),
				Header:        types.MapValueMust(types.StringType, map[string]attr.Value{"h1": types.StringValue("v1")}),
				DefaultHeader: types.MapValueMust(types.StringType, map[string]attr.Value{"h1": types.StringValue("v1")}),
				Query: types.MapValueMust(
					types.ListType{ElemType: types.StringType},
					map[string]attr.Value{
						"q1": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("v1")}),
					},
				),
				DefaultQuery: types.MapValueMust(
					types.ListType{ElemType: types.StringType},
					map[string]attr.Value{
						"q1": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("v1")}),
					},
				),
				ExpiryType:    types.StringValue("et"),
				ExpiryLocator: types.StringValue("el"),
			},
			expect: fmt.Sprintf(`
{
  "method": "POST",
  "path": "path",
  "body": %q,
  "default_header": {
    "h1": "v1"
  },
  "header": {
    "h1": "v1"
  },
  "default_query": {
    "q1": ["v1"]
  },
  "query": {
    "q1": ["v1"]
  },
  "expiry_type": "et",
  "expiry_locator": "el"
}`, base64.StdEncoding.EncodeToString([]byte(`{"foo":"bar"}`))),
		},

		{
			name: "only method",
			in: ephemeralResourcePrivateData{
				Method:        types.StringValue("POST"),
				Path:          types.StringNull(),
				Body:          types.DynamicNull(),
				DefaultHeader: types.MapNull(types.StringType),
				Header:        types.MapNull(types.StringType),
				DefaultQuery:  types.MapNull(types.ListType{ElemType: types.StringType}),
				Query:         types.MapNull(types.ListType{ElemType: types.StringType}),
				ExpiryType:    types.StringNull(),
				ExpiryLocator: types.StringNull(),
			},
			expect: `{"method": "POST"}`,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.in)
			require.NoError(t, err)
			require.JSONEq(t, tt.expect, string(b))
		})
	}
}

func TestUnMarshalEphemeralResourcePrivateData(t *testing.T) {
	cases := []struct {
		name   string
		in     string
		expect ephemeralResourcePrivateData
	}{
		{
			name: "all set",
			in: fmt.Sprintf(`
{
  "method": "POST",
  "path": "path",
  "body": %q,
  "default_header": {
    "h1": "v1"
  },
  "header": {
    "h1": "v1"
  },
  "default_query": {
    "q1": ["v1"]
  },
  "query": {
    "q1": ["v1"]
  },
  "expiry_type": "et",
  "expiry_locator": "el"
}`, base64.StdEncoding.EncodeToString([]byte(`{"foo":"bar"}`))),
			expect: ephemeralResourcePrivateData{
				Method:        types.StringValue("POST"),
				Path:          types.StringValue("path"),
				Body:          types.DynamicValue(types.ObjectValueMust(map[string]attr.Type{"foo": types.StringType}, map[string]attr.Value{"foo": types.StringValue("bar")})),
				Header:        types.MapValueMust(types.StringType, map[string]attr.Value{"h1": types.StringValue("v1")}),
				DefaultHeader: types.MapValueMust(types.StringType, map[string]attr.Value{"h1": types.StringValue("v1")}),
				Query: types.MapValueMust(
					types.ListType{ElemType: types.StringType},
					map[string]attr.Value{
						"q1": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("v1")}),
					},
				),
				DefaultQuery: types.MapValueMust(
					types.ListType{ElemType: types.StringType},
					map[string]attr.Value{
						"q1": types.ListValueMust(types.StringType, []attr.Value{types.StringValue("v1")}),
					},
				),
				ExpiryType:    types.StringValue("et"),
				ExpiryLocator: types.StringValue("el"),
			},
		},

		{
			name: "only method",
			in:   `{"method": "POST"}`,
			expect: ephemeralResourcePrivateData{
				Method:        types.StringValue("POST"),
				Path:          types.StringNull(),
				Body:          types.DynamicNull(),
				DefaultHeader: types.MapNull(types.StringType),
				Header:        types.MapNull(types.StringType),
				DefaultQuery:  types.MapNull(types.ListType{ElemType: types.StringType}),
				Query:         types.MapNull(types.ListType{ElemType: types.StringType}),
				ExpiryType:    types.StringNull(),
				ExpiryLocator: types.StringNull(),
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var d ephemeralResourcePrivateData
			err := json.Unmarshal([]byte(tt.in), &d)
			require.NoError(t, err)
			require.Equal(t, tt.expect, d)
		})
	}
}
