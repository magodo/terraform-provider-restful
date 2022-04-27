package provider

import (
	"context"
	"fmt"
	client2 "github.com/magodo/terraform-provider-restful/internal/client"
	"github.com/magodo/terraform-provider-restful/internal/validator"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type apiOption struct {
	CreateMethod string
	Query        map[string]string
}

type provider struct {
	client *client2.Client
	apiOpt apiOption
}

type providerData struct {
	BaseURL      string            `tfsdk:"base_url"`
	Security     *securityData     `tfsdk:"security"`
	CreateMethod *string           `tfsdk:"create_method"`
	Query        map[string]string `tfsdk:"query"`
}

type securityData struct {
	HTTP   *httpData   `tfsdk:"http"`
	OAuth2 *oauth2Data `tfsdk:"oauth2"`
}

type httpData struct {
	Type     string  `tfsdk:"type"`
	Username *string `tfsdk:"username"`
	Password *string `tfsdk:"password"`
}

type oauth2Data struct {
	ClientID       string              `tfsdk:"client_id"`
	ClientSecret   string              `tfsdk:"client_secret"`
	TokenUrl       string              `tfsdk:"token_url"`
	Scopes         []string            `tfsdk:"scopes"`
	EndpointParams map[string][]string `tfsdk:"endpoint_params"`
	AuthStyle      *string             `tfsdk:"auth_style"`
}

func New() tfsdk.Provider {
	return &provider{}
}

func (*provider) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description:         "The schema of magodo/terraform-provider-restful provider",
		MarkdownDescription: "The schema of magodo/terraform-provider-restful provider",
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
				Attributes: tfsdk.SingleNestedAttributes(
					map[string]tfsdk.Attribute{
						"http": {
							Description:         "Configuration for the HTTP authentication scheme",
							MarkdownDescription: "Configuration for the HTTP authentication scheme",
							Optional:            true,
							Attributes: tfsdk.SingleNestedAttributes(
								map[string]tfsdk.Attribute{
									"type": {
										Description:         fmt.Sprintf("The type of the authentication scheme. Possible values are `%s`", client2.HTTPAuthTypeBasic),
										MarkdownDescription: fmt.Sprintf("The type of the authentication scheme. Possible values are `%s`", client2.HTTPAuthTypeBasic),
										Required:            true,
										Type:                types.StringType,
										Validators:          []tfsdk.AttributeValidator{validator.StringInSlice(string(client2.HTTPAuthTypeBasic))},
									},
									"username": {
										Description:         "The username",
										MarkdownDescription: "The username",
										Type:                types.StringType,
										Optional:            true,
									},
									"password": {
										Description:         "The user password",
										MarkdownDescription: "The user password",
										Type:                types.StringType,
										Optional:            true,
										Sensitive:           true,
									},
								},
							),
						},
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
										Type: types.StringType,
										Description: fmt.Sprintf("How the endpoint wants the client ID & secret sent. Possible values are `%s` and `%s`. If absent, the style used will be auto detected.",
											client2.OAuth2AuthStyleInParams, client2.OAuth2AuthStyleInHeader),
										MarkdownDescription: fmt.Sprintf("How the endpoint wants the client ID & secret sent. Possible values are `%s` and `%s`. If absent, the style used will be auto detected.",
											client2.OAuth2AuthStyleInParams, client2.OAuth2AuthStyleInHeader),
										Optional:   true,
										Validators: []tfsdk.AttributeValidator{validator.StringInSlice(string(client2.OAuth2AuthStyleInParams), string(client2.OAuth2AuthStyleInHeader))},
									},
								},
							),
						},
					},
				),
			},
			"create_method": {
				Type:                types.StringType,
				Description:         "The method used to create the resource. Possible values are `PUT` and `POST`. Defaults to `POST`.",
				MarkdownDescription: "The method used to create the resource. Possible values are `PUT` and `POST`. Defaults to `POST`.",
				Optional:            true,
				// Need a way to set the default value, plan modifier doesn't work here.
				Validators: []tfsdk.AttributeValidator{validator.StringInSlice("PUT", "POST")},
			},
			"query": {
				Description:         "The query parameters that are applied to each request",
				MarkdownDescription: "The query parameters that are applied to each request",
				Type:                types.MapType{ElemType: types.StringType},
				Optional:            true,
			},
		},
	}, nil
}

func (p *provider) ValidateConfig(ctx context.Context, req tfsdk.ValidateProviderConfigRequest, resp *tfsdk.ValidateProviderConfigResponse) {
	type pt struct {
		BaseURL      types.String `tfsdk:"base_url"`
		Security     types.Object `tfsdk:"security"`
		CreateMethod types.String `tfsdk:"create_method"`
		Query        types.Map    `tfsdk:"query"`
	}

	var config pt
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if !config.BaseURL.Unknown {
		if _, err := url.Parse(config.BaseURL.Value); err != nil {
			resp.Diagnostics.AddError(
				"Invalid configuration",
				"The `base_url` is not a valid URL",
			)
			return
		}
	}

	if !config.Security.Unknown {
		httpObj := config.Security.Attrs["http"].(types.Object)
		oauth2Obj := config.Security.Attrs["oauth2"].(types.Object)

		l := []string{}
		if !httpObj.Null && !httpObj.Unknown {
			l = append(l, "http")
		}
		if !oauth2Obj.Null && !oauth2Obj.Unknown {
			l = append(l, "oauth2")
		}
		if len(l) > 1 {
			resp.Diagnostics.AddError(
				"Invalid configuration",
				"More than one scheme is specified: "+strings.Join(l, ","),
			)
			return
		}

		// In case any of the block is unknown, we don't know whether it will evaluate into null or not.
		// Here, we do best effort to ensure at least one of them is set.
		if httpObj.Null && oauth2Obj.Null {
			if len(l) == 0 {
				resp.Diagnostics.AddError(
					"Invalid configuration",
					"There is no security scheme specified",
				)
				return
			}
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

	clientOpt := client2.BuildOption{}
	if sec := config.Security; sec != nil {
		switch {
		case sec.HTTP != nil:
			sopt := client2.HTTPAuthOption{
				Type: client2.HTTPAuthTypeBasic,
			}
			if sec.HTTP.Username != nil {
				sopt.Username = *sec.HTTP.Username
			}
			if sec.HTTP.Password != nil {
				sopt.Password = *sec.HTTP.Password
			}
			clientOpt.Security = sopt
		case sec.OAuth2 != nil:
			sopt := client2.OAuth2ClientCredentialOption{
				ClientID:       sec.OAuth2.ClientID,
				ClientSecret:   sec.OAuth2.ClientSecret,
				TokenURL:       sec.OAuth2.TokenUrl,
				Scopes:         sec.OAuth2.Scopes,
				EndpointParams: sec.OAuth2.EndpointParams,
			}
			if sec.OAuth2.AuthStyle != nil {
				sopt.AuthStyle = client2.OAuth2AuthStyle(*sec.OAuth2.AuthStyle)
			}
			clientOpt.Security = sopt
		default:
			resp.Diagnostics.AddError(
				"Failed to configure provider",
				"There is no security scheme specified",
			)
			return
		}
	}

	var err error
	p.client, err = client2.New(config.BaseURL, &clientOpt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to configure provider",
			fmt.Sprintf("Failed to new client: %v", err),
		)
	}

	p.apiOpt = apiOption{
		CreateMethod: "POST",
		Query:        map[string]string{},
	}
	if config.CreateMethod != nil {
		p.apiOpt.CreateMethod = *config.CreateMethod
	}
	if config.Query != nil {
		p.apiOpt.Query = config.Query
	}

	return
}

func (*provider) GetResources(context.Context) (map[string]tfsdk.ResourceType, diag.Diagnostics) {
	return map[string]tfsdk.ResourceType{
		"restful_resource": resourceType{},
	}, nil
}

func (*provider) GetDataSources(context.Context) (map[string]tfsdk.DataSourceType, diag.Diagnostics) {
	return map[string]tfsdk.DataSourceType{
		"restful_resource": dataSourceType{},
	}, nil
}
