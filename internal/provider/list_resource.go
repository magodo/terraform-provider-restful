package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/list/schema"
	tfpath "github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/magodo/terraform-plugin-framework-helper/dynamic"
	"github.com/lfventura/terraform-provider-restful/internal/client"
	"github.com/lfventura/terraform-provider-restful/internal/exparam"
)

type ListResource struct {
	p *Provider
}

type ListResourceData struct {
	Path     types.String  `tfsdk:"path"`
	Name     types.String  `tfsdk:"name"`
	Method   types.String  `tfsdk:"method"`
	Query    types.Map     `tfsdk:"query"`
	Header   types.Map     `tfsdk:"header"`
	Body     types.Dynamic `tfsdk:"body"`
	Selector types.String  `tfsdk:"selector"`

	// Used for constructing eahc resource's identity and state
	ResourceId                   types.String  `tfsdk:"resource_id"`
	ResourcePath                 types.String  `tfsdk:"resource_path"`
	ResourceQuery                types.Map     `tfsdk:"resource_query"`
	ResourceHeader               types.Map     `tfsdk:"resource_header"`
	ResourceBody                 types.Dynamic `tfsdk:"resource_body"`
	ResourceReadSelector         types.String  `tfsdk:"resource_read_selector"`
	ResourceReadResponseTemplate types.String  `tfsdk:"resource_read_response_template"`
}

var _ list.ListResourceWithConfigure = &ListResource{}

func (l *ListResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource"
}

func (l *ListResource) ListResourceConfigSchema(ctx context.Context, req list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			//////////////////////////
			// List related attributes
			//////////////////////////
			"path": schema.StringAttribute{
				MarkdownDescription: "The API path of the List Resource, relative to the `base_url` of the provider. The response, optionally filtered by `selector`, shall be a JSON array.",
				Required:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "An expression in [gjson query syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md#queries), that is used to get the display name of each instance of the List Resource. The value " + bodyParamDescription + " Defaults to `$(body.name)`.",
				Optional:            true,
			},
			"method": schema.StringAttribute{
				MarkdownDescription: "The HTTP Method for the request. Allowed methods are a subset of methods defined in [RFC7231](https://datatracker.ietf.org/doc/html/rfc7231#section-4.3) namely, `GET`, `HEAD`, and `POST`. `POST` support is only intended for read-only URLs, such as submitting a search. Defaults to `GET`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("GET", "POST", "HEAD"),
				},
			},
			"query": schema.MapAttribute{
				MarkdownDescription: "The query parameters that are applied to each request. This overrides the `query` set in the provider block.",
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
			},
			"header": schema.MapAttribute{
				MarkdownDescription: "The header parameters that are applied to each request. This overrides the `header` set in the provider block.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"body": schema.DynamicAttribute{
				MarkdownDescription: "The request body that is sent when using `POST` method.",
				Optional:            true,
			},
			"selector": schema.StringAttribute{
				MarkdownDescription: "A selector in [gjson query syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md#queries), that is used to filter resources from the collection",
				Optional:            true,
			},

			//////////////////////////
			// Identity related attributes
			//////////////////////////
			"resource_path": schema.StringAttribute{
				MarkdownDescription: "The value of the `path` attribute used to compose the Resource Identity.",
				Required:            true,
			},
			"resource_id": schema.StringAttribute{
				MarkdownDescription: "An expression in [gjson query syntax](https://github.com/tidwall/gjson/blob/master/SYNTAX.md#queries), that is used to get the id of each instance of the List Resource. The value " + bodyOrPathParamDescription + " Defaults to `$(body.id)`.",
				Optional:            true,
			},
			"resource_query": schema.MapAttribute{
				MarkdownDescription: "The value of the `query` attribute used to compose the Resource Identity.",
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
			},
			"resource_header": schema.MapAttribute{
				MarkdownDescription: "The value of the `header` attribute used to compose the Resource Identity.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"resource_body": schema.DynamicAttribute{
				MarkdownDescription: "The value of the (nullified) `body` attribute used to compose the Resource Identity.",
				Optional:            true,
			},
			"resource_read_selector": schema.StringAttribute{
				MarkdownDescription: "The value of the `read_selector` attribute used to compose the Resource Identity.",
				Optional:            true,
			},
			"resource_read_response_template": schema.StringAttribute{
				MarkdownDescription: "The value of the `read_response_template` attribute used to compose the Resource Identity.",
				Optional:            true,
			},
		},
	}
}

func (l *ListResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	providerData, ok := req.ProviderData.(providerData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected List Resource Configure Type",
			fmt.Sprintf("got: %T.", req.ProviderData),
		)
		return
	}
	if diags := providerData.provider.Init(ctx, providerData.config); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	l.p = providerData.provider
}

func (l *ListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	c := l.p.client
	c.SetLoggerContext(ctx)

	var config ListResourceData

	diags := req.Config.Get(ctx, &config)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	opt, diags := l.p.apiOpt.ForListResourceRead(ctx, config)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	// Set body if provided
	var body []byte
	if !config.Body.IsNull() && config.Method.ValueString() == "POST" {
		var err error
		body, err = dynamic.ToJSON(config.Body)
		if err != nil {
			stream.Results = list.ListResultsStreamDiagnostics(diag.Diagnostics{diag.NewErrorDiagnostic(
				"Error to convert body",
				err.Error(),
			)})
			return
		}
	}

	response, err := c.ReadLR(ctx, config.Path.ValueString(), body, *opt)
	if err != nil {
		stream.Results = list.ListResultsStreamDiagnostics(diag.Diagnostics{diag.NewErrorDiagnostic(
			"Error to call Read",
			err.Error(),
		)})
		return
	}

	if !response.IsSuccess() && response.StatusCode() != http.StatusNotFound {
		stream.Results = list.ListResultsStreamDiagnostics(diag.Diagnostics{diag.NewErrorDiagnostic(
			fmt.Sprintf("Read API returns %d", response.StatusCode()),
			string(response.Body()),
		)})
		return
	}

	collections := []json.RawMessage{}
	if response.IsSuccess() {
		b := response.Body()
		if sel := config.Selector.ValueString(); sel != "" {
			bodyLocator := client.BodyLocator(sel)
			sb, ok := bodyLocator.LocateValueInResp(*response)
			if !ok {
				b = []byte("[]")
			} else {
				b = []byte(sb)
			}
		}
		if err := json.Unmarshal(b, &collections); err != nil {
			stream.Results = list.ListResultsStreamDiagnostics(diag.Diagnostics{diag.NewErrorDiagnostic(
				"json unmarshal the (selected) response body",
				err.Error(),
			)})
			return
		}
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, resRaw := range collections {
			result := req.NewListResult(ctx)

			// Set resource display name on the result
			nameExp := config.Name.ValueString()
			if nameExp == "" {
				nameExp = "$(body.name)"
			}
			name, err := exparam.ExpandBody(nameExp, resRaw)
			if err != nil {
				result.Diagnostics = append(result.Diagnostics, diag.NewErrorDiagnostic(
					"Failed to expand the `name` expression",
					err.Error(),
				))
				push(result)
				return
			}
			result.DisplayName = name

			// Set resource identity data on the result
			impspec := ImportSpec{
				Path: config.ResourcePath.ValueString(),
			}
			idExp := config.ResourceId.ValueString()
			if idExp == "" {
				idExp = "$(body.id)"
			}
			id, err := exparam.ExpandBodyOrPath(idExp, impspec.Path, resRaw)
			if err != nil {
				result.Diagnostics = append(result.Diagnostics, diag.NewErrorDiagnostic(
					"Failed to expand the `name` expression",
					err.Error(),
				))
				push(result)
				return
			}
			if id == "" {
				result.Diagnostics = append(result.Diagnostics, diag.NewErrorDiagnostic(
					"failed to query the id from the response",
					fmt.Sprintf("path=%s", idExp),
				))
				push(result)
				return
			}
			impspec.Id = id

			if !config.ResourceQuery.IsNull() {
				impspec.Query = url.Values(client.Query{}.TakeOrSelf(ctx, config.ResourceQuery))
			}
			if !config.ResourceHeader.IsNull() {
				impspec.Header = client.Header{}.TakeOrSelf(ctx, config.ResourceHeader)
			}
			if !config.ResourceBody.IsNull() {
				body, err := dynamic.ToJSON(config.ResourceBody)
				if err != nil {
					result.Diagnostics = append(result.Diagnostics, diag.NewErrorDiagnostic(
						"failed to convert the `resource_body` to json",
						err.Error(),
					))
					push(result)
					return
				}
				impspec.Body = ToPtr(json.RawMessage(body))
			}
			if !config.ResourceReadSelector.IsNull() {
				impspec.ReadSelector = config.ResourceReadSelector.ValueStringPointer()
			}
			if !config.ResourceReadResponseTemplate.IsNull() {
				impspec.ReadResponseTemplate = config.ResourceReadResponseTemplate.ValueStringPointer()
			}

			impspecJSON, err := json.Marshal(impspec)
			if err != nil {
				result.Diagnostics = append(result.Diagnostics, diag.NewErrorDiagnostic(
					"failed to JSON marshal import spec",
					err.Error(),
				))
				push(result)
				return
			}

			identity := resourceIdentityModel{
				ID: types.StringValue(string(impspecJSON)),
			}
			if diags := result.Identity.Set(ctx, identity); diags.HasError() {
				result.Diagnostics = append(result.Diagnostics, diags...)
				push(result)
				return
			}

			// Set the resource information on the result
			if diags := result.Resource.SetAttribute(ctx, tfpath.Root("id"), impspec.Id); diags.HasError() {
				result.Diagnostics = append(result.Diagnostics, diags...)
				push(result)
				return
			}
			if diags := result.Resource.SetAttribute(ctx, tfpath.Root("path"), impspec.Path); diags.HasError() {
				result.Diagnostics = append(result.Diagnostics, diags...)
				push(result)
				return
			}
			if diags := result.Resource.SetAttribute(ctx, tfpath.Root("read_selector"), impspec.ReadSelector); diags.HasError() {
				result.Diagnostics = append(result.Diagnostics, diags...)
				push(result)
				return
			}
			if diags := result.Resource.SetAttribute(ctx, tfpath.Root("read_response_template"), impspec.ReadResponseTemplate); diags.HasError() {
				result.Diagnostics = append(result.Diagnostics, diags...)
				push(result)
				return
			}

			if q := impspec.Query; q != nil {
				if diags := result.Resource.SetAttribute(ctx, tfpath.Root("query"), q); diags.HasError() {
					result.Diagnostics = append(result.Diagnostics, diags...)
					push(result)
					return
				}
			}
			if h := impspec.Header; h != nil {
				if diags := result.Resource.SetAttribute(ctx, tfpath.Root("header"), h); diags.HasError() {
					result.Diagnostics = append(result.Diagnostics, diags...)
					push(result)
					return
				}
			}

			body := types.DynamicNull()
			if nullBody := impspec.Body; nullBody != nil {
				nb, err := dynamic.FromJSONImplied(*nullBody)
				if err != nil {
					result.Diagnostics = append(result.Diagnostics, diag.NewErrorDiagnostic(
						"failed to convert the body in import spec to dynamic type",
						err.Error(),
					))
					push(result)
					return
				}
				body, err = dynamic.FromJSON(resRaw, nb.UnderlyingValue().Type(ctx))
			} else {
				body, err = dynamic.FromJSONImplied(resRaw)
			}
			if err != nil {
				result.Diagnostics = append(result.Diagnostics, diag.NewErrorDiagnostic(
					"failed to convert the response body to dynamic type",
					err.Error(),
				))
				push(result)
				return
			}
			if diags := result.Resource.SetAttribute(ctx, tfpath.Root("body"), body); diags.HasError() {
				result.Diagnostics = append(result.Diagnostics, diags...)
				push(result)
				return
			}

			// Send the result to the stream.
			if !push(result) {
				return
			}
		}
	}
}
