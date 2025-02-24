package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/magodo/terraform-provider-restful/internal/dynamic"
	"github.com/magodo/terraform-provider-restful/internal/jsonset"
)

// validateEphemeralBody validates a known ephemeral_body doesn't joint with the body.
// It returns the json representation of the ephemeral body as well (if known).
func validateEphemeralBody(body []byte, ephemeralBody types.Dynamic) ([]byte, diag.Diagnostics) {
	if ephemeralBody.IsUnknown() {
		return nil, nil
	}

	var diags diag.Diagnostics

	eb, err := dynamic.ToJSON(ephemeralBody)
	if err != nil {
		diags.AddError(
			"Invalid configuration",
			fmt.Sprintf(`marshal "ephemeral_body": %v`, err),
		)
		return nil, diags
	}
	disjointed, err := jsonset.Disjointed(body, eb)
	if err != nil {
		diags.AddError(
			"Invalid configuration",
			fmt.Sprintf(`checking disjoint of "body" and "ephemeral_body": %v`, err),
		)
		return nil, diags
	}
	if !disjointed {
		diags.AddError(
			"Invalid configuration",
			`"body" and "ephemeral_body" are not disjointed`,
		)
		return nil, diags
	}
	return eb, nil
}

// ephemeralBodyChangeInPlan checks if the ephemeral_body has changed in the plan modify phase.
func ephemeralBodyChangeInPlan(ctx context.Context, d PrivateData, ephemeralBody types.Dynamic) (ok bool, diags diag.Diagnostics) {
	// 1. ephemeral_body is null
	if ephemeralBody.IsNull() {
		return ephemeralBodyPrivateMgr.Diff(ctx, d, nil)
	}

	// 2. ephemeral_body is unknown (e.g. referencing an knonw-after-apply value)
	if ephemeralBody.IsUnknown() {
		return true, nil
	}

	// 3. ephemeral_body is known in the config, but has different hash than the private data
	eb, err := dynamic.ToJSON(ephemeralBody)
	if err != nil {
		diags.AddError(
			`Error to marshal "ephemeral_body"`,
			err.Error(),
		)
		return
	}
	return ephemeralBodyPrivateMgr.Diff(ctx, d, eb)
}
