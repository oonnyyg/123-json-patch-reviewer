package main

import (
	"fmt"
	"strconv"
	"strings"
)

// pointerToken 是 JSON Pointer 路径中的一个片段。
type pointerToken struct {
	raw     string // 原始（未反转义）文本
	key     string // 反转义后的键名
	isDash  bool   // 是否为 "-"（数组追加位置）
	isIndex bool   // 是否为数组下标
	index   int
}

// parsePointer 按 RFC 6901 解析 JSON Pointer。
// 空字符串表示整个根值；否则必须以 "/" 开头。
func parsePointer(s string) ([]pointerToken, error) {
	if s == "" {
		return nil, nil
	}
	if !strings.HasPrefix(s, "/") {
		return nil, fmt.Errorf("指针 %q 无效：必须以 / 开头或为空", s)
	}
	parts := strings.Split(s[1:], "/")
	tokens := make([]pointerToken, 0, len(parts))
	for _, p := range parts {
		tok := pointerToken{raw: p, key: unescape(p)}
		if p == "-" {
			tok.isDash = true
		} else if idx, ok := parseIndex(p); ok {
			tok.isIndex = true
			tok.index = idx
		}
		tokens = append(tokens, tok)
	}
	return tokens, nil
}

// unescape 处理 ~0 -> ~ 与 ~1 -> /，其它 ~ 序列按原样保留。
func unescape(s string) string {
	if !strings.Contains(s, "~") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '~' {
			if i+1 >= len(s) || (s[i+1] != '0' && s[i+1] != '1') {
				b.WriteByte(s[i])
				continue
			}
			if s[i+1] == '0' {
				b.WriteByte('~')
			} else {
				b.WriteByte('/')
			}
			i++
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// parseIndex 校验数组下标：纯数字、无前导零（"0" 本身允许）。
func parseIndex(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	if len(s) > 1 && s[0] == '0' {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}
