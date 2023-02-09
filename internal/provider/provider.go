package provider

import (
	"context"
	"fmt"
	"net/url"
	"sync"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/magodo/terraform-provider-restful/internal/client"
	myvalidator "github.com/magodo/terraform-provider-restful/internal/validator"
)

var _ provider.Provider = &Provider{}

type Provider struct {
	client *client.Client
	apiOpt apiOption
	once   sync.Once
}

type providerData struct {
	config   providerConfig
	provider *Provider
}

type providerConfig struct {
	BaseURL            types.String `tfsdk:"base_url"`
	Security           types.Object `tfsdk:"security"`
	CreateMethod       types.String `tfsdk:"create_method"`
	UpdateMethod       types.String `tfsdk:"update_method"`
	DeleteMethod       types.String `tfsdk:"delete_method"`
	MergePatchDisabled types.Bool   `tfsdk:"merge_patch_disabled"`
	Query              types.Map    `tfsdk:"query"`
	Header             types.Map    `tfsdk:"header"`
	CookieEnabled      types.Bool   `tfsdk:"cookie_enabled"`
}

type securityData struct {
	HTTP   types.Object `tfsdk:"http"`
	OAuth2 types.Object `tfsdk:"oauth2"`
	APIKey types.Set    `tfsdk:"apikey"`
}

type httpData struct {
	Basic types.Object `tfsdk:"basic"`
	Token types.Object `tfsdk:"token"`
}

type httpBasicData struct {
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

type httpTokenData struct {
	Token  types.String `tfsdk:"token"`
	Scheme types.String `tfsdk:"scheme"`
}

type apikeyData struct {
	Name  types.String `tfsdk:"name"`
	In    types.String `tfsdk:"in"`
	Value types.String `tfsdk:"value"`
}

type oauth2Data struct {
	Password          types.Object `tfsdk:"password"`
	ClientCredentials types.Object `tfsdk:"client_credentials"`
	RefreshToken      types.Object `tfsdk:"refresh_token"`
}

type oauth2PasswordData struct {
	TokenUrl types.String `tfsdk:"token_url"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`

	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	Scopes       types.List   `tfsdk:"scopes"`
	In           types.String `tfsdk:"in"`
}

type oauth2ClientCredentialsData struct {
	TokenUrl     types.String `tfsdk:"token_url"`
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`

	EndpointParams types.Map    `tfsdk:"endpoint_params"`
	Scopes         types.List   `tfsdk:"scopes"`
	In             types.String `tfsdk:"in"`
}

type oauth2RefreshTokenData struct {
	TokenUrl     types.String `tfsdk:"token_url"`
	RefreshToken types.String `tfsdk:"refresh_token"`

	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	Scopes       types.List   `tfsdk:"scopes"`
	In           types.String `tfsdk:"in"`
	TokenType    types.String `tfsdk:"token_type"`
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
							objectvalidator.ConflictsWith(
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
						},
						Validators: []validator.Set{
							setvalidator.ConflictsWith(
								path.MatchRoot("security").AtName("http"),
								path.MatchRoot("security").AtName("oauth2"),
							),
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
										Description:         `The type of the access token. Defaults to "Bearer".`,
										MarkdownDescription: `The type of the access token. Defaults to "Bearer".`,
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
							objectvalidator.ConflictsWith(
								path.MatchRoot("security").AtName("http"),
								path.MatchRoot("security").AtName("apikey"),
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
			"cookie_enabled": schema.BoolAttribute{
				Description:         "Save cookies during API contracting. Defaults to `false`.",
				MarkdownDescription: "Save cookies during API contracting. Defaults to `false`.",
				Optional:            true,
			},
		},
	}
}

func (p *Provider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	data := providerData{
		provider: &Provider{},
	}
	diags := req.Config.Get(ctx, &data.config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	resp.ResourceData = data
	resp.DataSourceData = data

	return
}

func (p *Provider) Init(ctx context.Context, config providerConfig) diag.Diagnostics {
	var odiags diag.Diagnostics
	p.once.Do(func() {
		clientOpt := client.BuildOption{
			CookieEnabled: config.CookieEnabled.ValueBool(),
		}

		if secRaw := config.Security; !secRaw.IsNull() {
			var sec securityData
			if diags := secRaw.As(ctx, &sec, basetypes.ObjectAsOptions{}); diags.HasError() {
				odiags = diags
				return
			}
			switch {
			case !sec.HTTP.IsNull():
				var http httpData
				if diags := sec.HTTP.As(ctx, &http, basetypes.ObjectAsOptions{}); diags.HasError() {
					odiags = diags
					return
				}
				switch {
				case !http.Basic.IsNull():
					var basic httpBasicData
					if diags := http.Basic.As(ctx, &basic, basetypes.ObjectAsOptions{}); diags.HasError() {
						odiags = diags
						return
					}
					opt := client.HTTPBasicOption{
						Username: basic.Username.ValueString(),
						Password: basic.Password.ValueString(),
					}
					clientOpt.Security = opt
				case !http.Token.IsNull():
					var token httpTokenData
					if diags := http.Token.As(ctx, &token, basetypes.ObjectAsOptions{}); diags.HasError() {
						odiags = diags
						return
					}
					opt := client.HTTPTokenOption{
						Token:  token.Token.ValueString(),
						Scheme: token.Scheme.ValueString(),
					}
					clientOpt.Security = opt
				}
			case !sec.APIKey.IsNull():
				opt := client.APIKeyAuthOption{}
				for _, apikeyRaw := range sec.APIKey.Elements() {
					apikeyObj := apikeyRaw.(types.Object)
					if apikeyObj.IsNull() {
						continue
					}
					var apikey apikeyData
					if diags := apikeyObj.As(ctx, &apikey, basetypes.ObjectAsOptions{}); diags.HasError() {
						odiags = diags
						return
					}
					opt = append(opt, client.APIKeyAuthOpt{
						Name:  apikey.Name.ValueString(),
						In:    client.APIKeyAuthIn(apikey.In.ValueString()),
						Value: apikey.Value.ValueString(),
					})
				}
				clientOpt.Security = opt
			case !sec.OAuth2.IsNull():
				var oauth2 oauth2Data
				if diags := sec.OAuth2.As(ctx, &oauth2, basetypes.ObjectAsOptions{}); diags.HasError() {
					odiags = diags
					return
				}
				switch {
				case !oauth2.Password.IsNull():
					var password oauth2PasswordData
					if diags := oauth2.Password.As(ctx, &oauth2, basetypes.ObjectAsOptions{}); diags.HasError() {
						odiags = diags
						return
					}
					opt := client.OAuth2PasswordOption{
						TokenURL:     password.TokenUrl.ValueString(),
						Username:     password.Username.ValueString(),
						Password:     password.Password.ValueString(),
						ClientId:     password.ClientID.ValueString(),
						ClientSecret: password.ClientSecret.ValueString(),
						AuthStyle:    client.OAuth2AuthStyle(password.In.ValueString()),
					}
					if !password.Scopes.IsNull() {
						var scopes []string
						for _, scope := range password.Scopes.Elements() {
							scope := scope.(types.String)
							if scope.IsNull() {
								continue
							}
							scopes = append(scopes, scope.ValueString())
						}
						opt.Scopes = scopes
					}
					clientOpt.Security = opt
				case !oauth2.ClientCredentials.IsNull():
					var cc oauth2ClientCredentialsData
					if diags := oauth2.ClientCredentials.As(ctx, &cc, basetypes.ObjectAsOptions{}); diags.HasError() {
						odiags = diags
						return
					}
					opt := client.OAuth2ClientCredentialOption{
						TokenURL:     cc.TokenUrl.ValueString(),
						ClientId:     cc.ClientID.ValueString(),
						ClientSecret: cc.ClientSecret.ValueString(),
						AuthStyle:    client.OAuth2AuthStyle(cc.In.ValueString()),
					}
					if !cc.Scopes.IsNull() {
						var scopes []string
						for _, scope := range cc.Scopes.Elements() {
							scope := scope.(types.String)
							if scope.IsNull() {
								continue
							}
							scopes = append(scopes, scope.ValueString())
						}
						opt.Scopes = scopes
					}
					if !cc.EndpointParams.IsNull() {
						endpointParams := map[string][]string{}
						for k, values := range cc.EndpointParams.Elements() {
							var vs []string
							values := values.(types.List)
							for _, value := range values.Elements() {
								value := value.(types.String)
								if value.IsNull() {
									continue
								}
								vs = append(vs, value.ValueString())
							}
							endpointParams[k] = vs
						}
						opt.EndpointParams = endpointParams
					}
					clientOpt.Security = opt
				case !oauth2.RefreshToken.IsNull():
					var refreshToken oauth2RefreshTokenData
					if diags := oauth2.RefreshToken.As(ctx, &refreshToken, basetypes.ObjectAsOptions{}); diags.HasError() {
						odiags = diags
						return
					}

					opt := client.OAuth2RefreshTokenOption{
						TokenURL:     refreshToken.TokenUrl.ValueString(),
						RefreshToken: refreshToken.RefreshToken.ValueString(),
						ClientId:     refreshToken.ClientID.ValueString(),
						ClientSecret: refreshToken.ClientSecret.ValueString(),
						AuthStyle:    client.OAuth2AuthStyle(refreshToken.In.ValueString()),
						TokenType:    refreshToken.TokenType.ValueString(),
					}
					if !refreshToken.Scopes.IsNull() {
						var scopes []string
						for _, scope := range refreshToken.Scopes.Elements() {
							scope := scope.(types.String)
							if scope.IsNull() {
								continue
							}
							scopes = append(scopes, scope.ValueString())
						}
						opt.Scopes = scopes
					}
					clientOpt.Security = opt
				}
			}
		}

		var (
			diags diag.Diagnostics
			err   error
		)
		p.client, err = client.New(ctx, config.BaseURL.ValueString(), &clientOpt)
		if err != nil {
			diags.AddError(
				"Failed to configure provider",
				fmt.Sprintf("Failed to new client: %v", err),
			)
			odiags = diags
			return
		}

		uRL, err := url.Parse(config.BaseURL.ValueString())
		if err != nil {
			diags.AddError(
				"Failed to configure provider",
				fmt.Sprintf("Parsing the base url %q: %v", config.BaseURL, err),
			)
			odiags = diags
			return
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
		if !config.CreateMethod.IsNull() {
			p.apiOpt.CreateMethod = config.CreateMethod.ValueString()
		}
		if !config.UpdateMethod.IsNull() {
			p.apiOpt.UpdateMethod = config.UpdateMethod.ValueString()
		}
		if !config.DeleteMethod.IsNull() {
			p.apiOpt.DeleteMethod = config.DeleteMethod.ValueString()
		}
		if !config.MergePatchDisabled.IsNull() {
			p.apiOpt.MergePatchDisabled = config.MergePatchDisabled.ValueBool()
		}
		if !config.Query.IsNull() {
			queries := map[string][]string{}
			for k, values := range config.Query.Elements() {
				var vs []string
				values := values.(types.List)
				for _, value := range values.Elements() {
					value := value.(types.String)
					if value.IsNull() {
						continue
					}
					vs = append(vs, value.ValueString())
				}
				queries[k] = vs
			}
			p.apiOpt.Query = queries
		}
		if !config.Header.IsNull() {
			headers := map[string]string{}
			for k, value := range config.Header.Elements() {
				value := value.(types.String)
				if value.IsNull() {
					continue
				}
				headers[k] = value.ValueString()
			}
			p.apiOpt.Header = headers
		}
	})

	return odiags
}
