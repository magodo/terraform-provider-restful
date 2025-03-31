package provider

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
)

type ExpiryType int

const (
	ExpiryTypeTime ExpiryType = iota
	ExpiryTypeDuration
)

func GetExpiryTime(typ string, loc string, ahead string, resp resty.Response) (time.Time, error) {
	locator, err := expandValueLocator(loc)
	if err != nil {
		return time.Time{}, err
	}

	v, ok := locator.LocateValueInResp(resp)
	if !ok {
		return time.Time{}, fmt.Errorf("No expiry value found via %s", loc)
	}

	var aheadDur time.Duration
	if ahead != "" {
		aheadDur, err = time.ParseDuration(ahead)
		if err != nil {
			return time.Time{}, fmt.Errorf("failed to parse ahead duration: %v", err)
		}
	}

	l, r, ok := strings.Cut(typ, ".")
	switch l {
	case "time":
		layout := time.RFC3339
		if ok {
			layout = r
		}
		t, err := time.Parse(layout, v)
		if err != nil {
			return time.Time{}, err
		}
		return t.Add(-aheadDur), nil
	case "duration":
		if ok {
			return time.Time{}, fmt.Errorf("invalid format of expiry type")
		}
		dur, err := time.ParseDuration(v)
		if err != nil {
			return time.Time{}, err
		}
		return time.Now().Add(dur).Add(-aheadDur), nil
	case "duration_in_seconds":
		if ok {
			return time.Time{}, fmt.Errorf("invalid format of expiry type")
		}
		dur, err := time.ParseDuration(v + "s")
		if err != nil {
			return time.Time{}, err
		}
		return time.Now().Add(dur).Add(-aheadDur), nil
	default:
		return time.Time{}, fmt.Errorf("invalid format of expiry type")
	}
}

func validateExpiryType(v string) error {
	l, r, ok := strings.Cut(v, ".")
	switch l {
	case "time":
		if ok {
			if _, err := time.Parse(r, v); err != nil {
				return err
			}
		}
	case "duration":
		if ok {
			return fmt.Errorf("invalid format of expiry type")
		}
	case "duration_in_seconds":
		if ok {
			return fmt.Errorf("invalid format of expiry type")
		}
	default:
		return fmt.Errorf("invalid format of expiry type")
	}
	return nil
}
