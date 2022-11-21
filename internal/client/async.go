package client

import (
	"context"
	"fmt"
	"strconv"
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

type ExactLocator string

func (loc ExactLocator) locateValueInResp(_ resty.Response) string {
	return string(loc)
}
func (loc ExactLocator) String() string {
	return fmt.Sprintf(`exact[%s]`, string(loc))
}

type HeaderLocator string

func (loc HeaderLocator) locateValueInResp(resp resty.Response) string {
	return resp.Header().Get(string(loc))
}
func (loc HeaderLocator) String() string {
	return fmt.Sprintf(`header[%s]`, string(loc))
}

type BodyLocator string

func (loc BodyLocator) locateValueInResp(resp resty.Response) string {
	result := gjson.GetBytes(resp.Body(), string(loc))
	return result.String()
}
func (loc BodyLocator) String() string {
	return fmt.Sprintf(`body[%s]`, string(loc))
}

type CodeLocator struct{}

func (loc CodeLocator) locateValueInResp(resp resty.Response) string {
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

	// DefaultDelay specifies the interval between two pollings. The `Retry-After` in the response header takes higher precedence than this.
	DefaultDelay time.Duration
}

func NewPollable(resp resty.Response, opt PollOption) (*Pollable, error) {
	p := Pollable{}

	if opt.DefaultDelay == 0 {
		opt.DefaultDelay = 10 * time.Second
	}
	p.DefaultDelay = opt.DefaultDelay

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

	if loc := opt.UrlLocator; loc != nil {
		url := loc.locateValueInResp(resp)
		if url == "" {
			return nil, fmt.Errorf("No polling URL found in %s", loc)
		}
		p.URL = url
	} else {
		p.URL = resp.Request.URL
	}

	return &p, nil
}

type Pollable struct {
	InitDelay     time.Duration
	URL           string
	Status        PollingStatus
	StatusLocator ValueLocator
	DefaultDelay  time.Duration
}

func (f *Pollable) PollUntilDone(ctx context.Context, client *Client) error {
	time.Sleep(f.InitDelay)
PollingLoop:
	for {
		// There is no need to retry here as resty client has embedded retry logic (by default 3 max retries).
		req := client.R().SetContext(ctx)
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
