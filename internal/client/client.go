package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"golang.org/x/net/publicsuffix"
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

	transport := http.DefaultTransport.(*http.Transport)
	transport.TLSClientConfig = &opt.TLSConfig
	httpClient := &http.Client{
		Transport: transport,
	}
	if opt.CookieEnabled {
		cookieJar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
		httpClient.Jar = cookieJar
	}

	client := resty.New()
	if opt.Security != nil {
		var err error
		client, err = opt.Security.newClient(ctx, httpClient)
		if err != nil {
			return nil, err
		}
	}

	if _, err := url.Parse(baseURL); err != nil {
		return nil, err
	}

	client.SetBaseURL(baseURL)

	return &Client{client}, nil
}

type RetryOption struct {
	StatusLocator ValueLocator
	Status        PollingStatus
	Count         int
	WaitTime      time.Duration
	MaxWaitTime   time.Duration
}

func (c *Client) resetRetry() {
	c.RetryCount = 0
	c.RetryWaitTime = 0
	c.RetryMaxWaitTime = 0
	c.RetryAfter = nil
	c.RetryConditions = nil
}

func (c *Client) setRetry(opt RetryOption) {
	c.RetryCount = opt.Count
	c.RetryWaitTime = opt.WaitTime
	c.RetryMaxWaitTime = opt.MaxWaitTime
	c.RetryAfter = func(c *resty.Client, r *resty.Response) (time.Duration, error) {
		dur := r.Header().Get("Retry-After")
		if dur == "" {
			// returning 0 will make the resty retry to using the backoff based on wait time, count and max wait time
			return 0, nil
		}
		d, err := time.ParseDuration(dur + "s")
		if err != nil {
			return 0, fmt.Errorf("invalid Retry-After value in the initiated response: %s", dur)
		}
		return d, nil
	}
	c.RetryConditions = []resty.RetryConditionFunc{
		func(r *resty.Response, err error) bool {
			if err != nil {
				return true
			}

			status := opt.StatusLocator.LocateValueInResp(*r)
			if status == "" {
				return false
			}
			// We tolerate case difference here to be pragmatic.
			if strings.EqualFold(status, opt.Status.Success) {
				return false
			}

			for _, ps := range opt.Status.Pending {
				if strings.EqualFold(status, ps) {
					return true
				}
			}

			return false
		},
	}
}

type CreateOption struct {
	Method string
	Query  Query
	Header Header
	Retry  *RetryOption
}

func (c *Client) Create(ctx context.Context, path string, body interface{}, opt CreateOption) (*resty.Response, error) {
	if opt.Retry != nil {
		c.setRetry(*opt.Retry)
		defer c.resetRetry()
	}
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
	Retry  *RetryOption
}

func (c *Client) Read(ctx context.Context, path string, opt ReadOption) (*resty.Response, error) {
	if opt.Retry != nil {
		c.setRetry(*opt.Retry)
		defer c.resetRetry()
	}

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
	Retry              *RetryOption
}

func (c *Client) Update(ctx context.Context, path string, body interface{}, opt UpdateOption) (*resty.Response, error) {
	if opt.Retry != nil {
		c.setRetry(*opt.Retry)
		defer c.resetRetry()
	}

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
	Method string
	Query  Query
	Header Header
	Retry  *RetryOption
}

func (c *Client) Delete(ctx context.Context, path string, opt DeleteOption) (*resty.Response, error) {
	if opt.Retry != nil {
		c.setRetry(*opt.Retry)
		defer c.resetRetry()
	}

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
	Retry  *RetryOption
}

func (c *Client) Operation(ctx context.Context, path string, body interface{}, opt OperationOption) (*resty.Response, error) {
	if opt.Retry != nil {
		c.setRetry(*opt.Retry)
		defer c.resetRetry()
	}

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

type ReadOptionDS struct {
	// Method used for reading, which defaults to GET
	Method string
	Query  Query
	Header Header
	Retry  *RetryOption
}

func (c *Client) ReadDS(ctx context.Context, path string, opt ReadOptionDS) (*resty.Response, error) {
	if opt.Retry != nil {
		c.setRetry(*opt.Retry)
		defer c.resetRetry()
	}

	req := c.R().SetContext(ctx)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)

	switch opt.Method {
	case "", "GET":
		return req.Get(path)
	case "POST":
		return req.Post(path)
	case "HEAD":
		return req.Head(path)
	default:
		return nil, fmt.Errorf("unknown read (ds) method: %s", opt.Method)
	}
}
