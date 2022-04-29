package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/go-resty/resty/v2"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ErrNotFound is expected to be returned for `Read` when the resource with the specified id doesn't exist.
var ErrNotFound = errors.New("resource not found")

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
	Method string
	Query  Query
}

func (c *Client) Create(ctx context.Context, path string, body interface{}, opt CreateOption) ([]byte, error) {
	req := c.R().SetContext(ctx).SetBody(body)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
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
	Query Query
}

func (c *Client) Read(ctx context.Context, path string, opt ReadOption) ([]byte, error) {
	req := c.R().SetContext(ctx)
	req.SetQueryParamsFromValues(url.Values(opt.Query))

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
	Query Query
}

func (c *Client) Update(ctx context.Context, path string, body interface{}, opt UpdateOption) ([]byte, error) {
	req := c.R().SetContext(ctx).SetBody(body)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
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
	Query Query
}

func (c *Client) Delete(ctx context.Context, path string, opt DeleteOption) ([]byte, error) {
	req := c.R().SetContext(ctx)
	req.SetQueryParamsFromValues(url.Values(opt.Query))

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
