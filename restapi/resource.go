package restapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/tidwall/gjson"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type resourceType struct{}

func (r resourceType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	tftypes.NewAttributePath()
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

	u, _ := url.Parse(r.p.url)
	u.Path = u.Path + plan.Path.Value
	url := u.String()
	response, err := r.p.Client.Post(url, "application/json", strings.NewReader(plan.Body.Value))
	if err != nil {
		resp.Diagnostics.AddError(
			"Creation failure",
			fmt.Sprintf("Sending create request: %v", err),
		)
		return
	}
	defer response.Body.Close()

	b, err := io.ReadAll(response.Body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Creation failure",
			fmt.Sprintf("Reading create response: %v", err),
		)
		return
	}

	// TODO: Support LRO
	if response.StatusCode/100 != 2 {
		resp.Diagnostics.AddError(
			"Creation failure",
			fmt.Sprintf("Unexpected response from create (%s - code: %d): %s", response.Status, response.StatusCode, string(b)),
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

	u, _ := url.Parse(r.p.url)
	u.Path = u.Path + state.ID.Value
	url := u.String()
	response, err := r.p.Client.Get(url)
	if err != nil {
		resp.Diagnostics.AddError(
			"Read failure",
			fmt.Sprintf("Sending read request: %v", err),
		)
		return
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	b, err := io.ReadAll(response.Body)
	if err != nil {
		resp.Diagnostics.AddError(
			"Read failure",
			fmt.Sprintf("Reading create response: %v", err),
		)
		return
	}

	if response.StatusCode/100 != 2 {
		resp.Diagnostics.AddError(
			"Read failure",
			fmt.Sprintf("Unexpected response from read (%s - code: %d): %s", response.Status, response.StatusCode, string(b)),
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

	state.Path = types.String{Value: filepath.Dir(strings.TrimPrefix(url, r.p.url))}
	state.Body = types.String{Value: string(body)}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
}

func (resource) Update(context.Context, tfsdk.UpdateResourceRequest, *tfsdk.UpdateResourceResponse) {
	panic("unimplemented")
}

func (r resource) Delete(ctx context.Context, req tfsdk.DeleteResourceRequest, resp *tfsdk.DeleteResourceResponse) {
	var state resourceData
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	u, _ := url.Parse(r.p.url)
	u.Path = u.Path + state.ID.Value
	url := u.String()
	deleteReq, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Delete failure",
			fmt.Sprintf("Building delete request: %v", err),
		)
		return
	}

	response, err := r.p.Client.Do(deleteReq)
	if err != nil {
		resp.Diagnostics.AddError(
			"Delete failure",
			fmt.Sprintf("Sending delete request: %v", err),
		)
		return
	}
	defer response.Body.Close()

	if response.StatusCode/100 == 2 {
		resp.State.RemoveResource(ctx)
		return
	}

	// TODO: Support LRO
	switch response.StatusCode {
	case http.StatusNotFound:
		resp.State.RemoveResource(ctx)
		return
	default:
		b, err := io.ReadAll(response.Body)
		if err != nil {
			resp.Diagnostics.AddError(
				"Delete failure",
				fmt.Sprintf("Reading delete response: %v", err),
			)
			return
		}
		resp.Diagnostics.AddError(
			"Delete failure",
			fmt.Sprintf("Unexpected response from delete (%s - code: %d): %s", response.Status, response.StatusCode, string(b)),
		)
		return
	}
}

func (resource) ImportState(ctx context.Context, req tfsdk.ImportResourceStateRequest, resp *tfsdk.ImportResourceStateResponse) {
	tfsdk.ResourceImportStatePassthroughID(ctx, tftypes.NewAttributePath().WithAttributeName("id"), req, resp)
}
