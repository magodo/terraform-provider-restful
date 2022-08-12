package provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestValidateProviderConfig(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	providerSchema, _ := New().GetSchema(ctx)
	providerType := providerSchema.TerraformType(ctx)

	typ := func(paths ...string) tftypes.Type {
		attr := providerSchema.GetAttributes()[paths[0]]
		for _, path := range paths[1:] {
			attr = attr.GetAttributes().GetAttributes()[path]
		}
		return attr.FrameworkType().TerraformType(ctx)
	}

	etyp := func(paths ...string) tftypes.Type {
		attr := providerSchema.GetAttributes()[paths[0]]
		for _, path := range paths[1:] {
			attr = attr.GetAttributes().GetAttributes()[path]
		}
		switch enclosed := attr.GetAttributes().Type().(type) {
		case types.ListType:
			return enclosed.ElementType().TerraformType(ctx)
		case types.SetType:
			return enclosed.ElementType().TerraformType(ctx)
		case types.MapType:
			return enclosed.ElementType().TerraformType(ctx)
		}
		panic(fmt.Sprintf("unsupported supported type: %T", attr.GetAttributes().Type()))
	}

	type testCase struct {
		config        tftypes.Value
		expectedDiags []*tfprotov6.Diagnostic
	}

	tests := map[string]testCase{
		"minimal config": {
			config: tftypes.NewValue(providerType, map[string]tftypes.Value{
				"base_url":      tftypes.NewValue(typ("base_url"), "http://localhost:8080"),
				"security":      tftypes.NewValue(typ("security"), nil),
				"create_method": tftypes.NewValue(typ("create_method"), nil),
				"update_method": tftypes.NewValue(typ("update_method"), nil),
				"query":         tftypes.NewValue(typ("query"), nil),
				"header":        tftypes.NewValue(typ("header"), nil),
			}),
			expectedDiags: nil,
		},
		"security: http scheme not specifying anything": {
			config: tftypes.NewValue(providerType, map[string]tftypes.Value{
				"base_url": tftypes.NewValue(typ("base_url"), "http://localhost:8080"),
				"security": tftypes.NewValue(typ("security"), map[string]tftypes.Value{
					"http": tftypes.NewValue(typ("security", "http"), map[string]tftypes.Value{
						"type":     tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
						"username": tftypes.NewValue(tftypes.String, nil),
						"password": tftypes.NewValue(tftypes.String, nil),
						"token":    tftypes.NewValue(tftypes.String, nil),
					}),
					"oauth2": tftypes.NewValue(typ("security", "oauth2"), nil),
					"apikey": tftypes.NewValue(typ("security", "apikey"), nil),
				}),
				"create_method": tftypes.NewValue(typ("create_method"), nil),
				"update_method": tftypes.NewValue(typ("update_method"), nil),
				"query":         tftypes.NewValue(typ("query"), nil),
				"header":        tftypes.NewValue(typ("header"), nil),
			}),
			expectedDiags: []*tfprotov6.Diagnostic{
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Invalid configuration: `security.http`",
					Detail:   "Either `username` & `password`, or `token` should be specified",
				},
			},
		},
		"security: http scheme specifying too much": {
			config: tftypes.NewValue(providerType, map[string]tftypes.Value{
				"base_url": tftypes.NewValue(typ("base_url"), "http://localhost:8080"),
				"security": tftypes.NewValue(typ("security"), map[string]tftypes.Value{
					"http": tftypes.NewValue(typ("security", "http"), map[string]tftypes.Value{
						"type":     tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
						"username": tftypes.NewValue(tftypes.String, "foo"),
						"password": tftypes.NewValue(tftypes.String, "bar"),
						"token":    tftypes.NewValue(tftypes.String, "baz"),
					}),
					"oauth2": tftypes.NewValue(typ("security", "oauth2"), nil),
					"apikey": tftypes.NewValue(typ("security", "apikey"), nil),
				}),
				"create_method": tftypes.NewValue(typ("create_method"), nil),
				"update_method": tftypes.NewValue(typ("update_method"), nil),
				"query":         tftypes.NewValue(typ("query"), nil),
				"header":        tftypes.NewValue(typ("header"), nil),
			}),
			expectedDiags: []*tfprotov6.Diagnostic{
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Invalid configuration: `security.http`",
					Detail:   "Either `username` & `password`, or `token` should be specified",
				},
			},
		},
		"security: http scheme config mismatches the type Basic": {
			config: tftypes.NewValue(providerType, map[string]tftypes.Value{
				"base_url": tftypes.NewValue(typ("base_url"), "http://localhost:8080"),
				"security": tftypes.NewValue(typ("security"), map[string]tftypes.Value{
					"http": tftypes.NewValue(typ("security", "http"), map[string]tftypes.Value{
						"type":     tftypes.NewValue(tftypes.String, "Basic"),
						"username": tftypes.NewValue(tftypes.String, nil),
						"password": tftypes.NewValue(tftypes.String, nil),
						"token":    tftypes.NewValue(tftypes.String, "baz"),
					}),
					"oauth2": tftypes.NewValue(typ("security", "oauth2"), nil),
					"apikey": tftypes.NewValue(typ("security", "apikey"), nil),
				}),
				"create_method": tftypes.NewValue(typ("create_method"), nil),
				"update_method": tftypes.NewValue(typ("update_method"), nil),
				"query":         tftypes.NewValue(typ("query"), nil),
				"header":        tftypes.NewValue(typ("header"), nil),
			}),
			expectedDiags: []*tfprotov6.Diagnostic{
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Invalid configuration: `security.http`",
					Detail:   "`username` is required when `type` is Basic",
				},
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Invalid configuration: `security.http`",
					Detail:   "`password` is required when `type` is Basic",
				},
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Invalid configuration: `security.http`",
					Detail:   "`token` can't be specified when `type` is Basic",
				},
			},
		},
		"security: http scheme config mismatches the type Bearer": {
			config: tftypes.NewValue(providerType, map[string]tftypes.Value{
				"base_url": tftypes.NewValue(typ("base_url"), "http://localhost:8080"),
				"security": tftypes.NewValue(typ("security"), map[string]tftypes.Value{
					"http": tftypes.NewValue(typ("security", "http"), map[string]tftypes.Value{
						"type":     tftypes.NewValue(tftypes.String, "Bearer"),
						"username": tftypes.NewValue(tftypes.String, "foo"),
						"password": tftypes.NewValue(tftypes.String, "bar"),
						"token":    tftypes.NewValue(tftypes.String, nil),
					}),
					"oauth2": tftypes.NewValue(typ("security", "oauth2"), nil),
					"apikey": tftypes.NewValue(typ("security", "apikey"), nil),
				}),
				"create_method": tftypes.NewValue(typ("create_method"), nil),
				"update_method": tftypes.NewValue(typ("update_method"), nil),
				"query":         tftypes.NewValue(typ("query"), nil),
				"header":        tftypes.NewValue(typ("header"), nil),
			}),
			expectedDiags: []*tfprotov6.Diagnostic{
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Invalid configuration: `security.http`",
					Detail:   "`username` can't be specified when `type` is Bearer",
				},
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Invalid configuration: `security.http`",
					Detail:   "`password` can't be specified when `type` is Bearer",
				},
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Invalid configuration: `security.http`",
					Detail:   "`token` is required when `type` is Bearer",
				},
			},
		},
		"security: http scheme corect config for Basic": {
			config: tftypes.NewValue(providerType, map[string]tftypes.Value{
				"base_url": tftypes.NewValue(typ("base_url"), "http://localhost:8080"),
				"security": tftypes.NewValue(typ("security"), map[string]tftypes.Value{
					"http": tftypes.NewValue(typ("security", "http"), map[string]tftypes.Value{
						"type":     tftypes.NewValue(tftypes.String, "Basic"),
						"username": tftypes.NewValue(tftypes.String, "foo"),
						"password": tftypes.NewValue(tftypes.String, "bar"),
						"token":    tftypes.NewValue(tftypes.String, nil),
					}),
					"oauth2": tftypes.NewValue(typ("security", "oauth2"), nil),
					"apikey": tftypes.NewValue(typ("security", "apikey"), nil),
				}),
				"create_method": tftypes.NewValue(typ("create_method"), nil),
				"update_method": tftypes.NewValue(typ("update_method"), nil),
				"query":         tftypes.NewValue(typ("query"), nil),
				"header":        tftypes.NewValue(typ("header"), nil),
			}),
			expectedDiags: nil,
		},
		"security: http scheme corect config for Bearer": {
			config: tftypes.NewValue(providerType, map[string]tftypes.Value{
				"base_url": tftypes.NewValue(typ("base_url"), "http://localhost:8080"),
				"security": tftypes.NewValue(typ("security"), map[string]tftypes.Value{
					"http": tftypes.NewValue(typ("security", "http"), map[string]tftypes.Value{
						"type":     tftypes.NewValue(tftypes.String, "Bearer"),
						"username": tftypes.NewValue(tftypes.String, nil),
						"password": tftypes.NewValue(tftypes.String, nil),
						"token":    tftypes.NewValue(tftypes.String, "baz"),
					}),
					"oauth2": tftypes.NewValue(typ("security", "oauth2"), nil),
					"apikey": tftypes.NewValue(typ("security", "apikey"), nil),
				}),
				"create_method": tftypes.NewValue(typ("create_method"), nil),
				"update_method": tftypes.NewValue(typ("update_method"), nil),
				"query":         tftypes.NewValue(typ("query"), nil),
				"header":        tftypes.NewValue(typ("header"), nil),
			}),
			expectedDiags: nil,
		},
		"security: oauth2 scheme not specifying anything": {
			config: tftypes.NewValue(providerType, map[string]tftypes.Value{
				"base_url": tftypes.NewValue(typ("base_url"), "http://localhost:8080"),
				"security": tftypes.NewValue(typ("security"), map[string]tftypes.Value{
					"http": tftypes.NewValue(typ("security", "http"), nil),
					"oauth2": tftypes.NewValue(typ("security", "oauth2"), map[string]tftypes.Value{
						"token_url":       tftypes.NewValue(tftypes.String, "http://localhost:8080/auth"),
						"client_id":       tftypes.NewValue(tftypes.String, nil),
						"client_secret":   tftypes.NewValue(tftypes.String, nil),
						"username":        tftypes.NewValue(tftypes.String, nil),
						"password":        tftypes.NewValue(tftypes.String, nil),
						"scopes":          tftypes.NewValue(typ("security", "oauth2", "scopes"), nil),
						"endpoint_params": tftypes.NewValue(typ("security", "oauth2", "endpoint_params"), nil),
						"in":              tftypes.NewValue(tftypes.String, nil),
					}),
					"apikey": tftypes.NewValue(typ("security", "apikey"), nil),
				}),
				"create_method": tftypes.NewValue(typ("create_method"), nil),
				"update_method": tftypes.NewValue(typ("update_method"), nil),
				"query":         tftypes.NewValue(typ("query"), nil),
				"header":        tftypes.NewValue(typ("header"), nil),
			}),
			expectedDiags: []*tfprotov6.Diagnostic{
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Invalid configuration: `security.oauth2`",
					Detail:   "Either `username` & `password`, or `client_id` & `client_secret` should be specified",
				},
			},
		},
		"security: oauth2 scheme not specifying too much": {
			config: tftypes.NewValue(providerType, map[string]tftypes.Value{
				"base_url": tftypes.NewValue(typ("base_url"), "http://localhost:8080"),
				"security": tftypes.NewValue(typ("security"), map[string]tftypes.Value{
					"http": tftypes.NewValue(typ("security", "http"), nil),
					"oauth2": tftypes.NewValue(typ("security", "oauth2"), map[string]tftypes.Value{
						"token_url":       tftypes.NewValue(tftypes.String, "http://localhost:8080/auth"),
						"client_id":       tftypes.NewValue(tftypes.String, "foo"),
						"client_secret":   tftypes.NewValue(tftypes.String, "bar"),
						"username":        tftypes.NewValue(tftypes.String, "foo"),
						"password":        tftypes.NewValue(tftypes.String, "bar"),
						"scopes":          tftypes.NewValue(typ("security", "oauth2", "scopes"), nil),
						"endpoint_params": tftypes.NewValue(typ("security", "oauth2", "endpoint_params"), nil),
						"in":              tftypes.NewValue(tftypes.String, nil),
					}),
					"apikey": tftypes.NewValue(typ("security", "apikey"), nil),
				}),
				"create_method": tftypes.NewValue(typ("create_method"), nil),
				"update_method": tftypes.NewValue(typ("update_method"), nil),
				"query":         tftypes.NewValue(typ("query"), nil),
				"header":        tftypes.NewValue(typ("header"), nil),
			}),
			expectedDiags: []*tfprotov6.Diagnostic{
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Invalid configuration: `security.oauth2`",
					Detail:   "Either `username` & `password`, or `client_id` & `client_secret` should be specified",
				},
			},
		},
		"security: oauth2 scheme correct config for client credential flow": {
			config: tftypes.NewValue(providerType, map[string]tftypes.Value{
				"base_url": tftypes.NewValue(typ("base_url"), "http://localhost:8080"),
				"security": tftypes.NewValue(typ("security"), map[string]tftypes.Value{
					"http": tftypes.NewValue(typ("security", "http"), nil),
					"oauth2": tftypes.NewValue(typ("security", "oauth2"), map[string]tftypes.Value{
						"token_url":       tftypes.NewValue(tftypes.String, "http://localhost:8080/auth"),
						"client_id":       tftypes.NewValue(tftypes.String, "foo"),
						"client_secret":   tftypes.NewValue(tftypes.String, "bar"),
						"username":        tftypes.NewValue(tftypes.String, nil),
						"password":        tftypes.NewValue(tftypes.String, nil),
						"scopes":          tftypes.NewValue(typ("security", "oauth2", "scopes"), nil),
						"endpoint_params": tftypes.NewValue(typ("security", "oauth2", "endpoint_params"), nil),
						"in":              tftypes.NewValue(tftypes.String, nil),
					}),
					"apikey": tftypes.NewValue(typ("security", "apikey"), nil),
				}),
				"create_method": tftypes.NewValue(typ("create_method"), nil),
				"update_method": tftypes.NewValue(typ("update_method"), nil),
				"query":         tftypes.NewValue(typ("query"), nil),
				"header":        tftypes.NewValue(typ("header"), nil),
			}),
			expectedDiags: nil,
		},
		"security: oauth2 scheme correct config for password credential flow": {
			config: tftypes.NewValue(providerType, map[string]tftypes.Value{
				"base_url": tftypes.NewValue(typ("base_url"), "http://localhost:8080"),
				"security": tftypes.NewValue(typ("security"), map[string]tftypes.Value{
					"http": tftypes.NewValue(typ("security", "http"), nil),
					"oauth2": tftypes.NewValue(typ("security", "oauth2"), map[string]tftypes.Value{
						"token_url":       tftypes.NewValue(tftypes.String, "http://localhost:8080/auth"),
						"client_id":       tftypes.NewValue(tftypes.String, nil),
						"client_secret":   tftypes.NewValue(tftypes.String, nil),
						"username":        tftypes.NewValue(tftypes.String, "foo"),
						"password":        tftypes.NewValue(tftypes.String, "bar"),
						"scopes":          tftypes.NewValue(typ("security", "oauth2", "scopes"), nil),
						"endpoint_params": tftypes.NewValue(typ("security", "oauth2", "endpoint_params"), nil),
						"in":              tftypes.NewValue(tftypes.String, nil),
					}),
					"apikey": tftypes.NewValue(typ("security", "apikey"), nil),
				}),
				"create_method": tftypes.NewValue(typ("create_method"), nil),
				"update_method": tftypes.NewValue(typ("update_method"), nil),
				"query":         tftypes.NewValue(typ("query"), nil),
				"header":        tftypes.NewValue(typ("header"), nil),
			}),
			expectedDiags: nil,
		},
		"security: api key scheme correct config": {
			config: tftypes.NewValue(providerType, map[string]tftypes.Value{
				"base_url": tftypes.NewValue(typ("base_url"), "http://localhost:8080"),
				"security": tftypes.NewValue(typ("security"), map[string]tftypes.Value{
					"http":   tftypes.NewValue(typ("security", "http"), nil),
					"oauth2": tftypes.NewValue(typ("security", "oauth2"), nil),
					"apikey": tftypes.NewValue(typ("security", "apikey"), []tftypes.Value{
						tftypes.NewValue(etyp("security", "apikey"), map[string]tftypes.Value{
							"name":  tftypes.NewValue(tftypes.String, "foo"),
							"value": tftypes.NewValue(tftypes.String, "bar"),
							"in":    tftypes.NewValue(tftypes.String, "query"),
						}),
					}),
				}),
				"create_method": tftypes.NewValue(typ("create_method"), nil),
				"update_method": tftypes.NewValue(typ("update_method"), nil),
				"query":         tftypes.NewValue(typ("query"), nil),
				"header":        tftypes.NewValue(typ("header"), nil),
			}),
			expectedDiags: nil,
		},
		"security: multiple schemes specified": {
			config: tftypes.NewValue(providerType, map[string]tftypes.Value{
				"base_url": tftypes.NewValue(typ("base_url"), "http://localhost:8080"),
				"security": tftypes.NewValue(typ("security"), map[string]tftypes.Value{
					"http": tftypes.NewValue(typ("security", "http"), map[string]tftypes.Value{
						"type":     tftypes.NewValue(tftypes.String, "Basic"),
						"username": tftypes.NewValue(tftypes.String, "foo"),
						"password": tftypes.NewValue(tftypes.String, "bar"),
						"token":    tftypes.NewValue(tftypes.String, nil),
					}),
					"oauth2": tftypes.NewValue(typ("security", "oauth2"), map[string]tftypes.Value{
						"token_url":       tftypes.NewValue(tftypes.String, "http://localhost:8080/auth"),
						"client_id":       tftypes.NewValue(tftypes.String, nil),
						"client_secret":   tftypes.NewValue(tftypes.String, nil),
						"username":        tftypes.NewValue(tftypes.String, "foo"),
						"password":        tftypes.NewValue(tftypes.String, "bar"),
						"scopes":          tftypes.NewValue(typ("security", "oauth2", "scopes"), nil),
						"endpoint_params": tftypes.NewValue(typ("security", "oauth2", "endpoint_params"), nil),
						"in":              tftypes.NewValue(tftypes.String, nil),
					}),
					"apikey": tftypes.NewValue(typ("security", "apikey"), []tftypes.Value{
						tftypes.NewValue(etyp("security", "apikey"), map[string]tftypes.Value{
							"name":  tftypes.NewValue(tftypes.String, "foo"),
							"value": tftypes.NewValue(tftypes.String, "bar"),
							"in":    tftypes.NewValue(tftypes.String, "query"),
						}),
					}),
				}),
				"create_method": tftypes.NewValue(typ("create_method"), nil),
				"update_method": tftypes.NewValue(typ("update_method"), nil),
				"query":         tftypes.NewValue(typ("query"), nil),
				"header":        tftypes.NewValue(typ("header"), nil),
			}),
			expectedDiags: []*tfprotov6.Diagnostic{
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Invalid configuration: `security`",
					Detail:   "More than one scheme is specified: http, oauth2, apikey",
				},
			},
		},
		"security: no schemes specified": {
			config: tftypes.NewValue(providerType, map[string]tftypes.Value{
				"base_url": tftypes.NewValue(typ("base_url"), "http://localhost:8080"),
				"security": tftypes.NewValue(typ("security"), map[string]tftypes.Value{
					"http":   tftypes.NewValue(typ("security", "http"), nil),
					"oauth2": tftypes.NewValue(typ("security", "oauth2"), nil),
					"apikey": tftypes.NewValue(typ("security", "apikey"), nil),
				}),
				"create_method": tftypes.NewValue(typ("create_method"), nil),
				"update_method": tftypes.NewValue(typ("update_method"), nil),
				"query":         tftypes.NewValue(typ("query"), nil),
				"header":        tftypes.NewValue(typ("header"), nil),
			}),
			expectedDiags: []*tfprotov6.Diagnostic{
				{
					Severity: tfprotov6.DiagnosticSeverityError,
					Summary:  "Invalid configuration: `security`",
					Detail:   "There is no security scheme specified",
				},
			},
		},
	}

	for name, tc := range tests {
		name, tc := name, tc

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			testServer := providerserver.NewProtocol6(New())()
			dv, err := tfprotov6.NewDynamicValue(providerType, tc.config)
			if err != nil {
				t.Errorf("Unexpected error: %s", err)
				return
			}
			req := &tfprotov6.ValidateProviderConfigRequest{
				Config: &dv,
			}
			got, err := testServer.ValidateProviderConfig(context.Background(), req)
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
