package client

import (
	"context"
	"net/http"
)

// NewWithSecurityFromExisting creates a new client by copying the base client's configs, but with a new SecurityOption.
func NewWithSecurityFromExisting(base *Client, security SecurityOption) (*Client, error) {
	// Usa o mesmo baseURL e BuildOption do client base, mas troca a security
	baseURL := base.BaseURL
	opt := &BuildOption{
		Security:      security,
		CookieEnabled: base.Client.GetClient().Jar != nil,
		TLSConfig:     *base.Client.GetClient().Transport.(*http.Transport).TLSClientConfig,
		// Retry pode ser nil ou copiado se necessário
	}
	return New(context.Background(), baseURL, opt)
}
