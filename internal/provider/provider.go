package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/magodo/terraform-provider-restful/internal/client"
	myvalidator "github.com/magodo/terraform-provider-restful/internal/validator"
)

type ProviderInterface interface {
	provider.ProviderWithMetadata
}

var _ ProviderInterface = &Provider{}

type Provider struct {
	client *client.Client
	apiOpt apiOption
}

type providerData struct {
	BaseURL            string              `tfsdk:"base_url"`
	Security           *securityData       `tfsdk:"security"`
	CreateMethod       *string             `tfsdk:"create_method"`
	UpdateMethod       *string             `tfsdk:"update_method"`
	DeleteMethod       *string             `tfsdk:"delete_method"`
	MergePatchDisabled *bool               `tfsdk:"merge_patch_disabled"`
	Query              map[string][]string `tfsdk:"query"`
	Header             map[string]string   `tfsdk:"header"`
}

type securityData struct {
	HTTP   *httpData    `tfsdk:"http"`
	OAuth2 *oauth2Data  `tfsdk:"oauth2"`
	APIKey []apikeyData `tfsdk:"apikey"`
}

type httpData struct {
	Basic *httpBasicData `tfsdk:"basic"`
	Token *httpTokenData `tfsdk:"token"`
}

type httpBasicData struct {
	Username *string `tfsdk:"username"`
	Password *string `tfsdk:"password"`
}

type httpTokenData struct {
	Token  *string `tfsdk:"token"`
	Scheme *string `tfsdk:"scheme"`
}

type apikeyData struct {
	Name  string `tfsdk:"name"`
	In    string `tfsdk:"in"`
	Value string `tfsdk:"value"`
}

type oauth2Data struct {
	Password          *oauth2PasswordData          `tfsdk:"password"`
	ClientCredentials *oauth2ClientCredentialsData `tfsdk:"client_credentials"`
	RefreshToken      *oauth2RefreshTokenData      `tfsdk:"refresh_token"`
}

type oauth2PasswordData struct {
	TokenUrl string `tfsdk:"token_url"`
	Username string `tfsdk:"username"`
	Password string `tfsdk:"password"`

	ClientID     *string  `tfsdk:"client_id"`
	ClientSecret *string  `tfsdk:"client_secret"`
	Scopes       []string `tfsdk:"scopes"`
	In           *string  `tfsdk:"in"`
}

type oauth2ClientCredentialsData struct {
	TokenUrl     string `tfsdk:"token_url"`
	ClientID     string `tfsdk:"client_id"`
	ClientSecret string `tfsdk:"client_secret"`

	EndpointParams map[string][]string `tfsdk:"endpoint_params"`
	Scopes         []string            `tfsdk:"scopes"`
	In             *string             `tfsdk:"in"`
}

type oauth2RefreshTokenData struct {
	TokenUrl     string `tfsdk:"token_url"`
	RefreshToken string `tfsdk:"refresh_token"`

	ClientID     *string  `tfsdk:"client_id"`
	ClientSecret *string  `tfsdk:"client_secret"`
	Scopes       []string `tfsdk:"scopes"`
	In           *string  `tfsdk:"in"`
	TokenType    *string  `tfsdk:"token_type"`
}

func New() provider.Provider {
	return &Provider{}
}

func (*Provider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "restful"
}

func (*Provider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		func() datasource.DataSource {
			return &DataSource{}
		},
	}
}

func (*Provider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource {
			return &Resource{}
		},
		func() resource.Resource {
			return &OperationResource{}
		},
	}
}

func (*Provider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "The restful provider provides resource and data source to interact with a platform that exposes a restful API.",
		MarkdownDescription: "The restful provider provides resource and data source to interact with a platform that exposes a restful API.",
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Description:         "The base URL of the API provider.",
				MarkdownDescription: "The base URL of the API provider.",
				Required:            true,
				Validators: []validator.String{
					myvalidator.StringIsParsable("HTTP url", func(s string) error {
						_, err := url.Parse(s)
						return err
					}),
				},
			},
			"security": schema.SingleNestedAttribute{
				Description:         "The OpenAPI security scheme that is be used for auth. Only one of `http`, `apikey` and `oauth2` can be specified.",
				MarkdownDescription: "The OpenAPI security scheme that is be used for auth. Only one of `http`, `apikey` and `oauth2` can be specified.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"http": schema.SingleNestedAttribute{
						Description:         "Configuration for the HTTP authentication scheme. Exactly one of `basic` and `token` must be specified.",
						MarkdownDescription: "Configuration for the HTTP authentication scheme. Exactly one of `basic` and `token` must be specified.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"basic": schema.SingleNestedAttribute{
								Description:         "Basic authentication",
								MarkdownDescription: "Basic authentication",
								Optional:            true,
								Attributes: map[string]schema.Attribute{
									"username": schema.StringAttribute{
										Description:         "The username",
										MarkdownDescription: "The username",
										Required:            true,
									},
									"password": schema.StringAttribute{
										Description:         "The password",
										MarkdownDescription: "The password",
										Required:            true,
										Sensitive:           true,
									},
								},
								Validators: []validator.Object{
									objectvalidator.ExactlyOneOf(
										path.MatchRoot("security").AtName("http").AtName("basic"),
										path.MatchRoot("security").AtName("http").AtName("token"),
									),
								},
							},
							"token": schema.SingleNestedAttribute{
								Description:         "Auth token (e.g. Bearer).",
								MarkdownDescription: "Auth token (e.g. Bearer).",
								Optional:            true,
								Attributes: map[string]schema.Attribute{
									"token": schema.StringAttribute{
										Description:         "The value of the token.",
										MarkdownDescription: "The value of the token.",
										Required:            true,
										Sensitive:           true,
									},
									"scheme": schema.StringAttribute{
										Description:         "The auth scheme. Defaults to `Bearer`.",
										MarkdownDescription: "The auth scheme. Defaults to `Bearer`.",
										Optional:            true,
									},
								},
								Validators: []validator.Object{
									objectvalidator.ExactlyOneOf(
										path.MatchRoot("security").AtName("http").AtName("basic"),
										path.MatchRoot("security").AtName("http").AtName("token"),
									),
								},
							},
						},
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRoot("security").AtName("http"),
								path.MatchRoot("security").AtName("apikey"),
								path.MatchRoot("security").AtName("oauth2"),
							),
						},
					},
					"apikey": schema.SetNestedAttribute{
						Description:         "Configuration for the API Key authentication scheme.",
						MarkdownDescription: "Configuration for the API Key authentication scheme.",
						Optional:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"name": schema.StringAttribute{
									Description:         "The API Key name",
									MarkdownDescription: "The API Key name",
									Required:            true,
								},
								"value": schema.StringAttribute{
									Description:         "The API Key value",
									MarkdownDescription: "The API Key value",
									Required:            true,
								},
								"in": schema.StringAttribute{
									Description: fmt.Sprintf("Specifies how is the API Key is sent. Possible values are `%s`, `%s` and `%s`.",
										client.APIKeyAuthInQuery, client.APIKeyAuthInHeader, client.APIKeyAuthInCookie),
									MarkdownDescription: fmt.Sprintf("Specifies how is the API Key is sent. Possible values are `%s`, `%s` and `%s`.",
										client.APIKeyAuthInQuery, client.APIKeyAuthInHeader, client.APIKeyAuthInCookie),
									Required: true,
									Validators: []validator.String{
										stringvalidator.OneOf(
											string(client.APIKeyAuthInHeader),
											string(client.APIKeyAuthInQuery),
											string(client.APIKeyAuthInCookie),
										),
									},
								},
							},
							Validators: []validator.Object{
								objectvalidator.ExactlyOneOf(
									path.MatchRoot("security").AtName("http"),
									path.MatchRoot("security").AtName("apikey"),
									path.MatchRoot("security").AtName("oauth2"),
								),
							},
						},
					},
					"oauth2": schema.SingleNestedAttribute{
						Description:         "Configuration for the OAuth2 authentication scheme. Exactly one of `password`, `client_credentials` and `refresh_token` must be specified.",
						MarkdownDescription: "Configuration for the OAuth2 authentication scheme. Exactly one of `password`, `client_credentials` and `refresh_token` must be specified.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"password": schema.SingleNestedAttribute{
								Description:         "Resource owner password credential.",
								MarkdownDescription: "[Resource owner password credential](https://www.rfc-editor.org/rfc/rfc6749#section-4.3).",
								Optional:            true,
								Attributes: map[string]schema.Attribute{
									"token_url": schema.StringAttribute{
										Description:         "The token URL to be used for this flow.",
										MarkdownDescription: "The token URL to be used for this flow.",
										Required:            true,
									},
									"username": schema.StringAttribute{
										Description:         "The username.",
										MarkdownDescription: "The username.",
										Required:            true,
									},
									"password": schema.StringAttribute{
										Sensitive:           true,
										Description:         "The password.",
										MarkdownDescription: "The password.",
										Required:            true,
									},
									"client_id": schema.StringAttribute{
										Description:         "The application's ID.",
										MarkdownDescription: "The application's ID.",
										Optional:            true,
									},
									"client_secret": schema.StringAttribute{
										Sensitive:           true,
										Description:         "The application's secret.",
										MarkdownDescription: "The application's secret.",
										Optional:            true,
									},
									"in": schema.StringAttribute{
										Description: fmt.Sprintf("Specifies how is the client ID & secret sent. Possible values are `%s` and `%s`. If absent, the style used will be auto detected.",
											client.OAuth2AuthStyleInParams, client.OAuth2AuthStyleInHeader),
										MarkdownDescription: fmt.Sprintf("Specifies how is th client ID & secret sent. Possible values are `%s` and `%s`. If absent, the style used will be auto detected.",
											client.OAuth2AuthStyleInParams, client.OAuth2AuthStyleInHeader),
										Optional:   true,
										Validators: []validator.String{stringvalidator.OneOf(string(client.OAuth2AuthStyleInParams), string(client.OAuth2AuthStyleInHeader))},
									},
									"scopes": schema.ListAttribute{
										ElementType:         types.StringType,
										Description:         "The optional requested permissions.",
										MarkdownDescription: "The optional requested permissions.",
										Optional:            true,
									},
								},
								Validators: []validator.Object{
									objectvalidator.ExactlyOneOf(
										path.MatchRoot("security").AtName("oauth2").AtName("password"),
										path.MatchRoot("security").AtName("oauth2").AtName("client_credentials"),
										path.MatchRoot("security").AtName("oauth2").AtName("refresh_token"),
									),
								},
							},
							"client_credentials": schema.SingleNestedAttribute{
								Description:         "Client credentials.",
								MarkdownDescription: "[Client credentials](https://www.rfc-editor.org/rfc/rfc6749#section-4.4).",
								Optional:            true,
								Attributes: map[string]schema.Attribute{
									"token_url": schema.StringAttribute{
										Description:         "The token URL to be used for this flow.",
										MarkdownDescription: "The token URL to be used for this flow.",
										Required:            true,
									},
									"client_id": schema.StringAttribute{
										Description:         "The application's ID.",
										MarkdownDescription: "The application's ID.",
										Required:            true,
									},
									"client_secret": schema.StringAttribute{
										Sensitive:           true,
										Description:         "The application's secret.",
										MarkdownDescription: "The application's secret.",
										Required:            true,
									},
									"in": schema.StringAttribute{
										Description: fmt.Sprintf("Specifies how is the client ID & secret sent. Possible values are `%s` and `%s`. If absent, the style used will be auto detected.",
											client.OAuth2AuthStyleInParams, client.OAuth2AuthStyleInHeader),
										MarkdownDescription: fmt.Sprintf("Specifies how is th client ID & secret sent. Possible values are `%s` and `%s`. If absent, the style used will be auto detected.",
											client.OAuth2AuthStyleInParams, client.OAuth2AuthStyleInHeader),
										Optional:   true,
										Validators: []validator.String{stringvalidator.OneOf(string(client.OAuth2AuthStyleInParams), string(client.OAuth2AuthStyleInHeader))},
									},
									"scopes": schema.ListAttribute{
										ElementType:         types.StringType,
										Description:         "The optional requested permissions.",
										MarkdownDescription: "The optional requested permissions.",
										Optional:            true,
									},
									"endpoint_params": schema.MapAttribute{
										ElementType:         types.ListType{ElemType: types.StringType},
										Description:         "The additional parameters for requests to the token endpoint.",
										MarkdownDescription: "The additional parameters for requests to the token endpoint.",
										Optional:            true,
									},
								},
								Validators: []validator.Object{
									objectvalidator.ExactlyOneOf(
										path.MatchRoot("security").AtName("oauth2").AtName("password"),
										path.MatchRoot("security").AtName("oauth2").AtName("client_credentials"),
										path.MatchRoot("security").AtName("oauth2").AtName("refresh_token"),
									),
								},
							},
							"refresh_token": schema.SingleNestedAttribute{
								Description:         "Refresh token.",
								MarkdownDescription: "[Refresh token](https://www.rfc-editor.org/rfc/rfc6749#section-6).",
								Optional:            true,
								Attributes: map[string]schema.Attribute{
									"token_url": schema.StringAttribute{
										Description:         "The token URL to be used for this flow.",
										MarkdownDescription: "The token URL to be used for this flow.",
										Required:            true,
									},
									"refresh_token": schema.StringAttribute{
										Description:         "The refresh token.",
										MarkdownDescription: "The refresh token.",
										Sensitive:           true,
										Required:            true,
									},
									"client_id": schema.StringAttribute{
										Description:         "The application's ID.",
										MarkdownDescription: "The application's ID.",
										Optional:            true,
									},
									"client_secret": schema.StringAttribute{
										Sensitive:           true,
										Description:         "The application's secret.",
										MarkdownDescription: "The application's secret.",
										Optional:            true,
									},
									"scopes": schema.ListAttribute{
										ElementType:         types.StringType,
										Description:         "The optional requested permissions.",
										MarkdownDescription: "The optional requested permissions.",
										Optional:            true,
									},
									"in": schema.StringAttribute{
										Description: fmt.Sprintf("Specifies how is the client ID & secret sent. Possible values are `%s` and `%s`. If absent, the style used will be auto detected.",
											client.OAuth2AuthStyleInParams, client.OAuth2AuthStyleInHeader),
										MarkdownDescription: fmt.Sprintf("Specifies how is th client ID & secret sent. Possible values are `%s` and `%s`. If absent, the style used will be auto detected.",
											client.OAuth2AuthStyleInParams, client.OAuth2AuthStyleInHeader),
										Optional:   true,
										Validators: []validator.String{stringvalidator.OneOf(string(client.OAuth2AuthStyleInParams), string(client.OAuth2AuthStyleInHeader))},
									},
									"token_type": schema.StringAttribute{
										Description:         `The type of the access token.`,
										MarkdownDescription: `The type of the access token.`,
										Optional:            true,
									},
								},
								Validators: []validator.Object{
									objectvalidator.ExactlyOneOf(
										path.MatchRoot("security").AtName("oauth2").AtName("password"),
										path.MatchRoot("security").AtName("oauth2").AtName("client_credentials"),
										path.MatchRoot("security").AtName("oauth2").AtName("refresh_token"),
									),
								},
							},
						},
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRoot("security").AtName("http"),
								path.MatchRoot("security").AtName("apikey"),
								path.MatchRoot("security").AtName("oauth2"),
							),
						},
					},
				},
			},
			"create_method": schema.StringAttribute{
				Description:         "The method used to create the resource. Possible values are `PUT` and `POST`. Defaults to `POST`.",
				MarkdownDescription: "The method used to create the resource. Possible values are `PUT` and `POST`. Defaults to `POST`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("PUT", "POST"),
				},
			},
			"update_method": schema.StringAttribute{
				Description:         "The method used to update the resource. Possible values are `PUT` and `PATCH`. Defaults to `PUT`.",
				MarkdownDescription: "The method used to update the resource. Possible values are `PUT` and `PATCH`. Defaults to `PUT`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("PUT", "PATCH"),
				},
			},
			"delete_method": schema.StringAttribute{
				Description:         "The method used to delete the resource. Possible values are `DELETE` and `POST`. Defaults to `DELETE`.",
				MarkdownDescription: "The method used to delete the resource. Possible values are `DELETE` and `POST`. Defaults to `DELETE`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("DELETE", "POST"),
				},
			},
			"merge_patch_disabled": schema.BoolAttribute{
				Description:         "Whether to use a JSON Merge Patch as the request body in the PATCH update? Defaults to `false`. This is only effective when `update_method` is set to `PATCH`.",
				MarkdownDescription: "Whether to use a JSON Merge Patch as the request body in the PATCH update? Defaults to `false`. This is only effective when `update_method` is set to `PATCH`.",
				Optional:            true,
			},
			"query": schema.MapAttribute{
				Description:         "The query parameters that are applied to each request.",
				MarkdownDescription: "The query parameters that are applied to each request.",
				ElementType:         types.ListType{ElemType: types.StringType},
				Optional:            true,
			},
			"header": schema.MapAttribute{
				Description:         "The header parameters that are applied to each request.",
				MarkdownDescription: "The header parameters that are applied to each request.",
				ElementType:         types.StringType,
				Optional:            true,
			},
		},
	}
}

func (p *Provider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	clientOpt := client.BuildOption{}
	if sec := config.Security; sec != nil {
		switch {
		case sec.HTTP != nil:
			switch {
			case sec.HTTP.Basic != nil:
				opt := client.HTTPBasicOption{
					Username: *sec.HTTP.Basic.Username,
					Password: *sec.HTTP.Basic.Password,
				}
				clientOpt.Security = opt
			case sec.HTTP.Token != nil:
				opt := client.HTTPTokenOption{
					Token: *sec.HTTP.Token.Token,
				}
				if scheme := sec.HTTP.Token.Scheme; scheme != nil {
					opt.Scheme = *scheme
				}
				clientOpt.Security = opt
			default:
				panic("`security.http` unhandled, this implies error in the provider schema definition")
			}
		case sec.APIKey != nil:
			opt := client.APIKeyAuthOption{}
			for _, apikey := range sec.APIKey {
				opt = append(opt, client.APIKeyAuthOpt{
					Name:  apikey.Name,
					In:    client.APIKeyAuthIn(apikey.In),
					Value: apikey.Value,
				})
			}
			clientOpt.Security = opt
		case sec.OAuth2 != nil:
			switch {
			case sec.OAuth2.Password != nil:
				opt := client.OAuth2PasswordOption{
					TokenURL: sec.OAuth2.Password.TokenUrl,
					Username: sec.OAuth2.Password.Username,
					Password: sec.OAuth2.Password.Password,
				}
				if v := sec.OAuth2.Password.ClientID; v != nil {
					opt.ClientId = *v
				}
				if v := sec.OAuth2.Password.ClientSecret; v != nil {
					opt.ClientSecret = *v
				}
				if v := sec.OAuth2.Password.In; v != nil {
					opt.AuthStyle = client.OAuth2AuthStyle(*v)
				}
				if v := sec.OAuth2.Password.Scopes; len(v) != 0 {
					opt.Scopes = v
				}
				clientOpt.Security = opt
			case sec.OAuth2.ClientCredentials != nil:
				opt := client.OAuth2ClientCredentialOption{
					TokenURL:     sec.OAuth2.ClientCredentials.TokenUrl,
					ClientId:     sec.OAuth2.ClientCredentials.ClientID,
					ClientSecret: sec.OAuth2.ClientCredentials.ClientSecret,
				}
				if v := sec.OAuth2.ClientCredentials.Scopes; len(v) != 0 {
					opt.Scopes = v
				}
				if v := sec.OAuth2.ClientCredentials.EndpointParams; len(v) != 0 {
					opt.EndpointParams = v
				}
				if v := sec.OAuth2.ClientCredentials.In; v != nil {
					opt.AuthStyle = client.OAuth2AuthStyle(*v)
				}
				clientOpt.Security = opt
			case sec.OAuth2.RefreshToken != nil:
				opt := client.OAuth2RefreshTokenOption{
					TokenURL:     sec.OAuth2.RefreshToken.TokenUrl,
					RefreshToken: sec.OAuth2.RefreshToken.RefreshToken,
				}
				if v := sec.OAuth2.RefreshToken.ClientID; v != nil {
					opt.ClientId = *v
				}
				if v := sec.OAuth2.RefreshToken.ClientSecret; v != nil {
					opt.RefreshToken = *v
				}
				if v := sec.OAuth2.RefreshToken.In; v != nil {
					opt.AuthStyle = client.OAuth2AuthStyle(*v)
				}
				if v := sec.OAuth2.RefreshToken.TokenType; v != nil {
					opt.TokenType = *v
				}
				if v := sec.OAuth2.RefreshToken.Scopes; len(v) != 0 {
					opt.Scopes = v
				}
				clientOpt.Security = opt
			default:
				panic("`security.oauth2` unhandled, this implies error in the provider schema definition")
			}
		default:
			panic("`security` unhandled, this implies error in the provider schema definition")
		}
	}

	var err error
	p.client, err = client.New(ctx, config.BaseURL, &clientOpt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to configure provider",
			fmt.Sprintf("Failed to new client: %v", err),
		)
	}

	uRL, err := url.Parse(config.BaseURL)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to configure provider",
			fmt.Sprintf("Parsing the base url %q: %v", config.BaseURL, err),
		)
	}

	p.apiOpt = apiOption{
		BaseURL:            *uRL,
		CreateMethod:       "POST",
		UpdateMethod:       "PUT",
		DeleteMethod:       "DELETE",
		MergePatchDisabled: false,
		Query:              map[string][]string{},
		Header:             map[string]string{},
	}
	if config.CreateMethod != nil {
		p.apiOpt.CreateMethod = *config.CreateMethod
	}
	if config.UpdateMethod != nil {
		p.apiOpt.UpdateMethod = *config.UpdateMethod
	}
	if config.DeleteMethod != nil {
		p.apiOpt.DeleteMethod = *config.DeleteMethod
	}
	if config.MergePatchDisabled != nil {
		p.apiOpt.MergePatchDisabled = *config.MergePatchDisabled
	}
	if config.Query != nil {
		p.apiOpt.Query = config.Query
	}
	if config.Header != nil {
		p.apiOpt.Header = config.Header
	}

	resp.ResourceData = p
	resp.DataSourceData = p

	return
}
