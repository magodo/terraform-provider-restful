package client

import (
	"context"
	"net/http"
)

// NewWithOverridesFromExisting creates a new client by copying the base client's configs, but with optional overrides for baseURL and SecurityOption.
// If an override is not needed, the existing value from the base client is used.
// We need this function because of resty's design, which does not allow changing the base URL or security settings after the client is created.
func NewWithOverridesFromExisting(base *Client, baseURL string, security SecurityOption) (*Client, error) {
	opt := &BuildOption{
		Security:      security,
		CookieEnabled: base.Client.GetClient().Jar != nil,
		TLSConfig:     *base.Client.GetClient().Transport.(*http.Transport).TLSClientConfig,
	}
	return New(context.Background(), baseURL, opt)
}