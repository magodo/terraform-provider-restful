package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/magodo/terraform-plugin-framework-helper/dynamic"
	"github.com/lfventura/terraform-provider-restful/internal/provider/migrate"
)

func (r *OperationResource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &migrate.OperationSchemaV0,
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var pd migrate.OperationDataV0

				resp.Diagnostics.Append(req.State.Get(ctx, &pd)...)

				if resp.Diagnostics.HasError() {
					return
				}

				var err error

				body := types.DynamicNull()
				if !pd.Body.IsNull() {
					body, err = dynamic.FromJSONImplied([]byte(pd.Body.ValueString()))
					if err != nil {
						resp.Diagnostics.AddError(
							"Upgrade State Error",
							fmt.Sprintf(`Converting "body": %v`, err),
						)
					}
				}

				deleteBody := types.DynamicNull()
				if !pd.DeleteBody.IsNull() {
					deleteBody, err = dynamic.FromJSONImplied([]byte(pd.DeleteBody.ValueString()))
					if err != nil {
						resp.Diagnostics.AddError(
							"Upgrade State Error",
							fmt.Sprintf(`Converting "delete_body": %v`, err),
						)
					}
				}

				output := types.DynamicNull()
				if !pd.Output.IsNull() {
					output, err = dynamic.FromJSONImplied([]byte(pd.Output.ValueString()))
					if err != nil {
						resp.Diagnostics.AddError(
							"Upgrade State Error",
							fmt.Sprintf(`Converting "output": %v`, err),
						)
					}
				}

				upgradedStateData := migrate.OperationDataV1{
					ID:             pd.ID,
					Path:           pd.Path,
					Method:         pd.Method,
					Body:           body,
					Query:          pd.Query,
					Header:         pd.Header,
					Precheck:       pd.Precheck,
					Poll:           pd.Poll,
					Retry:          pd.Retry,
					DeleteMethod:   pd.DeleteMethod,
					DeleteBody:     deleteBody,
					DeletePath:     pd.DeletePath,
					PrecheckDelete: pd.PrecheckDelete,
					PollDelete:     pd.PollDelete,
					RetryDelete:    pd.RetryDelete,
					OutputAttrs:    pd.OutputAttrs,
					Output:         output,
				}

				resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
			},
		},
		1: {
			PriorSchema: &migrate.OperationSchemaV1,
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var pd migrate.OperationDataV1

				resp.Diagnostics.Append(req.State.Get(ctx, &pd)...)

				if resp.Diagnostics.HasError() {
					return
				}

				upgradedStateData := operationResourceData{
					ID:             pd.ID,
					Path:           pd.Path,
					Method:         pd.Method,
					Body:           pd.Body,
					Query:          pd.Query,
					Header:         pd.Header,
					Precheck:       pd.Precheck,
					Poll:           pd.Poll,
					DeleteMethod:   pd.DeleteMethod,
					DeleteBody:     pd.DeleteBody,
					DeletePath:     pd.DeletePath,
					PrecheckDelete: pd.PrecheckDelete,
					PollDelete:     pd.PollDelete,
					OutputAttrs:    pd.OutputAttrs,
					Output:         pd.Output,
				}

				resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
			},
		},
	}
}
