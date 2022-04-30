package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"strings"

	"github.com/magodo/terraform-provider-restful/internal/client"
	"github.com/magodo/terraform-provider-restful/internal/planmodifier"
	"github.com/magodo/terraform-provider-restful/internal/validator"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/tidwall/gjson"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// Magic header used to indicate the value in the state is derived from import.
const __IMPORT_HEADER__ = "__RESTFUL_PROVIDER__"

type resourceType struct{}

func (r resourceType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	pollAttribute := func(s string) tfsdk.Attribute {
		return tfsdk.Attribute{
			Description:         "The polling option for the %q operation",
			MarkdownDescription: "The polling option for the %q operation",
			Optional:            true,
			Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
				"status_locator": {
					Description:         "Specifies how to discover the status property. The format is either `code` or `<scope>[<path>]` (where `<scope>` can be either `header` or `body`)",
					MarkdownDescription: "Specifies how to discover the status property. The format is either `code` or `<scope>[<path>]` (where `<scope>` can be either `header` or `body`)",
					Required:            true,
					Type:                types.StringType,
				},
				"status": {
					Description:         "The expected status sentinels for each polling state",
					MarkdownDescription: "The expected status sentinels for each polling state",
					Required:            true,
					Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
						"success": {
							Description:         "The expected status sentinel for suceess status",
							MarkdownDescription: "The expected status sentinel for suceess status",
							Required:            true,
							Type:                types.StringType,
						},
						"failure": {
							Description:         "The expected status sentinel for failure status",
							MarkdownDescription: "The expected status sentinel for failure status",
							Required:            true,
							Type:                types.StringType,
						},
						"pending": {
							Description:         "The expected status sentinels for pending status",
							MarkdownDescription: "The expected status sentinels for pending status",
							Optional:            true,
							Type:                types.ListType{ElemType: types.StringType},
						},
					}),
				},
				"url_locator": {
					Description:         "Specifies how to discover the polling location. The format is as `<scope>[path]`, where `<scope>` can be either `header` or `body`. When absent, the resource's path is used for polling",
					MarkdownDescription: "Specifies how to discover the polling location. The format is as `<scope>[path]`, where `<scope>` can be either `header` or `body`. When absent, the resource's path is used for polling",
					Optional:            true,
					Type:                types.StringType,
				},
				"default_delay_sec": {
					Description:         "The interval between two pollings if there is no `Retry-After` in the response header, in second",
					MarkdownDescription: "The interval between two pollings if there is no `Retry-After` in the response header, in second",
					Optional:            true,
					Type:                types.Int64Type,
				},
			}),
		}
	}
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
				Computed:            true,
				Validators:          []tfsdk.AttributeValidator{validator.StringInSlice("PUT", "POST")},
			},
			"query": {
				Description:         "The query parameters that are applied to each request. This won't clean up the `query` set in the provider block, expcet the value with the same key.",
				MarkdownDescription: "The query parameters that are applied to each request. This won't clean up the `query` set in the provider block, expcet the value with the same key.",
				Type:                types.MapType{ElemType: types.ListType{ElemType: types.StringType}},
				Optional:            true,
				Computed:            true,
			},
			"poll_create": pollAttribute("Create"),
			"poll_update": pollAttribute("Update"),
			"poll_delete": pollAttribute("Delete"),
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
	PollCreate    types.Object `tfsdk:"poll_create"`
	PollUpdate    types.Object `tfsdk:"poll_update"`
	PollDelete    types.Object `tfsdk:"poll_delete"`
	Output        types.String `tfsdk:"output"`
}

type pollDataGo struct {
	StatusLocator string `tfsdk:"status_locator"`
	Status        struct {
		Success string   `tfsdk:"success"`
		Failure string   `tfsdk:"failure"`
		Pending []string `tfsdk:"pending"`
	} `tfsdk:"status"`
	UrlLocator   *string `tfsdk:"url_locator"`
	DefaultDelay *int64  `tfsdk:"default_delay_sec"`
}

func (r resource) Create(ctx context.Context, req tfsdk.CreateResourceRequest, resp *tfsdk.CreateResourceResponse) {
	var plan resourceData
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	c := r.p.client

	opt, diags := r.p.apiOpt.ForResourceCreate(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	// Existance check for resources whose create method is `PUT`, in which case the `path` is the same as its ID.
	// It is not possible to query the resource prior creation for resources whose create method is `POST`, since the `path` in this case is not enough for a `GET`.
	if opt.CreateMethod == "PUT" {
		opt, diags := r.p.apiOpt.ForResourceRead(ctx, plan)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}
		response, err := c.Read(ctx, plan.Path.Value, *opt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Existance check failed",
				err.Error(),
			)
			return
		}
		if response.StatusCode() != http.StatusNotFound {
			resp.Diagnostics.AddError(
				"Resource already exists",
				fmt.Sprintf("A resource with the ID %q already exists - to be managed via Terraform this resource needs to be imported into the State. Please see the resource documentation for %q for more information.", plan.Path.Value, `restful_resource`),
			)
			return
		}
	}

	// Create the resource
	response, err := c.Create(ctx, plan.Path.Value, plan.Body.Value, *opt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Creation failure",
			fmt.Sprintf("Creating: %v", err),
		)
		return
	}
	if response.StatusCode()/100 != 2 {
		diags.AddError(
			"Creation failure",
			fmt.Sprintf("Unexpected response (%s - code: %d): %s", response.Status(), response.StatusCode(), string(response.Body())),
		)
		return
	}

	b := response.Body()

	// For POST create method, generate the resource id by combining the path and the id in response.
	var resourceId string
	switch opt.CreateMethod {
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

		result := gjson.GetBytes(b, plan.IdPath.Value)
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

	// For LRO, wait for completion
	if opt.PollOpt != nil {
		if opt.PollOpt.UrlLocator == nil {
			// Update the request URL to pointing to the resource path, which is mainly for resources whose create method is POST.
			// As it will be used to poll the resource status.
			response.Request.URL = path.Join(c.BaseURL, resourceId)
		}
		p, err := client.NewPollable(*response, *opt.PollOpt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to build poller from the response of the initiated request",
				err.Error(),
			)
		}
		if err := p.PollUntilDone(ctx, c); err != nil {
			resp.Diagnostics.AddError(
				"Polling failure",
				err.Error(),
			)
			return
		}
	}

	// Set overridable attributes from option to state
	plan.Query = opt.Query.ToTFValue()
	plan.CreateMethod = types.String{Value: opt.CreateMethod}

	// Set resource ID to state
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

	opt, diags := r.p.apiOpt.ForResourceRead(ctx, state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	response, err := c.Read(ctx, state.ID.Value, *opt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Read failure",
			err.Error(),
		)
		return
	}
	if response.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	b := response.Body()

	// In case id is not set, set its default value as is defined in schema. This can avoid unnecessary plan diff after import.
	if state.IdPath.Null || state.IdPath.Unknown {
		state.IdPath = types.String{Value: "id"}
	}

	var ignoreChanges []string
	// In case ignore_changes is not set, set its default value as is defined in schema. This can avoid unnecessary plan diff after import.
	if state.IgnoreChanges.Null || state.IgnoreChanges.Unknown {
		state.IgnoreChanges = types.List{
			ElemType: types.StringType,
			Elems:    []attr.Value{},
		}
	}
	diags = state.IgnoreChanges.ElementsAs(ctx, &ignoreChanges, false)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	var stateBody string
	if strings.HasPrefix(state.Body.Value, __IMPORT_HEADER__) {
		// This branch is only invoked during `terraform import`.
		stateBody = strings.TrimPrefix(state.Body.Value, __IMPORT_HEADER__)
	} else {
		stateBody = state.Body.Value
	}
	body, err := ModifyBody(stateBody, string(b), ignoreChanges)
	if err != nil {
		resp.Diagnostics.AddError(
			"Read failure",
			fmt.Sprintf("Modifing body: %v", err),
		)
		return
	}

	createMethod := r.p.apiOpt.CreateMethod
	if state.CreateMethod.Value != "" {
		createMethod = state.CreateMethod.Value
	}

	switch createMethod {
	case "POST":
		state.Path = types.String{Value: filepath.Dir(state.ID.Value)}
	case "PUT":
		state.Path = types.String{Value: state.ID.Value}
	}

	// Set overridable attributes from option to state
	state.Query = opt.Query.ToTFValue()
	state.CreateMethod = types.String{Value: createMethod}

	// Set computed attributes
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

	opt, diags := r.p.apiOpt.ForResourceUpdate(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	response, err := c.Update(ctx, state.ID.Value, plan.Body.Value, *opt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Update failure",
			err.Error(),
		)
		return
	}

	// For LRO, wait for completion
	if opt.PollOpt != nil {
		p, err := client.NewPollable(*response, *opt.PollOpt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to build poller from the response of the initiated request",
				err.Error(),
			)
		}
		if err := p.PollUntilDone(ctx, c); err != nil {
			resp.Diagnostics.AddError(
				"Polling failure",
				err.Error(),
			)
		}
	}

	// Set overridable attributes from option to state
	plan.Query = opt.Query.ToTFValue()
	if plan.CreateMethod.Unknown {
		plan.CreateMethod = state.CreateMethod
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

	opt, diags := r.p.apiOpt.ForResourceDelete(ctx, state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	response, err := c.Delete(ctx, state.ID.Value, *opt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Delete failure",
			err.Error(),
		)
		return
	}

	// For LRO, wait for completion
	if opt.PollOpt != nil {
		p, err := client.NewPollable(*response, *opt.PollOpt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to build poller from the response of the initiated request",
				err.Error(),
			)
		}
		if err := p.PollUntilDone(ctx, c); err != nil {
			resp.Diagnostics.AddError(
				"Polling failure",
				err.Error(),
			)
		}
	}

	return
}

type importSpec struct {
	// Id is the resource id, which is always required.
	Id string `json:"id"`

	// Query is only required when it is mandatory for reading the resource.
	Query url.Values `json:"query"`

	// CreateMethod is necessarily for correctly setting the `path` (a force new attribute) during Read.
	// However, it is optional for POST created resources, or the `create_method` is correctly set in the provider level.
	CreateMethod string `json:"create_method"`

	// Body represents the properties expected to be managed and tracked by Terraform. The value of these properties can be null as a place holder.
	// When absent, all the response payload read wil be set to `body`.
	Body map[string]interface{}
}

func (resource) ImportState(ctx context.Context, req tfsdk.ImportResourceStateRequest, resp *tfsdk.ImportResourceStateResponse) {
	idPath := tftypes.NewAttributePath().WithAttributeName("id")
	queryPath := tftypes.NewAttributePath().WithAttributeName("query")
	createMethodPath := tftypes.NewAttributePath().WithAttributeName("create_method")
	bodyPath := tftypes.NewAttributePath().WithAttributeName("body")

	var imp importSpec
	if err := json.Unmarshal([]byte(req.ID), &imp); err != nil {
		resp.Diagnostics.AddError(
			"Resource Import Error",
			fmt.Sprintf("failed to unmarshal ID: %v", err),
		)
		return
	}

	if len(imp.Body) != 0 {
		b, err := json.Marshal(imp.Body)
		if err != nil {
			resp.Diagnostics.AddError(
				"Resource Import Error",
				fmt.Sprintf("failed to marshal id.body: %v", err),
			)
			return
		}
		body := __IMPORT_HEADER__ + string(b)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, bodyPath, body)...)
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, idPath, imp.Id)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, queryPath, imp.Query)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, createMethodPath, imp.CreateMethod)...)
}
