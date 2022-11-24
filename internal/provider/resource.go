package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	tfpath "github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/magodo/terraform-provider-restful/internal/client"
	"github.com/magodo/terraform-provider-restful/internal/planmodifier"
	"github.com/magodo/terraform-provider-restful/internal/validator"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/tidwall/gjson"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Magic header used to indicate the value in the state is derived from import.
const __IMPORT_HEADER__ = "__RESTFUL_PROVIDER__"

type Resource struct {
	p *Provider
}

var _ resource.Resource = &Resource{}

type resourceData struct {
	ID types.String `tfsdk:"id"`

	Path       types.String `tfsdk:"path"`
	ReadPath   types.String `tfsdk:"read_path"`
	UpdatePath types.String `tfsdk:"update_path"`
	DeletePath types.String `tfsdk:"delete_path"`

	CreateMethod types.String `tfsdk:"create_method"`
	UpdateMethod types.String `tfsdk:"update_method"`
	DeleteMethod types.String `tfsdk:"delete_method"`

	Body                types.String `tfsdk:"body"`
	WriteOnlyAttributes types.List   `tfsdk:"write_only_attrs"`

	PollCreate types.Object `tfsdk:"poll_create"`
	PollUpdate types.Object `tfsdk:"poll_update"`
	PollDelete types.Object `tfsdk:"poll_delete"`

	MergePatchDisabled types.Bool `tfsdk:"merge_patch_disabled"`
	Query              types.Map  `tfsdk:"query"`
	Header             types.Map  `tfsdk:"header"`

	Output types.String `tfsdk:"output"`
}

type pollData struct {
	StatusLocator types.String `tfsdk:"status_locator"`
	Status        types.Object `tfsdk:"status"`
	UrlLocator    types.String `tfsdk:"url_locator"`
	DefaultDelay  types.Int64  `tfsdk:"default_delay_sec"`
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

func (r *Resource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource"
}

func pollAttribute(attr, s string) tfsdk.Attribute {
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
				Description:         "Specifies how to discover the polling url. The format is as `<k>[<v>]`, which can be one of `header[path]` (use the property at `path` in response header), `body[path]` (use the property at `path` in response body) or `exact[value]` (use the exact `value`). When absent, the resource's path is used for polling.",
				MarkdownDescription: "Specifies how to discover the polling url. The format is as `<k>[<v>]`, which can be one of `header[path]` (use the property at `path` in response header), `body[path]` (use the property at `path` in response body) or `exact[value]` (use the exact `value`). When absent, the resource's path is used for polling.",
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

func (r *Resource) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	pathDescription := "The path can be string literal, or combined by followings: `$(path)` expanded to `path`, `$(body.x.y.z)` expands to the `x.y.z` property in API body, `#(body.id)` expands to the `id` property, with `base_url` prefix trimmed."
	return tfsdk.Schema{
		Description:         "`restful_resource` manages a restful resource.",
		MarkdownDescription: "`restful_resource` manages a restful resource.",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Description:         "The ID of the Resource.",
				MarkdownDescription: "The ID of the Resource.",
				Type:                types.StringType,
				Computed:            true,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.UseStateForUnknown(),
				},
			},
			"path": {
				Description:         "The path used to create the resource, relative to the `base_url` of the provider.",
				MarkdownDescription: "The path used to create the resource, relative to the `base_url` of the provider.",
				Type:                types.StringType,
				Required:            true,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.RequiresReplace(),
				},
			},
			"read_path": {
				Description:         "The API path used to read the resource, which is used as the `id`. The `path` is used as the `id` instead if `read_path` is absent. " + pathDescription,
				MarkdownDescription: "The API path used to read the resource, which is used as the `id`. The `path` is used as the `id` instead if `read_path` is absent. " + pathDescription,
				Optional:            true,
				Type:                types.StringType,
			},
			"update_path": {
				Description:         "The API path used to update the resource. The `id` is used instead if `update_path` is absent. " + pathDescription,
				MarkdownDescription: "The API path used to update the resource. The `id` is used instead if `update_path` is absent. " + pathDescription,
				Optional:            true,
				Type:                types.StringType,
			},
			"delete_path": {
				Description:         "The API path used to delete the resource. The `id` is used instead if `delete_path` is absent. " + pathDescription,
				MarkdownDescription: "The API path used to delete the resource. The `id` is used instead if `delete_path` is absent. " + pathDescription,
				Optional:            true,
				Type:                types.StringType,
			},

			"body": {
				Description:         "The properties of the resource.",
				MarkdownDescription: "The properties of the resource.",
				Type:                types.StringType,
				Required:            true,
			},
			"poll_create": pollAttribute("poll_create", "Create"),
			"poll_update": pollAttribute("poll_update", "Update"),
			"poll_delete": pollAttribute("poll_delete", "Delete"),

			"write_only_attrs": {
				Description:         "A list of paths (in gjson syntax) to the attributes that are only settable, but won't be read in GET response.",
				MarkdownDescription: "A list of paths (in [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)) to the attributes that are only settable, but won't be read in GET response.",
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
			"update_method": {
				Description:         "The method used to update the resource. Possible values are `PUT` and `PATCH`. This overrides the `update_method` set in the provider block (defaults to PUT).",
				MarkdownDescription: "The method used to update the resource. Possible values are `PUT` and `PATCH`. This overrides the `update_method` set in the provider block (defaults to PUT).",
				Type:                types.StringType,
				Optional:            true,
				Computed:            true,
				Validators:          []tfsdk.AttributeValidator{validator.StringInSlice("PUT", "PATCH")},
			},
			"delete_method": {
				Description:         "The method used to delete the resource. Possible values are `DELETE` and `POST`. This overrides the `delete_method` set in the provider block (defaults to DELETE).",
				MarkdownDescription: "The method used to delete the resource. Possible values are `DELETE` and `POST`. This overrides the `delete_method` set in the provider block (defaults to DELETE).",
				Type:                types.StringType,
				Optional:            true,
				Computed:            true,
				Validators:          []tfsdk.AttributeValidator{validator.StringInSlice("DELETE", "POST")},
			},
			"merge_patch_disabled": {
				Description:         "Whether to use a JSON Merge Patch as the request body in the PATCH update? This is only effective when `update_method` is set to `PATCH`. This overrides the `merge_patch_disabled` set in the provider block (defaults to `false`).",
				MarkdownDescription: "Whether to use a JSON Merge Patch as the request body in the PATCH update? This is only effective when `update_method` is set to `PATCH`. This overrides the `merge_patch_disabled` set in the provider block (defaults to `false`).",
				Type:                types.BoolType,
				Optional:            true,
				Computed:            true,
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

func validatePoll(ctx context.Context, pollObj types.Object, attrName string, resp *resource.ValidateConfigResponse) {
	if pollObj.IsNull() || pollObj.IsUnknown() {
		return
	}
	var pd pollData
	diags := pollObj.As(ctx, &pd, types.ObjectAsOptions{})
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if !pd.StatusLocator.IsUnknown() && !pd.StatusLocator.IsNull() {
		if _, err := parseLocator(pd.StatusLocator.ValueString()); err != nil {
			resp.Diagnostics.AddError(
				"Invalid configuration",
				fmt.Sprintf("Failed to parse status locator for %q: %s", attrName, err.Error()),
			)
		}
	}

	if !pd.UrlLocator.IsUnknown() && !pd.UrlLocator.IsNull() {
		if _, err := parseLocator(pd.UrlLocator.ValueString()); err != nil {
			resp.Diagnostics.AddError(
				"Invalid configuration",
				fmt.Sprintf("Failed to parse url locator for %q: %s", attrName, err.Error()),
			)
		}
	}
}

func (r *Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config resourceData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	validatePoll(ctx, config.PollCreate, "poll_create", resp)
	validatePoll(ctx, config.PollUpdate, "poll_update", resp)
	validatePoll(ctx, config.PollDelete, "poll_delete", resp)

	if !config.Body.IsUnknown() {
		var body map[string]interface{}
		if err := json.Unmarshal([]byte(config.Body.ValueString()), &body); err != nil {
			resp.Diagnostics.AddError(
				"Invalid configuration",
				fmt.Sprintf(`Failed to unmarshal "body": %s: %s`, err.Error(), config.Body.String()),
			)
		}

		if !config.WriteOnlyAttributes.IsUnknown() && !config.WriteOnlyAttributes.IsNull() {
			for _, ie := range config.WriteOnlyAttributes.Elements() {
				ie := ie.(types.String)
				if !ie.IsUnknown() && !ie.IsNull() {
					if !gjson.Get(config.Body.ValueString(), ie.ValueString()).Exists() {
						resp.Diagnostics.AddError(
							"Invalid configuration",
							fmt.Sprintf(`Invalid path in "write_only_attrs": %s`, ie.String()),
						)
					}
				}
			}
		}
	}
}

func (r *Resource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.p = &Provider{}
	if req.ProviderData != nil {
		r.p = req.ProviderData.(*Provider)
	}
	return
}

func (r Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
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
	if opt.Method == "PUT" {
		opt, diags := r.p.apiOpt.ForResourceRead(ctx, plan)
		resp.Diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}
		response, err := c.Read(ctx, plan.Path.ValueString(), *opt)
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
				fmt.Sprintf("A resource with the ID %q already exists - to be managed via Terraform this resource needs to be imported into the State. Please see the resource documentation for %q for more information.", plan.Path.ValueString(), `restful_resource`),
			)
			return
		}
	}

	// Create the resource
	response, err := c.Create(ctx, plan.Path.ValueString(), plan.Body.ValueString(), *opt)
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

	// Construct the resource id, which is used as the path to read the resource later on. By default, it is the same as the "path", unless "read_path" is specified.
	resourceId := plan.Path.ValueString()
	if !plan.ReadPath.IsNull() {
		resourceId, err = BuildPath(plan.ReadPath.ValueString(), r.p.client.BaseURL, plan.Path.ValueString(), b)
		if err != nil {
			resp.Diagnostics.AddError(
				fmt.Sprintf("Failed to build the path for reading the resource"),
				fmt.Sprintf("Can't build resource id with `read_path`: %q, `path`: %q, `body`: %q", plan.ReadPath.ValueString(), plan.Path.ValueString(), string(b)),
			)
			return
		}
	}

	// Set the value for overridable (O+C) attributes in plan, which will affect read
	plan.Query = opt.Query.ToTFValue()
	plan.Header = opt.Header.ToTFValue()
	// create_method is already resolved in the create opt here
	plan.CreateMethod = types.String{Value: opt.Method}
	// Since the update_method is O+C, it is unknown in the plan when not specified.
	if plan.UpdateMethod.IsUnknown() {
		plan.UpdateMethod = types.String{Value: r.p.apiOpt.UpdateMethod}
	}
	// Since the delete is O+C, it is unknown in the plan when not specified.
	if plan.DeleteMethod.IsUnknown() {
		plan.DeleteMethod = types.String{Value: r.p.apiOpt.DeleteMethod}
	}
	// Since the merge_patch_disabled is O+C, it is unknown in the plan when not specified.
	if plan.MergePatchDisabled.IsUnknown() {
		plan.MergePatchDisabled = types.Bool{Value: r.p.apiOpt.MergePatchDisabled}
	}

	// Set resource ID
	plan.ID = types.String{Value: resourceId}

	// Early set the state using the plan. There is another state setting in the read right after the polling (if any).
	// Here is mainly for setting the resource id to the state, in order to avoid resource halfly created not tracked by terraform.
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	// For LRO, wait for completion
	if opt.PollOpt != nil {
		if opt.PollOpt.UrlLocator == nil {
			// Update the request URL to pointing to the resource path, which is mainly for resources whose create method is POST.
			// As it will be used to poll the resource status.
			response.Request.RawRequest.URL.Path = resourceId
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

	rreq := resource.ReadRequest{
		State:        resp.State,
		ProviderMeta: req.ProviderMeta,
	}
	rresp := resource.ReadResponse{
		State:       resp.State,
		Diagnostics: resp.Diagnostics,
	}
	r.Read(ctx, rreq, &rresp)

	*resp = resource.CreateResponse{
		State:       rresp.State,
		Diagnostics: rresp.Diagnostics,
	}
}

func (r Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
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

	response, err := c.Read(ctx, state.ID.ValueString(), *opt)
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

	var writeOnlyAttributes []string
	// In case write_only_attrs (O+C) is not set, set its default value as is defined in schema. This can avoid unnecessary plan diff after import.
	if state.WriteOnlyAttributes.IsNull() {
		state.WriteOnlyAttributes = types.List{
			ElemType: types.StringType,
			Elems:    []attr.Value{},
		}
	}
	diags = state.WriteOnlyAttributes.ElementsAs(ctx, &writeOnlyAttributes, false)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	var body string
	if strings.HasPrefix(state.Body.ValueString(), __IMPORT_HEADER__) {
		// This branch is only invoked during `terraform import`.
		body, err = ModifyBodyForImport(strings.TrimPrefix(state.Body.ValueString(), __IMPORT_HEADER__), string(b))
	} else {
		body, err = ModifyBody(state.Body.ValueString(), string(b), writeOnlyAttributes)
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
	if state.CreateMethod.ValueString() != "" {
		createMethod = state.CreateMethod.ValueString()
	}

	updateMethod := r.p.apiOpt.UpdateMethod
	if state.UpdateMethod.ValueString() != "" {
		updateMethod = state.UpdateMethod.ValueString()
	}

	deleteMethod := r.p.apiOpt.DeleteMethod
	if state.DeleteMethod.ValueString() != "" {
		deleteMethod = state.DeleteMethod.ValueString()
	}

	mergePatchDisabled := r.p.apiOpt.MergePatchDisabled
	if !state.MergePatchDisabled.IsNull() {
		mergePatchDisabled = state.MergePatchDisabled.ValueBool()
	}

	// Set overridable (O+C) attributes from option to state
	state.Query = opt.Query.ToTFValue()
	state.Header = opt.Header.ToTFValue()
	state.CreateMethod = types.String{Value: createMethod}
	state.UpdateMethod = types.String{Value: updateMethod}
	state.DeleteMethod = types.String{Value: deleteMethod}
	state.MergePatchDisabled = types.Bool{Value: mergePatchDisabled}

	// Set computed attributes
	state.Output = types.String{Value: string(b)}

	diags = resp.State.Set(ctx, state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
}

func (r Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
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

	// Invoke API to Update the resource only when there are changes in the body.
	if state.Body.ValueString() != plan.Body.ValueString() {
		body := plan.Body.ValueString()
		if opt.Method == "PATCH" && !opt.MergePatchDisabled {
			b, err := jsonpatch.CreateMergePatch([]byte(state.Body.ValueString()), []byte(plan.Body.ValueString()))
			if err != nil {
				resp.Diagnostics.AddError(
					"Update failure",
					fmt.Sprintf("failed to create a merge patch: %s", err.Error()),
				)
				return
			}
			body = string(b)
		}

		path := plan.ID.ValueString()
		if !plan.UpdatePath.IsNull() {
			var err error
			path, err = BuildPath(plan.UpdatePath.ValueString(), r.p.client.BaseURL, plan.Path.ValueString(), []byte(state.Output.ValueString()))
			if err != nil {
				resp.Diagnostics.AddError(
					fmt.Sprintf("Failed to build the path for updating the resource"),
					fmt.Sprintf("Can't build path with `update_path`: %q, `path`: %q, `body`: %q", plan.UpdatePath.ValueString(), plan.Path.ValueString(), string(state.Output.ValueString())),
				)
				return
			}
		}

		response, err := c.Update(ctx, path, body, *opt)
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
	}

	// Set the value for overridable (O+C) attributes in plan, which will affect read
	plan.Query = opt.Query.ToTFValue()
	plan.Header = opt.Header.ToTFValue()
	// update_method is already resolved in the update opt here
	plan.UpdateMethod = types.String{Value: opt.Method}
	// Since the create_method is O+C, it is unknown in the plan when not specified.
	if plan.CreateMethod.IsUnknown() {
		plan.CreateMethod = types.String{Value: r.p.apiOpt.CreateMethod}
	}
	// Since the delete is O+C, it is unknown in the plan when not specified.
	if plan.DeleteMethod.IsUnknown() {
		plan.DeleteMethod = types.String{Value: r.p.apiOpt.DeleteMethod}
	}
	// merge_patch_disabled is already resolved in the update opt here
	plan.MergePatchDisabled = types.Bool{Value: opt.MergePatchDisabled}

	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	rreq := resource.ReadRequest{
		State:        resp.State,
		ProviderMeta: req.ProviderMeta,
	}
	rresp := resource.ReadResponse{
		State:       resp.State,
		Diagnostics: resp.Diagnostics,
	}
	r.Read(ctx, rreq, &rresp)

	*resp = resource.UpdateResponse{
		State:       rresp.State,
		Diagnostics: rresp.Diagnostics,
	}
}

func (r Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
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

	path := state.ID.ValueString()
	if !state.DeletePath.IsNull() {
		var err error
		path, err = BuildPath(state.DeletePath.ValueString(), r.p.client.BaseURL, state.Path.ValueString(), []byte(state.Output.ValueString()))
		if err != nil {
			resp.Diagnostics.AddError(
				fmt.Sprintf("Failed to build the path for deleting the resource"),
				fmt.Sprintf("Can't build path with `delete_path`: %q, `path`: %q, `body`: %q", state.DeletePath.ValueString(), state.Path.ValueString(), string(state.Output.ValueString())),
			)
			return
		}
	}

	response, err := c.Delete(ctx, path, *opt)
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

	// Path is the path used to create the resource.
	Path string `json:"path"`

	// UpdatePath is the path used to update the resource
	UpdatePath *string `json:"update_path"`

	// DeletePath is the path used to delte the resource
	DeletePath *string `json:"delete_path"`

	// Query is only required when it is mandatory for reading the resource.
	Query url.Values `json:"query"`

	// Header is only required when it is mandatory for reading the resource.
	Header url.Values `json:"header"`

	CreateMethod *string `json:"create_method"`
	UpdateMethod *string `json:"update_method"`
	DeleteMethod *string `json:"delete_method"`

	// Body represents the properties expected to be managed and tracked by Terraform. The value of these properties can be null as a place holder.
	// When absent, all the response payload read wil be set to `body`.
	Body map[string]interface{}
}

func (Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idPath := tfpath.Root("id")
	path := tfpath.Root("path")
	updatePath := tfpath.Root("update_path")
	deletePath := tfpath.Root("delete_path")
	queryPath := tfpath.Root("query")
	headerPath := tfpath.Root("header")
	createMethodPath := tfpath.Root("create_method")
	updateMethodPath := tfpath.Root("update_method")
	deleteMethodPath := tfpath.Root("delete_method")
	bodyPath := tfpath.Root("body")

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
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path, imp.Path)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, updatePath, imp.UpdatePath)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, deletePath, imp.DeletePath)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, queryPath, imp.Query)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, headerPath, imp.Header)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, createMethodPath, imp.CreateMethod)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, updateMethodPath, imp.UpdateMethod)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, deleteMethodPath, imp.DeleteMethod)...)
}
