package client

type Option struct {
	Security securityOption

	// The HTTP verb used for creating the resource. Possible values are `POST` (default) and `PUT`.
	CreateMethod string

	// The value set to `Content-Type` for create and update request. Defaults to `application/json`.
	ContentType string
}

type securityOption interface {
	isSecurityOption()
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

func (opt OAuth2ClientCredentialOption) isSecurityOption() {}

// func (opt OAuth2ClientCredentialOption) configureClient(ctx context.Context, client *http.Client) {
// 	cfg := clientcredentials.Config{
// 		ClientID:       opt.ClientID,
// 		ClientSecret:   opt.ClientSecret,
// 		TokenURL:       opt.TokenURL,
// 		Scopes:         opt.Scopes,
// 		EndpointParams: opt.EndpointParams,
// 		AuthStyle:      oauth2.AuthStyleAutoDetect,
// 	}
// 	switch opt.AuthStyle {
// 	case OAuth2AuthStyleInHeader:
// 		cfg.AuthStyle = oauth2.AuthStyleInHeader
// 	case OAuth2AuthStyleInParams:
// 		cfg.AuthStyle = oauth2.AuthStyleInParams
// 	}

// 	ts := cfg.TokenSource(ctx)
// 	oauth2.NewClient(ctx, ts)
// }
