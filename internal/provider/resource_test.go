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

	providerSchema, _ := New().GetSchema(ctx)
	providerType := providerSchema.Type().TerraformType(ctx)

	ptyp := func(paths ...string) tftypes.Type {
		attr := providerSchema.GetAttributes()[paths[0]]
		for _, path := range paths[1:] {
			attr = attr.GetAttributes().GetAttributes()[path]
		}
		return attr.FrameworkType().TerraformType(ctx)
	}

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
				"body":                 tftypes.NewValue(typ("body"), "{}"),
				"poll_create":          tftypes.NewValue(typ("poll_create"), nil),
				"poll_update":          tftypes.NewValue(typ("poll_update"), nil),
				"poll_delete":          tftypes.NewValue(typ("poll_delete"), nil),
				"name_path":            tftypes.NewValue(typ("name_path"), nil),
				"url_path":             tftypes.NewValue(typ("url_path"), nil),
				"write_only_attrs":     tftypes.NewValue(typ("write_only_attrs"), nil),
				"create_method":        tftypes.NewValue(typ("create_method"), nil),
				"update_method":        tftypes.NewValue(typ("update_method"), nil),
				"merge_patch_disabled": tftypes.NewValue(typ("merge_patch_disabled"), nil),
				"query":                tftypes.NewValue(typ("query"), nil),
				"header":               tftypes.NewValue(typ("header"), nil),
				"output":               tftypes.NewValue(typ("output"), nil),
			}),
			expectedDiags: nil,
		},
		"create_method is PUT, but set name_path and url_path": {
			config: tftypes.NewValue(resourceType, map[string]tftypes.Value{
				"id":                   tftypes.NewValue(typ("id"), nil),
				"path":                 tftypes.NewValue(typ("path"), "/foos"),
				"body":                 tftypes.NewValue(typ("body"), "{}"),
				"poll_create":          tftypes.NewValue(typ("poll_create"), nil),
				"poll_update":          tftypes.NewValue(typ("poll_update"), nil),
				"poll_delete":          tftypes.NewValue(typ("poll_delete"), nil),
				"name_path":            tftypes.NewValue(typ("name_path"), "foo"),
				"url_path":             tftypes.NewValue(typ("url_path"), "bar"),
				"write_only_attrs":     tftypes.NewValue(typ("write_only_attrs"), nil),
				"create_method":        tftypes.NewValue(typ("create_method"), "PUT"),
				"update_method":        tftypes.NewValue(typ("update_method"), nil),
				"merge_patch_disabled": tftypes.NewValue(typ("merge_patch_disabled"), nil),
				"query":                tftypes.NewValue(typ("query"), nil),
				"header":               tftypes.NewValue(typ("header"), nil),
				"output":               tftypes.NewValue(typ("output"), nil),
			}),
			expectedDiags: []*tfprotov6.Diagnostic{
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Invalid configuration",
					Detail:   "The `name_path` can not be specified when `create_method` is `PUT`",
				},
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Invalid configuration",
					Detail:   "The `url_path` can not be specified when `create_method` is `PUT`",
				},
			},
		},
		"create_method is POST, but set both name_path and url_path": {
			config: tftypes.NewValue(resourceType, map[string]tftypes.Value{
				"id":                   tftypes.NewValue(typ("id"), nil),
				"path":                 tftypes.NewValue(typ("path"), "/foos"),
				"body":                 tftypes.NewValue(typ("body"), "{}"),
				"poll_create":          tftypes.NewValue(typ("poll_create"), nil),
				"poll_update":          tftypes.NewValue(typ("poll_update"), nil),
				"poll_delete":          tftypes.NewValue(typ("poll_delete"), nil),
				"name_path":            tftypes.NewValue(typ("name_path"), "foo"),
				"url_path":             tftypes.NewValue(typ("url_path"), "bar"),
				"write_only_attrs":     tftypes.NewValue(typ("write_only_attrs"), nil),
				"create_method":        tftypes.NewValue(typ("create_method"), "POST"),
				"update_method":        tftypes.NewValue(typ("update_method"), nil),
				"merge_patch_disabled": tftypes.NewValue(typ("merge_patch_disabled"), nil),
				"query":                tftypes.NewValue(typ("query"), nil),
				"header":               tftypes.NewValue(typ("header"), nil),
				"output":               tftypes.NewValue(typ("output"), nil),
			}),
			expectedDiags: []*tfprotov6.Diagnostic{
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Invalid configuration",
					Detail:   "Exactly one of `name_path` and `url_path` should be specified when `create_method` is `POST`",
				},
			},
		},
		"create_method is POST, but not set both name_path or url_path": {
			config: tftypes.NewValue(resourceType, map[string]tftypes.Value{
				"id":                   tftypes.NewValue(typ("id"), nil),
				"path":                 tftypes.NewValue(typ("path"), "/foos"),
				"body":                 tftypes.NewValue(typ("body"), "{}"),
				"poll_create":          tftypes.NewValue(typ("poll_create"), nil),
				"poll_update":          tftypes.NewValue(typ("poll_update"), nil),
				"poll_delete":          tftypes.NewValue(typ("poll_delete"), nil),
				"name_path":            tftypes.NewValue(typ("name_path"), nil),
				"url_path":             tftypes.NewValue(typ("url_path"), nil),
				"write_only_attrs":     tftypes.NewValue(typ("write_only_attrs"), nil),
				"create_method":        tftypes.NewValue(typ("create_method"), "POST"),
				"update_method":        tftypes.NewValue(typ("update_method"), nil),
				"merge_patch_disabled": tftypes.NewValue(typ("merge_patch_disabled"), nil),
				"query":                tftypes.NewValue(typ("query"), nil),
				"header":               tftypes.NewValue(typ("header"), nil),
				"output":               tftypes.NewValue(typ("output"), nil),
			}),
			expectedDiags: []*tfprotov6.Diagnostic{
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Invalid configuration",
					Detail:   "Exactly one of `name_path` and `url_path` should be specified when `create_method` is `POST`",
				},
			},
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
			providerConfig := tftypes.NewValue(providerType, map[string]tftypes.Value{
				"base_url":             tftypes.NewValue(ptyp("base_url"), "http://localhost:8080"),
				"security":             tftypes.NewValue(ptyp("security"), nil),
				"create_method":        tftypes.NewValue(ptyp("create_method"), "PUT"),
				"update_method":        tftypes.NewValue(ptyp("update_method"), nil),
				"merge_patch_disabled": tftypes.NewValue(ptyp("merge_patch_disabled"), nil),
				"query":                tftypes.NewValue(ptyp("query"), nil),
				"header":               tftypes.NewValue(ptyp("header"), nil),
			})
			pdv, err := tfprotov6.NewDynamicValue(providerType, providerConfig)
			if err != nil {
				t.Errorf("Unexpected error: %s", err)
				return
			}
			if _, err := testServer.ConfigureProvider(ctx, &tfprotov6.ConfigureProviderRequest{
				Config: &pdv,
			}); err != nil {
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
