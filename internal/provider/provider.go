package provider

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/magodo/terraform-provider-restful/internal/client"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type ProviderInterface interface {
	provider.ProviderWithMetadata
	provider.ProviderWithValidateConfig
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
	Type     string  `tfsdk:"type"`
	Username *string `tfsdk:"username"`
	Password *string `tfsdk:"password"`
	Token    *string `tfsdk:"token"`
}

type oauth2Data struct {
	ClientID       *string             `tfsdk:"client_id"`
	ClientSecret   *string             `tfsdk:"client_secret"`
	Username       *string             `tfsdk:"username"`
	Password       *string             `tfsdk:"password"`
	TokenUrl       string              `tfsdk:"token_url"`
	Scopes         []string            `tfsdk:"scopes"`
	EndpointParams map[string][]string `tfsdk:"endpoint_params"`
	In             *string             `tfsdk:"in"`
}

type apikeyData struct {
	Name  string `tfsdk:"name"`
	In    string `tfsdk:"in"`
	Value string `tfsdk:"value"`
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

func (*Provider) GetSchema(context.Context) (tfsdk.Schema, diag.Diagnostics) {
	return tfsdk.Schema{
		Description:         "The restful provider provides resource and data source to interact with a platform that exposes a restful API.",
		MarkdownDescription: "The restful provider provides resource and data source to interact with a platform that exposes a restful API.",
		Attributes: map[string]tfsdk.Attribute{
			"base_url": {
				Type:                types.StringType,
				Description:         "The base URL of the API provider.",
				MarkdownDescription: "The base URL of the API provider.",
				Required:            true,
			},
			"security": {
				Description:         "The OpenAPI security scheme that is be used for auth.",
				MarkdownDescription: "The OpenAPI security scheme that is be used for auth.",
				Optional:            true,
				Attributes: tfsdk.SingleNestedAttributes(
					map[string]tfsdk.Attribute{
						"http": {
							Description:         "Configuration for the HTTP authentication scheme.",
							MarkdownDescription: "Configuration for the HTTP authentication scheme.",
							Optional:            true,
							Attributes: tfsdk.SingleNestedAttributes(
								map[string]tfsdk.Attribute{
									"type": {
										Description:         fmt.Sprintf("The type of the authentication scheme. Possible values are `%s`, `%s`.", client.HTTPAuthTypeBasic, client.HTTPAuthTypeBearer),
										MarkdownDescription: fmt.Sprintf("The type of the authentication scheme. Possible values are `%s`, `%s`.", client.HTTPAuthTypeBasic, client.HTTPAuthTypeBearer),
										Required:            true,
										Type:                types.StringType,
										Validators: []tfsdk.AttributeValidator{stringvalidator.OneOf(
											string(client.HTTPAuthTypeBasic),
											string(client.HTTPAuthTypeBearer),
										)},
									},
									"username": {
										Description:         fmt.Sprintf("The username, required when `type` is `%s`.", client.HTTPAuthTypeBasic),
										MarkdownDescription: fmt.Sprintf("The username, required when `type` is `%s`.", client.HTTPAuthTypeBasic),
										Type:                types.StringType,
										Optional:            true,
									},
									"password": {
										Description:         fmt.Sprintf("The password, required when `type` is `%s`.", client.HTTPAuthTypeBasic),
										MarkdownDescription: fmt.Sprintf("The password, required when `type` is `%s`.", client.HTTPAuthTypeBasic),
										Type:                types.StringType,
										Optional:            true,
										Sensitive:           true,
									},
									"token": {
										Description:         fmt.Sprintf("The value of the token, required when `type` is `%s`.", client.HTTPAuthTypeBearer),
										MarkdownDescription: fmt.Sprintf("The value of the token, required when `type` is `%s`.", client.HTTPAuthTypeBearer),
										Type:                types.StringType,
										Optional:            true,
										Sensitive:           true,
									},
								},
							),
						},
						"apikey": {
							Description:         "Configuration for the API Key authentication scheme.",
							MarkdownDescription: "Configuration for the API Key authentication scheme.",
							Optional:            true,
							Attributes: tfsdk.SetNestedAttributes(
								map[string]tfsdk.Attribute{
									"name": {
										Description:         "The API Key name",
										MarkdownDescription: "The API Key name",
										Required:            true,
										Type:                types.StringType,
									},
									"value": {
										Description:         "The API Key value",
										MarkdownDescription: "The API Key value",
										Required:            true,
										Type:                types.StringType,
									},
									"in": {
										Description: fmt.Sprintf("Specifies how is the API Key is sent. Possible values are `%s`, `%s` and `%s`.",
											client.APIKeyAuthInQuery, client.APIKeyAuthInHeader, client.APIKeyAuthInCookie),
										MarkdownDescription: fmt.Sprintf("Specifies how is the API Key is sent. Possible values are `%s`, `%s` and `%s`.",
											client.APIKeyAuthInQuery, client.APIKeyAuthInHeader, client.APIKeyAuthInCookie),
										Required: true,
										Type:     types.StringType,
										Validators: []tfsdk.AttributeValidator{
											stringvalidator.OneOf(
												string(client.APIKeyAuthInHeader),
												string(client.APIKeyAuthInQuery),
												string(client.APIKeyAuthInCookie),
											),
										},
									},
								},
							),
						},
						"oauth2": {
							Description:         "Configuration for the OAuth2 authentication scheme.",
							MarkdownDescription: "Configuration for the OAuth2 authentication scheme.",
							Optional:            true,
							Attributes: tfsdk.SingleNestedAttributes(
								map[string]tfsdk.Attribute{
									"token_url": {
										Type:                types.StringType,
										Description:         "The token URL to be used for this flow.",
										MarkdownDescription: "The token URL to be used for this flow.",
										Required:            true,
									},
									"client_id": {
										Type:                types.StringType,
										Description:         "The application's ID (client credential flow only).",
										MarkdownDescription: "The application's ID (client credential flow only).",
										Optional:            true,
									},
									"client_secret": {
										Type:                types.StringType,
										Sensitive:           true,
										Description:         "The application's secret (client credential flow only).",
										MarkdownDescription: "The application's secret (client credential flow only).",
										Optional:            true,
									},
									"username": {
										Type:                types.StringType,
										Description:         "The username (password credential flow only).",
										MarkdownDescription: "The username (password credential flow only).",
										Optional:            true,
									},
									"password": {
										Type:                types.StringType,
										Sensitive:           true,
										Description:         "The password (password credential flow only).",
										MarkdownDescription: "The password (password credential flow only).",
										Optional:            true,
									},
									"scopes": {
										Type:                types.ListType{ElemType: types.StringType},
										Description:         "The optional requested permissions.",
										MarkdownDescription: "The optional requested permissions.",
										Optional:            true,
									},
									"endpoint_params": {
										Type:                types.MapType{ElemType: types.ListType{ElemType: types.StringType}},
										Description:         "The additional parameters for requests to the token endpoint (client credential flow only).",
										MarkdownDescription: "The additional parameters for requests to the token endpoint (client credential flow only).",
										Optional:            true,
									},
									"in": {
										Type: types.StringType,
										Description: fmt.Sprintf("Specifies how is the client ID & secret sent. Possible values are `%s` and `%s`. If absent, the style used will be auto detected.",
											client.OAuth2AuthStyleInParams, client.OAuth2AuthStyleInHeader),
										MarkdownDescription: fmt.Sprintf("Specifies how is th client ID & secret sent. Possible values are `%s` and `%s`. If absent, the style used will be auto detected.",
											client.OAuth2AuthStyleInParams, client.OAuth2AuthStyleInHeader),
										Optional:   true,
										Validators: []tfsdk.AttributeValidator{stringvalidator.OneOf(string(client.OAuth2AuthStyleInParams), string(client.OAuth2AuthStyleInHeader))},
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
				// Need a way to set the default value, plan modifier doesn't work here even it is Optional+Computed, because it is at provider level?
				// Currently, we are setting the default value during the provider configuration.
				Validators: []tfsdk.AttributeValidator{
					stringvalidator.OneOf("PUT", "POST"),
				},
			},
			"update_method": {
				Type:                types.StringType,
				Description:         "The method used to update the resource. Possible values are `PUT` and `PATCH`. Defaults to `PUT`.",
				MarkdownDescription: "The method used to update the resource. Possible values are `PUT` and `PATCH`. Defaults to `PUT`.",
				Optional:            true,
				// Need a way to set the default value, plan modifier doesn't work here even it is Optional+Computed, because it is at provider level?
				// Currently, we are setting the default value during the provider configuration.
				Validators: []tfsdk.AttributeValidator{
					stringvalidator.OneOf("PUT", "PATCH"),
				},
			},
			"delete_method": {
				Type:                types.StringType,
				Description:         "The method used to delete the resource. Possible values are `DELETE` and `POST`. Defaults to `DELETE`.",
				MarkdownDescription: "The method used to delete the resource. Possible values are `DELETE` and `POST`. Defaults to `DELETE`.",
				Optional:            true,
				// Need a way to set the default value, plan modifier doesn't work here even it is Optional+Computed, because it is at provider level?
				// Currently, we are setting the default value during the provider configuration.
				Validators: []tfsdk.AttributeValidator{
					stringvalidator.OneOf("DELETE", "POST"),
				},
			},
			"merge_patch_disabled": {
				Type:                types.BoolType,
				Description:         "Whether to use a JSON Merge Patch as the request body in the PATCH update? Defaults to `false`. This is only effective when `update_method` is set to `PATCH`.",
				MarkdownDescription: "Whether to use a JSON Merge Patch as the request body in the PATCH update? Defaults to `false`. This is only effective when `update_method` is set to `PATCH`.",
				Optional:            true,
			},
			"query": {
				Description:         "The query parameters that are applied to each request.",
				MarkdownDescription: "The query parameters that are applied to each request.",
				Type:                types.MapType{ElemType: types.ListType{ElemType: types.StringType}},
				Optional:            true,
			},
			"header": {
				Description:         "The header parameters that are applied to each request.",
				MarkdownDescription: "The header parameters that are applied to each request.",
				Type:                types.MapType{ElemType: types.StringType},
				Optional:            true,
			},
		},
	}, nil
}

func (*Provider) ValidateConfig(ctx context.Context, req provider.ValidateConfigRequest, resp *provider.ValidateConfigResponse) {
	type pt struct {
		BaseURL            types.String `tfsdk:"base_url"`
		Security           types.Object `tfsdk:"security"`
		CreateMethod       types.String `tfsdk:"create_method"`
		UpdateMethod       types.String `tfsdk:"update_method"`
		DeleteMethod       types.String `tfsdk:"delete_method"`
		MergePatchDisabled types.Bool   `tfsdk:"merge_patch_disabled"`
		Query              types.Map    `tfsdk:"query"`
		Header             types.Map    `tfsdk:"header"`
	}

	var config pt
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}

	if !config.BaseURL.IsUnknown() {
		if _, err := url.Parse(config.BaseURL.ValueString()); err != nil {
			resp.Diagnostics.AddError(
				"Invalid configuration",
				"The `base_url` is not a valid URL",
			)
			return
		}
	}

	if !config.Security.IsUnknown() && !config.Security.IsNull() {
		httpObj := config.Security.Attributes()["http"].(types.Object)
		apikeyObj := config.Security.Attributes()["apikey"].(types.Set)
		oauth2Obj := config.Security.Attributes()["oauth2"].(types.Object)

		l := []string{}
		if !httpObj.IsNull() && !httpObj.IsUnknown() {
			l = append(l, "http")
			type httpData struct {
				Type     types.String `tfsdk:"type"`
				Username types.String `tfsdk:"username"`
				Password types.String `tfsdk:"password"`
				Token    types.String `tfsdk:"token"`
			}
			var d httpData
			if diags := httpObj.As(ctx, &d, types.ObjectAsOptions{}); diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}
			if !d.Username.IsUnknown() && !d.Password.IsUnknown() && !d.Token.IsUnknown() {
				if !d.Type.IsUnknown() {
					switch d.Type.ValueString() {
					case string(client.HTTPAuthTypeBasic):
						if d.Username.IsNull() {
							resp.Diagnostics.AddError(
								"Invalid configuration: `security.http`",
								fmt.Sprintf("`username` is required when `type` is %s", client.HTTPAuthTypeBasic),
							)
						}
						if d.Password.IsNull() {
							resp.Diagnostics.AddError(
								"Invalid configuration: `security.http`",
								fmt.Sprintf("`password` is required when `type` is %s", client.HTTPAuthTypeBasic),
							)
						}
						if !d.Token.IsNull() {
							resp.Diagnostics.AddError(
								"Invalid configuration: `security.http`",
								fmt.Sprintf("`token` can't be specified when `type` is %s", client.HTTPAuthTypeBasic),
							)
						}
					case string(client.HTTPAuthTypeBearer):
						if !d.Username.IsNull() {
							resp.Diagnostics.AddError(
								"Invalid configuration: `security.http`",
								fmt.Sprintf("`username` can't be specified when `type` is %s", client.HTTPAuthTypeBearer),
							)
						}
						if !d.Password.IsNull() {
							resp.Diagnostics.AddError(
								"Invalid configuration: `security.http`",
								fmt.Sprintf("`password` can't be specified when `type` is %s", client.HTTPAuthTypeBearer),
							)
						}
						if d.Token.IsNull() {
							resp.Diagnostics.AddError(
								"Invalid configuration: `security.http`",
								fmt.Sprintf("`token` is required when `type` is %s", client.HTTPAuthTypeBearer),
							)
						}
					}
					if resp.Diagnostics.HasError() {
						return
					}
				}

				if !(d.Username.IsNull() && d.Password.IsNull() && !d.Token.IsNull()) && !(!d.Username.IsNull() && !d.Password.IsNull() && d.Token.IsNull()) {
					resp.Diagnostics.AddError(
						"Invalid configuration: `security.http`",
						"Either `username` & `password`, or `token` should be specified",
					)
					return
				}
			}
		}
		if !oauth2Obj.IsNull() && !oauth2Obj.IsUnknown() {
			l = append(l, "oauth2")
			type oauth2Data struct {
				TokenUrl       types.String `tfsdk:"token_url"`
				ClientId       types.String `tfsdk:"client_id"`
				ClientSecret   types.String `tfsdk:"client_secret"`
				Username       types.String `tfsdk:"username"`
				Password       types.String `tfsdk:"password"`
				Scopes         types.List   `tfsdk:"scopes"`
				EndpointParams types.Map    `tfsdk:"endpoint_params"`
				In             types.String `tfsdk:"in"`
			}
			var d oauth2Data
			if diags := oauth2Obj.As(ctx, &d, types.ObjectAsOptions{}); diags.HasError() {
				resp.Diagnostics.Append(diags...)
				return
			}
			if !d.ClientId.IsUnknown() && !d.ClientSecret.IsUnknown() && !d.Username.IsUnknown() && !d.Password.IsUnknown() {
				if !(d.ClientId.IsNull() && d.ClientSecret.IsNull() && !d.Username.IsNull() && !d.Password.IsNull()) && !(!d.ClientId.IsNull() && !d.ClientSecret.IsNull() && d.Username.IsNull() && d.Password.IsNull()) {
					resp.Diagnostics.AddError(
						"Invalid configuration: `security.oauth2`",
						"Either `username` & `password`, or `client_id` & `client_secret` should be specified",
					)
					return
				}
			}
		}
		if !apikeyObj.IsNull() && !apikeyObj.IsUnknown() {
			l = append(l, "apikey")
		}

		// In case any of the block is unknown, we don't know whether it will evaluate into null or not.
		// Here, we do best effort to ensure at least one of them is set.
		if httpObj.IsNull() && oauth2Obj.IsNull() && apikeyObj.IsNull() {
			resp.Diagnostics.AddError(
				"Invalid configuration: `security`",
				"There is no security scheme specified",
			)
			return
		}

		if len(l) > 1 {
			resp.Diagnostics.AddError(
				"Invalid configuration: `security`",
				"More than one scheme is specified: "+strings.Join(l, ", "),
			)
			return
		}
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
			sopt := client.HTTPAuthOption{
				Type: client.HTTPAuthType(sec.HTTP.Type),
			}
			if sec.HTTP.Username != nil {
				sopt.Username = *sec.HTTP.Username
			}
			if sec.HTTP.Password != nil {
				sopt.Password = *sec.HTTP.Password
			}
			if sec.HTTP.Token != nil {
				sopt.Token = *sec.HTTP.Token
			}
			clientOpt.Security = sopt
		case sec.APIKey != nil:
			sopt := client.APIKeyAuthOption{}
			for _, apikey := range sec.APIKey {
				sopt = append(sopt, client.APIKeyAuthOpt{
					Name:  apikey.Name,
					In:    client.APIKeyAuthIn(apikey.In),
					Value: apikey.Value,
				})
			}
			clientOpt.Security = sopt
		case sec.OAuth2 != nil:
			if sec.OAuth2.Username == nil {
				sopt := client.OAuth2ClientCredentialOption{
					ClientID:       *sec.OAuth2.ClientID,
					ClientSecret:   *sec.OAuth2.ClientSecret,
					TokenURL:       sec.OAuth2.TokenUrl,
					Scopes:         sec.OAuth2.Scopes,
					EndpointParams: sec.OAuth2.EndpointParams,
				}
				if sec.OAuth2.In != nil {
					sopt.AuthStyle = client.OAuth2AuthStyle(*sec.OAuth2.In)
				}
				clientOpt.Security = sopt
			} else {
				sopt := client.OAuth2PasswordOption{
					Username: *sec.OAuth2.Username,
					Password: *sec.OAuth2.Password,
					TokenURL: sec.OAuth2.TokenUrl,
					Scopes:   sec.OAuth2.Scopes,
				}
				if sec.OAuth2.In != nil {
					sopt.AuthStyle = client.OAuth2AuthStyle(*sec.OAuth2.In)
				}
				clientOpt.Security = sopt
			}
		default:
			resp.Diagnostics.AddError(
				"Failed to configure provider",
				"There is no security scheme specified",
			)
			return
		}
	}

	var err error
	p.client, err = client.New(config.BaseURL, &clientOpt)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to configure provider",
			fmt.Sprintf("Failed to new client: %v", err),
		)
	}

	p.apiOpt = apiOption{
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
