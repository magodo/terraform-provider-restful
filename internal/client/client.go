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
	"github.com/magodo/terraform-provider-restful/internal/exparam"
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
}

// resolvePath returns the effective target URL to pass to resty.
// When baseURL is non-empty, it is joined with the path producing an absolute URL, which
// overrides the client's default base URL. Otherwise, the path is returned as-is and resty
// will resolve it against the client's configured base URL.
func resolvePath(baseURL, path string) (string, error) {
	if baseURL == "" {
		return path, nil
	}
	joined, err := url.JoinPath(baseURL, path)
	if err != nil {
		return "", fmt.Errorf("joining base url %q with path %q: %v", baseURL, path, err)
	}
	return joined, nil
}

func New(ctx context.Context, opt *BuildOption) (*Client, error) {
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

	return &Client{client}, nil
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
	// BaseURL overrides the client's default base URL for this request. Optional.
	BaseURL string
	Method  string
	Query   Query
	Header  Header
}

func (c *Client) Create(ctx context.Context, path string, body string, opt CreateOption) (*resty.Response, error) {
	target, err := resolvePath(opt.BaseURL, path)
	if err != nil {
		return nil, err
	}
	req := c.R().SetContext(ctx).SetBody(body)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)
	if req.Header.Get("content-type") == "" {
		req = req.SetHeader("Content-Type", "application/json")
	}

	switch opt.Method {
	case "POST":
		return req.Post(target)
	case "PUT":
		return req.Put(target)
	case "PATCH":
		return req.Patch(target)
	default:
		return nil, fmt.Errorf("unknown create method: %s", opt.Method)
	}
}

type ReadOption struct {
	BaseURL string
	Query   Query
	Header  Header
}

func (c *Client) Read(ctx context.Context, path string, opt ReadOption) (*resty.Response, error) {
	target, err := resolvePath(opt.BaseURL, path)
	if err != nil {
		return nil, err
	}
	req := c.R().SetContext(ctx)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)

	return req.Get(target)
}

type UpdateOption struct {
	BaseURL            string
	Method             string
	MergePatchDisabled bool
	Query              Query
	Header             Header
}

func (c *Client) Update(ctx context.Context, path string, body string, opt UpdateOption) (*resty.Response, error) {
	target, err := resolvePath(opt.BaseURL, path)
	if err != nil {
		return nil, err
	}
	req := c.R().SetContext(ctx).SetBody(body)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)
	if req.Header.Get("content-type") == "" {
		req = req.SetHeader("Content-Type", "application/json")
	}

	switch opt.Method {
	case "PATCH":
		return req.Patch(target)
	case "PUT":
		return req.Put(target)
	case "POST":
		return req.Post(target)
	default:
		return nil, fmt.Errorf("unknown update method: %s", opt.Method)
	}
}

type DeleteOption struct {
	BaseURL string
	Method  string
	Query   Query
	Header  Header
}

func (c *Client) Delete(ctx context.Context, path string, body string, opt DeleteOption) (*resty.Response, error) {
	target, err := resolvePath(opt.BaseURL, path)
	if err != nil {
		return nil, err
	}
	req := c.R().SetContext(ctx)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)
	if body != "" {
		req.SetBody(body)
		if req.Header.Get("content-type") == "" {
			req = req.SetHeader("Content-Type", "application/json")
		}
	}

	switch opt.Method {
	case "POST":
		return req.Post(target)
	case "PATCH":
		return req.Patch(target)
	case "PUT":
		return req.Put(target)
	case "DELETE":
		return req.Delete(target)
	default:
		return nil, fmt.Errorf("unknown delete method: %s", opt.Method)
	}
}

type OperationOption struct {
	BaseURL string
	Method  string
	Query   Query
	Header  Header
}

func (c *Client) Operation(ctx context.Context, path string, body []byte, opt OperationOption) (*resty.Response, error) {
	target, err := resolvePath(opt.BaseURL, path)
	if err != nil {
		return nil, err
	}
	req := c.R().SetContext(ctx)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)
	if req.Header.Get("content-type") == "" {
		req = req.SetHeader("Content-Type", "application/json")
	}

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
		return req.Get(target)
	case "POST":
		return req.Post(target)
	case "PUT":
		return req.Put(target)
	case "PATCH":
		return req.Patch(target)
	case "DELETE":
		return req.Delete(target)
	default:
		return nil, fmt.Errorf("unknown create method: %s", opt.Method)
	}
}

type ReadOptionDS struct {
	BaseURL string
	// Method used for reading, which defaults to GET
	Method string
	Query  Query
	Header Header
}

func (c *Client) ReadDS(ctx context.Context, path string, body []byte, opt ReadOptionDS) (*resty.Response, error) {
	target, err := resolvePath(opt.BaseURL, path)
	if err != nil {
		return nil, err
	}
	req := c.R().SetContext(ctx)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)

	// Set the body for POST requests if provided
	if len(body) != 0 && opt.Method == "POST" {
		req = req.SetBody(body)
		if req.Header.Get("content-type") == "" {
			req = req.SetHeader("Content-Type", "application/json")
		}
	}

	switch opt.Method {
	case "", "GET":
		return req.Get(target)
	case "POST":
		return req.Post(target)
	case "HEAD":
		return req.Head(target)
	default:
		return nil, fmt.Errorf("unknown read (ds) method: %s", opt.Method)
	}
}

type ReadOptionLR struct {
	BaseURL string
	// Method used for reading, which defaults to GET
	Method string
	Query  Query
	Header Header
}

func (c *Client) ReadLR(ctx context.Context, path string, body []byte, opt ReadOptionLR) (*resty.Response, error) {
	target, err := resolvePath(opt.BaseURL, path)
	if err != nil {
		return nil, err
	}
	req := c.R().SetContext(ctx)
	req.SetQueryParamsFromValues(url.Values(opt.Query))
	req.SetHeaders(opt.Header)

	// Set the body for POST requests if provided
	if len(body) != 0 && opt.Method == "POST" {
		req = req.SetBody(body)
		if req.Header.Get("content-type") == "" {
			req = req.SetHeader("Content-Type", "application/json")
		}
	}

	switch opt.Method {
	case "", "GET":
		return req.Get(target)
	case "POST":
		return req.Post(target)
	case "HEAD":
		return req.Head(target)
	default:
		return nil, fmt.Errorf("unknown read (lr) method: %s", opt.Method)
	}
}
