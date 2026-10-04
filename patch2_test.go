package main

import (
	"strings"
	"testing"
)

func TestMidPatchTestFailureIsAtomic(t *testing.T) {
	doc := mustParse(t, `{"n": 1, "s": "keep"}`)
	_, err := ApplyPatch(doc, []Operation{
		{Op: "replace", Path: "/n", Value: float64(2)},
		{Op: "test", Path: "/n", Value: float64(3)},
		{Op: "replace", Path: "/s", Value: "changed"},
	})
	if err == nil || !strings.Contains(err.Error(), "第 2 个操作") {
		t.Fatalf("应报第 2 个操作失败, err=%v", err)
	}
	// 原文档不被修改
	m := doc.(map[string]any)
	if m["n"] != float64(1) || m["s"] != "keep" {
		t.Fatalf("失败后原文档被修改: %v", m)
	}
}

func TestUnknownOpReportsIndex(t *testing.T) {
	_, err := ApplyPatch(mustParse(t, `{}`), []Operation{
		{Op: "add", Path: "/a", Value: float64(1)},
		{Op: "frobnicate", Path: "/a"},
	})
	if err == nil || !strings.Contains(err.Error(), "第 2 个操作") {
		t.Fatalf("未知操作应带序号, err=%v", err)
	}
}

func TestIndexValidation(t *testing.T) {
	cases := []Operation{
		{Op: "add", Path: "/list/01", Value: "x"},   // 前导零
		{Op: "add", Path: "/list/5", Value: "x"},    // 越界
		{Op: "remove", Path: "/list/-"},             // 删除不能用 -
		{Op: "add", Path: "/missing/key", Value: 1}, // 父节点缺失
	}
	for _, op := range cases {
		_, err := ApplyPatch(mustParse(t, `{"list": ["a"]}`), []Operation{op})
		if err == nil {
			t.Fatalf("操作 %+v 应失败", op)
		}
	}
}

func TestDashAppends(t *testing.T) {
	got := applyOK(t, `{"list": ["a"]}`,
		Operation{Op: "add", Path: "/list/-", Value: "b"})
	assertFinal(t, got, `{"list": ["a", "b"]}`)
}

func TestRootReplaceAllowedRootRemoveRejected(t *testing.T) {
	got := applyOK(t, `{"a": 1}`,
		Operation{Op: "replace", Path: "", Value: mustParse(t, `[1,2]`)})
	assertFinal(t, got, `[1, 2]`)

	_, err := ApplyPatch(mustParse(t, `{"a": 1}`),
		[]Operation{{Op: "remove", Path: ""}})
	if err == nil {
		t.Fatalf("删除根值应被拒绝")
	}
}

func TestTestOpSemantics(t *testing.T) {
	doc := `{"num": 1, "flag": true, "obj": {"x": 1, "y": 2}, "arr": [1, 2]}`
	ok := []Operation{
		{Op: "test", Path: "/num", Value: mustParse(t, `1.0`)}, // 数学值相等
		{Op: "test", Path: "/obj", Value: mustParse(t, `{"y": 2, "x": 1}`)}, // 键序无关
		{Op: "test", Path: "/arr", Value: mustParse(t, `[1, 2]`)},
	}
	if _, err := ApplyPatch(mustParse(t, doc), ok); err != nil {
		t.Fatalf("这些 test 应通过: %v", err)
	}
	bad := []Operation{
		{Op: "test", Path: "/flag", Value: mustParse(t, `1`)},   // 布尔 != 数值
		{Op: "test", Path: "/arr", Value: mustParse(t, `[2, 1]`)}, // 数组顺序相关
	}
	for i, op := range bad {
		if _, err := ApplyPatch(mustParse(t, doc), []Operation{op}); err == nil {
			t.Fatalf("bad[%d] 应失败", i)
		}
	}
}
