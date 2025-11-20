package provider

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/list"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/lfventura/terraform-provider-restful/internal/client"
	"github.com/lfventura/terraform-provider-restful/internal/defaults"
	myvalidator "github.com/lfventura/terraform-provider-restful/internal/validator"
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
	Client             types.Object `tfsdk:"client"`
	Security           types.Object `tfsdk:"security"`
	CreateMethod       types.String `tfsdk:"create_method"`
	UpdateMethod       types.String `tfsdk:"update_method"`
	DeleteMethod       types.String `tfsdk:"delete_method"`
	MergePatchDisabled types.Bool   `tfsdk:"merge_patch_disabled"`
	Query              types.Map    `tfsdk:"query"`
	Header             types.Map    `tfsdk:"header"`
}

type clientData struct {
	CookieEnabled          types.Bool   `tfsdk:"cookie_enabled"`
	TlsInsecureSkipVerify  types.Bool   `tfsdk:"tls_insecure_skip_verify"`
	Certificates           types.List   `tfsdk:"certificates"`
	RootCACertificates     types.List   `tfsdk:"root_ca_certificates"`
	RootCACertificateFiles types.List   `tfsdk:"root_ca_certificate_files"`
	Retry                  types.Object `tfsdk:"retry"`
}

type certificateData struct {
	Certificate     types.String `tfsdk:"certificate"`
	CertificateFile types.String `tfsdk:"certificate_file"`
	Key             types.String `tfsdk:"key"`
	KeyFile         types.String `tfsdk:"key_file"`
}

type retryData struct {
	StatusCodes  types.List  `tfsdk:"status_codes"`
	Count        types.Int64 `tfsdk:"count"`
	WaitInSec    types.Int64 `tfsdk:"wait_in_sec"`
	MaxWaitInSec types.Int64 `tfsdk:"max_wait_in_sec"`
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

func (*Provider) EphemeralResources(context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		func() ephemeral.EphemeralResource {
			return &EphemeralResource{}
		},
	}
}

func (*Provider) ListResources(_ context.Context) []func() list.ListResource {
	return []func() list.ListResource{
		func() list.ListResource {
			return &ListResource{}
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
			"client": schema.SingleNestedAttribute{
				Description:         "The client configuration",
				MarkdownDescription: "The client configuration",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"cookie_enabled": schema.BoolAttribute{
						Description:         "Save cookies during API contracting. Defaults to `false`.",
						MarkdownDescription: "Save cookies during API contracting. Defaults to `false`.",
						Optional:            true,
					},
					"tls_insecure_skip_verify": schema.BoolAttribute{
						Description:         "Whether a client verifies the server's certificate chain and host name. Defaults to `false`.",
						MarkdownDescription: "Whether a client verifies the server's certificate chain and host name. Defaults to `false`.",
						Optional:            true,
					},
					"certificates": schema.ListNestedAttribute{
						Description:         "The client certificates for mTLS.",
						MarkdownDescription: "The client certificates for mTLS.",
						Optional:            true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"certificate": schema.StringAttribute{
									Description:         "The client certificate for mTLS. Conflicts with `certificate_file`. Requires `key_file` or `key`.",
									MarkdownDescription: "The client certificate for mTLS. Conflicts with `certificate_file`. Requires `key_file` or `key`.",
									Optional:            true,
									Validators: []validator.String{
										stringvalidator.ConflictsWith(
											path.MatchRelative().AtParent().AtName("certificate_file"),
										),
										stringvalidator.AtLeastOneOf(
											path.MatchRelative().AtParent().AtName("key"),
											path.MatchRelative().AtParent().AtName("key_file"),
										),
									},
								},
								"certificate_file": schema.StringAttribute{
									Description:         "The path of the client certificate file for mTLS. Conflicts with `certificate`. Requires `key_file` or `key`.",
									MarkdownDescription: "The path of the client certificate file for mTLS. Conflicts with `certificate`. Requires `key_file` or `key`.",
									Optional:            true,
									Validators: []validator.String{
										stringvalidator.ConflictsWith(
											path.MatchRelative().AtParent().AtName("certificate"),
										),
										stringvalidator.AtLeastOneOf(
											path.MatchRelative().AtParent().AtName("key"),
											path.MatchRelative().AtParent().AtName("key_file"),
										),
									},
								},
								"key": schema.StringAttribute{
									Description:         "The client private key for mTLS. Conflicts with `key_file`.",
									MarkdownDescription: "The client private key for mTLS. Conflicts with `key_file`.",
									Optional:            true,
									Validators: []validator.String{
										stringvalidator.ConflictsWith(
											path.MatchRelative().AtParent().AtName("key_file"),
										),
										stringvalidator.AtLeastOneOf(
											path.MatchRelative().AtParent().AtName("certificate"),
											path.MatchRelative().AtParent().AtName("certificate_file"),
										),
									},
								},
								"key_file": schema.StringAttribute{
									Description:         "The path of the client private key file for mTLS. Conflicts with `key`. Requires `certificate_file` or `certificate`.",
									MarkdownDescription: "The path of the client private key file for mTLS. Conflicts with `key`. Requires `certificate_file` or `certificate`.",
									Optional:            true,
									Validators: []validator.String{
										stringvalidator.ConflictsWith(
											path.MatchRelative().AtParent().AtName("key"),
										),
										stringvalidator.AtLeastOneOf(
											path.MatchRelative().AtParent().AtName("certificate"),
											path.MatchRelative().AtParent().AtName("certificate_file"),
										),
									},
								},
							},
						},
					},
					"root_ca_certificates": schema.ListAttribute{
						Description:         "The list of certificates of root certificate authorities that clients use when verifying server certificates. If not specified, TLS uses the host's root CA set. Conflicts with `root_ca_certificate_files`.",
						MarkdownDescription: "The list of certificates of root certificate authorities that clients use when verifying server certificates. If not specified, TLS uses the host's root CA set. Conflicts with `root_ca_certificate_files`.",
						Optional:            true,
						ElementType:         types.StringType,
						Validators: []validator.List{
							listvalidator.ConflictsWith(
								path.MatchRoot("client").AtName("root_ca_certificate_files"),
							),
							listvalidator.AlsoRequires(
								path.MatchRoot("client").AtName("certificates"),
							),
						},
					},
					"root_ca_certificate_files": schema.ListAttribute{
						Description:         "The list of certificate file paths of root certificate authorities that clients use when verifying server certificates. If not specified, TLS uses the host's root CA set. Conflicts with `root_ca_certificate_files`.",
						MarkdownDescription: "The list of certificate file paths of root certificate authorities that clients use when verifying server certificates. If not specified, TLS uses the host's root CA set. Conflicts with `root_ca_certificate_files`.",
						Optional:            true,
						ElementType:         types.StringType,
						Validators: []validator.List{
							listvalidator.ConflictsWith(
								path.MatchRoot("client").AtName("root_ca_certificate_files"),
							),
							listvalidator.AlsoRequires(
								path.MatchRoot("client").AtName("certificates"),
							),
						},
					},
					"retry": schema.SingleNestedAttribute{
						Description:         "The retry option for the client",
						MarkdownDescription: "The retry option for the client",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"status_codes": schema.ListAttribute{
								Description:         "The status codes that will retry.",
								MarkdownDescription: "The status codes that will retry.",
								Required:            true,
								ElementType:         types.Int64Type,
							},
							"count": schema.Int64Attribute{
								Description:         fmt.Sprintf("The maximum allowed retries. Defaults to `%d`.", defaults.RetryCount),
								MarkdownDescription: fmt.Sprintf("The maximum allowed retries. Defaults to `%d`.", defaults.RetryCount),
								Optional:            true,
							},
							"wait_in_sec": schema.Int64Attribute{
								Description:         fmt.Sprintf("The initial retry wait time between two retries in second, if there is no `Retry-After` in the response header, or the `Retry-After` is less than this. The wait time will be increased in capped exponential backoff with jitter, at most up to `max_wait_in_sec` (if not null). Defaults to `%v`.", defaults.RetryWaitTime.Seconds()),
								MarkdownDescription: fmt.Sprintf("The initial retry wait time between two retries in second, if there is no `Retry-After` in the response header, or the `Retry-After` is less than this. The wait time will be increased in capped exponential backoff with jitter, at most up to `max_wait_in_sec` (if not null). Defaults to `%v`.", defaults.RetryWaitTime.Seconds()),
								Optional:            true,
							},
							"max_wait_in_sec": schema.Int64Attribute{
								Description:         fmt.Sprintf("The maximum allowed retry wait time. Defaults to `%v`.", defaults.RetryMaxWaitTime.Seconds()),
								MarkdownDescription: fmt.Sprintf("The maximum allowed retry wait time. Defaults to `%v`.", defaults.RetryMaxWaitTime.Seconds()),
								Optional:            true,
							},
						},
					},
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
	resp.EphemeralResourceData = data
	resp.ListResourceData = data
}

func (p *Provider) Init(ctx context.Context, config providerConfig) diag.Diagnostics {
	var odiags diag.Diagnostics
	p.once.Do(func() {
		clientOpt := &client.BuildOption{}
		if cRaw := config.Client; !cRaw.IsNull() {
			var c clientData
			if diags := cRaw.As(ctx, &c, basetypes.ObjectAsOptions{}); diags.HasError() {
				odiags = diags
				return
			}
			clientOpt, odiags = c.ToClientBuildOption(ctx)
			if odiags.HasError() {
				return
			}
		}

		if secRaw := config.Security; !secRaw.IsNull() {
			security, diags := populateSecurity(ctx, secRaw)
			if diags.HasError() {
				odiags = diags
				return
			}
			clientOpt.Security = security
		}

		var (
			diags diag.Diagnostics
			err   error
		)
		p.client, err = client.New(ctx, config.BaseURL.ValueString(), clientOpt)
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

func (c clientData) ToClientBuildOption(ctx context.Context) (*client.BuildOption, diag.Diagnostics) {
	var diags diag.Diagnostics
	var clientOpt client.BuildOption

	clientOpt.TLSConfig.InsecureSkipVerify = c.TlsInsecureSkipVerify.ValueBool()

	var caCerts [][]byte
	switch {
	case !c.RootCACertificateFiles.IsNull():
		for _, f := range c.RootCACertificateFiles.Elements() {
			f := f.(types.String).ValueString()
			b, err := os.ReadFile(f)
			if err != nil {
				diags.AddError(
					"Failed to build client option",
					fmt.Sprintf("reading %s: %v", f, err),
				)
				return nil, diags
			}
			caCerts = append(caCerts, b)
		}
	case !c.RootCACertificates.IsNull():
		for _, cert := range c.RootCACertificates.Elements() {
			cert := cert.(types.String).ValueString()
			caCerts = append(caCerts, []byte(cert))
		}
	}
	if len(caCerts) != 0 {
		caPool := x509.NewCertPool()
		for _, cert := range caCerts {
			caPool.AppendCertsFromPEM(cert)
		}
		clientOpt.TLSConfig.RootCAs = caPool
	}

	if !c.Certificates.IsNull() {
		var certs []tls.Certificate
		for _, e := range c.Certificates.Elements() {
			obj := e.(types.Object)
			var cd certificateData
			if diags := obj.As(ctx, &cd, basetypes.ObjectAsOptions{}); diags.HasError() {
				return nil, diags
			}

			var certB, keyB []byte
			var err error

			switch {
			case !cd.Certificate.IsNull():
				certB = []byte(cd.Certificate.ValueString())
			case !cd.CertificateFile.IsNull():
				certB, err = os.ReadFile(cd.CertificateFile.ValueString())
				if err != nil {
					diags.AddError(
						"Failed to build client option",
						fmt.Sprintf("reading %s: %v", cd.CertificateFile.ValueString(), err),
					)
					return nil, diags
				}
			}

			switch {
			case !cd.Key.IsNull():
				keyB = []byte(cd.Key.ValueString())
			case !cd.KeyFile.IsNull():
				keyB, err = os.ReadFile(cd.KeyFile.ValueString())
				if err != nil {
					diags.AddError(
						"Failed to build client option",
						fmt.Sprintf("reading %s: %v", cd.KeyFile.ValueString(), err),
					)
					return nil, diags
				}
			}

			cert, err := tls.X509KeyPair(certB, keyB)
			if err != nil {
				diags.AddError(
					"Failed to build client option",
					fmt.Sprintf("building x509 key pair: %v", err),
				)
				return nil, diags
			}
			certs = append(certs, cert)
		}
		clientOpt.TLSConfig.Certificates = certs
	}

	clientOpt.CookieEnabled = c.CookieEnabled.ValueBool()

	if !c.Retry.IsNull() {
		retryOpt, diags := populateRetry(ctx, c.Retry)
		if diags.HasError() {
			return nil, diags
		}
		clientOpt.Retry = retryOpt
	}

	return &clientOpt, nil
}

func populateRetry(ctx context.Context, retryObj basetypes.ObjectValue) (*client.RetryOption, diag.Diagnostics) {
	var retry retryData
	if diags := retryObj.As(ctx, &retry, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}

	var statusCodes []int64
	for _, sc := range retry.StatusCodes.Elements() {
		if sc.IsNull() || sc.IsUnknown() {
			continue
		}

		statusCodes = append(statusCodes, sc.(basetypes.Int64Value).ValueInt64())
	}

	count := defaults.RetryCount
	if !retry.Count.IsNull() && !retry.Count.IsUnknown() {
		count = int(retry.Count.ValueInt64())
	}

	waitTime := defaults.RetryWaitTime
	if !retry.WaitInSec.IsNull() && !retry.WaitInSec.IsUnknown() {
		waitTime = time.Duration(int(retry.WaitInSec.ValueInt64())) * time.Second
	}

	maxWaitTime := defaults.RetryMaxWaitTime
	if !retry.MaxWaitInSec.IsNull() && !retry.MaxWaitInSec.IsUnknown() {
		waitTime = time.Duration(int(retry.MaxWaitInSec.ValueInt64())) * time.Second
	}

	return &client.RetryOption{
		StatusCodes: statusCodes,
		Count:       count,
		WaitTime:    waitTime,
		MaxWaitTime: maxWaitTime,
	}, nil
}

func populateSecurity(ctx context.Context, secRaw basetypes.ObjectValue) (client.SecurityOption, diag.Diagnostics) {
	var sec securityData
	if diags := secRaw.As(ctx, &sec, basetypes.ObjectAsOptions{}); diags.HasError() {
		return nil, diags
	}
	switch {
	case !sec.HTTP.IsNull():
		var http httpData
		if diags := sec.HTTP.As(ctx, &http, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}
		switch {
		case !http.Basic.IsNull():
			var basic httpBasicData
			if diags := http.Basic.As(ctx, &basic, basetypes.ObjectAsOptions{}); diags.HasError() {
				return nil, diags
			}
			opt := client.HTTPBasicOption{
				Username: basic.Username.ValueString(),
				Password: basic.Password.ValueString(),
			}
			return opt, nil
		case !http.Token.IsNull():
			var token httpTokenData
			if diags := http.Token.As(ctx, &token, basetypes.ObjectAsOptions{}); diags.HasError() {
				return nil, diags
			}
			opt := client.HTTPTokenOption{
				Token:  token.Token.ValueString(),
				Scheme: token.Scheme.ValueString(),
			}
			return opt, nil
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
				return nil, diags
			}
			opt = append(opt, client.APIKeyAuthOpt{
				Name:  apikey.Name.ValueString(),
				In:    client.APIKeyAuthIn(apikey.In.ValueString()),
				Value: apikey.Value.ValueString(),
			})
		}
		return opt, nil
	case !sec.OAuth2.IsNull():
		var oauth2 oauth2Data
		if diags := sec.OAuth2.As(ctx, &oauth2, basetypes.ObjectAsOptions{}); diags.HasError() {
			return nil, diags
		}
		switch {
		case !oauth2.Password.IsNull():
			var password oauth2PasswordData
			if diags := oauth2.Password.As(ctx, &password, basetypes.ObjectAsOptions{}); diags.HasError() {
				return nil, diags
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
			return opt, nil
		case !oauth2.ClientCredentials.IsNull():
			var cc oauth2ClientCredentialsData
			if diags := oauth2.ClientCredentials.As(ctx, &cc, basetypes.ObjectAsOptions{}); diags.HasError() {
				return nil, diags
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
			return opt, nil
		case !oauth2.RefreshToken.IsNull():
			var refreshToken oauth2RefreshTokenData
			if diags := oauth2.RefreshToken.As(ctx, &refreshToken, basetypes.ObjectAsOptions{}); diags.HasError() {
				return nil, diags
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
			return opt, nil
		}
	}
	return nil, nil
}
