package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// 逆操作反序执行必须恢复原文及数组顺序。
func TestInverseOpsRestoreOriginal(t *testing.T) {
	doc := mustParse(t, `{"list": ["a", "b", "c"], "obj": {"k": 1}}`)
	ops := []Operation{
		{Op: "add", Path: "/list/1", Value: "x"},
		{Op: "move", From: "/list/0", Path: "/list/3"},
		{Op: "replace", Path: "/obj/k", Value: float64(2)},
		{Op: "copy", From: "/obj", Path: "/obj2"},
		{Op: "remove", Path: "/obj2"},
	}
	report, err := ApplyPatch(doc, ops)
	if err != nil {
		t.Fatalf("应用补丁失败: %v", err)
	}
	restored := report.Final
	for i := len(report.Steps) - 1; i >= 0; i-- {
		r, err := ApplyPatch(restored, report.Steps[i].Inverse)
		if err != nil {
			t.Fatalf("第 %d 步逆操作失败: %v", i, err)
		}
		restored = r.Final
	}
	if !jsonEqual(restored, doc) {
		got, _ := json.Marshal(restored)
		t.Fatalf("逆操作未恢复原文: %s", got)
	}
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// 预览与落盘必须得出相同结果。
func TestPreviewAndWriteConsistency(t *testing.T) {
	docPath := writeTemp(t, "doc.json", `{"a": [1, 2]}`)
	patchPath := writeTemp(t, "patch.json", `[
		{"op": "add", "path": "/a/-", "value": 3},
		{"op": "replace", "path": "/a/0", "value": 9}
	]`)
	outPath := filepath.Join(t.TempDir(), "out.json")
	reportPath := filepath.Join(t.TempDir(), "report.json")

	if err := run(docPath, patchPath, outPath, reportPath); err != nil {
		t.Fatalf("落盘模式失败: %v", err)
	}
	out, err := readJSON(outPath)
	if err != nil {
		t.Fatal(err)
	}
	assertFinal(t, out, `{"a": [9, 2, 3]}`)

	// 预览模式（无 -out）得出的最终文档应与落盘一致
	doc, _ := readJSON(docPath)
	var ops []Operation
	if err := decodeFile(patchPath, &ops); err != nil {
		t.Fatal(err)
	}
	report, err := ApplyPatch(doc, ops)
	if err != nil {
		t.Fatal(err)
	}
	if !jsonEqual(report.Final, out) {
		t.Fatalf("预览与落盘结果不一致")
	}
}

// 失败时已有输出文件不得被覆盖。
func TestFailureKeepsExistingOutput(t *testing.T) {
	docPath := writeTemp(t, "doc.json", `{"n": 1}`)
	patchPath := writeTemp(t, "patch.json", `[
		{"op": "replace", "path": "/n", "value": 2},
		{"op": "test", "path": "/n", "value": 999}
	]`)
	outPath := writeTemp(t, "out.json", `{"untouched": true}`)

	if err := run(docPath, patchPath, outPath, ""); err == nil {
		t.Fatalf("应返回失败")
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"untouched": true}` {
		t.Fatalf("失败后输出文件被覆盖: %s", data)
	}
}
