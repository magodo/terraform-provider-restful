package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/magodo/terraform-plugin-framework-helper/dynamic"
	"github.com/lfventura/terraform-provider-restful/internal/provider/migrate"
)

func (r *Resource) UpgradeState(context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &migrate.ResourceSchemaV0,
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var pd migrate.ResourceDataV0

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

				output := types.DynamicNull()
				if !output.IsNull() {
					output, err = dynamic.FromJSONImplied([]byte(pd.Output.ValueString()))
					if err != nil {
						resp.Diagnostics.AddError(
							"Upgrade State Error",
							fmt.Sprintf(`Converting "output": %v`, err),
						)
					}
				}

				upgradedStateData := migrate.ResourceDataV1{
					ID:                  pd.ID,
					Path:                pd.Path,
					CreateSelector:      pd.CreateSelector,
					ReadSelector:        pd.ReadSelector,
					ReadPath:            pd.ReadPath,
					UpdatePath:          pd.UpdatePath,
					DeletePath:          pd.DeletePath,
					CreateMethod:        pd.CreateMethod,
					UpdateMethod:        pd.UpdateMethod,
					DeleteMethod:        pd.DeleteMethod,
					PrecheckCreate:      pd.PrecheckCreate,
					PrecheckUpdate:      pd.PrecheckUpdate,
					PrecheckDelete:      pd.PrecheckDelete,
					Body:                body,
					PollCreate:          pd.PollCreate,
					PollUpdate:          pd.PollUpdate,
					PollDelete:          pd.PollDelete,
					RetryCreate:         pd.RetryCreate,
					RetryRead:           pd.RetryRead,
					RetryUpdate:         pd.RetryUpdate,
					RetryDelete:         pd.RetryDelete,
					WriteOnlyAttributes: pd.WriteOnlyAttributes,
					MergePatchDisabled:  pd.MergePatchDisabled,
					Query:               pd.Query,
					Header:              pd.Header,
					CheckExistance:      pd.CheckExistance,
					ForceNewAttrs:       pd.ForceNewAttrs,
					OutputAttrs:         pd.OutputAttrs,
					Output:              output,
				}

				resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
			},
		},
		1: {
			PriorSchema: &migrate.ResourceSchemaV1,
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var pd migrate.ResourceDataV1

				resp.Diagnostics.Append(req.State.Get(ctx, &pd)...)

				if resp.Diagnostics.HasError() {
					return
				}

				upgradedStateData := resourceData{
					ID:                  pd.ID,
					Path:                pd.Path,
					CreateSelector:      pd.CreateSelector,
					ReadSelector:        pd.ReadSelector,
					ReadPath:            pd.ReadPath,
					UpdatePath:          pd.UpdatePath,
					DeletePath:          pd.DeletePath,
					CreateMethod:        pd.CreateMethod,
					UpdateMethod:        pd.UpdateMethod,
					DeleteMethod:        pd.DeleteMethod,
					PrecheckCreate:      pd.PrecheckCreate,
					PrecheckUpdate:      pd.PrecheckUpdate,
					PrecheckDelete:      pd.PrecheckDelete,
					Body:                pd.Body,
					PollCreate:          pd.PollCreate,
					PollUpdate:          pd.PollUpdate,
					PollDelete:          pd.PollDelete,
					WriteOnlyAttributes: pd.WriteOnlyAttributes,
					MergePatchDisabled:  pd.MergePatchDisabled,
					Query:               pd.Query,
					Header:              pd.Header,
					CheckExistance:      pd.CheckExistance,
					ForceNewAttrs:       pd.ForceNewAttrs,
					OutputAttrs:         pd.OutputAttrs,
					Output:              pd.Output,
				}

				resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
			},
		},
	}
}
