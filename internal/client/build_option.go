package client

import (
	"context"

	"github.com/go-resty/resty/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type BuildOption struct {
	Security securityOption
}

type securityOption interface {
	newClient() *resty.Client
}

type HTTPAuthType string

const (
	HTTPAuthTypeBasic HTTPAuthType = "Basic"
)

type HTTPAuthOption struct {
	Type     HTTPAuthType
	Username string
	Password string
}

func (opt HTTPAuthOption) newClient() *resty.Client {
	client := resty.New()
	switch opt.Type {
	case HTTPAuthTypeBasic:
		client.SetBasicAuth(opt.Username, opt.Password)
	}
	return client
}

type OAuth2AuthStyle string

const (
	OAuth2AuthStyleInParams OAuth2AuthStyle = "in_params"
	OAuth2AuthStyleInHeader OAuth2AuthStyle = "in_header"
)

type OAuth2ClientCredentialOption struct {
	ClientID       string
	ClientSecret   string
	TokenURL       string
	Scopes         []string
	EndpointParams map[string][]string
	AuthStyle      OAuth2AuthStyle
}

func (opt OAuth2ClientCredentialOption) newClient() *resty.Client {
	cfg := clientcredentials.Config{
		ClientID:       opt.ClientID,
		ClientSecret:   opt.ClientSecret,
		TokenURL:       opt.TokenURL,
		Scopes:         opt.Scopes,
		EndpointParams: opt.EndpointParams,
		AuthStyle:      oauth2.AuthStyleAutoDetect,
	}
	switch opt.AuthStyle {
	case OAuth2AuthStyleInHeader:
		cfg.AuthStyle = oauth2.AuthStyleInHeader
	case OAuth2AuthStyleInParams:
		cfg.AuthStyle = oauth2.AuthStyleInParams
	}

	// We use background context here when constructing the client since we are building the client during the provider configuration, where the context is used only for that purpose.
	// Especially, when we use this client, we will set the operation bound context for each request.
	ctx := context.Background()
	ts := cfg.TokenSource(ctx)
	return resty.NewWithClient(oauth2.NewClient(ctx, ts))
}
