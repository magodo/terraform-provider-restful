package client

import (
	"context"
	"fmt"
	"net/url"

	"github.com/go-resty/resty/v2"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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

// MergeFromTFValue merges TF value of type MapType{ElemType: ListType{ElemType: StringType}} to the receiver query. Other types will cause panic.
func (q Query) MergeFromTFValue(ctx context.Context, v types.Map) Query {
	if len(v.Elems) != 0 {
		for k, v := range v.Elems {
			vs := []string{}
			diags := v.(types.List).ElementsAs(ctx, &vs, false)
			if diags.HasError() {
				panic(diags)
			}
			q[k] = vs
		}
	}
	return q
}

func (q Query) ToTFValue() types.Map {
	out := types.Map{
		ElemType: types.ListType{ElemType: types.StringType},
		Elems:    map[string]attr.Value{},
	}
	for k, vs := range q {
		l := types.List{
			ElemType: types.StringType,
		}
		for _, v := range vs {
			l.Elems = append(l.Elems, types.String{Value: v})
		}
		out.Elems[k] = l
	}
	return out
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
	PollOpt      *PollOption
}

func (c *Client) Create(ctx context.Context, path string, body interface{}, opt CreateOption) (*resty.Response, error) {
	req := c.R().SetContext(ctx).SetBody(body)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
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
	Query Query
}

func (c *Client) Read(ctx context.Context, path string, opt ReadOption) (*resty.Response, error) {
	req := c.R().SetContext(ctx)
	req.SetQueryParamsFromValues(url.Values(opt.Query))

	return req.Get(path)
}

type UpdateOption struct {
	Query   Query
	PollOpt *PollOption
}

func (c *Client) Update(ctx context.Context, path string, body interface{}, opt UpdateOption) (*resty.Response, error) {
	req := c.R().SetContext(ctx).SetBody(body)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req = req.SetHeader("Content-Type", "application/json")

	return req.Put(path)
}

type DeleteOption struct {
	Query   Query
	PollOpt *PollOption
}

func (c *Client) Delete(ctx context.Context, path string, opt DeleteOption) (*resty.Response, error) {
	req := c.R().SetContext(ctx)
	req.SetQueryParamsFromValues(url.Values(opt.Query))

	return req.Delete(path)
}
