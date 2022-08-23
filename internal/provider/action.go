package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"path"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/magodo/terraform-provider-restful/internal/client"
	"github.com/magodo/terraform-provider-restful/internal/validator"
)

type actionResourceType struct{}

func (r actionResourceType) GetSchema(_ context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description:         "`restful_action` represents a one-time API call action.",
		MarkdownDescription: "`restful_action` represents a one-time API call action.",
		Attributes: map[string]tfsdk.Attribute{
			"id": {
				Description:         "The ID of the action. Same as the `path`.",
				MarkdownDescription: "The ID of the action. Same as the `path`.",
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
			"poll": pollAttribute("API"),
			"output": {
				Description:         "The response body.",
				MarkdownDescription: "The response body.",
				Type:                types.StringType,
				Computed:            true,
			},
		},
	}, nil
}

func (r ActionResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config actionResourceData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	validatePoll(ctx, config.Poll, "poll", resp)

	if !config.Body.IsUnknown() {
		var body map[string]interface{}
		if err := json.Unmarshal([]byte(config.Body.Value), &body); err != nil {
			resp.Diagnostics.AddError(
				"Invalid configuration",
				fmt.Sprintf(`Failed to unmarshal "body": %s: %s`, err.Error(), config.Body.String()),
			)
		}
	}
}

func (r actionResourceType) NewResource(_ context.Context, p provider.Provider) (resource.Resource, diag.Diagnostics) {
	return ActionResource{p: *p.(*Provider)}, nil
}

type ActionResource struct {
	p Provider
}

var _ resource.Resource = ActionResource{}

type actionResourceData struct {
	ID     types.String `tfsdk:"id"`
	Path   types.String `tfsdk:"path"`
	Method types.String `tfsdk:"method"`
	Body   types.String `tfsdk:"body"`
	Query  types.Map    `tfsdk:"query"`
	Header types.Map    `tfsdk:"header"`
	Poll   types.Object `tfsdk:"poll"`
	Output types.String `tfsdk:"output"`
}

func (r ActionResource) createOrUpdate(ctx context.Context, tfplan tfsdk.Plan, state *tfsdk.State, diagnostics *diag.Diagnostics) {
	var plan actionResourceData
	diags := tfplan.Get(ctx, &plan)
	diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	c := r.p.client

	opt, diags := r.p.apiOpt.ForResourceAction(ctx, plan)
	diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	response, err := c.Action(ctx, plan.Path.Value, plan.Body.Value, *opt)
	if err != nil {
		diagnostics.AddError(
			"Error to call action",
			err.Error(),
		)
		return
	}
	if !response.IsSuccess() {
		diagnostics.AddError(
			fmt.Sprintf("Action API returns %d", response.StatusCode()),
			string(response.Body()),
		)
		return
	}

	b := response.Body()

	// For POST create method, generate the resource id by combining the path and the id in response.
	resourceId := plan.Path.Value

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
				"Action: Failed to build poller from the response of the initiated request",
				err.Error(),
			)
			return
		}
		if err := p.PollUntilDone(ctx, c); err != nil {
			diagnostics.AddError(
				"Action: Polling failure",
				err.Error(),
			)
			return
		}
	}

	// Set overridable attributes from option to state
	plan.Query = opt.Query.ToTFValue()
	plan.Header = opt.Header.ToTFValue()

	// Set resource ID to state
	plan.ID = types.String{Value: resourceId}

	// Set Output to state
	plan.Output = types.String{Value: string(b)}

	diags = state.Set(ctx, plan)
	diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
}

func (r ActionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.createOrUpdate(ctx, req.Plan, &resp.State, &resp.Diagnostics)
	return
}

func (r ActionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.createOrUpdate(ctx, req.Plan, &resp.State, &resp.Diagnostics)
	return
}

func (r ActionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	return
}

func (r ActionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	return
}
