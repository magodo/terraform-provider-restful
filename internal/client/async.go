package client

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/tidwall/gjson"
)

// ValueLocator indicates where a value is located in a HTTP response.
type ValueLocator interface {
	LocateValueInResp(resty.Response) string
	String() string
}

type ExactLocator string

func (loc ExactLocator) LocateValueInResp(_ resty.Response) string {
	return string(loc)
}
func (loc ExactLocator) String() string {
	return fmt.Sprintf(`exact.%s`, string(loc))
}

type HeaderLocator string

func (loc HeaderLocator) LocateValueInResp(resp resty.Response) string {
	return resp.Header().Get(string(loc))
}
func (loc HeaderLocator) String() string {
	return fmt.Sprintf(`header.%s`, string(loc))
}

type BodyLocator string

func (loc BodyLocator) LocateValueInResp(resp resty.Response) string {
	result := gjson.GetBytes(resp.Body(), string(loc))
	return result.String()
}
func (loc BodyLocator) String() string {
	return fmt.Sprintf(`body.%s`, string(loc))
}

type CodeLocator struct{}

func (loc CodeLocator) LocateValueInResp(resp resty.Response) string {
	return strconv.Itoa(resp.StatusCode())
}
func (loc CodeLocator) String() string {
	return "code"
}

type PollingStatus struct {
	Pending []string
	Success string
}

type PollOption struct {
	// StatusLocator indicates where the polling status is located in the response of the polling requests.
	StatusLocator ValueLocator

	// Status the status sentinels for polling.
	Status PollingStatus

	// UrlLocator configures the how to discover the polling location.
	// If it is nil, the original request URL is used for polling.
	UrlLocator ValueLocator

	Header Header

	Query Query

	// DefaultDelay specifies the interval between two pollings. The `Retry-After` in the response header takes higher precedence than this.
	DefaultDelay time.Duration
}

func NewPollableForPoll(resp resty.Response, opt PollOption) (*Pollable, error) {
	p := Pollable{
		DefaultDelay: opt.DefaultDelay,
		Header:       opt.Header,
		Query:        opt.Query,
	}

	if opt.Status.Success == "" {
		return nil, fmt.Errorf("Status.Success is required but not set")
	}
	p.Status = opt.Status

	if opt.StatusLocator == nil {
		return nil, fmt.Errorf("StatusLocator is required but not set")
	}
	p.StatusLocator = opt.StatusLocator

	if dur := resp.Header().Get("Retry-After"); dur != "" {
		d, err := time.ParseDuration(dur + "s")
		if err != nil {
			return nil, fmt.Errorf("invalid Retry-After value in the initiated response: %s", dur)
		}
		p.InitDelay = d
	}

	var rawURL string
	if loc := opt.UrlLocator; loc != nil {
		rawURL = loc.LocateValueInResp(resp)
		if rawURL == "" {
			return nil, fmt.Errorf("No polling URL found in %s", loc)
		}
	} else {
		rawURL = resp.Request.URL
	}
	urL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parsing raw URL %q: %v", rawURL, err)
	}

	// In case the url_locator is specified, we will overwrite the query by the query parameters contained in the polling URL,
	// as typically the polling URL is a complete URL with both the path and query parameters (probably contains auth code).
	// Otherwise, if the url_locator not specified, which means we will GET the same URL as the original request. Hence, we continue
	// to use the same query parameter as the original request (as is passed in the opt) .
	if opt.UrlLocator != nil {
		p.Query = Query(urL.Query())
	}
	urL.RawQuery = ""
	p.URL = urL.String()

	return &p, nil
}

func NewPollableForPrecheck(opt PollOption) (*Pollable, error) {
	p := Pollable{
		DefaultDelay: opt.DefaultDelay,
		Header:       opt.Header,
	}

	if opt.Status.Success == "" {
		return nil, fmt.Errorf("Status.Success is required but not set")
	}
	p.Status = opt.Status

	if opt.StatusLocator == nil {
		return nil, fmt.Errorf("StatusLocator is required but not set")
	}
	p.StatusLocator = opt.StatusLocator

	if opt.UrlLocator == nil {
		return nil, fmt.Errorf("UrlLocator is required but not set")
	}
	eloc, ok := opt.UrlLocator.(ExactLocator)
	if !ok {
		return nil, fmt.Errorf("expect UrlLocator to be ExactLocator, but got %T", opt.UrlLocator)
	}
	p.URL = string(eloc)

	return &p, nil
}

type Pollable struct {
	InitDelay     time.Duration
	URL           string
	Header        Header
	Query         Query
	Status        PollingStatus
	StatusLocator ValueLocator
	DefaultDelay  time.Duration
}

func (f *Pollable) PollUntilDone(ctx context.Context, client *Client) error {
	time.Sleep(f.InitDelay)
PollingLoop:
	for {
		// There is no need to retry here as resty client has embedded retry logic (by default 3 max retries).
		req := client.R().SetContext(ctx).SetHeaders(f.Header).SetQueryParamsFromValues(url.Values(f.Query))
		resp, err := req.Get(f.URL)
		if err != nil {
			return fmt.Errorf("polling %s: %v", f.URL, err)
		}

		// In case this is status_locator is not a code locator, then we shall firstly ensure the GET succeeded,
		// to avoid the status retrieving error hides the actual GET error.
		if _, ok := f.StatusLocator.(CodeLocator); !ok {
			if !resp.IsSuccess() {
				return fmt.Errorf("polling returns %d: %s", resp.StatusCode(), string(resp.Body()))
			}
		}

		status := f.StatusLocator.LocateValueInResp(*resp)
		if status == "" {
			return fmt.Errorf("No status value found from %s", f.StatusLocator)
		}
		// We tolerate case difference here to be pragmatic.
		if strings.EqualFold(status, f.Status.Success) {
			return nil
		}
		for _, ps := range f.Status.Pending {
			if strings.EqualFold(status, ps) {
				dur := resp.Header().Get("Retry-After")
				if dur == "" {
					time.Sleep(f.DefaultDelay)
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
		return fmt.Errorf("Unexpected status %q. Full response: %v", status, string(resp.Body()))
	}
}
