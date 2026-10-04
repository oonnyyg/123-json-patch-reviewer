package main

// deepCopy 返回值的完全独立副本，复制的子树不与来源共享可变状态。
func deepCopy(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, val := range t {
			m[k] = deepCopy(val)
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i, val := range t {
			s[i] = deepCopy(val)
		}
		return s
	default:
		return v
	}
}

// jsonEqual 按 JSON 结构比较：对象键顺序无关，数组顺序相关，
// 数值按数学值比较，布尔与数值不相等。
func jsonEqual(a, b any) bool {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for k, va := range av {
			vb, ok := bv[k]
			if !ok || !jsonEqual(va, vb) {
				return false
			}
		}
		return true
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !jsonEqual(av[i], bv[i]) {
				return false
			}
		}
		return true
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	default:
		// nil、string、bool 直接比较，类型不同即不相等
		return a == b
	}
}
