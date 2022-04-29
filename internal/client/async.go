package client

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
)

// ValueLocator indicates where a value is located in a HTTP response.
type ValueLocator interface {
	locateValueInResp(resty.Response) string
	String() string
}

type headerKeyLocator string

func (loc headerKeyLocator) locateValueInResp(resp resty.Response) string {
	return resp.Header().Get(string(loc))
}
func (loc headerKeyLocator) String() string {
	return fmt.Sprintf(`header[%s]`, string(loc))
}

type bodyPathLocator string

func (loc bodyPathLocator) locateValueInResp(resp resty.Response) string {
	result := gjson.GetBytes(resp.Body(), string(loc))
	return result.String()
}
func (loc bodyPathLocator) String() string {
	return fmt.Sprintf(`body[%s]`, string(loc))
}

type PollingStatus struct {
	Pending []string
	Failed  string
	Success string
}

type FutureOption struct {
	// LocationOption configures the how to discover and access the polling location.
	// If it is nil, the original request URL is used for polling.
	LocationOption *LocationOption

	// Status the status sentinels for polling.
	Status PollingStatus

	// StatusLocator indicates where the polling status is located in the response of the polling requests.
	StatusLocator ValueLocator

	// PollingDelay specifies the interval between two pollings. The `Retry-After` in the response header takes higher precedence than this.
	PollingDelay time.Duration
}

type LocationOption struct {
	// URLLocator indicates where the async polling URL is located in the response of the initiated request.
	URLLocator ValueLocator

	// Query is the query parameters used for polling requests against the polling URL.
	Query url.Values
}

func NewFuture(resp resty.Response, opt FutureOption) (*Future, error) {
	f := Future{}

	if opt.PollingDelay == 0 {
		opt.PollingDelay = 10 * time.Second
	}
	f.Delay = opt.PollingDelay

	if opt.Status.Failed == "" {
		return nil, fmt.Errorf("Status.Failed is required but not set")
	}
	if opt.Status.Success == "" {
		return nil, fmt.Errorf("Status.Success is required but not set")
	}
	f.Status = opt.Status

	if opt.StatusLocator == nil {
		return nil, fmt.Errorf("StatusLocator is required but not set")
	}
	f.StatusLocator = opt.StatusLocator

	if dur := resp.Header().Get("Retry-After"); dur != "" {
		d, err := time.ParseDuration(dur + "s")
		if err != nil {
			return nil, fmt.Errorf("invalid Retry-After value in the initiated response: %s", dur)
		}
		f.InitDelay = d
	}

	if lopt := opt.LocationOption; lopt != nil {
		if lopt.URLLocator == nil {
			return nil, fmt.Errorf("URLLocator is required but not set")
		}
		url := lopt.URLLocator.locateValueInResp(resp)
		if url == "" {
			return nil, fmt.Errorf("No polling URL found in %s", lopt.URLLocator)
		}
		f.URL = url
		f.Query = lopt.Query
	} else {
		f.URL = resp.Request.URL
		f.Query = resp.Request.QueryParam
	}

	return &f, nil
}

type Future struct {
	InitDelay     time.Duration
	URL           string
	Query         url.Values
	Status        PollingStatus
	StatusLocator ValueLocator
	Delay         time.Duration
}

func (f *Future) WaitForComplete(ctx context.Context, client *Client) error {
	time.Sleep(f.InitDelay)
PollingLoop:
	for {
		// There is no need to retry here as resty client has embedded retry logic (by default 3 max retries).
		req := client.R().SetContext(ctx).SetQueryParamsFromValues(f.Query)
		resp, err := req.Get(f.URL)
		if err != nil {
			return fmt.Errorf("polling %s: %v", f.URL, err)
		}

		status := f.StatusLocator.locateValueInResp(*resp)
		if status == "" {
			return fmt.Errorf("No status value found from %s", f.StatusLocator)
		}
		// We tolerate case difference here to be pragmatic.
		if strings.EqualFold(status, f.Status.Success) {
			return nil
		}
		if strings.EqualFold(status, f.Status.Failed) {
			return fmt.Errorf("LRO failed: %s", string(resp.Body()))
		}
		for _, ps := range f.Status.Pending {
			if strings.EqualFold(status, ps) {
				dur := resp.Header().Get("Retry-After")
				if dur == "" {
					time.Sleep(f.Delay)
					continue PollingLoop
				}
				d, err := time.ParseDuration(dur + "s")
				if err != nil {
					return fmt.Errorf("invalid Retry-After value in the initiated response: %s", dur)
				}
				time.Sleep(d)
				continue PollingLoop
			}
		}
		return fmt.Errorf("Unknown status %q. Full response: %v", status, string(resp.Body()))
	}
}
