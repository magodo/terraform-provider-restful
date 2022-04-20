package restapi

import (
	"context"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type provider struct {
	url string
	*http.Client
}

type providerData struct {
	BaseURL string `tfsdk:"base_url"`
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
}

func (p *provider) Configure(ctx context.Context, req tfsdk.ConfigureProviderRequest, resp *tfsdk.ConfigureProviderResponse) {
	var config providerData
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if diags.HasError() {
		return
	}
	p.url = config.BaseURL
	p.Client = &http.Client{}
	return
}

func (*provider) GetResources(context.Context) (map[string]tfsdk.ResourceType, diag.Diagnostics) {
	return map[string]tfsdk.ResourceType{
		"restapi_resource": resourceType{},
	}, nil
}

func (*provider) GetDataSources(context.Context) (map[string]tfsdk.DataSourceType, diag.Diagnostics) {
	return nil, nil
}
