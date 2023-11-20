package client

import (
	"context"
	"crypto/tls"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type BuildOption struct {
	Security      securityOption
	CookieEnabled bool
	TLSConfig     tls.Config
}

type securityOption interface {
	newClient(ctx context.Context, client *http.Client) (*resty.Client, error)
}

type HTTPBasicOption struct {
	Username string
	Password string
}

func (opt HTTPBasicOption) newClient(_ context.Context, client *http.Client) (*resty.Client, error) {
	return resty.NewWithClient(client).SetBasicAuth(opt.Username, opt.Password), nil
}

type HTTPTokenOption struct {
	Token  string
	Scheme string
}

func (opt HTTPTokenOption) newClient(_ context.Context, client *http.Client) (*resty.Client, error) {
	return resty.NewWithClient(client).SetAuthToken(opt.Token).SetScheme(opt.Scheme), nil
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

func (opt APIKeyAuthOption) newClient(_ context.Context, client *http.Client) (*resty.Client, error) {
	c := resty.NewWithClient(client)
	for _, key := range opt {
		switch key.In {
		case APIKeyAuthInHeader:
			c.SetHeader(key.Name, key.Value)
		case APIKeyAuthInQuery:
			c.SetQueryParam(key.Name, key.Value)
		case APIKeyAuthInCookie:
			c.SetCookie(&http.Cookie{
				Name:  key.Name,
				Value: key.Value,
			})
		}
	}
	return c, nil
}

type OAuth2AuthStyle string

const (
	OAuth2AuthStyleInParams OAuth2AuthStyle = "params"
	OAuth2AuthStyleInHeader OAuth2AuthStyle = "header"
)

type OAuth2PasswordOption struct {
	TokenURL string
	ClientId string
	Username string
	Password string

	ClientSecret string
	AuthStyle    OAuth2AuthStyle
	Scopes       []string
}

func (opt OAuth2PasswordOption) newClient(ctx context.Context, client *http.Client) (*resty.Client, error) {
	cfg := oauth2.Config{
		ClientID:     opt.ClientId,
		ClientSecret: opt.ClientSecret,
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

	tk, err := cfg.PasswordCredentialsToken(ctx, opt.Username, opt.Password)
	if err != nil {
		return nil, err
	}

	// We use background context here when constructing the client since we are building the client during the provider configuration, where the context is used only for that purpose.
	// Especially, when we use this client, we will set the operation bound context for each request.
	ctx = context.WithValue(context.Background(), oauth2.HTTPClient, client)

	// We use background context here when constructing the client since we are building the client during the provider configuration, where the context is used only for that purpose.
	// Especially, when we use this client, we will set the operation bound context for each request.
	return resty.NewWithClient(cfg.Client(ctx, tk)), nil
}

type OAuth2ClientCredentialOption struct {
	TokenURL     string
	ClientId     string
	ClientSecret string

	Scopes         []string
	EndpointParams map[string][]string
	AuthStyle      OAuth2AuthStyle
}

func (opt OAuth2ClientCredentialOption) newClient(_ context.Context, client *http.Client) (*resty.Client, error) {
	cfg := clientcredentials.Config{
		ClientID:       opt.ClientId,
		ClientSecret:   opt.ClientSecret,
		TokenURL:       opt.TokenURL,
		Scopes:         opt.Scopes,
		EndpointParams: opt.EndpointParams,
	}

	switch opt.AuthStyle {
	case OAuth2AuthStyleInHeader:
		cfg.AuthStyle = oauth2.AuthStyleInHeader
	case OAuth2AuthStyleInParams:
		cfg.AuthStyle = oauth2.AuthStyleInParams
	}

	// We use background context here when constructing the client since we are building the client during the provider configuration, where the context is used only for that purpose.
	// Especially, when we use this client, we will set the operation bound context for each request.
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, client)
	ts := cfg.TokenSource(ctx)
	return resty.NewWithClient(oauth2.NewClient(ctx, ts)), nil
}

type OAuth2RefreshTokenOption struct {
	TokenURL     string
	ClientId     string
	RefreshToken string

	ClientSecret string
	AuthStyle    OAuth2AuthStyle
	TokenType    string
	Scopes       []string
}

func (opt OAuth2RefreshTokenOption) newClient(_ context.Context, client *http.Client) (*resty.Client, error) {
	cfg := oauth2.Config{
		ClientID:     opt.ClientId,
		ClientSecret: opt.ClientSecret,
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
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, client)
	return resty.NewWithClient(cfg.Client(ctx, &oauth2.Token{
		RefreshToken: opt.RefreshToken,
		TokenType:    opt.TokenType,
		Expiry:       time.Now(),
	})), nil
}
