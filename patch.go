package main

import "fmt"

// Operation 表示一条 RFC 6902 补丁操作。
type Operation struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	From  string `json:"from,omitempty"`
	Value any    `json:"value,omitempty"`
}

// StepReport 记录单步操作的结果与逆操作。
type StepReport struct {
	Index    int         `json:"index"`
	Op       string      `json:"op"`
	Path     string      `json:"path"`
	Document any         `json:"document"`
	Inverse  []Operation `json:"inverse"`
}

// Report 是整份补丁的审查报告。
type Report struct {
	Steps []StepReport `json:"steps"`
	Final any          `json:"final"`
}

// opError 包装错误并携带操作序号（从 1 开始）。
func opError(idx int, format string, args ...any) error {
	return fmt.Errorf("第 %d 个操作失败：%s", idx+1, fmt.Sprintf(format, args...))
}

// ApplyPatch 按数组顺序把补丁应用到 doc 的深拷贝上，
// 任一步失败返回错误，原始 doc 不被修改。
func ApplyPatch(doc any, ops []Operation) (*Report, error) {
	current := deepCopy(doc)
	report := &Report{Steps: make([]StepReport, 0, len(ops))}
	for i, op := range ops {
		inverse, err := applyOne(&current, op)
		if err != nil {
			return nil, opError(i, "%v", err)
		}
		report.Steps = append(report.Steps, StepReport{
			Index:    i,
			Op:       op.Op,
			Path:     op.Path,
			Document: deepCopy(current),
			Inverse:  inverse,
		})
	}
	report.Final = current
	return report, nil
}

// applyOne 应用单个操作并返回其逆操作序列。
func applyOne(doc *any, op Operation) ([]Operation, error) {
	switch op.Op {
	case "add", "replace", "remove", "move", "copy", "test":
	default:
		return nil, fmt.Errorf("未知操作 %q", op.Op)
	}
	tokens, err := parsePointer(op.Path)
	if err != nil {
		return nil, err
	}
	switch op.Op {
	case "test":
		actual, err := getValue(*doc, tokens)
		if err != nil {
			return nil, err
		}
		if !jsonEqual(actual, op.Value) {
			return nil, fmt.Errorf("test 断言不成立：路径 %q 的值与期望不符", op.Path)
		}
		return nil, nil
	case "remove":
		if len(tokens) == 0 {
			return nil, fmt.Errorf("不允许删除根值")
		}
		old, err := removeValue(doc, tokens)
		if err != nil {
			return nil, err
		}
		return []Operation{{Op: "add", Path: op.Path, Value: old}}, nil
	case "add", "replace":
		return applyAddReplace(doc, op, tokens)
	case "move", "copy":
		if op.From == "" && op.Op == "move" {
			return nil, fmt.Errorf("%s 操作缺少 from 字段", op.Op)
		}
		fromTokens, err := parsePointer(op.From)
		if err != nil {
			return nil, err
		}
		if op.Op == "move" {
			if len(fromTokens) == 0 {
				return nil, fmt.Errorf("不允许移动根值")
			}
			if isPrefix(fromTokens, tokens) && len(tokens) > len(fromTokens) {
				return nil, fmt.Errorf("不允许把值移动到自身后代路径 %q", op.Path)
			}
			old, err := removeValue(doc, fromTokens)
			if err != nil {
				return nil, err
			}
			if _, _, err := addValue(doc, tokens, old, true); err != nil {
				return nil, err
			}
			return []Operation{
				{Op: "remove", Path: op.Path},
				{Op: "add", Path: op.From, Value: old},
			}, nil
		}
		src, err := getValue(*doc, fromTokens)
		if err != nil {
			return nil, err
		}
		replaced, hadOld, err := addValue(doc, tokens, deepCopy(src), true)
		if err != nil {
			return nil, err
		}
		return inverseForAdd(op.Path, replaced, hadOld), nil
	}
	return nil, fmt.Errorf("未知操作 %q", op.Op)
}

// applyAddReplace 处理 add 与 replace，返回逆操作。
func applyAddReplace(doc *any, op Operation, tokens []pointerToken) ([]Operation, error) {
	if len(tokens) == 0 {
		// 替换整个根值
		old := *doc
		*doc = deepCopy(op.Value)
		inv := "replace"
		return []Operation{{Op: inv, Path: "", Value: old}}, nil
	}
	if op.Op == "replace" {
		old, err := replaceValue(doc, tokens, deepCopy(op.Value))
		if err != nil {
			return nil, err
		}
		return []Operation{{Op: "replace", Path: op.Path, Value: old}}, nil
	}
	replaced, hadOld, err := addValue(doc, tokens, deepCopy(op.Value), true)
	if err != nil {
		return nil, err
	}
	return inverseForAdd(op.Path, replaced, hadOld), nil
}

// inverseForAdd 生成 add/copy 的逆操作：
// 若覆盖了已有对象键则恢复旧值，否则移除新增内容。
func inverseForAdd(path string, replaced any, hadOld bool) []Operation {
	if hadOld {
		return []Operation{{Op: "replace", Path: path, Value: replaced}}
	}
	return []Operation{{Op: "remove", Path: path}}
}

// isPrefix 判断 a 是否为 b 的前缀。
func isPrefix(a, b []pointerToken) bool {
	if len(a) > len(b) {
		return false
	}
	for i := range a {
		if a[i].raw != b[i].raw {
			return false
		}
	}
	return true
}
