package main

import "fmt"

// getValue 按路径取值，路径为空时返回根值。
func getValue(doc any, tokens []pointerToken) (any, error) {
	cur := doc
	for _, tok := range tokens {
		next, err := descend(cur, tok)
		if err != nil {
			return nil, err
		}
		cur = next
	}
	return cur, nil
}

// descend 进入下一层容器。
func descend(cur any, tok pointerToken) (any, error) {
	switch c := cur.(type) {
	case map[string]any:
		if tok.isDash {
			return nil, fmt.Errorf("对象中不允许使用 -")
		}
		v, ok := c[tok.key]
		if !ok {
			return nil, fmt.Errorf("对象缺少键 %q", tok.key)
		}
		return v, nil
	case []any:
		if !tok.isIndex {
			return nil, fmt.Errorf("数组下标 %q 无效", tok.raw)
		}
		if tok.index >= len(c) {
			return nil, fmt.Errorf("数组下标 %d 越界（长度 %d）", tok.index, len(c))
		}
		return c[tok.index], nil
	default:
		return nil, fmt.Errorf("路径经过非容器值（键 %q）", tok.key)
	}
}

// parentOf 定位到最后一个 token 的父容器，缺失的父节点不会自动创建。
func parentOf(doc any, tokens []pointerToken) (any, pointerToken, error) {
	last := tokens[len(tokens)-1]
	parent, err := getValue(doc, tokens[:len(tokens)-1])
	if err != nil {
		return nil, last, err
	}
	return parent, last, nil
}

// addValue 在路径处插入/设置值。
// 返回被覆盖的旧值（仅对象键已存在时）及是否覆盖了旧值。
func addValue(doc *any, tokens []pointerToken, value any, allowOverwrite bool) (any, bool, error) {
	parent, last, err := parentOf(*doc, tokens)
	if err != nil {
		return nil, false, err
	}
	switch p := parent.(type) {
	case map[string]any:
		if last.isDash {
			return nil, false, fmt.Errorf("对象中不允许使用 -")
		}
		old, existed := p[last.key]
		if existed && !allowOverwrite {
			return nil, false, fmt.Errorf("键 %q 已存在", last.key)
		}
		p[last.key] = value
		return old, existed, nil
	case []any:
		idx := len(p)
		if last.isDash {
			// 追加到末尾
		} else if last.isIndex {
			if last.index > len(p) {
				return nil, false, fmt.Errorf("数组下标 %d 越界（长度 %d）", last.index, len(p))
			}
			idx = last.index
		} else {
			return nil, false, fmt.Errorf("数组下标 %q 无效", last.raw)
		}
		// 插入并后移后续元素
		p = append(p, nil)
		copy(p[idx+1:], p[idx:])
		p[idx] = value
		setChild(doc, tokens[:len(tokens)-1], p)
		return nil, false, nil
	default:
		return nil, false, fmt.Errorf("路径 %q 的父节点不是容器", last.key)
	}
}

// replaceValue 替换已存在的值，返回旧值。
func replaceValue(doc *any, tokens []pointerToken, value any) (any, error) {
	parent, last, err := parentOf(*doc, tokens)
	if err != nil {
		return nil, err
	}
	switch p := parent.(type) {
	case map[string]any:
		old, ok := p[last.key]
		if !ok {
			return nil, fmt.Errorf("对象缺少键 %q", last.key)
		}
		p[last.key] = value
		return old, nil
	case []any:
		if !last.isIndex || last.index >= len(p) {
			return nil, fmt.Errorf("数组下标 %q 无效或越界（长度 %d）", last.raw, len(p))
		}
		old := p[last.index]
		p[last.index] = value
		return old, nil
	default:
		return nil, fmt.Errorf("路径 %q 的父节点不是容器", last.key)
	}
}

// removeValue 删除路径处的值并返回旧值。
func removeValue(doc *any, tokens []pointerToken) (any, error) {
	parent, last, err := parentOf(*doc, tokens)
	if err != nil {
		return nil, err
	}
	switch p := parent.(type) {
	case map[string]any:
		old, ok := p[last.key]
		if !ok {
			return nil, fmt.Errorf("对象缺少键 %q", last.key)
		}
		delete(p, last.key)
		return old, nil
	case []any:
		if !last.isIndex || last.index >= len(p) {
			return nil, fmt.Errorf("数组下标 %q 无效或越界（长度 %d）", last.raw, len(p))
		}
		old := p[last.index]
		p = append(p[:last.index], p[last.index+1:]...)
		setChild(doc, tokens[:len(tokens)-1], p)
		return old, nil
	default:
		return nil, fmt.Errorf("路径 %q 的父节点不是容器", last.key)
	}
}

// setChild 数组长度变化后回写父级；路径为空时替换根。
func setChild(doc *any, tokens []pointerToken, value any) {
	if len(tokens) == 0 {
		*doc = value
		return
	}
	parent, last, err := parentOf(*doc, tokens)
	if err != nil {
		return
	}
	switch p := parent.(type) {
	case map[string]any:
		p[last.key] = value
	case []any:
		if last.isIndex && last.index < len(p) {
			p[last.index] = value
		}
	}
}
