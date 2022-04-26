package restapi

import (
	"encoding/json"
	"fmt"

	"github.com/tidwall/sjson"
)

// ModifyBody modifies the body based on the current state, removing anyattribute
// attribute that only exists in the body, or is specified to be ignored.
func ModifyBody(state, body string, ignoreChanges []string) (string, error) {
	// This happens when importing resource, where there is no corresponding state.
	if state == "" {
		return body, nil
	}
	var stateJSON map[string]interface{}
	if err := json.Unmarshal([]byte(state), &stateJSON); err != nil {
		return "", fmt.Errorf("unmarshal the state %s: %v", state, err)
	}
	var bodyJSON map[string]interface{}
	if err := json.Unmarshal([]byte(body), &bodyJSON); err != nil {
		return "", fmt.Errorf("unmarshal the body %s: %v", body, err)
	}
	for _, path := range ignoreChanges {
		var err error
		body, err = sjson.Delete(body, path)
		if err != nil {
			return "", fmt.Errorf("deleting attribute in path %q: %v", path, err)
		}
	}
	b, err := json.Marshal(getUpdatedJSON(stateJSON, bodyJSON))
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
