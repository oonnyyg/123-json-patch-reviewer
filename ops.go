package patch

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// deepCopy returns a copy of v that shares no mutable state with the source.
func deepCopy(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		m := make(map[string]interface{}, len(t))
		for k, val := range t {
			m[k] = deepCopy(val)
		}
		return m
	case []interface{}:
		s := make([]interface{}, len(t))
		for i, val := range t {
			s[i] = deepCopy(val)
		}
		return s
	default:
		return v
	}
}

// jsonEqual compares two JSON values structurally. Numbers (json.Number)
// compare by mathematical value; booleans never equal numbers.
func jsonEqual(a, b interface{}) bool {
	an, aIsNum := a.(json.Number)
	bn, bIsNum := b.(json.Number)
	if aIsNum || bIsNum {
		if !aIsNum || !bIsNum {
			return false
		}
		af, aerr := an.Float64()
		bf, berr := bn.Float64()
		if aerr == nil && berr == nil {
			return af == bf
		}
		return an.String() == bn.String()
	}
	switch av := a.(type) {
	case map[string]interface{}:
		bv, ok := b.(map[string]interface{})
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, val := range av {
			bval, ok := bv[k]
			if !ok || !jsonEqual(val, bval) {
				return false
			}
		}
		return true
	case []interface{}:
		bv, ok := b.([]interface{})
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !jsonEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(a, b)
	}
}

// get returns the value at ptr within doc.
func get(doc interface{}, ptr Pointer) (interface{}, error) {
	cur := doc
	for _, seg := range ptr {
		switch node := cur.(type) {
		case map[string]interface{}:
			v, ok := node[seg]
			if !ok {
				return nil, fmt.Errorf("object has no key %q", seg)
			}
			cur = v
		case []interface{}:
			i, err := arrayIndex(seg, len(node), false)
			if err != nil {
				return nil, err
			}
			cur = node[i]
		default:
			return nil, fmt.Errorf("cannot descend into %T at segment %q", cur, seg)
		}
	}
	return cur, nil
}

// parentOf resolves all but the last segment and returns the container.
func parentOf(doc interface{}, ptr Pointer) (interface{}, string, error) {
	if len(ptr) == 0 {
		return nil, "", fmt.Errorf("pointer refers to the document root")
	}
	parent, err := get(doc, ptr[:len(ptr)-1])
	if err != nil {
		return nil, "", fmt.Errorf("missing parent: %v", err)
	}
	return parent, ptr[len(ptr)-1], nil
}

// add inserts or sets value at ptr, returning the new document root.
func add(doc interface{}, ptr Pointer, value interface{}) (interface{}, error) {
	if len(ptr) == 0 {
		return value, nil
	}
	parent, last, err := parentOf(doc, ptr)
	if err != nil {
		return nil, err
	}
	switch p := parent.(type) {
	case map[string]interface{}:
		p[last] = value
	case []interface{}:
		i, err := arrayIndex(last, len(p), true)
		if err != nil {
			return nil, err
		}
		p = append(p, nil)
		copy(p[i+1:], p[i:])
		p[i] = value
		if len(ptr) == 1 {
			return p, nil // root array may have been reallocated
		}
		parentOfArrayFix(doc, ptr, p)
	default:
		return nil, fmt.Errorf("cannot add into %T", parent)
	}
	return doc, nil
}

// parentOfArrayFix writes a grown slice back into its container, since
// append may reallocate. The root case is handled by add returning the root.
func parentOfArrayFix(doc interface{}, ptr Pointer, grown []interface{}) {
	if len(ptr) == 1 {
		return // root array: caller must use the returned root
	}
	grand, seg, err := parentOf(doc, ptr[:len(ptr)-1])
	if err != nil {
		return
	}
	switch g := grand.(type) {
	case map[string]interface{}:
		g[seg] = grown
	case []interface{}:
		if i, err := arrayIndex(seg, len(g), false); err == nil {
			g[i] = grown
		}
	}
}
