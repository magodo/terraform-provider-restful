package provider

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/magodo/terraform-provider-restful/internal/jsonset"
)

const (
	pkEphemeralBody = "ephemeral_body"
)

type PrivateData interface {
	GetKey(ctx context.Context, key string) ([]byte, diag.Diagnostics)
	SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics
}

type EphemeralBodyPrivateMgr struct{}

var ephemeralBodyPrivateMgr = EphemeralBodyPrivateMgr{}

func (m EphemeralBodyPrivateMgr) Exists(ctx context.Context, d PrivateData) (bool, diag.Diagnostics) {
	b, diags := d.GetKey(ctx, pkEphemeralBody)
	if diags.HasError() {
		return false, diags
	}
	return b != nil, diags
}

// Set sets the hash of the ephemeral_body to the private state.
// If `ebody` is nil, it removes the hash from the private state.
func (m EphemeralBodyPrivateMgr) Set(ctx context.Context, d PrivateData, ebody []byte) (diags diag.Diagnostics) {
	if ebody == nil {
		d.SetKey(ctx, pkEphemeralBody, nil)
		return
	}

	// Calculate the hash of the ephemeral body
	h := sha256.New()
	if _, err := h.Write(ebody); err != nil {
		diags.AddError(
			`Error to hash "ephemeral_body"`,
			err.Error(),
		)
		return
	}
	hash := h.Sum(nil)

	// Nullify ephemeral body
	nb, err := jsonset.NullifyObject(ebody)
	if err != nil {
		diags.AddError(
			`Error to nullify "ephemeral_body"`,
			err.Error(),
		)
		return
	}

	b, err := json.Marshal(map[string]interface{}{
		// []byte will be marshaled to base64 encoded string
		"hash": hash,
		"null": nb,
	})
	if err != nil {
		diags.AddError(
			`Error to marshal "ephemeral_body" private data`,
			err.Error(),
		)
		return
	}

	return d.SetKey(ctx, pkEphemeralBody, b)
}

// Diff tells whether the ephemeral_body is different than the hash stored in the private state.
// In case private state doesn't have the record, regard the record as "nil" (i.e. will return true if ebody is non-nil).
// In case private state has the record (guaranteed to be non-nil), while ebody is nil, it also returns true.
func (m EphemeralBodyPrivateMgr) Diff(ctx context.Context, d PrivateData, ebody []byte) (bool, diag.Diagnostics) {
	b, diags := d.GetKey(ctx, pkEphemeralBody)
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

// GetNullBody gets the nullified ephemeral_body from the private data.
// If it doesn't exist, nil is returned.
func (m EphemeralBodyPrivateMgr) GetNullBody(ctx context.Context, d PrivateData) ([]byte, diag.Diagnostics) {
	b, diags := d.GetKey(ctx, pkEphemeralBody)
	if diags.HasError() {
		return nil, diags
	}
	if b == nil {
		return nil, nil
	}

	var mm map[string]interface{}
	if err := json.Unmarshal(b, &mm); err != nil {
		diags.AddError(
			`Error to unmarshal "ephemeral_body" private data`,
			err.Error(),
		)
		return nil, diags
	}
	bEnc, ok := mm["null"]
	if !ok {
		return nil, nil
	}
	b, err := base64.StdEncoding.DecodeString(bEnc.(string))
	if err != nil {
		diags.AddError(
			`Error base64 decoding the nullified "ephemeral_body" in the private data`,
			err.Error(),
		)
		return nil, diags
	}
	return b, nil
}
