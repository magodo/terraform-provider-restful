package provider

import (
	"context"
	"fmt"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/magodo/terraform-plugin-framework-helper/dynamic"
	"github.com/magodo/terraform-plugin-framework-helper/ephemeral"
	"github.com/magodo/terraform-plugin-framework-helper/jsonset"
	"github.com/lfventura/terraform-provider-restful/internal/client"
	"github.com/lfventura/terraform-provider-restful/internal/exparam"
	myvalidator "github.com/lfventura/terraform-provider-restful/internal/validator"
)

type OperationResource struct {
	p *Provider
}

var _ resource.Resource = &OperationResource{}
var _ resource.ResourceWithUpgradeState = &OperationResource{}

type operationResourceData struct {
	ID        types.String `tfsdk:"id"`
	Path      types.String `tfsdk:"path"`
	IdBuilder types.String `tfsdk:"id_builder"`
	Method    types.String `tfsdk:"method"`

	Body          types.Dynamic `tfsdk:"body"`
	EphemeralBody types.Dynamic `tfsdk:"ephemeral_body"`

	Query          types.Map `tfsdk:"query"`
	OperationQuery types.Map `tfsdk:"operation_query"`
	DeleteQuery    types.Map `tfsdk:"delete_query"`

	Header          types.Map `tfsdk:"header"`
	OperationHeader types.Map `tfsdk:"operation_header"`
	DeleteHeader    types.Map `tfsdk:"delete_header"`

	Precheck       types.List    `tfsdk:"precheck"`
	Poll           types.Object  `tfsdk:"poll"`
	DeleteMethod   types.String  `tfsdk:"delete_method"`
	DeleteBody     types.Dynamic `tfsdk:"delete_body"`
	DeletePath     types.String  `tfsdk:"delete_path"`
	PrecheckDelete types.List    `tfsdk:"precheck_delete"`
	PollDelete     types.Object  `tfsdk:"poll_delete"`
	OutputAttrs    types.Set     `tfsdk:"output_attrs"`

	UseSensitiveOutput types.Bool    `tfsdk:"use_sensitive_output"`
	Output             types.Dynamic `tfsdk:"output"`
	SensitiveOutput    types.Dynamic `tfsdk:"sensitive_output"`
}

func (r *OperationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_operation"
}

func (r *OperationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	precheckDelete := precheckAttribute("`Delete`", false, "By default, the `path` of this resource is used.", true)
	precheckDelete.Validators = append(precheckDelete.Validators, listvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("delete_method")))

	pollDelete := pollAttribute("`Delete`")
	pollDelete.Validators = append(pollDelete.Validators, objectvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("delete_method")))

	resp.Schema = schema.Schema{
		Description:         "`restful_operation` represents a one-time API call operation.",
		MarkdownDescription: "`restful_operation` represents a one-time API call operation.",
		Version:             2,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:         "The ID of the operation.",
				MarkdownDescription: "The ID of the operation.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"path": schema.StringAttribute{
				Description:         "The path for the `Create`/`Update` call, relative to the `base_url` of the provider.",
				MarkdownDescription: "The path for the `Create`/`Update` call, relative to the `base_url` of the provider.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			// This is actually the same as the `read_path` of restful_resource, besides the name
			"id_builder": schema.StringAttribute{
				Description:         "The pattern used to build the `id`. The `path` is used as the `id` instead if absent." + bodyOrPathParamDescription,
				MarkdownDescription: "The pattern used to build the `id`. The `path` is used as the `id` instead if absent." + bodyOrPathParamDescription,
				Optional:            true,
				Validators: []validator.String{
					myvalidator.StringIsPathBuilder(),
				},
			},
			"method": schema.StringAttribute{
				Description:         "The HTTP method for the `Create`/`Update` call. Possible values are `GET`, `PUT`, `POST`, `PATCH` and `DELETE`.",
				MarkdownDescription: "The HTTP method for the `Create`/`Update` call. Possible values are `GET`, `PUT`, `POST`, `PATCH` and `DELETE`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("GET", "PUT", "POST", "PATCH", "DELETE"),
				},
			},
			"body": schema.DynamicAttribute{
				Description:         "The payload for the `Create`/`Update` call.",
				MarkdownDescription: "The payload for the `Create`/`Update` call.",
				Optional:            true,
			},
			"ephemeral_body": schema.DynamicAttribute{
				Description:         "The ephemeral (write-only) properties of the resource. This will be merge-patched to the `body` to construct the actual request body.",
				MarkdownDescription: "The ephemeral (write-only) properties of the resource. This will be merge-patched to the `body` to construct the actual request body.",
				Optional:            true,
				WriteOnly:           true,
			},
			"query": schema.MapAttribute{
				Description:         "The query parameters that are applied to each request. This overrides the `query` set in the provider block.",
				MarkdownDescription: "The query parameters that are applied to each request. This overrides the `query` set in the provider block.",
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
			},
			"operation_query": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("query", "operation"),
				MarkdownDescription: operationOverridableAttrDescription("query", "operation"),
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
			},
			"delete_query": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("query", "delete"),
				MarkdownDescription: operationOverridableAttrDescription("query", "delete"),
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
			},
			"header": schema.MapAttribute{
				Description:         "The header parameters that are applied to each request. This overrides the `header` set in the provider block.",
				MarkdownDescription: "The header parameters that are applied to each request. This overrides the `header` set in the provider block.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"operation_header": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("header", "operation"),
				MarkdownDescription: operationOverridableAttrDescription("header", "operation"),
				ElementType:         types.StringType,
				Optional:            true,
			},
			"delete_header": schema.MapAttribute{
				Description:         operationOverridableAttrDescription("header", "delete"),
				MarkdownDescription: operationOverridableAttrDescription("header", "delete"),
				ElementType:         types.StringType,
				Optional:            true,
			},

			"precheck": precheckAttribute("`Create`/`Update`", true, "", false),
			"poll":     pollAttribute("`Create`/`Update`"),

			"delete_method": schema.StringAttribute{
				Description:         "The method for the `Delete` call. Possible values are `POST`, `PUT`, `PATCH` and `DELETE`. If this is not specified, no `Delete` call will occur.",
				MarkdownDescription: "The method for the `Delete` call. Possible values are `POST`, `PUT`, `PATCH` and `DELETE`. If this is not specified, no `Delete` call will occur.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("POST", "PUT", "PATCH", "DELETE"),
				},
			},

			"delete_path": schema.StringAttribute{
				Description:         "The path for the `Delete` call, relative to the `base_url` of the provider. The `path` is used instead if `delete_path` is absent.",
				MarkdownDescription: "The path for the `Delete` call, relative to the `base_url` of the provider. The `path` is used instead if `delete_path` is absent.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("delete_method")),
					myvalidator.StringIsPathBuilder(),
				},
			},

			"delete_body": schema.DynamicAttribute{
				Description:         "The payload for the `Delete` call.",
				MarkdownDescription: "The payload for the `Delete` call.",
				Optional:            true,
			},

			"precheck_delete": precheckDelete,
			"poll_delete":     pollDelete,

			"output_attrs": schema.SetAttribute{
				Description:         "A set of `output` attribute paths (in gjson syntax) that will be exported in the `output`. If this is not specified, all attributes will be exported by `output`.",
				MarkdownDescription: "A set of `output` attribute paths (in [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)) that will be exported in the `output`. If this is not specified, all attributes will be exported by `output`.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"use_sensitive_output": schema.BoolAttribute{
				Description:         "Whether to use `sensitive_output` instead of `output`. When true, the response will be stored in `sensitive_output` (which is marked as sensitive). Defaults to `false`.",
				MarkdownDescription: "Whether to use `sensitive_output` instead of `output`. When true, the response will be stored in `sensitive_output` (which is marked as sensitive). Defaults to `false`.",
				Optional:            true,
			},

			"output": schema.DynamicAttribute{
				Description:         "The response body. If `ephemeral_body` get returned by API, it will be removed from `output`. This is only populated when `use_sensitive_output` is false.",
				MarkdownDescription: "The response body. If `ephemeral_body` get returned by API, it will be removed from `output`. This is only populated when `use_sensitive_output` is false.",
				Computed:            true,
			},
			"sensitive_output": schema.DynamicAttribute{
				Description:         "The response body (sensitive). If `ephemeral_body` get returned by API, it will be removed from `sensitive_output`. This is only populated when `use_sensitive_output` is true.",
				MarkdownDescription: "The response body (sensitive). If `ephemeral_body` get returned by API, it will be removed from `sensitive_output`. This is only populated when `use_sensitive_output` is true.",
				Computed:            true,
				Sensitive:           true,
			},
		},
	}
}

func (r *OperationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config operationResourceData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if !config.Body.IsUnknown() {
		b, err := dynamic.ToJSON(config.Body)
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalid configuration",
				fmt.Sprintf("marshal body: %v", err),
			)
			return
		}

		_, diags := ephemeral.ValidateEphemeralBody(b, config.EphemeralBody)
		resp.Diagnostics = append(resp.Diagnostics, diags...)
		if diags.HasError() {
			return
		}
	}
}

func (r *OperationResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		// If the entire plan is null, the resource is planned for destruction.
		return
	}
	if req.State.Raw.IsNull() {
		// If the entire state is null, the resource is planned for creation.
		return
	}

	var plan operationResourceData
	if diags := req.Plan.Get(ctx, &plan); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	var config operationResourceData
	if diags := req.Config.Get(ctx, &config); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	defer func() {
		resp.Plan.Set(ctx, plan)
	}()

	// Set output as unknown to trigger a plan diff, if ephemral body has changed
	diff, diags := ephemeral.Diff(ctx, req.Private, config.EphemeralBody)
	resp.Diagnostics = append(resp.Diagnostics, diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if diff {
		tflog.Info(ctx, `"ephemeral_body" has changed`)
		// Mark the appropriate output as unknown based on use_sensitive_output
		if !plan.UseSensitiveOutput.IsNull() && plan.UseSensitiveOutput.ValueBool() {
			plan.SensitiveOutput = types.DynamicUnknown()
		} else {
			plan.Output = types.DynamicUnknown()
		}
	}
}

func (r *OperationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, ok := req.ProviderData.(providerData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Operation Resource Configure Type",
			fmt.Sprintf("got: %T.", req.ProviderData),
		)
		return
	}
	if diags := providerData.provider.Init(ctx, providerData.config); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	r.p = providerData.provider
}

func (r *OperationResource) createOrUpdate(ctx context.Context, reqConfig tfsdk.Config, reqPlan tfsdk.Plan, reqState tfsdk.State, respPrivate ephemeral.PrivateData, respState *tfsdk.State, respDiags *diag.Diagnostics, forCreate bool) {
	c := r.p.client
	c.SetLoggerContext(ctx)

	var plan operationResourceData
	diags := reqPlan.Get(ctx, &plan)
	respDiags.Append(diags...)
	if respDiags.HasError() {
		return
	}

	var config operationResourceData
	diags = reqConfig.Get(ctx, &config)
	respDiags.Append(diags...)
	if respDiags.HasError() {
		return
	}

	if forCreate {
		tflog.Info(ctx, "Create an operation resource", map[string]interface{}{"id": plan.Path.ValueString()})
	} else {
		tflog.Info(ctx, "Update an operation resource", map[string]interface{}{"id": plan.ID.ValueString()})
	}

	opt, diags := r.p.apiOpt.ForOperation(ctx, plan.Method, plan.Query, plan.Header, plan.OperationQuery, plan.OperationHeader, nil)
	respDiags.Append(diags...)
	if respDiags.HasError() {
		return
	}

	// Precheck
	if !plan.Precheck.IsNull() {
		unlockFunc, diags := precheck(ctx, c, r.p.apiOpt, plan.Path.ValueString(), opt.Header, opt.Query, plan.Precheck, basetypes.NewDynamicNull())
		respDiags.Append(diags...)
		if respDiags.HasError() {
			return
		}
		defer unlockFunc()
	}

	// Build the body
	var eb, body []byte
	if !plan.Body.IsNull() {
		var err error
		body, err = dynamic.ToJSON(plan.Body)
		if err != nil {
			respDiags.AddError(
				`Error to marshal "body"`,
				err.Error(),
			)
			return
		}

		if !config.EphemeralBody.IsNull() {
			eb, diags = ephemeral.ValidateEphemeralBody(body, config.EphemeralBody)
			respDiags.Append(diags...)
			if respDiags.HasError() {
				return
			}

			// Merge patch the ephemeral body to the body
			body, err = jsonpatch.MergePatch(body, eb)
			if err != nil {
				respDiags.AddError(
					"Merge patching `body` with `ephemeral_body`",
					err.Error(),
				)
				return
			}
		}
	}

	response, err := c.Operation(ctx, plan.Path.ValueString(), body, *opt)
	if err != nil {
		respDiags.AddError(
			"Error to call operation",
			err.Error(),
		)
		return
	}
	if !response.IsSuccess() {
		respDiags.AddError(
			fmt.Sprintf("Operation API returns %d", response.StatusCode()),
			string(response.Body()),
		)
		return
	}

	resourceId := plan.Path.ValueString()
	if !plan.IdBuilder.IsNull() {
		resourceId, err = exparam.ExpandBodyOrPath(plan.IdBuilder.ValueString(), plan.Path.ValueString(), response.Body())
		if err != nil {
			respDiags.AddError(
				"Failed to build the id for this resource",
				fmt.Sprintf("Can't build resource id with `id_builder`: %q, `path`: %q: %v", plan.IdBuilder.ValueString(), plan.Path.ValueString(), err),
			)
			return
		}
	}

	// For LRO, wait for completion
	if !plan.Poll.IsNull() {
		var d pollData
		if diags := plan.Poll.As(ctx, &d, basetypes.ObjectAsOptions{}); diags.HasError() {
			respDiags.Append(diags...)
			return
		}

		var body basetypes.DynamicValue
		if forCreate {
			body, err = dynamic.FromJSONImplied(response.Body())
			if err != nil {
				respDiags.AddError(
					"Operation: Failed to get dynamic from JSON for the operation response",
					err.Error(),
				)
				return
			}
		} else {
			var state operationResourceData
			diags = reqState.Get(ctx, &state)
			respDiags.Append(diags...)
			if diags.HasError() {
				return
			}
			body = state.Output
		}
		opt, diags := r.p.apiOpt.ForPoll(ctx, opt.Header, opt.Query, d, body)

		if diags.HasError() {
			respDiags.Append(diags...)
			return
		}
		if opt.UrlLocator == nil {
			response.Request.URL = resourceId
		}
		p, err := client.NewPollableForPoll(*response, *opt)
		if err != nil {
			respDiags.AddError(
				"Operation: Failed to build poller from the response of the initiated request",
				err.Error(),
			)
			return
		}
		if err := p.PollUntilDone(ctx, c); err != nil {
			respDiags.AddError(
				"Operation: Polling failure",
				err.Error(),
			)
			return
		}
	}

	// Set resource ID to state
	plan.ID = types.StringValue(resourceId)

	// Set Output to state
	rb := response.Body()
	if !plan.OutputAttrs.IsNull() {
		// Update the output to only contain the specified attributes.
		var outputAttrs []string
		diags = plan.OutputAttrs.ElementsAs(ctx, &outputAttrs, false)
		respDiags.Append(diags...)
		if diags.HasError() {
			return
		}
		fb, err := FilterAttrsInJSON(string(rb), outputAttrs)
		if err != nil {
			respDiags.AddError(
				"Filter `output` during operation",
				err.Error(),
			)
			return
		}
		rb = []byte(fb)
	}

	if eb != nil {
		rb, err = jsonset.Difference(rb, eb)
		if err != nil {
			respDiags.AddError(
				"Removing `ephemeral_body` from `output`",
				err.Error(),
			)
			return
		}
	}

	output, err := dynamic.FromJSONImplied(rb)
	if err != nil {
		respDiags.AddError(
			"Converting `output` from JSON to dynamic",
			err.Error(),
		)
		return
	}
	// Populate the appropriate output based on use_sensitive_output
	if !plan.UseSensitiveOutput.IsNull() && plan.UseSensitiveOutput.ValueBool() {
		plan.SensitiveOutput = output
		plan.Output = types.DynamicNull()
	} else {
		plan.Output = output
		plan.SensitiveOutput = types.DynamicNull()
	}

	diags = respState.Set(ctx, plan)
	respDiags.Append(diags...)
	if respDiags.HasError() {
		return
	}

	diags = ephemeral.Set(ctx, respPrivate, eb)
	respDiags.Append(diags...)
	if respDiags.HasError() {
		return
	}
}

func (r *OperationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.createOrUpdate(ctx, req.Config, req.Plan, tfsdk.State{}, resp.Private, &resp.State, &resp.Diagnostics, true)
}

func (r *OperationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.createOrUpdate(ctx, req.Config, req.Plan, req.State, resp.Private, &resp.State, &resp.Diagnostics, false)
}

func (r *OperationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
}

func (r *OperationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	c := r.p.client
	c.SetLoggerContext(ctx)

	var state operationResourceData
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	output, err := dynamic.ToJSON(state.Output)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to build the path for deleting the operation resource",
			fmt.Sprintf("Failed to marshal the output: %v", err),
		)
		return
	}

	tflog.Info(ctx, "Delete an operation resource", map[string]interface{}{"id": state.ID.ValueString()})

	if state.DeleteMethod.IsNull() {
		return
	}

	opt, diags := r.p.apiOpt.ForOperation(ctx, state.DeleteMethod, state.Query, state.Header, state.DeleteQuery, state.DeleteHeader, output)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	// Precheck
	if !state.PrecheckDelete.IsNull() {
		unlockFunc, diags := precheck(ctx, c, r.p.apiOpt, state.ID.ValueString(), opt.Header, opt.Query, state.PrecheckDelete, state.Output)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		defer unlockFunc()
	}

	path := state.ID.ValueString()
	if !state.DeletePath.IsNull() {
		path, err = exparam.ExpandBodyOrPath(state.DeletePath.ValueString(), state.Path.ValueString(), output)
		if err != nil {
			resp.Diagnostics.AddError(
				"Failed to build the path for deleting the operation resource",
				fmt.Sprintf("Can't build path with `delete_path`: %q, `path`: %q, `body`: %q, error: %v", state.DeletePath.ValueString(), state.Path.ValueString(), string(output), err),
			)
			return
		}
	}

	b, err := dynamic.ToJSON(state.DeleteBody)
	if err != nil {
		resp.Diagnostics.AddError(
			`Error to marshal "delete_body"`,
			err.Error(),
		)
		return
	}

	response, err := c.Operation(ctx, path, b, *opt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Delete: Error to call operation",
			err.Error(),
		)
		return
	}
	if !response.IsSuccess() {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Delete: Operation API returns %d", response.StatusCode()),
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
		opt, diags := r.p.apiOpt.ForPoll(ctx, opt.Header, opt.Query, d, state.Output)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		p, err := client.NewPollableForPoll(*response, *opt)
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
}
