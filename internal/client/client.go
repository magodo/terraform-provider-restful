package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/lfventura/terraform-provider-restful/internal/exparam"
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

func (q Query) TakeWithExparamOrSelf(ctx context.Context, v types.Map, body []byte) Query {
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
		vvs := []string{}
		for _, v := range vs {
			vv, err := exparam.ExpandBody(v, body)
			if err != nil {
				vv = v
			}
			vvs = append(vvs, vv)
		}
		nq[k] = vvs
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

func (h Header) TakeWithExparamOrSelf(ctx context.Context, v types.Map, body []byte) Header {
	if len(v.Elements()) == 0 {
		return h
	}
	nh := Header{}
	for k, v := range v.Elements() {
		v := v.(types.String).ValueString()
		vv, err := exparam.ExpandBody(v, body)
		if err != nil {
			vv = v
		}
		nh[k] = vv
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
	Security SecurityOption
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

	client := resty.NewWithClient(httpClient)
	client.SetDebug(true)

	if opt.Retry != nil {
		setRetry(client, *opt.Retry)
	}

	if opt.Security != nil {
		if err := opt.Security.configureClient(ctx, client); err != nil {
			return nil, err
		}
	}

	client.SetBaseURL(baseURL)

	return &Client{Client: client, Security: opt.Security}, nil
}

type RetryOption struct {
	StatusCodes []int64
	Count       int
	WaitTime    time.Duration
	MaxWaitTime time.Duration
}

func setRetry(c *resty.Client, opt RetryOption) {
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

			for _, ps := range opt.StatusCodes {
				if r.StatusCode() == int(ps) {
					return true
				}
			}

			return false
		},
	}
}

// SetLoggerContext sets the ctx to the internal resty logger, as the tflog requires the current ctx.
// This needs to be called at the start of each CRUD function.
func (c *Client) SetLoggerContext(ctx context.Context) {
	c.Client.SetLogger(tflogger{ctx: ctx})
}

type CreateOption struct {
	Method string
	Query  Query
	Header Header
}

func (c *Client) Create(ctx context.Context, path string, body string, opt CreateOption) (*resty.Response, error) {
	req := c.R().SetContext(ctx).SetBody(body)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)
	if req.Header.Get("content-type") == "" {
		req = req.SetHeader("Content-Type", "application/json")
	}

	switch opt.Method {
	case "POST":
		return req.Post(path)
	case "PUT":
		return req.Put(path)
	case "PATCH":
		return req.Patch(path)
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

func (c *Client) Update(ctx context.Context, path string, body string, opt UpdateOption) (*resty.Response, error) {
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
}

func (c *Client) Delete(ctx context.Context, path string, body string, opt DeleteOption) (*resty.Response, error) {
	req := c.R().SetContext(ctx)
	if body != "" {
		req = req.SetHeader("Content-Type", "application/json")
		req.SetBody(body)
	}
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)

	switch opt.Method {
	case "POST":
		return req.Post(path)
	case "PATCH":
		return req.Patch(path)
	case "PUT":
		return req.Put(path)
	case "DELETE":
		return req.Delete(path)
	default:
		return nil, fmt.Errorf("unknown delete method: %s", opt.Method)
	}
}

type OperationOption struct {
	Method string
	Query  Query
	Header Header
}

func (c *Client) Operation(ctx context.Context, path string, body []byte, opt OperationOption) (*resty.Response, error) {
	req := c.R().SetContext(ctx)
	req.SetQueryParamsFromValues(url.Values(opt.Query))

	// By default set the content-type to application/json
	// This can be replaced by the opt.Header if defined.
	req = req.SetHeader("Content-Type", "application/json")
	req.SetHeaders(opt.Header)

	if len(body) != 0 {
		switch req.Header.Get("Content-Type") {
		case "application/x-www-form-urlencoded":
			m := map[string]string{}
			if err := json.Unmarshal(body, &m); err != nil {
				return nil, fmt.Errorf("unmarshaling the operation body to a map of string: %v", err)
			}
			req.SetFormData(m)
		default:
			req.SetBody(string(body))
		}
	}

	switch opt.Method {
	case "GET":
		return req.Get(path)
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
}

func (c *Client) ReadDS(ctx context.Context, path string, body []byte, opt ReadOptionDS) (*resty.Response, error) {
	req := c.R().SetContext(ctx)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)

	// Set the body for POST requests if provided
	if len(body) != 0 && opt.Method == "POST" {
		req = req.SetHeader("Content-Type", "application/json")
		req = req.SetBody(body)
	}

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

type ReadOptionLR struct {
	// Method used for reading, which defaults to GET
	Method string
	Query  Query
	Header Header
}

func (c *Client) ReadLR(ctx context.Context, path string, body []byte, opt ReadOptionLR) (*resty.Response, error) {
	req := c.R().SetContext(ctx)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)

	// Set the body for POST requests if provided
	if len(body) != 0 && opt.Method == "POST" {
		req = req.SetHeader("Content-Type", "application/json")
		req = req.SetBody(body)
	}

	switch opt.Method {
	case "", "GET":
		return req.Get(path)
	case "POST":
		return req.Post(path)
	case "HEAD":
		return req.Head(path)
	default:
		return nil, fmt.Errorf("unknown read (lr) method: %s", opt.Method)
	}
}
