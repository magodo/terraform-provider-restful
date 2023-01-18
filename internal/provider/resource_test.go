package provider_test

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func CheckJSONEqual(attr, expect string) resource.CheckResourceAttrWithFunc {
	return func(actual string) error {
		equal, err := JSONEqual(actual, expect)
		if err != nil {
			return err
		}
		if !equal {
			return fmt.Errorf("Attribute %q expected %q, got %q", attr, expect, actual)
		}
		return nil
	}
}

func JSONEqual(x, y string) (bool, error) {
	var xm, ym map[string]interface{}
	if err := json.Unmarshal([]byte(x), &xm); err != nil {
		return false, err
	}
	if err := json.Unmarshal([]byte(y), &ym); err != nil {
		return false, err
	}
	return reflect.DeepEqual(xm, ym), nil
}
