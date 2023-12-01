package provider

import (
	"context"
	"fmt"

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
	"github.com/magodo/terraform-provider-restful/internal/buildpath"
	"github.com/magodo/terraform-provider-restful/internal/client"
	myvalidator "github.com/magodo/terraform-provider-restful/internal/validator"
)

type OperationResource struct {
	p *Provider
}

var _ resource.Resource = &OperationResource{}

type operationResourceData struct {
	ID             types.String `tfsdk:"id"`
	Path           types.String `tfsdk:"path"`
	Method         types.String `tfsdk:"method"`
	Body           types.String `tfsdk:"body"`
	Query          types.Map    `tfsdk:"query"`
	Header         types.Map    `tfsdk:"header"`
	Precheck       types.List   `tfsdk:"precheck"`
	Poll           types.Object `tfsdk:"poll"`
	Retry          types.Object `tfsdk:"retry"`
	DeleteMethod   types.String `tfsdk:"delete_method"`
	DeleteBody     types.String `tfsdk:"delete_body"`
	DeletePath     types.String `tfsdk:"delete_path"`
	PrecheckDelete types.List   `tfsdk:"precheck_delete"`
	PollDelete     types.Object `tfsdk:"poll_delete"`
	RetryDelete    types.Object `tfsdk:"retry_delete"`
	OutputAttrs    types.Set    `tfsdk:"output_attrs"`
	Output         types.String `tfsdk:"output"`
}

func (r *OperationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_operation"
}

func (r *OperationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	precheckDelete := precheckAttribute("`Delete`", false, "By default, the `path` of this resource is used.")
	precheckDelete.Validators = append(precheckDelete.Validators, listvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("delete_method")))
	pollDelete := pollAttribute("`Delete`")
	pollDelete.Validators = append(pollDelete.Validators, objectvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("delete_method")))
	retryDelete := retryAttribute("`Delete`")
	retryDelete.Validators = append(retryDelete.Validators, objectvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("delete_method")))

	resp.Schema = schema.Schema{
		Description:         "`restful_operation` represents a one-time API call operation.",
		MarkdownDescription: "`restful_operation` represents a one-time API call operation.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:         "The ID of the operation. Same as the `path`.",
				MarkdownDescription: "The ID of the operation. Same as the `path`.",
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
			"method": schema.StringAttribute{
				Description:         "The HTTP method for the `Create`/`Update` call. Possible values are `PUT`, `POST`, `PATCH` and `DELETE`.",
				MarkdownDescription: "The HTTP method for the `Create`/`Update` call. Possible values are `PUT`, `POST`, `PATCH` and `DELETE`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("PUT", "POST", "PATCH", "DELETE"),
				},
			},
			"body": schema.StringAttribute{
				Description:         "The payload for the `Create`/`Update` call.",
				MarkdownDescription: "The payload for the `Create`/`Update` call.",
				Optional:            true,
				Validators: []validator.String{
					myvalidator.StringIsJSON(),
				},
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

			"precheck": precheckAttribute("`Create`/`Update`", true, ""),
			"poll":     pollAttribute("`Create`/`Update`"),
			"retry":    retryAttribute("`Create`/`Update`"),

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

			"delete_body": schema.StringAttribute{
				Description:         "The payload for the `Delete` call.",
				MarkdownDescription: "The payload for the `Delete` call.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("delete_method")),
					myvalidator.StringIsJSON(),
				},
			},

			"precheck_delete": precheckDelete,
			"poll_delete":     pollDelete,
			"retry_delete":    retryDelete,

			"output_attrs": schema.SetAttribute{
				Description:         "A set of `output` attribute paths (in gjson syntax) that will be exported in the `output`. If this is not specified, all attributes will be exported by `output`.",
				MarkdownDescription: "A set of `output` attribute paths (in [gjson syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md)) that will be exported in the `output`. If this is not specified, all attributes will be exported by `output`.",
				Optional:            true,
				ElementType:         types.StringType,
			},

			"output": schema.StringAttribute{
				Description:         "The response body.",
				MarkdownDescription: "The response body.",
				Computed:            true,
			},
		},
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

func (r *OperationResource) createOrUpdate(ctx context.Context, tfplan tfsdk.Plan, state *tfsdk.State, diagnostics *diag.Diagnostics) {
	var plan operationResourceData
	diags := tfplan.Get(ctx, &plan)
	diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	c := r.p.client

	opt, diags := r.p.apiOpt.ForResourceOperation(ctx, plan)
	diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	// Precheck
	unlockFunc, diags := precheck(ctx, c, r.p.apiOpt, plan.Path.ValueString(), opt.Header, opt.Query, plan.Precheck)
	diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	defer unlockFunc()

	response, err := c.Operation(ctx, plan.Path.ValueString(), plan.Body.ValueString(), *opt)
	if err != nil {
		diagnostics.AddError(
			"Error to call operation",
			err.Error(),
		)
		return
	}
	if !response.IsSuccess() {
		diagnostics.AddError(
			fmt.Sprintf("Operation API returns %d", response.StatusCode()),
			string(response.Body()),
		)
		return
	}

	b := response.Body()

	resourceId := plan.Path.ValueString()

	// For LRO, wait for completion
	if !plan.Poll.IsNull() {
		var d pollData
		if diags := plan.Poll.As(ctx, &d, basetypes.ObjectAsOptions{}); diags.HasError() {
			diagnostics.Append(diags...)
			return
		}
		opt, diags := r.p.apiOpt.ForPoll(ctx, opt.Header, opt.Query, d)
		if diags.HasError() {
			diagnostics.Append(diags...)
			return
		}
		p, err := client.NewPollableForPoll(*response, *opt)
		if err != nil {
			diagnostics.AddError(
				"Operation: Failed to build poller from the response of the initiated request",
				err.Error(),
			)
			return
		}
		if err := p.PollUntilDone(ctx, c); err != nil {
			diagnostics.AddError(
				"Operation: Polling failure",
				err.Error(),
			)
			return
		}
	}

	// Set resource ID to state
	plan.ID = types.StringValue(resourceId)

	// Set Output to state
	plan.Output = types.StringValue(string(b))
	output := string(b)
	if !plan.OutputAttrs.IsNull() {
		// Update the output to only contain the specified attributes.
		var outputAttrs []string
		diags = plan.OutputAttrs.ElementsAs(ctx, &outputAttrs, false)
		diagnostics.Append(diags...)
		if diags.HasError() {
			return
		}
		output, err = FilterAttrsInJSON(output, outputAttrs)
		if err != nil {
			diagnostics.AddError(
				"Filter `output` during operation",
				err.Error(),
			)
			return
		}
	}
	plan.Output = types.StringValue(output)

	diags = state.Set(ctx, plan)
	diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
}

func (r *OperationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.createOrUpdate(ctx, req.Plan, &resp.State, &resp.Diagnostics)
	return
}

func (r *OperationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.createOrUpdate(ctx, req.Plan, &resp.State, &resp.Diagnostics)
	return
}

func (r *OperationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	return
}

func (r *OperationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state operationResourceData
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if state.DeleteMethod.IsNull() {
		return
	}

	c := r.p.client

	opt, diags := r.p.apiOpt.ForResourceOperationDelete(ctx, state)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	// Precheck
	unlockFunc, diags := precheck(ctx, c, r.p.apiOpt, state.ID.ValueString(), opt.Header, opt.Query, state.PrecheckDelete)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	defer unlockFunc()

	path := state.ID.ValueString()
	if !state.DeletePath.IsNull() {
		var err error
		path, err = buildpath.BuildPath(state.DeletePath.ValueString(), r.p.apiOpt.BaseURL.String(), state.Path.ValueString(), []byte(state.Output.ValueString()))
		if err != nil {
			resp.Diagnostics.AddError(
				fmt.Sprintf("Failed to build the path for deleting the operation resource"),
				fmt.Sprintf("Can't build path with `delete_path`: %q, `path`: %q, `body`: %q", state.DeletePath.ValueString(), state.Path.ValueString(), string(state.Output.ValueString())),
			)
			return
		}
	}

	response, err := c.Operation(ctx, path, state.DeleteBody.ValueString(), *opt)
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
		opt, diags := r.p.apiOpt.ForPoll(ctx, opt.Header, opt.Query, d)
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

	return
}
