package provider

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

type PrivateData interface {
	GetKey(ctx context.Context, key string) ([]byte, diag.Diagnostics)
	SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics
}

type EphemeralBodyPrivateMgr struct{}

var ephemeralBodyPrivateMgr = EphemeralBodyPrivateMgr{}

func (m EphemeralBodyPrivateMgr) key() string {
	return "ephemeral_body"
}

// Set sets the hash of the ephemeral_body to the private state.
// If `ebody` is nil, it removes the hash from the private state.
func (m EphemeralBodyPrivateMgr) Set(ctx context.Context, d PrivateData, ebody []byte) (diags diag.Diagnostics) {
	if ebody == nil {
		d.SetKey(ctx, m.key(), nil)
		return
	}
	h := sha256.New()
	if _, err := h.Write(ebody); err != nil {
		diags.AddError(
			`Error to hash "ephemeral_body"`,
			err.Error(),
		)
		return
	}
	hash := h.Sum(nil)
	b, err := json.Marshal(map[string]interface{}{
		"hash": hash, // []byte will be marshaled to base64 encoded string
	})
	if err != nil {
		diags.AddError(
			`Error to marshal "ephemeral_body" private data`,
			err.Error(),
		)
		return
	}
	return d.SetKey(ctx, m.key(), b)
}

// Diff tells whether the ephemeral_body is different than the hash stored in the private state,
// including the private state doesn't have it recorded.
func (m EphemeralBodyPrivateMgr) Diff(ctx context.Context, d PrivateData, ebody []byte) (bool, diag.Diagnostics) {
	b, diags := d.GetKey(ctx, m.key())
	if diags.HasError() {
		return false, diags
	}
	if b == nil {
		// In case private state doesn't store the key yet, it only diffs when the ebody is not nil.
		return ebody != nil, diags
	}
	var mm map[string]interface{}
	if err := json.Unmarshal(b, &mm); err != nil {
		diags.AddError(
			`Error to unmarshal "ephemeral_body" private data`,
			err.Error(),
		)
		return false, diags
	}
	privateHashEnc, ok := mm["hash"]
	if !ok {
		diags.AddError(
			`Invalid "ephemeral_body" private data`,
			`Key "hash" not found`,
		)
		return false, diags
	}

	h := sha256.New()
	if _, err := h.Write(ebody); err != nil {
		diags.AddError(
			`Error to hash "ephemeral_body"`,
			err.Error(),
		)
		return false, diags
	}
	hash := h.Sum(nil)

	hashEnc := base64.StdEncoding.EncodeToString(hash)

	return hashEnc != privateHashEnc.(string), diags
}
