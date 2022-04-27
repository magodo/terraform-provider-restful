package restapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"path/filepath"

	"github.com/magodo/terraform-provider-restapi/restapi/planmodifier"
	"github.com/magodo/terraform-provider-restapi/restapi/validator"

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
				Description:         "The ID of the Resource. Same as the `path` when the `create_method` is `PUT`",
				MarkdownDescription: "The ID of the Resource. Same as the `path` when the `create_method` is `PUT`",
				Type:                types.StringType,
				Computed:            true,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					tfsdk.UseStateForUnknown(),
				},
			},
			"path": {
				Description:         "The path of the resource, relative to the `base_url` of the provider. It differs when `create_method` is `PUT` and `POST`",
				MarkdownDescription: "The path of the resource, relative to the `base_url` of the provider. It differs when `create_method` is `PUT` and `POST`",
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
				Description:         "The path to the id attribute in the response. This is ignored when `create_method` is `PUT`.",
				MarkdownDescription: "The path to the id attribute in the response, which is only used during creation of the resource to construct the resource identifier. This is ignored when `create_method` is `PUT`.",
				Optional:            true,
				Computed:            true,
				Type:                types.StringType,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					planmodifier.DefaultAttributePlanModifier{
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
					planmodifier.DefaultAttributePlanModifier{
						Default: types.List{
							ElemType: types.StringType,
							Elems:    []attr.Value{},
						},
					},
				},
			},
			"create_method": {
				Description:         "The method used to create the resource. Possible values are `PUT` and `POST`. Defaults to `POST`. This overrides the `create_method` set in the provider block.",
				MarkdownDescription: "The method used to create the resource. Possible values are `PUT` and `POST`. Defaults to `POST`. This overrides the `create_method` set in the provider block.",
				Type:                types.StringType,
				Optional:            true,
				Validators:          []tfsdk.AttributeValidator{validator.StringInSlice("PUT", "POST")},
			},
			"query": {
				Description:         "The query parameters that are applied to each request. This won't clean up the `query` set in the provider block, expcet the value with the same key.",
				MarkdownDescription: "The query parameters that are applied to each request. This won't clean up the `query` set in the provider block, expcet the value with the same key.",
				Type:                types.MapType{ElemType: types.StringType},
				Optional:            true,
			},
			"output": {
				Description:         "The response body after reading the resource",
				MarkdownDescription: "The response body after reading the resource",
				Type:                types.StringType,
				Computed:            true,
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
	CreateMethod  types.String `tfsdk:"create_method"`
	Query         types.Map    `tfsdk:"query"`
	Output        types.String `tfsdk:"output"`
}

func (r resource) Create(ctx context.Context, req tfsdk.CreateResourceRequest, resp *tfsdk.CreateResourceResponse) {
	var plan resourceData
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	c := r.p.client

	// Existance check for resources whose create method is `PUT`, in which case the `path` is the same as its ID.
	// It is not possible to query the resource prior creation for resources whose create method is `POST`, since the `path` in this case is not enough for a `GET`.
	if r.p.apiOpt.CreateMethod == "PUT" {
		opt := client.ReadOption{
			Query: r.p.apiOpt.Query,
		}
		if len(plan.Query.Elems) != 0 {
			for k, v := range plan.Query.Elems {
				opt.Query[k] = v.(types.String).Value
			}
		}
		_, err := c.Read(ctx, plan.Path.Value, opt)
		if err == nil {
			resp.Diagnostics.AddError(
				"Resource already exists",
				fmt.Sprintf("A resource with the ID %q already exists - to be managed via Terraform this resource needs to be imported into the State. Please see the resource documentation for %q for more information.", plan.Path.Value, `restapi_resource`),
			)
			return
		}
		if err != nil && err != client.ErrNotFound {
			resp.Diagnostics.AddError(
				"Existance check failed",
				err.Error(),
			)
			return
		}
	}

	opt := client.CreateOption{
		Method: r.p.apiOpt.CreateMethod,
		Query:  r.p.apiOpt.Query,
	}
	if len(plan.Query.Elems) != 0 {
		for k, v := range plan.Query.Elems {
			opt.Query[k] = v.(types.String).Value
		}
	}
	if !plan.CreateMethod.Null {
		opt.Method = plan.CreateMethod.Value
	}

	b, err := c.Create(ctx, plan.Path.Value, plan.Body.Value, opt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Creation failure",
			fmt.Sprintf("Creating: %v", err),
		)
		return
	}

	var resourceId string
	switch r.p.apiOpt.CreateMethod {
	case "POST":
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
		resourceId = path.Join(plan.Path.Value, id)
	case "PUT":
		resourceId = plan.Path.Value
	}

	plan.ID = types.String{Value: resourceId}
	diags = resp.State.Set(ctx, plan)
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

	c := r.p.client

	opt := client.ReadOption{
		Query: r.p.apiOpt.Query,
	}
	if len(state.Query.Elems) != 0 {
		for k, v := range state.Query.Elems {
			opt.Query[k] = v.(types.String).Value
		}
	}

	b, err := c.Read(ctx, state.ID.Value, opt)
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

	switch r.p.apiOpt.CreateMethod {
	case "POST":
		state.Path = types.String{Value: filepath.Dir(state.ID.Value)}
	case "PUT":
		state.Path = types.String{Value: state.ID.Value}
	}
	state.Body = types.String{Value: string(body)}
	state.Output = types.String{Value: string(b)}

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

	var plan resourceData
	diags = req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	c := r.p.client

	opt := client.UpdateOption{
		Query: r.p.apiOpt.Query,
	}
	if len(plan.Query.Elems) != 0 {
		for k, v := range plan.Query.Elems {
			opt.Query[k] = v.(types.String).Value
		}
	}

	if _, err := c.Update(ctx, state.ID.Value, plan.Body.Value, opt); err != nil {
		resp.Diagnostics.AddError(
			"Update failure",
			err.Error(),
		)
		return
	}

	diags = resp.State.Set(ctx, plan)
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

	c := r.p.client

	opt := client.DeleteOption{
		Query: r.p.apiOpt.Query,
	}
	if len(state.Query.Elems) != 0 {
		for k, v := range state.Query.Elems {
			opt.Query[k] = v.(types.String).Value
		}
	}

	if _, err := c.Delete(ctx, state.ID.Value, opt); err != nil {
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
	idPath := tftypes.NewAttributePath().WithAttributeName("id")
	if idPath == nil || tftypes.NewAttributePath().Equal(idPath) {
		resp.Diagnostics.AddError(
			"Resource Import Error",
			"The attribute path `id` is nil or empty",
		)
	}
	queryPath := tftypes.NewAttributePath().WithAttributeName("query")
	if queryPath == nil || tftypes.NewAttributePath().Equal(queryPath) {
		resp.Diagnostics.AddError(
			"Resource Import Error",
			"The attribute path `query` is nil or empty",
		)
	}

	u, err := url.Parse(req.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Resource Import Error",
			fmt.Sprintf("Invalid id format: %v", err),
		)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, idPath, u.Path)...)

	if len(u.Query()) != 0 {
		m := map[string]string{}
		for k, l := range u.Query() {
			// Here we only accept unique query keys, since the resty's SetQueryParams method assume one key only has one value.
			if len(l) == 1 {
				m[k] = l[0]
			}
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, queryPath, m)...)
	}
}
