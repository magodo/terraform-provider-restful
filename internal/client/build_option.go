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
	Security      SecurityOption
	CookieEnabled bool
	TLSConfig     tls.Config
	Retry         *RetryOption
}

type SecurityOption interface {
	configureClient(ctx context.Context, client *resty.Client) error
}

type HTTPBasicOption struct {
	Username string
	Password string
}

func (opt HTTPBasicOption) configureClient(_ context.Context, client *resty.Client) error {
	client.SetBasicAuth(opt.Username, opt.Password)
	return nil
}

type HTTPTokenOption struct {
	Token  string
	Scheme string
}

func (opt HTTPTokenOption) configureClient(_ context.Context, client *resty.Client) error {
	client.SetAuthToken(opt.Token)
	if opt.Scheme != "" {
		client.SetAuthScheme(opt.Scheme)
	}
	return nil
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

func (opt APIKeyAuthOption) configureClient(_ context.Context, client *resty.Client) error {
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
	return nil
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

func (opt OAuth2PasswordOption) configureClient(ctx context.Context, client *resty.Client) error {
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
		return err
	}

	// We use background context here when constructing the client since we are building the client during the provider configuration, where the context is used only for that purpose.
	// Especially, when we use this client, we will set the operation bound context for each request.
	httpClient := client.GetClient()
	ctx = context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)

	// We use background context here when constructing the client since we are building the client during the provider configuration, where the context is used only for that purpose.
	// Especially, when we use this client, we will set the operation bound context for each request.
	*client = *resty.NewWithClient(cfg.Client(ctx, tk))
	return nil
}

type OAuth2ClientCredentialOption struct {
	TokenURL     string
	ClientId     string
	ClientSecret string

	Scopes         []string
	EndpointParams map[string][]string
	AuthStyle      OAuth2AuthStyle
}

func (opt OAuth2ClientCredentialOption) configureClient(_ context.Context, client *resty.Client) error {
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
	httpClient := client.GetClient()
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)
	ts := cfg.TokenSource(ctx)
	*client = *resty.NewWithClient(oauth2.NewClient(ctx, ts))
	return nil
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

func (opt OAuth2RefreshTokenOption) configureClient(_ context.Context, client *resty.Client) error {
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
	httpClient := client.GetClient()
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)
	*client = *resty.NewWithClient(cfg.Client(ctx, &oauth2.Token{
		RefreshToken: opt.RefreshToken,
		TokenType:    opt.TokenType,
		Expiry:       time.Now(),
	}))
	return nil
}
