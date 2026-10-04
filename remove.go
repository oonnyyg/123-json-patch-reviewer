package patch

import "fmt"

// remove deletes the value at ptr and returns it along with the new root.
// Removing the document root is rejected.
func remove(doc interface{}, ptr Pointer) (interface{}, interface{}, error) {
	if len(ptr) == 0 {
		return nil, nil, fmt.Errorf("removing the document root is not allowed")
	}
	parent, last, err := parentOf(doc, ptr)
	if err != nil {
		return nil, nil, err
	}
	switch p := parent.(type) {
	case map[string]interface{}:
		v, ok := p[last]
		if !ok {
			return nil, nil, fmt.Errorf("object has no key %q", last)
		}
		delete(p, last)
		return doc, v, nil
	case []interface{}:
		i, err := arrayIndex(last, len(p), false)
		if err != nil {
			return nil, nil, err
		}
		v := p[i]
		shrunk := append(p[:i], p[i+1:]...)
		if len(ptr) == 1 {
			return shrunk, v, nil
		}
		parentOfArrayFix(doc, ptr, shrunk)
		return doc, v, nil
	default:
		return nil, nil, fmt.Errorf("cannot remove from %T", parent)
	}
}
