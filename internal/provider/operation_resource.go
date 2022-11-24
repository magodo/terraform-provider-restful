package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/magodo/terraform-provider-restful/internal/client"
	"github.com/magodo/terraform-provider-restful/internal/validator"
)

type OperationResource struct {
	p *Provider
}

var _ resource.Resource = &OperationResource{}

type operationResourceData struct {
	ID     types.String `tfsdk:"id"`
	Path   types.String `tfsdk:"path"`
	Method types.String `tfsdk:"method"`
	Body   types.String `tfsdk:"body"`
	Query  types.Map    `tfsdk:"query"`
	Header types.Map    `tfsdk:"header"`
	Poll   types.Object `tfsdk:"poll"`
	Output types.String `tfsdk:"output"`
}

func (r *OperationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_operation"
}

func (r *OperationResource) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description:         "`restful_operation` represents a one-time API call operation.",
		MarkdownDescription: "`restful_operation` represents a one-time API call operation.",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Description:         "The ID of the operation. Same as the `path`.",
				MarkdownDescription: "The ID of the operation. Same as the `path`.",
				Type:                types.StringType,
				Computed:            true,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.UseStateForUnknown(),
				},
			},
			"path": {
				Description:         "The path of the API call, relative to the `base_url` of the provider.",
				MarkdownDescription: "The path of the API call, relative to the `base_url` of the provider.",
				Type:                types.StringType,
				Required:            true,
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.RequiresReplace(),
				},
			},
			"method": {
				Description:         "The HTTP method of the API call. Possible values are `PUT`, `POST`, `PATCH` and `DELETE`.",
				MarkdownDescription: "The HTTP method of the API call. Possible values are `PUT`, `POST`, `PATCH` and `DELETE`.",
				Type:                types.StringType,
				Required:            true,
				Validators:          []tfsdk.AttributeValidator{validator.StringInSlice("PUT", "POST", "PATCH", "DELETE")},
				PlanModifiers: []tfsdk.AttributePlanModifier{
					resource.RequiresReplace(),
				},
			},
			"body": {
				Description:         "The payload of the API call.",
				MarkdownDescription: "The payload of the API call.",
				Type:                types.StringType,
				Optional:            true,
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
			"poll": pollAttribute("poll", "API"),
			"output": {
				Description:         "The response body.",
				MarkdownDescription: "The response body.",
				Type:                types.StringType,
				Computed:            true,
			},
		},
	}, nil
}

func (r *OperationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.p = &Provider{}
	if req.ProviderData != nil {
		r.p = req.ProviderData.(*Provider)
	}
	return
}

func (r *OperationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config operationResourceData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	validatePoll(ctx, config.Poll, "poll", resp)

	if !config.Body.IsUnknown() && !config.Body.IsNull() {
		var body map[string]interface{}
		if err := json.Unmarshal([]byte(config.Body.ValueString()), &body); err != nil {
			resp.Diagnostics.AddError(
				"Invalid configuration",
				fmt.Sprintf(`Failed to unmarshal "body": %s: %s`, err.Error(), config.Body.String()),
			)
		}
	}
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

	// For POST create method, generate the resource id by combining the path and the id in response.
	resourceId := plan.Path.ValueString()

	// For LRO, wait for completion
	if opt.PollOpt != nil {
		if opt.PollOpt.UrlLocator == nil {
			// Update the request URL to pointing to the resource path, which is mainly for resources whose create method is POST.
			// As it will be used to poll the resource status.
			response.Request.URL = path.Join(c.BaseURL, resourceId)
		}
		p, err := client.NewPollable(*response, *opt.PollOpt)
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

	// Set overridable attributes from option to state
	plan.Query = opt.Query.ToTFValue()
	plan.Header = opt.Header.ToTFValue()

	// Set resource ID to state
	plan.ID = types.StringValue(resourceId)

	// Set Output to state
	plan.Output = types.StringValue(string(b))

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
	return
}
