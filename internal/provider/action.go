package provider

import (
	"context"
	"fmt"

	"github.com/go-resty/resty/v2"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/action"
	"github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/magodo/terraform-plugin-framework-helper/dynamic"
	"github.com/magodo/terraform-provider-restful/internal/client"
	"github.com/magodo/terraform-provider-restful/internal/exparam"
)

type Action struct {
	p *Provider
}

var _ action.Action = &Action{}

type actionData struct {
	Path   types.String  `tfsdk:"path"`
	Query  types.Map     `tfsdk:"query"`
	Header types.Map     `tfsdk:"header"`
	Method types.String  `tfsdk:"method"`
	Body   types.Dynamic `tfsdk:"body"`

	Precheck types.List   `tfsdk:"precheck"`
	Poll     types.Object `tfsdk:"poll"`
}

type actionPollData struct {
	pollData
	MessageTemplate types.String `tfsdk:"message_template"`
	Selector        types.String `tfsdk:"selector"`
}

const defaultActionProgressMessage = "Waiting for HTTP request to finish..."

func (a *Action) Metadata(ctx context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_action"
}

func (a *Action) Schema(ctx context.Context, req action.SchemaRequest, resp *action.SchemaResponse) {
	pollSchema := pollAttribute("Invoke")
	pollSchema.Attributes["message_template"] = schema.StringAttribute{
		MarkdownDescription: "The raw template for the progress message that will be displayed in the Terraform UI. This" + bodyParamDescription + " By default, it displays \"" + defaultActionProgressMessage + "\".",
		Optional:            true,
	}
	pollSchema.Attributes["selector"] = schema.StringAttribute{
		MarkdownDescription: "A selector expression in [gjson query syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md#queries), that is used when read returns a collection of resources, to select exactly one member resource of from it. This" + bodyParamDescription + " By default, the whole response body is used as the body.",
		Optional:            true,
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "`restful_action` represents an ad-hoc API call action.",
		Attributes: map[string]schema.Attribute{
			"path": schema.StringAttribute{
				MarkdownDescription: "The path for the `Invoke` call, relative to the `base_url` of the provider.",
				Required:            true,
			},
			"query": schema.MapAttribute{
				MarkdownDescription: "The query parameters for the `Invoke` call. This overrides the `query` set in the provider block.",
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
			},
			"header": schema.MapAttribute{
				MarkdownDescription: "The header parameters for the `Invoke` call. This overrides the `header` set in the provider block.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"method": schema.StringAttribute{
				MarkdownDescription: "The HTTP method for the `Invoke` call. Possible values are `HEAD`, `GET`, `PUT`, `POST`, `PATCH` and `DELETE`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("HEAD", "GET", "PUT", "POST", "PATCH", "DELETE"),
				},
			},
			"body": schema.DynamicAttribute{
				MarkdownDescription: "The payload for the `Invoke` call.",
				Optional:            true,
			},

			"precheck": precheckAttribute("Invoke", true, "", false),
			"poll":     pollSchema,
		},
	}
}

func (a *Action) Configure(ctx context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, ok := req.ProviderData.(providerData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Action Configure Type",
			fmt.Sprintf("got: %T.", req.ProviderData),
		)
		return
	}
	if diags := providerData.provider.Init(ctx, providerData.config); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	a.p = providerData.provider
}

func (a *Action) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	c := a.p.client
	c.SetLoggerContext(ctx)

	var config actionData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Info(ctx, "Invoking an action", map[string]any{"path": config.Path.ValueString()})

	opt, diags := a.p.apiOpt.ForOperation(ctx, config.Method, config.Query, config.Header, types.MapNull(types.StringType), types.MapNull(types.StringType), nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Precheck
	if !config.Precheck.IsNull() {
		resp.SendProgress(action.InvokeProgressEvent{
			Message: "Prechecking the action...",
		})
		unlockFunc, diags := precheck(ctx, c, a.p.apiOpt, config.Path.ValueString(), opt.Header, opt.Query, config.Precheck, types.DynamicNull())
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		defer unlockFunc()
	}

	// Build the body
	var body []byte
	if !config.Body.IsNull() {
		var err error
		body, err = dynamic.ToJSON(config.Body)
		if err != nil {
			resp.Diagnostics.AddError(
				`Error to marshal "body"`,
				err.Error(),
			)
			return
		}
	}

	resp.SendProgress(action.InvokeProgressEvent{
		Message: "Invoking the action...",
	})
	response, err := c.Operation(ctx, config.Path.ValueString(), body, *opt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error to invoke action",
			err.Error(),
		)
		return
	}
	if !response.IsSuccess() {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Invoke API returns %d", response.StatusCode()),
			string(response.Body()),
		)
		return
	}

	// For progressing
	if !config.Poll.IsNull() {
		var pd actionPollData
		if diags := config.Poll.As(ctx, &pd, basetypes.ObjectAsOptions{}); diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}

		body, err := dynamic.FromJSONImplied(response.Body())
		if err != nil {
			resp.Diagnostics.AddError(
				"Action: Failed to get dynamic from JSON for the invoke response",
				err.Error(),
			)
			return
		}

		opt, diags := a.p.apiOpt.ForPoll(ctx, opt.Header, opt.Query, pd.pollData, body)

		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		if opt.UrlLocator == nil {
			response.Request.URL = config.Path.ValueString()
		}
		p, err := client.NewPollableForPoll(*response, *opt)
		if err != nil {
			resp.Diagnostics.AddError(
				"Action: Failed to build poller from the response of the initiated request",
				err.Error(),
			)
			return
		}

		cb := func(ctx context.Context, httpResp *resty.Response) {
			body := httpResp.Body()
			if !pd.Selector.IsNull() {
				bodyLocator := client.BodyLocator(pd.Selector.ValueString())
				sb, ok := bodyLocator.LocateValueInResp(*response)
				if !ok {
					tflog.Warn(ctx, "Failed to select response", map[string]any{
						"selector": pd.Selector.ValueString(),
					})
					return
				}
				body = []byte(sb)
			}

			message := defaultActionProgressMessage
			if !pd.MessageTemplate.IsNull() {
				message, err = exparam.ExpandBody(pd.MessageTemplate.ValueString(), body)
				if err != nil {
					tflog.Warn(ctx, "Failed to expand body for progress message", map[string]any{
						"template": pd.MessageTemplate.ValueString(),
					})
					return
				}
			}
			resp.SendProgress(action.InvokeProgressEvent{
				Message: message,
			})
		}

		if err := p.PollUntilDone(ctx, c, cb); err != nil {
			resp.Diagnostics.AddError(
				"Action: Polling failure",
				err.Error(),
			)
			return
		}
	}
}
