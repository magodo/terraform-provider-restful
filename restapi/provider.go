package restapi

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/magodo/terraform-provider-restapi/client"
	"github.com/magodo/terraform-provider-restapi/restapi/validator"
)

type provider struct {
	*client.Client
}

type providerData struct {
	BaseURL  string        `tfsdk:"base_url"`
	Security *securityData `tfsdk:"security"`
}

type securityData struct {
	OAuth2 *oauth2Data `tfsdk:"oauth2"`
}

type oauth2Data struct {
	ClientID       string              `tfsdk:"client_id"`
	ClientSecret   string              `tfsdk:"client_secret"`
	TokenUrl       string              `tfsdk:"token_url"`
	Scopes         []string            `tfsdk:"scopes"`
	EndpointParams map[string][]string `tfsdk:"endpoint_params"`
	AuthStyle      string              `tfsdk:"auth_style"`
}

func New() tfsdk.Provider {
	return &provider{}
}

func (*provider) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description:         "The schema of magodo/terraform-provider-restapi provider",
		MarkdownDescription: "The schema of magodo/terraform-provider-restapi provider",
		Attributes: map[string]tfsdk.Attribute{
			"base_url": {
				Type:                types.StringType,
				Description:         "The base URL of the API provider",
				MarkdownDescription: "The base URL of the API provider",
				Required:            true,
			},
			"security": {
				Description:         "The OpenAPI security scheme that is be used by the operations",
				MarkdownDescription: "The OpenAPI security scheme that is be used by the operations",
				Optional:            true,
				// TODO: Add Validator to ensure when this is non-null, there is only one specific scheme is set.
				Attributes: tfsdk.SingleNestedAttributes(
					map[string]tfsdk.Attribute{
						"oauth2": {
							Description:         "Configuration for the OAuth Client Credentials flow",
							MarkdownDescription: "Configuration for the OAuth Client Credentials flow",
							Optional:            true,
							Attributes: tfsdk.SingleNestedAttributes(
								map[string]tfsdk.Attribute{
									"client_id": {
										Type:                types.StringType,
										Description:         "The application's ID",
										MarkdownDescription: "The application's ID",
										Required:            true,
									},
									"client_secret": {
										Type:                types.StringType,
										Sensitive:           true,
										Description:         "The application's secret",
										MarkdownDescription: "The application's secret",
										Required:            true,
									},
									"token_url": {
										Type:                types.StringType,
										Description:         "The token URL to be used for this flow",
										MarkdownDescription: "The token URL to be used for this flow",
										Required:            true,
									},
									"scopes": {
										Type:                types.ListType{ElemType: types.StringType},
										Description:         "The optional requested permissions",
										MarkdownDescription: "The optional requested permissions",
										Optional:            true,
									},
									"endpoint_params": {
										Type:                types.MapType{ElemType: types.ListType{ElemType: types.StringType}},
										Description:         "The additional parameters for requests to the token endpoint.",
										MarkdownDescription: "The additional parameters for requests to the token endpoint.",
										Optional:            true,
									},
									"auth_style": {
										Type:                types.StringType,
										Description:         "How the endpoint wants the client ID & secret sent. Possible values are `in_params` and `in_header`. If absent, the style used will be auto detected.",
										MarkdownDescription: "How the endpoint wants the client ID & secret sent. Possible values are `in_params` and `in_header`. If absent, the style used will be auto detected.",
										Optional:            true,
										Validators:          []tfsdk.AttributeValidator{validator.StringInSlice(string(client.OAuth2AuthStyleInParams), string(client.OAuth2AuthStyleInHeader))},
									},
								},
							),
						},
					},
				),
			},
		},
	}, nil
}

func (p *provider) ValidateConfig(ctx context.Context, req tfsdk.ValidateProviderConfigRequest, resp *tfsdk.ValidateProviderConfigResponse) {
	var config providerData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	if _, err := url.Parse(config.BaseURL); err != nil {
		resp.Diagnostics.AddError(
			"Invalid configuration",
			"The `base_url` is not a valid URL",
		)
		return
	}
	if sec := config.Security; sec != nil {
		// Check whether there is exactly one scheme is defined.
		// TODO check there are not multiple schemes defined.

		switch {
		case sec.OAuth2 != nil:
			// Nothing to check further here.
		default:
			resp.Diagnostics.AddError(
				"Invalid configuration",
				"There is no security scheme defined",
			)
			return
		}
	}
}

func (p *provider) Configure(ctx context.Context, req tfsdk.ConfigureProviderRequest, resp *tfsdk.ConfigureProviderResponse) {
	var config providerData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	var opt client.Option
	if sec := config.Security; sec != nil {
		switch {
		case sec.OAuth2 != nil:
			opt.Security = client.OAuth2ClientCredentialOption{
				ClientID:       sec.OAuth2.ClientID,
				ClientSecret:   sec.OAuth2.ClientSecret,
				TokenURL:       sec.OAuth2.TokenUrl,
				Scopes:         sec.OAuth2.Scopes,
				EndpointParams: sec.OAuth2.EndpointParams,
				AuthStyle:      client.OAuth2AuthStyle(sec.OAuth2.AuthStyle),
			}
		}
	}

	client, err := client.NewClient(ctx, config.BaseURL, &opt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Provider configuration failure",
			fmt.Sprintf("failed to new client: %v", err),
		)
		return
	}
	p.Client = client
	return
}

func (*provider) GetResources(context.Context) (map[string]tfsdk.ResourceType, diag.Diagnostics) {
	return map[string]tfsdk.ResourceType{
		"restapi_resource": resourceType{},
	}, nil
}

func (*provider) GetDataSources(context.Context) (map[string]tfsdk.DataSourceType, diag.Diagnostics) {
	return map[string]tfsdk.DataSourceType{
		"restapi_resource": dataSourceType{},
	}, nil
}
