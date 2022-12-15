package provider

import (
	"encoding/json"
	"fmt"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ModifyBody modifies the body based on the base body, only keeps attributes that exist on both sides.
// If compensateBaseAttrs is set, then any attribute path element only found in the base body will
// be added up to the result body.
func ModifyBody(base, body string, compensateBaseAttrs []string) (string, error) {
	var baseJSON map[string]interface{}
	if err := json.Unmarshal([]byte(base), &baseJSON); err != nil {
		return "", fmt.Errorf("unmarshal the base %q: %v", base, err)
	}

	var bodyJSON map[string]interface{}
	if err := json.Unmarshal([]byte(body), &bodyJSON); err != nil {
		return "", fmt.Errorf("unmarshal the body %q: %v", body, err)
	}

	b, err := json.Marshal(getUpdatedJSON(baseJSON, bodyJSON))
	if err != nil {
		return "", err
	}
	result := string(b)

	for _, path := range compensateBaseAttrs {
		if gjson.Get(base, path).Exists() && !gjson.Get(body, path).Exists() {
			var err error
			result, err = sjson.Set(result, path, gjson.Get(base, path).Value())
			if err != nil {
				return "", err
			}
		}
	}

	// Remarshal to keep order.
	var m interface{}
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		return "", err
	}
	b, err = json.Marshal(m)
	return string(b), err
}

func getUpdatedJSON(oldJSON, newJSON interface{}) interface{} {
	switch oldJSON := oldJSON.(type) {
	case map[string]interface{}:
		if newJSON, ok := newJSON.(map[string]interface{}); ok {
			out := map[string]interface{}{}
			for k, ov := range oldJSON {
				if nv, ok := newJSON[k]; ok {
					out[k] = getUpdatedJSON(ov, nv)
				}
			}
			return out
		}
	case []interface{}:
		if newJSON, ok := newJSON.([]interface{}); ok {
			if len(newJSON) != len(oldJSON) {
				return newJSON
			}
			out := make([]interface{}, len(newJSON))
			for i := range newJSON {
				out[i] = getUpdatedJSON(oldJSON[i], newJSON[i])
			}
			return out
		}
	}
	return newJSON
}

// ModifyBodyForImport is similar as ModifyBody, but is based on the body from import spec, rather than from state.
func ModifyBodyForImport(base, body string) (string, error) {
	// This happens when importing resource without specifying the "body", where there is no state for "body".
	if base == "" {
		return body, nil
	}
	var baseJSON map[string]interface{}
	if err := json.Unmarshal([]byte(base), &baseJSON); err != nil {
		return "", fmt.Errorf("unmarshal the base %q: %v", base, err)
	}
	var bodyJSON map[string]interface{}
	if err := json.Unmarshal([]byte(body), &bodyJSON); err != nil {
		return "", fmt.Errorf("unmarshal the body %q: %v", body, err)
	}
	updatedBody, err := getUpdatedJSONForImport(baseJSON, bodyJSON)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(updatedBody)
	return string(b), err
}

func getUpdatedJSONForImport(oldJSON, newJSON interface{}) (interface{}, error) {
	switch oldJSON := oldJSON.(type) {
	case map[string]interface{}:
		if newJSON, ok := newJSON.(map[string]interface{}); ok {
			out := map[string]interface{}{}
			for k, ov := range oldJSON {
				if nv, ok := newJSON[k]; ok {
					var err error
					out[k], err = getUpdatedJSONForImport(ov, nv)
					if err != nil {
						return nil, fmt.Errorf("failed to update json for key %q: %v", k, err)
					}
				}
			}
			return out, nil
		}
	case []interface{}:
		if newJSON, ok := newJSON.([]interface{}); ok {
			switch len(oldJSON) {
			case 0:
				// The same as setting to null, just return the newJSON.
				return newJSON, nil
			case 1:
				out := make([]interface{}, len(newJSON))
				for i := range newJSON {
					var err error
					out[i], err = getUpdatedJSONForImport(oldJSON[0], newJSON[i])
					if err != nil {
						return nil, fmt.Errorf("failed to update json for the %dth element: %v", i, err)
					}
				}
				return out, nil
			default:
				return newJSON, fmt.Errorf("the length of array should be 1")
			}
		}
	}
	return newJSON, nil
}

// DiffBody diffs the body against a base body. The result is a
func DiffBody(base, body string, compensateBaseAttrs []string) (string, error) {
	var baseJSON map[string]interface{}
	if err := json.Unmarshal([]byte(base), &baseJSON); err != nil {
		return "", fmt.Errorf("unmarshal the base %q: %v", base, err)
	}

	var bodyJSON map[string]interface{}
	if err := json.Unmarshal([]byte(body), &bodyJSON); err != nil {
		return "", fmt.Errorf("unmarshal the body %q: %v", body, err)
	}

	b, err := json.Marshal(getUpdatedJSON(baseJSON, bodyJSON))
	if err != nil {
		return "", err
	}
	result := string(b)

	for _, path := range compensateBaseAttrs {
		if gjson.Get(base, path).Exists() && !gjson.Get(body, path).Exists() {
			var err error
			result, err = sjson.Set(result, path, gjson.Get(base, path).Value())
			if err != nil {
				return "", err
			}
		}
	}

	// Remarshal to keep order.
	var m interface{}
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		return "", err
	}
	b, err = json.Marshal(m)
	return string(b), err
}
