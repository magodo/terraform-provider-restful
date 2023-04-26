package client

import (
	"context"
	"fmt"
	"net/url"

	"github.com/go-resty/resty/v2"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type Query url.Values

func (q Query) Clone() Query {
	m := url.Values{}
	for k, v := range q {
		m[k] = v
	}
	return Query(m)
}

func (q Query) TakeOrSelf(ctx context.Context, v types.Map) Query {
	if len(v.Elements()) == 0 {
		return q
	}
	nq := Query{}
	for k, v := range v.Elements() {
		vs := []string{}
		diags := v.(types.List).ElementsAs(ctx, &vs, false)
		if diags.HasError() {
			panic(diags)
		}
		nq[k] = vs
	}
	return nq
}

func (q Query) ToTFValue() types.Map {
	var result types.Map
	tfsdk.ValueFrom(context.Background(), q, types.MapType{ElemType: types.ListType{ElemType: types.StringType}}, &result)
	return result
}

type Header map[string]string

func (h Header) Clone() Header {
	nh := Header{}
	for k, v := range h {
		nh[k] = v
	}
	return nh
}

func (h Header) TakeOrSelf(ctx context.Context, v types.Map) Header {
	if len(v.Elements()) == 0 {
		return h
	}
	nh := Header{}
	for k, v := range v.Elements() {
		nh[k] = v.(types.String).ValueString()
	}
	return nh
}

func (h Header) ToTFValue() types.Map {
	var result types.Map
	tfsdk.ValueFrom(context.Background(), h, types.MapType{ElemType: types.StringType}, &result)
	return result
}

type Client struct {
	*resty.Client
}

func New(ctx context.Context, baseURL string, opt *BuildOption) (*Client, error) {
	if opt == nil {
		opt = &BuildOption{}
	}

	client := resty.New()
	if opt.Security != nil {
		var err error
		client, err = opt.Security.newClient(ctx)
		if err != nil {
			return nil, err
		}
	}

	if !opt.CookieEnabled {
		client.SetCookieJar(nil)
	}

	client.SetTLSClientConfig(&opt.TLSConfig)

	if _, err := url.Parse(baseURL); err != nil {
		return nil, err
	}

	client.SetBaseURL(baseURL)

	return &Client{client}, nil
}

type CreateOption struct {
	Method string
	Query  Query
	Header Header
}

func (c *Client) Create(ctx context.Context, path string, body interface{}, opt CreateOption) (*resty.Response, error) {
	req := c.R().SetContext(ctx).SetBody(body)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)
	req = req.SetHeader("Content-Type", "application/json")

	switch opt.Method {
	case "POST":
		return req.Post(path)
	case "PUT":
		return req.Put(path)
	default:
		return nil, fmt.Errorf("unknown create method: %s", opt.Method)
	}
}

type ReadOption struct {
	Query  Query
	Header Header
}

func (c *Client) Read(ctx context.Context, path string, opt ReadOption) (*resty.Response, error) {
	req := c.R().SetContext(ctx)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)

	return req.Get(path)
}

type UpdateOption struct {
	Method             string
	MergePatchDisabled bool
	Query              Query
	Header             Header
}

func (c *Client) Update(ctx context.Context, path string, body interface{}, opt UpdateOption) (*resty.Response, error) {
	req := c.R().SetContext(ctx).SetBody(body)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)
	req = req.SetHeader("Content-Type", "application/json")

	switch opt.Method {
	case "PATCH":
		return req.Patch(path)
	case "PUT":
		return req.Put(path)
	case "POST":
		return req.Post(path)
	default:
		return nil, fmt.Errorf("unknown update method: %s", opt.Method)
	}
}

type DeleteOption struct {
	Method  string
	Query   Query
	Header  Header
	PollOpt *PollOption
}

func (c *Client) Delete(ctx context.Context, path string, opt DeleteOption) (*resty.Response, error) {
	req := c.R().SetContext(ctx)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)

	switch opt.Method {
	case "DELETE":
		return req.Delete(path)
	case "POST":
		return req.Post(path)
	default:
		return nil, fmt.Errorf("unknown delete method: %s", opt.Method)
	}
}

type OperationOption struct {
	Method string
	Query  Query
	Header Header
}

func (c *Client) Operation(ctx context.Context, path string, body interface{}, opt OperationOption) (*resty.Response, error) {
	req := c.R().SetContext(ctx)
	if body != "" {
		req.SetBody(body)
	}
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)
	req = req.SetHeader("Content-Type", "application/json")

	switch opt.Method {
	case "POST":
		return req.Post(path)
	case "PUT":
		return req.Put(path)
	case "PATCH":
		return req.Patch(path)
	case "DELETE":
		return req.Delete(path)
	default:
		return nil, fmt.Errorf("unknown create method: %s", opt.Method)
	}
}
