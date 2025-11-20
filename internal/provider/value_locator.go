package provider

import (
	"fmt"
	"strings"

	"github.com/lfventura/terraform-provider-restful/internal/client"
	"github.com/lfventura/terraform-provider-restful/internal/exparam"
)

func validateLocator(locator string) error {
	if locator == "code" {
		return nil
	}
	l, r, ok := strings.Cut(locator, ".")
	if !ok {
		return fmt.Errorf("locator doesn't contain `.`: %s", locator)
	}
	if r == "" {
		return fmt.Errorf("empty right hand value for locator: %s", locator)
	}
	switch l {
	case "exact", "header", "body":
		return nil
	default:
		return fmt.Errorf("unknown locator key: %s", l)
	}
}

func expandValueLocatorWithParam(locator string, body []byte) (client.ValueLocator, error) {
	if locator == "code" {
		return client.CodeLocator{}, nil
	}
	l, r, ok := strings.Cut(locator, ".")
	if !ok {
		return nil, fmt.Errorf("locator doesn't contain `.`: %s", locator)
	}
	if r == "" {
		return nil, fmt.Errorf("empty right hand value for locator: %s", locator)
	}
	switch l {
	case "exact":
		return client.ExactLocator(r), nil
	case "header":
		rr, err := exparam.ExpandBody(r, body)
		if err != nil {
			return nil, fmt.Errorf("expand param of %q: %v", r, err)
		}
		return client.HeaderLocator(rr), nil
	case "body":
		rr, err := exparam.ExpandBody(r, body)
		if err != nil {
			return nil, fmt.Errorf("expand param of %q: %v", r, err)
		}
		return client.BodyLocator(rr), nil
	default:
		return nil, fmt.Errorf("unknown locator key: %s", l)
	}
}

func expandValueLocator(locator string) (client.ValueLocator, error) {
	l, r, ok := strings.Cut(locator, ".")
	if !ok {
		return nil, fmt.Errorf("locator doesn't contain `.`: %s", locator)
	}
	if r == "" {
		return nil, fmt.Errorf("empty right hand value for locator: %s", locator)
	}
	switch l {
	case "exact":
		return client.ExactLocator(r), nil
	case "header":
		return client.HeaderLocator(r), nil
	case "body":
		return client.BodyLocator(r), nil
	default:
		return nil, fmt.Errorf("unknown locator key: %s", l)
	}
}
