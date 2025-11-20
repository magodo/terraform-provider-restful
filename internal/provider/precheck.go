package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/lfventura/terraform-provider-restful/internal/client"
	"github.com/lfventura/terraform-provider-restful/internal/locks"
)

func precheck(ctx context.Context, c *client.Client, apiOpt apiOption, defaultPath string, defaultHeader client.Header, defaultQuery client.Query, prechecks basetypes.ListValue, body basetypes.DynamicValue) (func(), diag.Diagnostics) {
	lockedNames := []string{}
	var checks []precheckData
	if diags := prechecks.ElementsAs(ctx, &checks, false); diags.HasError() {
		return nil, diags
	}

	for i, check := range checks {
		switch {
		case !check.Api.IsNull():
			var d precheckDataApi
			if diags := check.Api.As(ctx, &d, basetypes.ObjectAsOptions{}); diags.HasError() {
				return nil, diags
			}
			opt, diags := apiOpt.ForPrecheck(ctx, defaultPath, defaultHeader, defaultQuery, d, body)
			if diags.HasError() {
				return nil, diags
			}
			p, err := client.NewPollableForPrecheck(*opt)
			if err != nil {
				return nil, diag.Diagnostics{
					diag.NewErrorDiagnostic(
						fmt.Sprintf("Failed to build poller for %d-th check (api)", i),
						err.Error(),
					),
				}
			}
			if err := p.PollUntilDone(ctx, c); err != nil {
				return nil, diag.Diagnostics{
					diag.NewErrorDiagnostic(
						fmt.Sprintf("Pre-checking %d-th check (api) failure", i),
						err.Error(),
					),
				}
			}
		case !check.Mutex.IsNull():
			key := check.Mutex.ValueString()
			if err := locks.Lock(ctx, key); err != nil {
				return nil, diag.Diagnostics{
					diag.NewErrorDiagnostic(
						fmt.Sprintf("Pre-checking %d-th check (mutex) failure", i),
						err.Error(),
					),
				}
			}
			lockedNames = append(lockedNames, key)
		}
	}

	return func() {
		for i := len(lockedNames) - 1; i >= 0; i-- {
			locks.Unlock(lockedNames[i])
		}
	}, nil
}
