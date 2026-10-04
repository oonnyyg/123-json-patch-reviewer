package main

import (
	"encoding/json"
	"strings"
	"testing"
)

func mustParse(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatalf("解析 JSON 失败: %v", err)
	}
	return v
}

func applyOK(t *testing.T, doc string, ops ...Operation) any {
	t.Helper()
	report, err := ApplyPatch(mustParse(t, doc), ops)
	if err != nil {
		t.Fatalf("应用补丁失败: %v", err)
	}
	return report.Final
}

func assertFinal(t *testing.T, got any, want string) {
	t.Helper()
	if !jsonEqual(got, mustParse(t, want)) {
		g, _ := json.Marshal(got)
		t.Fatalf("结果不符: got %s, want %s", g, want)
	}
}

func TestEscapedKeys(t *testing.T) {
	doc := `{"a/b": 1, "m~n": 2}`
	got := applyOK(t, doc,
		Operation{Op: "replace", Path: "/a~1b", Value: float64(10)},
		Operation{Op: "add", Path: "/m~0n", Value: float64(20)},
	)
	assertFinal(t, got, `{"a/b": 10, "m~n": 20}`)
}

func TestArrayInsertShiftsAndRemoveReindexes(t *testing.T) {
	got := applyOK(t, `{"list": ["a", "b", "c"]}`,
		Operation{Op: "add", Path: "/list/1", Value: "x"},
		Operation{Op: "remove", Path: "/list/2"},
	)
	assertFinal(t, got, `{"list": ["a", "x", "c"]}`)
}

func TestArrayMove(t *testing.T) {
	got := applyOK(t, `{"list": ["a", "b", "c", "d"]}`,
		Operation{Op: "move", From: "/list/0", Path: "/list/2"},
	)
	assertFinal(t, got, `{"list": ["b", "c", "a", "d"]}`)
}

func TestMoveIntoOwnDescendantRejected(t *testing.T) {
	_, err := ApplyPatch(mustParse(t, `{"a": {"b": 1}}`),
		[]Operation{{Op: "move", From: "/a", Path: "/a/b/c"}})
	if err == nil || !strings.Contains(err.Error(), "第 1 个操作") {
		t.Fatalf("应拒绝移入自身后代, err=%v", err)
	}
}

func TestCopyDoesNotShareState(t *testing.T) {
	doc := mustParse(t, `{"src": {"n": 1}}`)
	report, err := ApplyPatch(doc, []Operation{
		{Op: "copy", From: "/src", Path: "/dst"},
		{Op: "replace", Path: "/dst/n", Value: float64(99)},
	})
	if err != nil {
		t.Fatalf("应用补丁失败: %v", err)
	}
	final := report.Final.(map[string]any)
	if final["src"].(map[string]any)["n"] != float64(1) {
		t.Fatalf("copy 的子树与来源共享了状态")
	}
}
