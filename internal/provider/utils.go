package provider

import (
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/hashicorp/terraform-plugin-framework/diag"
)

func DiagToError(diags diag.Diagnostics) error {
	if !diags.HasError() {
		return nil
	}

	var err error
	for _, ed := range diags.Errors() {
		err = multierror.Append(err, fmt.Errorf("%s: %s", ed.Summary(), ed.Detail()))
	}
	return err
}

func ToPtr[T any](v T) *T {
	return &v
}

func FromPtr[T any](p *T) T {
	var v T
	if p != nil {
		v = *p
	}
	return v
}
