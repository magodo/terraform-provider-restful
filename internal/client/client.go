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
	if len(v.Elems) == 0 {
		return q
	}
	nq := Query{}
	for k, v := range v.Elems {
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
	if len(v.Elems) == 0 {
		return h
	}
	nh := Header{}
	for k, v := range v.Elems {
		nh[k] = v.(types.String).Value
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
	CreateMethod string
	Query        Query
	Header       Header
	PollOpt      *PollOption
}

func (c *Client) Create(ctx context.Context, path string, body interface{}, opt CreateOption) (*resty.Response, error) {
	req := c.R().SetContext(ctx).SetBody(body)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)
	req = req.SetHeader("Content-Type", "application/json")

	switch opt.CreateMethod {
	case "POST":
		return req.Post(path)
	case "PUT":
		return req.Put(path)
	default:
		return nil, fmt.Errorf("unknown create method: %s", opt.CreateMethod)
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
	UpdateMethod       string
	MergePatchDisabled bool
	Query              Query
	Header             Header
	PollOpt            *PollOption
}

func (c *Client) Update(ctx context.Context, path string, body interface{}, opt UpdateOption) (*resty.Response, error) {
	req := c.R().SetContext(ctx).SetBody(body)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)
	req = req.SetHeader("Content-Type", "application/json")

	switch opt.UpdateMethod {
	case "PATCH":
		return req.Patch(path)
	case "PUT":
		return req.Put(path)
	default:
		return nil, fmt.Errorf("unknown create method: %s", opt.UpdateMethod)
	}
}

type DeleteOption struct {
	Query   Query
	Header  Header
	PollOpt *PollOption
}

func (c *Client) Delete(ctx context.Context, path string, opt DeleteOption) (*resty.Response, error) {
	req := c.R().SetContext(ctx)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)

	return req.Delete(path)
}
