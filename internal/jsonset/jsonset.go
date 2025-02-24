package jsonset

import (
	"encoding/json"
	"fmt"
	"maps"
)

// Disjointed tells whether two valid json values are disjointed.
// They are disjointed in one of the following cases:
//   - Both are objects: Having different sets of keys, or the values of the common key are disjointed.
//   - Otherwise, the two json values are regarded jointed, including both values have different types, or
//     different values.
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

// Difference removes the subset rhs from the lhs.
// If both are objects, remove the subset of the same keyed value, recursively, until reach to
// a non-object value for either lhs or rhs (regardless of the values), that key will be removed
// from lhs. If this is the last key in lhs, it will be removed one level upwards.
func Difference(lhs, rhs []byte) ([]byte, error) {
	var lv, rv interface{}
	if err := json.Unmarshal(lhs, &lv); err != nil {
		return nil, fmt.Errorf("JSON unmarshal lhs: %v", err)
	}
	if err := json.Unmarshal(rhs, &rv); err != nil {
		return nil, fmt.Errorf("JSON unmarshal rhs: %v", err)
	}
	v, _ := diffValue(lv, rv)
	return json.Marshal(v)
}

func diffValue(lv, rv interface{}) (interface{}, bool) {
	switch lv := lv.(type) {
	case map[string]interface{}:
		if rv, ok := rv.(map[string]interface{}); ok {
			return diffMap(lv, rv)
		}
		return lv, true
	default:
		return lv, true
	}
}

func diffMap(lm, rm map[string]interface{}) (map[string]interface{}, bool) {
	var diff bool
	for k := range maps.Keys(lm) {
		rv, ok := rm[k]
		if !ok {
			continue
		}
		lv := lm[k]
		v, changed := diffValue(lv, rv)
		if changed {
			mv, ok := v.(map[string]interface{})
			if !ok || len(mv) == 0 {
				// Remove the key in either of the following cases:
				// - Non map value diff
				// - Empty map after diff
				diff = true
				delete(lm, k)
			}
		}
	}
	return lm, diff
}
