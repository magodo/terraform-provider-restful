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
}

func New(baseURL string, opt *BuildOption) (*Client, error) {
	if opt == nil {
		opt = &BuildOption{}
	}

	client := resty.New()
	if opt.Security != nil {
		client = opt.Security.newClient()
	}

	if _, err := url.Parse(baseURL); err != nil {
		return nil, err
	}

	client.SetBaseURL(baseURL)
	return &Client{client}, nil
}

type CreateOption struct {
	Method string
	Query  map[string]string
}

func (c *Client) Create(ctx context.Context, path string, body interface{}, opt CreateOption) ([]byte, error) {
	req := c.R().SetContext(ctx).SetBody(body)
	req.SetQueryParams(opt.Query)
	req = req.SetHeader("Content-Type", "application/json")

	switch opt.Method {
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
	default:
		return nil, fmt.Errorf("unknown create method: %s", opt.Method)
	}
}

type ReadOption struct {
	Query map[string]string
}

func (c *Client) Read(ctx context.Context, path string, opt ReadOption) ([]byte, error) {
	req := c.R().SetContext(ctx)
	req.SetQueryParams(opt.Query)

	resp, err := req.Get(path)
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

type UpdateOption struct {
	Query map[string]string
}

func (c *Client) Update(ctx context.Context, path string, body interface{}, opt UpdateOption) ([]byte, error) {
	req := c.R().SetContext(ctx).SetBody(body)
	req.SetQueryParams(opt.Query)
	req = req.SetHeader("Content-Type", "application/json")

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

type DeleteOption struct {
	Query map[string]string
}

func (c *Client) Delete(ctx context.Context, path string, opt DeleteOption) ([]byte, error) {
	req := c.R().SetContext(ctx)
	req.SetQueryParams(opt.Query)

	resp, err := req.Delete(path)
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
