package jsonset

import (
	"encoding/json"
	"fmt"
)

// Disjointed tells whether two json values are disjointed.
// They are disjointed in one of the following cases:
// - Both are objects: Having different sets of keys, or the values of the common key are disjointed.
// - Both are arrays: The same indexed elements are disjointed.
// Otherwise, the two json values are regarded jointed, including both values have different types, or
// different values.
func Disjointed(lhs, rhs []byte) (bool, error) {
	var lv, rv interface{}
	if err := json.Unmarshal(lhs, &lv); err != nil {
		return false, fmt.Errorf("JSON unmarshal lhs: %v", err)
	}
	if err := json.Unmarshal(rhs, &rv); err != nil {
		return false, fmt.Errorf("JSON unmarshal rhs: %v", err)
	}
	return disjointValue(lv, rv), nil
}

func disjointValue(lv, rv interface{}) bool {
	switch lv := lv.(type) {
	case map[string]interface{}:
		rv, ok := rv.(map[string]interface{})
		if !ok {
			return false
		}
		return disjointedMap(lv, rv)
	case []interface{}:
		rv, ok := rv.([]interface{})
		if !ok {
			return false
		}
		return disjointedArray(lv, rv)
	default:
		return false
	}
}

func disjointedMap(lm, rm map[string]interface{}) bool {
	for lk, lv := range lm {
		if rv, ok := rm[lk]; ok {
			if !disjointValue(lv, rv) {
				return false
			}
		}
	}
	return true
}

func disjointedArray(la, ra []interface{}) bool {
	for i, lv := range la {
		if i >= len(ra) {
			return true
		}
		rv := ra[i]
		if !disjointValue(lv, rv) {
			return false
		}
	}
	return true
}
