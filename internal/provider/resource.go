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
			Description:         fmt.Sprintf("The polling option for the %q operation", s),
			MarkdownDescription: fmt.Sprintf("The polling option for the %q operation", s),
			Optional:            true,
			Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
				"status_locator": {
					Description:         "Specifies how to discover the status property. The format is either `code` or `<scope>[<path>]`, where `<scope>` can be either `header` or `body`, and the `<path>` is using the gjson syntax.",
					MarkdownDescription: "Specifies how to discover the status property. The format is either `code` or `<scope>[<path>]`, where `<scope>` can be either `header` or `body`, and the `<path>` is using the [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md).",
					Required:            true,
					Type:                types.StringType,
				},
				"status": {
					Description:         "The expected status sentinels for each polling state.",
					MarkdownDescription: "The expected status sentinels for each polling state.",
					Required:            true,
					Attributes: tfsdk.SingleNestedAttributes(map[string]tfsdk.Attribute{
						"success": {
							Description:         "The expected status sentinel for suceess status.",
							MarkdownDescription: "The expected status sentinel for suceess status.",
							Required:            true,
							Type:                types.StringType,
						},
						"pending": {
							Description:         "The expected status sentinels for pending status.",
							MarkdownDescription: "The expected status sentinels for pending status.",
							Optional:            true,
							Type:                types.ListType{ElemType: types.StringType},
						},
					}),
				},
				"url_locator": {
					Description:         "Specifies how to discover the polling location. The format is as `<scope>[<path>]`, where `<scope>` can be either `header` or `body`, and the `<path>` is using the gjson syntax. When absent, the resource's path is used for polling.",
					MarkdownDescription: "Specifies how to discover the polling location. The format is as `<scope>[<path>]`, where `<scope>` can be either `header` or `body`, and the `<path>` is using the [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md). When absent, the resource's path is used for polling.",
					Optional:            true,
					Type:                types.StringType,
				},
				"default_delay_sec": {
					Description:         "The interval between two pollings if there is no `Retry-After` in the response header, in second.",
					MarkdownDescription: "The interval between two pollings if there is no `Retry-After` in the response header, in second.",
					Optional:            true,
					Type:                types.Int64Type,
				},
			}),
		}
	}
	return tfsdk.Schema{
		Description:         "`restful_resource` manages a restful resource.",
		MarkdownDescription: "`restful_resource` manages a restful resource.",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Description:         "The ID of the Resource. Same as the `path` when the `create_method` is `PUT`.",
				MarkdownDescription: "The ID of the Resource. Same as the `path` when the `create_method` is `PUT`.",
				Type:                types.StringType,
				Computed:            true,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					tfsdk.UseStateForUnknown(),
				},
			},
			"path": {
				Description:         "The path of the resource, relative to the `base_url` of the provider. It differs when `create_method` is `PUT` and `POST`.",
				MarkdownDescription: "The path of the resource, relative to the `base_url` of the provider. It differs when `create_method` is `PUT` and `POST`.",
				Type:                types.StringType,
				Required:            true,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					tfsdk.RequiresReplace(),
				},
			},
			"body": {
				Description:         "The properties of the resource.",
				MarkdownDescription: "The properties of the resource.",
				Type:                types.StringType,
				Required:            true,
			},
			"poll_create": pollAttribute("Create"),
			"poll_update": pollAttribute("Update"),
			"poll_delete": pollAttribute("Delete"),
			"name_path": {
				Description:         "The path (in gjson syntax) to the name attribute in the response, which is only used during creation of the resource to construct the resource identifier. This is ignored when `create_method` is `PUT`. Either `name_path` or `url_path` needs to set when `create_method` is `POST`.",
				MarkdownDescription: "The path (in [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)) to the name attribute in the response, which is only used during creation of the resource to construct the resource identifier. This is ignored when `create_method` is `PUT`. Either `name_path` or `url_path` needs to set when `create_method` is `POST`.",
				Optional:            true,
				Type:                types.StringType,
			},
			"url_path": {
				Description:         "The path (in gjson syntax) to the id attribute in the response, which is only used during creation of the resource to be as the resource identifier. This is ignored when `create_method` is `PUT`. Either `name_path` or `url_path` needs to set when `create_method` is `POST`.",
				MarkdownDescription: "The path (in [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)) to the id attribute in the response, which is only used during creation of the resource to be as the resource identifier. This is ignored when `create_method` is `PUT`. Either `name_path` or `url_path` needs to set when `create_method` is `POST`.",
				Optional:            true,
				Type:                types.StringType,
			},
			"ignore_changes": {
				Description:         "A list of paths (in gjson syntax) to the attributes that should not affect the resource after its creation.",
				MarkdownDescription: "A list of paths (in [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)) to the attributes that should not affect the resource after its creation.",
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
				Description:         "The method used to create the resource. Possible values are `PUT` and `POST`. This overrides the `create_method` set in the provider block (defaults to POST).",
				MarkdownDescription: "The method used to create the resource. Possible values are `PUT` and `POST`. This overrides the `create_method` set in the provider block (defaults to POST).",
				Type:                types.StringType,
				Optional:            true,
				Computed:            true,
				Validators:          []tfsdk.AttributeValidator{validator.StringInSlice("PUT", "POST")},
			},
			"query": {
				Description:         "The query parameters that are applied to each request. This overrides the `query` set in the provider block.",
				MarkdownDescription: "The query parameters that are applied to each request. This overrides the `query` set in the provider block.",
				Type:                types.MapType{ElemType: types.ListType{ElemType: types.StringType}},
				Optional:            true,
				Computed:            true,
			},
			"header": {
				Description:         "The header parameters that are applied to each request. This overrides the `header` set in the provider block.",
				MarkdownDescription: "The header parameters that are applied to each request. This overrides the `header` set in the provider block.",
				Type:                types.MapType{ElemType: types.StringType},
				Optional:            true,
				Computed:            true,
			},
			"output": {
				Description:         "The response body after reading the resource.",
				MarkdownDescription: "The response body after reading the resource.",
				Type:                types.StringType,
				Computed:            true,
			},
		},
	}, nil
}

func (r resource) ValidateConfig(ctx context.Context, req tfsdk.ValidateResourceConfigRequest, resp *tfsdk.ValidateResourceConfigResponse) {
	var config resourceData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	createMethod := r.p.apiOpt.CreateMethod
	if !config.CreateMethod.Unknown {
		if !config.CreateMethod.Null {
			createMethod = config.CreateMethod.Value
		}
		if !config.NamePath.Unknown && !config.UrlPath.Unknown {
			if createMethod == "PUT" {
				if !config.NamePath.Null {
					resp.Diagnostics.AddError(
						"Invalid configuration",
						"The `name_path` can not be specified when `create_method` is `PUT`",
					)
				}
				if !config.UrlPath.Null {
					resp.Diagnostics.AddError(
						"Invalid configuration",
						"The `url_path` can not be specified when `create_method` is `PUT`",
					)
				}
			} else if createMethod == "POST" {
				if config.NamePath.Null && config.UrlPath.Null || !config.NamePath.Null && !config.UrlPath.Null {
					resp.Diagnostics.AddError(
						"Invalid configuration",
						"Exactly one of `name_path` and `url_path` should be specified when `create_method` is `POST`",
					)
				}
			}
		}
	}

	validatePoll := func(pollObj types.Object, attrName string) {
		if pollObj.Null || pollObj.Unknown {
			return
		}
		var pd pollDataGo
		diags := pollObj.As(ctx, &pd, types.ObjectAsOptions{})
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}

		if _, err := parseLocator(pd.StatusLocator); err != nil {
			resp.Diagnostics.AddError(
				"Invalid configuration",
				fmt.Sprintf("Failed to parse status locator for %q: %s", attrName, err.Error()),
			)
		}

		if pd.UrlLocator != nil {
			if _, err := parseLocator(*pd.UrlLocator); err != nil {
				resp.Diagnostics.AddError(
					"Invalid configuration",
					fmt.Sprintf("Failed to parse url locator for %q: %s", attrName, err.Error()),
				)
			}
		}
	}

	validatePoll(config.PollCreate, "poll_create")
	validatePoll(config.PollUpdate, "poll_update")
	validatePoll(config.PollDelete, "poll_delete")

	if resp.Diagnostics.HasError() {
		return
	}
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
	NamePath      types.String `tfsdk:"name_path"`
	UrlPath       types.String `tfsdk:"url_path"`
	IgnoreChanges types.List   `tfsdk:"ignore_changes"`
	PollCreate    types.Object `tfsdk:"poll_create"`
	PollUpdate    types.Object `tfsdk:"poll_update"`
	PollDelete    types.Object `tfsdk:"poll_delete"`
	CreateMethod  types.String `tfsdk:"create_method"`
	Query         types.Map    `tfsdk:"query"`
	Header        types.Map    `tfsdk:"header"`
	Output        types.String `tfsdk:"output"`
}

type pollDataGo struct {
	StatusLocator string `tfsdk:"status_locator"`
	Status        struct {
		Success string   `tfsdk:"success"`
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
			"Error to call create",
			err.Error(),
		)
		return
	}
	if !response.IsSuccess() {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Create API returns %d", response.StatusCode()),
			string(response.Body()),
		)
		return
	}

	b := response.Body()

	// For POST create method, generate the resource id by combining the path and the id in response.
	var resourceId string
	switch opt.CreateMethod {
	case "POST":
		switch {
		case !plan.NamePath.Null:
			result := gjson.GetBytes(b, plan.NamePath.Value)
			if !result.Exists() {
				resp.Diagnostics.AddError(
					fmt.Sprintf("Failed to identify resource name"),
					fmt.Sprintf("Can't find resource name in path %q", plan.NamePath.Value),
				)
				return
			}
			resourceId = path.Join(plan.Path.Value, result.String())
		case !plan.UrlPath.Null:
			result := gjson.GetBytes(b, plan.UrlPath.Value)
			if !result.Exists() {
				resp.Diagnostics.AddError(
					fmt.Sprintf("Failed to identify resource id"),
					fmt.Sprintf("Can't find resource id in path %q", plan.UrlPath.Value),
				)
				return
			}
			resourceId = strings.TrimPrefix(result.String(), c.BaseURL)
		}
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
				"Create: Failed to build poller from the response of the initiated request",
				err.Error(),
			)
		}
		if err := p.PollUntilDone(ctx, c); err != nil {
			resp.Diagnostics.AddError(
				"Create: Polling failure",
				err.Error(),
			)
			return
		}
	}

	// Set overridable attributes from option to state
	plan.Query = opt.Query.ToTFValue()
	plan.Header = opt.Header.ToTFValue()
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
			"Error to call read",
			err.Error(),
		)
		return
	}
	if response.StatusCode() == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}
	if !response.IsSuccess() {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Read API returns %d", response.StatusCode()),
			string(response.Body()),
		)
		return
	}

	b := response.Body()

	var ignoreChanges []string
	// In case ignore_changes (O+C) is not set, set its default value as is defined in schema. This can avoid unnecessary plan diff after import.
	if state.IgnoreChanges.Null {
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

	var body string
	if strings.HasPrefix(state.Body.Value, __IMPORT_HEADER__) {
		// This branch is only invoked during `terraform import`.
		body, err = ModifyBodyForImport(strings.TrimPrefix(state.Body.Value, __IMPORT_HEADER__), string(b))
	} else {
		body, err = ModifyBody(state.Body.Value, string(b), ignoreChanges)
	}
	if err != nil {
		resp.Diagnostics.AddError(
			"Modifying `body` during Read",
			err.Error(),
		)
		return
	}
	// Set body, which is modified during read.
	state.Body = types.String{Value: string(body)}

	createMethod := r.p.apiOpt.CreateMethod
	if state.CreateMethod.Value != "" {
		createMethod = state.CreateMethod.Value
	}

	// Set force new properties
	switch createMethod {
	case "POST":
		state.Path = types.String{Value: filepath.Dir(state.ID.Value)}
	case "PUT":
		state.Path = types.String{Value: state.ID.Value}
	}

	// Set overridable (O+C) attributes from option to state
	state.Query = opt.Query.ToTFValue()
	state.Header = opt.Header.ToTFValue()
	state.CreateMethod = types.String{Value: createMethod}

	// Set computed attributes
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
			"Error to call update",
			err.Error(),
		)
		return
	}
	if !response.IsSuccess() {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Update API returns %d", response.StatusCode()),
			string(response.Body()),
		)
		return
	}

	// For LRO, wait for completion
	if opt.PollOpt != nil {
		p, err := client.NewPollable(*response, *opt.PollOpt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Update: Failed to build poller from the response of the initiated request",
				err.Error(),
			)
		}
		if err := p.PollUntilDone(ctx, c); err != nil {
			resp.Diagnostics.AddError(
				"Update: Polling failure",
				err.Error(),
			)
		}
	}

	// Set overridable attributes from option to state
	plan.Query = opt.Query.ToTFValue()
	plan.Header = opt.Header.ToTFValue()
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
			"Error to call delete",
			err.Error(),
		)
		return
	}
	if response.StatusCode() == http.StatusNotFound {
		return
	}

	if !response.IsSuccess() {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Delete API returns %d", response.StatusCode()),
			string(response.Body()),
		)
		return
	}

	// For LRO, wait for completion
	if opt.PollOpt != nil {
		p, err := client.NewPollable(*response, *opt.PollOpt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Delete: Failed to build poller from the response of the initiated request",
				err.Error(),
			)
		}
		if err := p.PollUntilDone(ctx, c); err != nil {
			resp.Diagnostics.AddError(
				"Delete: Polling failure",
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

	// Header is only required when it is mandatory for reading the resource.
	Header url.Values `json:"header"`

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
	headerPath := tftypes.NewAttributePath().WithAttributeName("header")
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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, headerPath, imp.Header)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, createMethodPath, imp.CreateMethod)...)
}
