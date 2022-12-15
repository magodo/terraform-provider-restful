package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	tfpath "github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/magodo/terraform-provider-restful/internal/buildpath"
	"github.com/magodo/terraform-provider-restful/internal/client"
	myplanmodifier "github.com/magodo/terraform-provider-restful/internal/planmodifier"
	myvalidator "github.com/magodo/terraform-provider-restful/internal/validator"
	"github.com/tidwall/gjson"
)

// Magic header used to indicate the value in the state is derived from import.
const __IMPORT_HEADER__ = "__RESTFUL_PROVIDER__"

type Resource struct {
	p *Provider
}

var _ resource.Resource = &Resource{}

type resourceData struct {
	ID types.String `tfsdk:"id"`

	Path types.String `tfsdk:"path"`

	ReadBodyLocator types.String `tfsdk:"read_body_locator"`

	ReadPath   types.String `tfsdk:"read_path"`
	UpdatePath types.String `tfsdk:"update_path"`
	DeletePath types.String `tfsdk:"delete_path"`

	CreateMethod types.String `tfsdk:"create_method"`
	UpdateMethod types.String `tfsdk:"update_method"`
	DeleteMethod types.String `tfsdk:"delete_method"`

	PrecheckCreate types.Object `tfsdk:"precheck_create"`
	PrecheckUpdate types.Object `tfsdk:"precheck_update"`
	PrecheckDelete types.Object `tfsdk:"precheck_delete"`

	Body                types.String `tfsdk:"body"`
	WriteOnlyAttributes types.List   `tfsdk:"write_only_attrs"`

	PollCreate types.Object `tfsdk:"poll_create"`
	PollUpdate types.Object `tfsdk:"poll_update"`
	PollDelete types.Object `tfsdk:"poll_delete"`

	MergePatchDisabled types.Bool `tfsdk:"merge_patch_disabled"`
	Query              types.Map  `tfsdk:"query"`
	Header             types.Map  `tfsdk:"header"`

	CheckExistance types.Bool `tfsdk:"check_existance"`
	ForceNewAttrs  types.Set  `tfsdk:"force_new_attrs"`

	Output types.String `tfsdk:"output"`
}

type pollData struct {
	StatusLocator types.String `tfsdk:"status_locator"`
	Status        types.Object `tfsdk:"status"`
	UrlLocator    types.String `tfsdk:"url_locator"`
	Header        types.Map    `tfsdk:"header"`
	DefaultDelay  types.Int64  `tfsdk:"default_delay_sec"`
}

type precheckData struct {
	StatusLocator types.String `tfsdk:"status_locator"`
	Status        types.Object `tfsdk:"status"`
	Path          types.String `tfsdk:"path"`
	Query         types.Map    `tfsdk:"query"`
	Header        types.Map    `tfsdk:"header"`
	DefaultDelay  types.Int64  `tfsdk:"default_delay_sec"`
}

type pollStatusGo struct {
	Success string   `tfsdk:"success"`
	Pending []string `tfsdk:"pending"`
}

func (r *Resource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource"
}

func precheckAttribute(s string, pathIsRequired bool, suffixDesc string) schema.Attribute {
	pathDesc := "The path used to query readiness, relative to the `base_url` of the provider."
	if suffixDesc != "" {
		pathDesc += " " + suffixDesc
	}

	return schema.SingleNestedAttribute{
		Description:         fmt.Sprintf("The precheck that is prior to the %q operation.", s),
		MarkdownDescription: fmt.Sprintf("The precheck that is prior to the %q operation.", s),
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"status_locator": schema.StringAttribute{
				Description:         "Specifies how to discover the status property. The format is either `code` or `scope.path`, where `scope` can be either `header` or `body`, and the `path` is using the gjson syntax.",
				MarkdownDescription: "Specifies how to discover the status property. The format is either `code` or `scope.path`, where `scope` can be either `header` or `body`, and the `path` is using the [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md).",
				Required:            true,
				Validators: []validator.String{
					myvalidator.StringIsParsable("locator", func(s string) error {
						_, err := parseLocator(s)
						return err
					}),
				},
			},
			"status": schema.SingleNestedAttribute{
				Description:         "The expected status sentinels for each polling state.",
				MarkdownDescription: "The expected status sentinels for each polling state.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"success": schema.StringAttribute{
						Description:         "The expected status sentinel for suceess status.",
						MarkdownDescription: "The expected status sentinel for suceess status.",
						Required:            true,
					},
					"pending": schema.ListAttribute{
						Description:         "The expected status sentinels for pending status.",
						MarkdownDescription: "The expected status sentinels for pending status.",
						Optional:            true,
						ElementType:         types.StringType,
					},
				},
			},
			"path": schema.StringAttribute{
				Description:         pathDesc,
				MarkdownDescription: pathDesc,
				Required:            pathIsRequired,
				Optional:            !pathIsRequired,
			},
			"query": schema.MapAttribute{
				Description:         "The query parameters. This overrides the `query` set in the resource block.",
				MarkdownDescription: "The query parameters. This overrides the `query` set in the resource block.",
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
			},
			"header": schema.MapAttribute{
				Description:         "The header parameters. This overrides the `header` set in the resource block.",
				MarkdownDescription: "The header parameters. This overrides the `header` set in the resource block.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"default_delay_sec": schema.Int64Attribute{
				Description:         "The interval between two pollings if there is no `Retry-After` in the response header, in second.",
				MarkdownDescription: "The interval between two pollings if there is no `Retry-After` in the response header, in second.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					myplanmodifier.DefaultAttribute(types.Int64Value(10)),
				},
			},
		},
	}
}

func pollAttribute(s string) schema.Attribute {
	return schema.SingleNestedAttribute{
		Description:         fmt.Sprintf("The polling option for the %q operation", s),
		MarkdownDescription: fmt.Sprintf("The polling option for the %q operation", s),
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"status_locator": schema.StringAttribute{
				Description:         "Specifies how to discover the status property. The format is either `code` or `scope.path`, where `scope` can be either `header` or `body`, and the `path` is using the gjson syntax.",
				MarkdownDescription: "Specifies how to discover the status property. The format is either `code` or `scope.path`, where `scope` can be either `header` or `body`, and the `path` is using the [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md).",
				Required:            true,
				Validators: []validator.String{
					myvalidator.StringIsParsable("locator", func(s string) error {
						_, err := parseLocator(s)
						return err
					}),
				},
			},
			"status": schema.SingleNestedAttribute{
				Description:         "The expected status sentinels for each polling state.",
				MarkdownDescription: "The expected status sentinels for each polling state.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"success": schema.StringAttribute{
						Description:         "The expected status sentinel for suceess status.",
						MarkdownDescription: "The expected status sentinel for suceess status.",
						Required:            true,
					},
					"pending": schema.ListAttribute{
						Description:         "The expected status sentinels for pending status.",
						MarkdownDescription: "The expected status sentinels for pending status.",
						Optional:            true,
						ElementType:         types.StringType,
					},
				},
			},
			"url_locator": schema.StringAttribute{
				Description:         "Specifies how to discover the polling url. The format can be one of `header.path` (use the property at `path` in response header), `body.path` (use the property at `path` in response body) or `exact.value` (use the exact `value`). When absent, the resource's path is used for polling.",
				MarkdownDescription: "Specifies how to discover the polling url. The format can be one of `header.path` (use the property at `path` in response header), `body.path` (use the property at `path` in response body) or `exact.value` (use the exact `value`). When absent, the resource's path is used for polling.",
				Optional:            true,
				Validators: []validator.String{
					myvalidator.StringIsParsable("locator", func(s string) error {
						_, err := parseLocator(s)
						return err
					}),
				},
			},
			"header": schema.MapAttribute{
				Description:         "The header parameters. This overrides the `header` set in the resource block.",
				MarkdownDescription: "The header parameters. This overrides the `header` set in the resource block.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"default_delay_sec": schema.Int64Attribute{
				Description:         "The interval between two pollings if there is no `Retry-After` in the response header, in second.",
				MarkdownDescription: "The interval between two pollings if there is no `Retry-After` in the response header, in second.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					myplanmodifier.DefaultAttribute(types.Int64Value(10)),
				},
			},
		},
	}
}

func (r *Resource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	const pathDescription = "The path can be string literal, or combined by followings: `$(path)` expanded to `path`, `$(body.x.y.z)` expands to the `x.y.z` property (urlencoded) in API body, `#(body.id)` expands to the `id` property, with `base_url` prefix trimmed."
	resp.Schema = schema.Schema{
		Description:         "`restful_resource` manages a restful resource.",
		MarkdownDescription: "`restful_resource` manages a restful resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:         "The ID of the Resource.",
				MarkdownDescription: "The ID of the Resource.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"path": schema.StringAttribute{
				Description:         "The path used to create the resource, relative to the `base_url` of the provider.",
				MarkdownDescription: "The path used to create the resource, relative to the `base_url` of the provider.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"read_body_locator": schema.StringAttribute{
				Description:         "Specifies how to locate the resource body in the read response. The format is `body.path`, where the `path` is using the gjson syntax.",
				MarkdownDescription: "Specifies how to locate the resource body in the read response. The format is `body.path`, where the `path` is using the gjson syntax[gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md).",
				Optional:            true,
				Validators: []validator.String{
					myvalidator.StringIsParsable("locator", func(s string) error {
						_, err := parseLocator(s)
						return err
					}),
				},
			},

			"read_path": schema.StringAttribute{
				Description:         "The API path used to read the resource, which is used as the `id`. The `path` is used as the `id` instead if `read_path` is absent. " + pathDescription,
				MarkdownDescription: "The API path used to read the resource, which is used as the `id`. The `path` is used as the `id` instead if `read_path` is absent. " + pathDescription,
				Optional:            true,
				Validators: []validator.String{
					myvalidator.StringIsPathBuilder(),
				},
			},
			"update_path": schema.StringAttribute{
				Description:         "The API path used to update the resource. The `id` is used instead if `update_path` is absent. " + pathDescription,
				MarkdownDescription: "The API path used to update the resource. The `id` is used instead if `update_path` is absent. " + pathDescription,
				Optional:            true,
				Validators: []validator.String{
					myvalidator.StringIsPathBuilder(),
				},
			},
			"delete_path": schema.StringAttribute{
				Description:         "The API path used to delete the resource. The `id` is used instead if `delete_path` is absent. " + pathDescription,
				MarkdownDescription: "The API path used to delete the resource. The `id` is used instead if `delete_path` is absent. " + pathDescription,
				Optional:            true,
				Validators: []validator.String{
					myvalidator.StringIsPathBuilder(),
				},
			},

			"body": schema.StringAttribute{
				Description:         "The properties of the resource.",
				MarkdownDescription: "The properties of the resource.",
				Required:            true,
				Validators: []validator.String{
					myvalidator.StringIsJSON(),
				},
			},

			"poll_create": pollAttribute("Create"),
			"poll_update": pollAttribute("Update"),
			"poll_delete": pollAttribute("Delete"),

			"precheck_create": precheckAttribute("Create", true, ""),
			"precheck_update": precheckAttribute("Update", false, "By default, the `id` of this resource is used."),
			"precheck_delete": precheckAttribute("Delete", false, "By default, the `id` of this resource is used."),

			"write_only_attrs": schema.ListAttribute{
				Description:         "A list of paths (in gjson syntax) to the attributes that are only settable, but won't be read in GET response.",
				MarkdownDescription: "A list of paths (in [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)) to the attributes that are only settable, but won't be read in GET response.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"create_method": schema.StringAttribute{
				Description:         "The method used to create the resource. Possible values are `PUT` and `POST`. This overrides the `create_method` set in the provider block (defaults to POST).",
				MarkdownDescription: "The method used to create the resource. Possible values are `PUT` and `POST`. This overrides the `create_method` set in the provider block (defaults to POST).",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("PUT", "POST"),
				},
			},
			"update_method": schema.StringAttribute{
				Description:         "The method used to update the resource. Possible values are `PUT`, `POST` and `PATCH`. This overrides the `update_method` set in the provider block (defaults to PUT).",
				MarkdownDescription: "The method used to update the resource. Possible values are `PUT`, `POST`, and `PATCH`. This overrides the `update_method` set in the provider block (defaults to PUT).",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("PUT", "PATCH", "POST"),
				},
			},
			"delete_method": schema.StringAttribute{
				Description:         "The method used to delete the resource. Possible values are `DELETE` and `POST`. This overrides the `delete_method` set in the provider block (defaults to DELETE).",
				MarkdownDescription: "The method used to delete the resource. Possible values are `DELETE` and `POST`. This overrides the `delete_method` set in the provider block (defaults to DELETE).",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("DELETE", "POST"),
				},
			},
			"merge_patch_disabled": schema.BoolAttribute{
				Description:         "Whether to use a JSON Merge Patch as the request body in the PATCH update? This is only effective when `update_method` is set to `PATCH`. This overrides the `merge_patch_disabled` set in the provider block (defaults to `false`).",
				MarkdownDescription: "Whether to use a JSON Merge Patch as the request body in the PATCH update? This is only effective when `update_method` is set to `PATCH`. This overrides the `merge_patch_disabled` set in the provider block (defaults to `false`).",
				Optional:            true,
			},
			"query": schema.MapAttribute{
				Description:         "The query parameters that are applied to each request. This overrides the `query` set in the provider block.",
				MarkdownDescription: "The query parameters that are applied to each request. This overrides the `query` set in the provider block.",
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
			},
			"header": schema.MapAttribute{
				Description:         "The header parameters that are applied to each request. This overrides the `header` set in the provider block.",
				MarkdownDescription: "The header parameters that are applied to each request. This overrides the `header` set in the provider block.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"check_existance": schema.BoolAttribute{
				Description:         "Whether to check resource already existed? Defaults to `false`.",
				MarkdownDescription: "Whether to check resource already existed? Defaults to `false`.",
				Optional:            true,
			},
			"force_new_attrs": schema.SetAttribute{
				Description:         "A set of `body` attribute paths (in gjson syntax) whose value once changed, will trigger a replace of this resource. Note this only take effects when the `body` is a unknown before apply. Technically, we do a JSON merge patch and check whether the attribute path appear in the merge patch.",
				MarkdownDescription: "A set of `body` attribute paths (in [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)) whose value once changed, will trigger a replace of this resource. Note this only take effects when the `body` is a unknown before apply. Technically, we do a JSON merge patch and check whether the attribute path appear in the merge patch.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"output": schema.StringAttribute{
				Description:         "The response body after reading the resource.",
				MarkdownDescription: "The response body after reading the resource.",
				Computed:            true,
			},
		},
	}
}

func (r *Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config resourceData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if !config.Body.IsUnknown() {
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

func (r *Resource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		// // If the entire plan is null, the resource is planned for destruction.
		return
	}

	if req.State.Raw.IsNull() {
		// // If the entire state is null, the resource is planned for creation.
		return
	}
	var plan resourceData
	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if !plan.ForceNewAttrs.IsUnknown() && !plan.Body.IsUnknown() {
		var forceNewAttrs []types.String
		if diags := plan.ForceNewAttrs.ElementsAs(ctx, &forceNewAttrs, false); diags != nil {
			resp.Diagnostics.Append(diags...)
			return
		}
		var knownForceNewAttrs []string
		for _, attr := range forceNewAttrs {
			if attr.IsUnknown() {
				continue
			}
			knownForceNewAttrs = append(knownForceNewAttrs, attr.ValueString())
		}

		if len(knownForceNewAttrs) != 0 {
			var state resourceData
			if diags := req.State.Get(ctx, &state); diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}

			originJson := state.Body.ValueString()
			if originJson == "" {
				originJson = "{}"
			}

			modifiedJson := plan.Body.ValueString()
			if modifiedJson == "" {
				modifiedJson = "{}"
			}
			patch, err := jsonpatch.CreateMergePatch([]byte(originJson), []byte(modifiedJson))
			if err != nil {
				resp.Diagnostics.AddError("failed to create merge patch", err.Error())
				return
			}

			for _, attr := range knownForceNewAttrs {
				result := gjson.Get(string(patch), attr)
				if result.Exists() {
					resp.RequiresReplace = []tfpath.Path{tfpath.Root("body")}
					break
				}
			}
		}
	}
}

func (r *Resource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.p = &Provider{}
	if req.ProviderData != nil {
		p, diags := req.ProviderData.(providerData).ConfigureProvider(ctx)
		resp.Diagnostics.Append(diags...)
		r.p = p
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

	if plan.CheckExistance.ValueBool() {
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

	// Precheck
	if !plan.PrecheckCreate.IsNull() {
		var d precheckData
		if diags := plan.PrecheckCreate.As(ctx, &d, basetypes.ObjectAsOptions{}); diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		opt, diags := r.p.apiOpt.ForPrecheck(ctx, "", opt.Header, opt.Query, d)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		p, err := client.NewPollable(*opt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Create: Failed to build poller for precheck",
				err.Error(),
			)
			return
		}
		if err := p.PollUntilDone(ctx, c); err != nil {
			resp.Diagnostics.AddError(
				"Create: Pre-checking failure",
				err.Error(),
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
		resourceId, err = buildpath.BuildPath(plan.ReadPath.ValueString(), r.p.apiOpt.BaseURL.String(), plan.Path.ValueString(), b)
		if err != nil {
			resp.Diagnostics.AddError(
				fmt.Sprintf("Failed to build the path for reading the resource"),
				fmt.Sprintf("Can't build resource id with `read_path`: %q, `path`: %q, `body`: %q: %v", plan.ReadPath.ValueString(), plan.Path.ValueString(), string(b), err),
			)
			return
		}
	}

	// Set resource ID
	plan.ID = types.StringValue(resourceId)

	// Early set the state using the plan. There is another state setting in the read right after the polling (if any).
	// Here is mainly for setting the resource id to the state, in order to avoid resource halfly created not tracked by terraform.
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	// For LRO, wait for completion
	if !plan.PollCreate.IsNull() {
		var d pollData
		if diags := plan.PollCreate.As(ctx, &d, basetypes.ObjectAsOptions{}); diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		opt, diags := r.p.apiOpt.ForPoll(ctx, opt.Header, d)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		if opt.UrlLocator == nil {
			// Update the request URL to pointing to the resource path, which is mainly for resources whose create method is POST.
			// As it will be used to poll the resource status.
			response.Request.RawRequest.URL.Path = resourceId
		}
		p, err := client.NewPollableFromResp(*response, *opt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Create: Failed to build poller from the response of the initiated request",
				err.Error(),
			)
			return
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

	if loc := state.ReadBodyLocator.ValueString(); loc != "" {
		// Guaranteed by schema
		bodyLocator, _ := parseLocator(loc)
		b = []byte(bodyLocator.LocateValueInResp(*response))
	}

	var writeOnlyAttributes []string
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
	state.Body = types.StringValue(string(body))

	// Set computed attributes
	state.Output = types.StringValue(string(b))

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

		// Precheck
		if !plan.PrecheckUpdate.IsNull() {
			var d precheckData
			if diags := plan.PrecheckUpdate.As(ctx, &d, basetypes.ObjectAsOptions{}); diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}
			opt, diags := r.p.apiOpt.ForPrecheck(ctx, state.ID.ValueString(), opt.Header, opt.Query, d)
			if diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}
			p, err := client.NewPollable(*opt)
			if err != nil {
				resp.Diagnostics.AddError(
					"Update: Failed to build poller for precheck",
					err.Error(),
				)
				return
			}
			if err := p.PollUntilDone(ctx, c); err != nil {
				resp.Diagnostics.AddError(
					"Update: Pre-checking failure",
					err.Error(),
				)
				return
			}
		}

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
			path, err = buildpath.BuildPath(plan.UpdatePath.ValueString(), r.p.apiOpt.BaseURL.String(), plan.Path.ValueString(), []byte(state.Output.ValueString()))
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
		if !plan.PollUpdate.IsNull() {
			var d pollData
			if diags := plan.PollUpdate.As(ctx, &d, basetypes.ObjectAsOptions{}); diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}
			opt, diags := r.p.apiOpt.ForPoll(ctx, opt.Header, d)
			if diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}
			p, err := client.NewPollableFromResp(*response, *opt)
			if err != nil {
				resp.Diagnostics.AddError(
					"Update: Failed to build poller from the response of the initiated request",
					err.Error(),
				)
				return
			}
			if err := p.PollUntilDone(ctx, c); err != nil {
				resp.Diagnostics.AddError(
					"Update: Polling failure",
					err.Error(),
				)
				return
			}
		}
	}

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

	// Precheck
	if !state.PrecheckDelete.IsNull() {
		var d precheckData
		if diags := state.PrecheckDelete.As(ctx, &d, basetypes.ObjectAsOptions{}); diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		opt, diags := r.p.apiOpt.ForPrecheck(ctx, state.ID.ValueString(), opt.Header, opt.Query, d)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		p, err := client.NewPollable(*opt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Delete: Failed to build poller for precheck",
				err.Error(),
			)
			return
		}
		if err := p.PollUntilDone(ctx, c); err != nil {
			resp.Diagnostics.AddError(
				"Delete: Pre-checking failure",
				err.Error(),
			)
			return
		}
	}

	path := state.ID.ValueString()
	if !state.DeletePath.IsNull() {
		var err error
		path, err = buildpath.BuildPath(state.DeletePath.ValueString(), r.p.apiOpt.BaseURL.String(), state.Path.ValueString(), []byte(state.Output.ValueString()))
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
	if !state.PollDelete.IsNull() {
		var d pollData
		if diags := state.PollDelete.As(ctx, &d, basetypes.ObjectAsOptions{}); diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		opt, diags := r.p.apiOpt.ForPoll(ctx, opt.Header, d)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		p, err := client.NewPollableFromResp(*response, *opt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Delete: Failed to build poller from the response of the initiated request",
				err.Error(),
			)
			return
		}
		if err := p.PollUntilDone(ctx, c); err != nil {
			resp.Diagnostics.AddError(
				"Delete: Polling failure",
				err.Error(),
			)
			return
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
