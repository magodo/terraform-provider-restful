package restapi

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/magodo/terraform-provider-restapi/client"
	"github.com/tidwall/gjson"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type resourceType struct{}

func (r resourceType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description:         "Restful resource",
		MarkdownDescription: "Restful resource",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Description:         "The ID of the Resource",
				MarkdownDescription: "The ID of the Resource",
				Type:                types.StringType,
				Computed:            true,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					tfsdk.UseStateForUnknown(),
				},
			},
			"path": {
				Description:         "The path of the resource, relative to the `base_url`",
				MarkdownDescription: "The path of the resource, relative to the `base_url`",
				Type:                types.StringType,
				Required:            true,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					tfsdk.RequiresReplace(),
				},
			},
			"body": {
				Description:         "The properties of the resource",
				MarkdownDescription: "The properties of the resource",
				Type:                types.StringType,
				Required:            true,
			},

			"id_path": {
				Description:         "The path to the id attribute in the response",
				MarkdownDescription: "The path to the id attribute in the response, which is only used during creation of the resource to construct the resource identifier",
				Optional:            true,
				Computed:            true,
				Type:                types.StringType,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					DefaultAttributePlanModifier{
						Default: types.String{Value: "id"},
					},
				},
			},
			"ignore_changes": {
				Description:         "A list of paths to the attributes that should not affect the resource after its creation",
				MarkdownDescription: "A list of paths to the attributes that should not affect the resource after its creation",
				Optional:            true,
				Computed:            true,
				Type:                types.ListType{ElemType: types.StringType},
				PlanModifiers: []tfsdk.AttributePlanModifier{
					DefaultAttributePlanModifier{
						Default: types.List{
							ElemType: types.StringType,
							Elems:    []attr.Value{},
						},
					},
				},
			},
		},
	}, nil
}

func (r resourceType) NewResource(_ context.Context, p tfsdk.Provider) (tfsdk.Resource, diag.Diagnostics) {
	return resource{p: *p.(*provider)}, nil
}

type resource struct {
	p provider
}

var _ tfsdk.Resource = resource{}

type resourceData struct {
	ID            types.String `tfsdk:"id"`
	Path          types.String `tfsdk:"path"`
	Body          types.String `tfsdk:"body"`
	IdPath        types.String `tfsdk:"id_path"`
	IgnoreChanges types.List   `tfsdk:"ignore_changes"`
}

func (r resource) Create(ctx context.Context, req tfsdk.CreateResourceRequest, resp *tfsdk.CreateResourceResponse) {
	var plan resourceData
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	b, err := r.p.Client.Create(plan.Path.Value, plan.Body.Value)
	if err != nil {
		resp.Diagnostics.AddError(
			"Creation failure",
			err.Error(),
		)
		return
	}

	// TODO: Is the response always guaranteed to be an object, maybe array?
	var body map[string]interface{}
	if err := json.Unmarshal(b, &body); err != nil {
		resp.Diagnostics.AddError(
			"Creation failure",
			fmt.Sprintf("Unmarshalling create response: %v", err),
		)
		return
	}

	result := gjson.Get(string(b), plan.IdPath.Value)
	if !result.Exists() {
		resp.Diagnostics.AddError(
			"Creation failure",
			fmt.Sprintf("Can't find resource id in path %q", plan.IdPath.Value),
		)
		return
	}
	id := result.String()

	resourceId := path.Join(plan.Path.Value, id)
	state := plan
	state.ID = types.String{Value: resourceId}
	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	rreq := tfsdk.ReadResourceRequest{
		State:        resp.State,
		ProviderMeta: req.ProviderMeta,
	}
	rresp := tfsdk.ReadResourceResponse{
		State:       resp.State,
		Diagnostics: resp.Diagnostics,
	}
	r.Read(ctx, rreq, &rresp)

	*resp = tfsdk.CreateResourceResponse{
		State:       rresp.State,
		Diagnostics: rresp.Diagnostics,
	}
}

func (r resource) Read(ctx context.Context, req tfsdk.ReadResourceRequest, resp *tfsdk.ReadResourceResponse) {
	var state resourceData
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	b, err := r.p.Client.Read(state.ID.Value)
	if err != nil {
		if err == client.ErrNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Read failure",
			err.Error(),
		)
		return
	}

	var ignoreChanges []string
	if !state.IgnoreChanges.Unknown && !state.IgnoreChanges.Null {
		diags = state.IgnoreChanges.ElementsAs(ctx, &ignoreChanges, false)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}
	}

	body, err := ModifyBody(state.Body.Value, string(b), ignoreChanges)
	if err != nil {
		resp.Diagnostics.AddError(
			"Read failure",
			fmt.Sprintf("Modifing body: %v", err),
		)
		return
	}

	state.Path = types.String{Value: filepath.Dir(state.ID.Value)}
	state.Body = types.String{Value: string(body)}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
}

func (r resource) Update(ctx context.Context, req tfsdk.UpdateResourceRequest, resp *tfsdk.UpdateResourceResponse) {
	var state resourceData
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	id := state.ID.Value

	var plan resourceData
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if _, err := r.p.Client.Update(id, plan.Body.Value); err != nil {
		resp.Diagnostics.AddError(
			"Update failure",
			err.Error(),
		)
		return
	}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	rreq := tfsdk.ReadResourceRequest{
		State:        resp.State,
		ProviderMeta: req.ProviderMeta,
	}
	rresp := tfsdk.ReadResourceResponse{
		State:       resp.State,
		Diagnostics: resp.Diagnostics,
	}
	r.Read(ctx, rreq, &rresp)

	*resp = tfsdk.UpdateResourceResponse{
		State:       rresp.State,
		Diagnostics: rresp.Diagnostics,
	}
}

func (r resource) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
	var state resourceData
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	_, err := r.p.Client.Delete(state.ID.Value)
	if err != nil {
		if err == client.ErrNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError(
			"Delete failure",
			err.Error(),
		)
		return
	}
	resp.State.RemoveResource(ctx)
	return
}

func (resource) ImportState(ctx context.Context, req tfsdk.ImportResourceStateRequest, resp *tfsdk.ImportResourceStateResponse) {
	tfsdk.ResourceImportStatePassthroughID(ctx, tftypes.NewAttributePath().WithAttributeName("id"), req, resp)
}
