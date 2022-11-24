package provider

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestValidateResourceConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	res := &Resource{}
	resourceSchema, _ := res.GetSchema(ctx)
	resourceType := resourceSchema.Type().TerraformType(ctx)

	typ := func(paths ...string) tftypes.Type {
		attr := resourceSchema.GetAttributes()[paths[0]]
		for _, path := range paths[1:] {
			attr = attr.GetAttributes().GetAttributes()[path]
		}
		return attr.FrameworkType().TerraformType(ctx)
	}

	type testCase struct {
		config        tftypes.Value
		expectedDiags []*tfprotov6.Diagnostic
	}

	tests := map[string]testCase{
		"minimal config": {
			config: tftypes.NewValue(resourceType, map[string]tftypes.Value{
				"id":                   tftypes.NewValue(typ("id"), nil),
				"path":                 tftypes.NewValue(typ("path"), "/foos"),
				"read_path":            tftypes.NewValue(typ("read_path"), nil),
				"update_path":          tftypes.NewValue(typ("update_path"), nil),
				"delete_path":          tftypes.NewValue(typ("delete_path"), nil),
				"body":                 tftypes.NewValue(typ("body"), "{}"),
				"poll_create":          tftypes.NewValue(typ("poll_create"), nil),
				"poll_update":          tftypes.NewValue(typ("poll_update"), nil),
				"poll_delete":          tftypes.NewValue(typ("poll_delete"), nil),
				"write_only_attrs":     tftypes.NewValue(typ("write_only_attrs"), nil),
				"create_method":        tftypes.NewValue(typ("create_method"), nil),
				"update_method":        tftypes.NewValue(typ("update_method"), nil),
				"delete_method":        tftypes.NewValue(typ("delete_method"), nil),
				"merge_patch_disabled": tftypes.NewValue(typ("merge_patch_disabled"), nil),
				"query":                tftypes.NewValue(typ("query"), nil),
				"header":               tftypes.NewValue(typ("header"), nil),
				"output":               tftypes.NewValue(typ("output"), nil),
			}),
			expectedDiags: nil,
		},
		"full config": {
			config: tftypes.NewValue(resourceType, map[string]tftypes.Value{
				"id":                   tftypes.NewValue(typ("id"), nil),
				"path":                 tftypes.NewValue(typ("path"), "/foos"),
				"read_path":            tftypes.NewValue(typ("read_path"), "${path}"),
				"update_path":          tftypes.NewValue(typ("update_path"), "${path}"),
				"delete_path":          tftypes.NewValue(typ("delete_path"), "${path}"),
				"body":                 tftypes.NewValue(typ("body"), "{}"),
				"poll_create":          tftypes.NewValue(typ("poll_create"), nil),
				"poll_update":          tftypes.NewValue(typ("poll_update"), nil),
				"poll_delete":          tftypes.NewValue(typ("poll_delete"), nil),
				"write_only_attrs":     tftypes.NewValue(typ("write_only_attrs"), nil),
				"create_method":        tftypes.NewValue(typ("create_method"), "POST"),
				"update_method":        tftypes.NewValue(typ("update_method"), "PATCH"),
				"delete_method":        tftypes.NewValue(typ("delete_method"), "POST"),
				"merge_patch_disabled": tftypes.NewValue(typ("merge_patch_disabled"), nil),
				"query":                tftypes.NewValue(typ("query"), nil),
				"header":               tftypes.NewValue(typ("header"), nil),
				"output":               tftypes.NewValue(typ("output"), nil),
			}),
			expectedDiags: nil,
		},
	}

	for name, tc := range tests {
		name, tc := name, tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			p := New()
			testServer := providerserver.NewProtocol6(p)()
			ctx := context.Background()
			if _, err := testServer.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{}); err != nil {
				t.Errorf("Unexpected error: %s", err)
				return
			}
			dv, err := tfprotov6.NewDynamicValue(resourceType, tc.config)
			if err != nil {
				t.Errorf("Unexpected error: %s", err)
				return
			}
			req := &tfprotov6.ValidateResourceConfigRequest{
				TypeName: "restful_resource",
				Config:   &dv,
			}
			got, err := testServer.ValidateResourceConfig(ctx, req)
			if err != nil {
				t.Errorf("Unexpected error: %s", err)
				return
			}
			if diff := cmp.Diff(got.Diagnostics, tc.expectedDiags); diff != "" {
				t.Errorf("Unexpected diff in diagnostics (+wanted, -got): %s", diff)
			}
		})
	}
}
