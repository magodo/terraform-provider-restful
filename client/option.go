package client

import (
	"context"

	"github.com/go-resty/resty/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type Option struct {
	Security securityOption

	// The HTTP verb used for creating the resource. Possible values are `POST` (default) and `PUT`.
	CreateMethod string

	// The value set to `Content-Type` for create and update request. Defaults to `application/json`.
	ContentType string
}

type securityOption interface {
	newClient(ctx context.Context) *resty.Client
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

func (opt HTTPAuthOption) newClient(ctx context.Context) *resty.Client {
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

func (opt OAuth2ClientCredentialOption) newClient(ctx context.Context) *resty.Client {
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

	ts := cfg.TokenSource(ctx)
	return resty.NewWithClient(oauth2.NewClient(ctx, ts))
}
