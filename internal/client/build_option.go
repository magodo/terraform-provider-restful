package client

import (
	"context"
	"net/http"

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
	HTTPAuthTypeBasic  HTTPAuthType = "Basic"
	HTTPAuthTypeBearer HTTPAuthType = "Bearer"
)

type HTTPAuthOption struct {
	Type     HTTPAuthType
	Username string
	Password string
	Token    string
}

func (opt HTTPAuthOption) newClient() *resty.Client {
	client := resty.New()
	switch opt.Type {
	case HTTPAuthTypeBasic:
		client.SetBasicAuth(opt.Username, opt.Password)
	case HTTPAuthTypeBearer:
		client.SetAuthToken(opt.Token)
	}
	return client
}

type APIKeyAuthIn string

const (
	APIKeyAuthInHeader APIKeyAuthIn = "header"
	APIKeyAuthInQuery  APIKeyAuthIn = "query"
	APIKeyAuthInCookie APIKeyAuthIn = "cookie"
)

type APIKeyAuthOpt struct {
	Name  string
	In    APIKeyAuthIn
	Value string
}

type APIKeyAuthOption []APIKeyAuthOpt

func (opt APIKeyAuthOption) newClient() *resty.Client {
	client := resty.New()
	for _, key := range opt {
		switch key.In {
		case APIKeyAuthInHeader:
			client.SetHeader(key.Name, key.Value)
		case APIKeyAuthInQuery:
			client.SetQueryParam(key.Name, key.Value)
		case APIKeyAuthInCookie:
			client.SetCookie(&http.Cookie{
				Name:  key.Name,
				Value: key.Value,
			})
		}
	}
	return client
}

type OAuth2AuthStyle string

const (
	OAuth2AuthStyleInParams OAuth2AuthStyle = "params"
	OAuth2AuthStyleInHeader OAuth2AuthStyle = "header"
)

type OAuth2PasswordOption struct {
	Username  string
	Password  string
	TokenURL  string
	Scopes    []string
	AuthStyle OAuth2AuthStyle
}

func (opt OAuth2PasswordOption) Token() (*oauth2.Token, error) {
	cfg := oauth2.Config{
		Endpoint: oauth2.Endpoint{
			TokenURL: opt.TokenURL,
		},
		Scopes: opt.Scopes,
	}

	switch opt.AuthStyle {
	case OAuth2AuthStyleInHeader:
		cfg.Endpoint.AuthStyle = oauth2.AuthStyleInHeader
	case OAuth2AuthStyleInParams:
		cfg.Endpoint.AuthStyle = oauth2.AuthStyleInParams
	}
	// We use background context here when constructing the client since we are building the client during the provider configuration, where the context is used only for that purpose.
	// Especially, when we use this client, we will set the operation bound context for each request.
	ctx := context.Background()
	return cfg.PasswordCredentialsToken(ctx, opt.Username, opt.Password)
}

func (opt OAuth2PasswordOption) newClient() *resty.Client {
	ctx := context.Background()
	return resty.NewWithClient(oauth2.NewClient(ctx, opt))
}

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
