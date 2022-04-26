package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-resty/resty/v2"
)

// ErrNotFound is expected to be returned for `Read` when the resource with the specified id doesn't exist.
var ErrNotFound = errors.New("resource not found")

type Client struct {
	*resty.Client

	CreateMethod string
	ContentType  string
}

func New(baseURL string, opt *Option) (*Client, error) {
	if opt == nil {
		opt = &Option{}
	}

	client := resty.New()
	if opt.Security != nil {
		client = opt.Security.newClient()
	}

	if _, err := url.Parse(baseURL); err != nil {
		return nil, err
	}

	client.SetBaseURL(baseURL)

	createMethod := "POST"
	if opt.CreateMethod != "" {
		createMethod = opt.CreateMethod
	}

	contentType := "application/json"
	if opt.ContentType != "" {
		contentType = opt.ContentType
	}

	return &Client{
		Client:       client,
		CreateMethod: createMethod,
		ContentType:  contentType,
	}, nil
}

func (c *Client) Create(ctx context.Context, path string, body interface{}) ([]byte, error) {
	req := c.R().SetContext(ctx).SetBody(body)
	if c.ContentType != "" {
		req = req.SetHeader("Content-Type", c.ContentType)
	}
	switch c.CreateMethod {
	case "POST":
		resp, err := req.Post(path)
		if err != nil {
			return nil, err
		}
		// TODO: Support LRO
		if resp.StatusCode()/100 != 2 {
			return nil, fmt.Errorf("Unexpected response (%s - code: %d): %s", resp.Status(), resp.StatusCode(), string(resp.Body()))
		}
		return resp.Body(), nil
	case "PUT":
		resp, err := req.Put(path)
		if err != nil {
			return nil, err
		}
		// TODO: Support LRO
		if resp.StatusCode()/100 != 2 {
			return nil, fmt.Errorf("Unexpected response (%s - code: %d): %s", resp.Status(), resp.StatusCode(), string(resp.Body()))
		}
		return resp.Body(), nil
	}
	return nil, fmt.Errorf("unknown create method: %s", c.CreateMethod)
}

func (c *Client) Read(ctx context.Context, path string) ([]byte, error) {
	resp, err := c.R().SetContext(ctx).Get(path)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode()/100 != 2 {
		return nil, fmt.Errorf("Unexpected response (%s - code: %d): %s", resp.Status(), resp.StatusCode(), string(resp.Body()))
	}
	return resp.Body(), nil
}

func (c *Client) Update(ctx context.Context, path string, body interface{}) ([]byte, error) {
	req := c.R().SetContext(ctx).SetBody(body)
	if c.ContentType != "" {
		req = req.SetHeader("Content-Type", c.ContentType)
	}
	resp, err := req.Put(path)
	if err != nil {
		return nil, err
	}
	// TODO: Support LRO
	if resp.StatusCode()/100 != 2 {
		return nil, fmt.Errorf("Unexpected response (%s - code: %d): %s", resp.Status(), resp.StatusCode(), string(resp.Body()))
	}
	return resp.Body(), nil
}

func (c *Client) Delete(ctx context.Context, path string) ([]byte, error) {
	resp, err := c.R().SetContext(ctx).Delete(path)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() == http.StatusNotFound {
		return nil, ErrNotFound
	}
	// TODO: Support LRO
	if resp.StatusCode()/100 != 2 {
		return nil, fmt.Errorf("Unexpected response (%s - code: %d): %s", resp.Status(), resp.StatusCode(), string(resp.Body()))
	}
	return resp.Body(), nil
}
