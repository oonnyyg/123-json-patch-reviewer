package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

func main() {
	docPath := flag.String("doc", "", "输入 JSON 文档路径（必填）")
	patchPath := flag.String("patch", "", "JSON 补丁数组路径（必填）")
	outPath := flag.String("out", "", "落盘输出路径；缺省时仅预览，不写文件")
	reportPath := flag.String("report", "", "审查报告输出路径；缺省时打印到标准输出")
	flag.Parse()

	if *docPath == "" || *patchPath == "" {
		fmt.Fprintln(os.Stderr, "用法: json-patch-reviewer -doc <文档.json> -patch <补丁.json> [-out <输出.json>] [-report <报告.json>]")
		os.Exit(2)
	}
	if err := run(*docPath, *patchPath, *outPath, *reportPath); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(1)
	}
}

func run(docPath, patchPath, outPath, reportPath string) error {
	doc, err := readJSON(docPath)
	if err != nil {
		return fmt.Errorf("读取文档失败: %w", err)
	}
	var ops []Operation
	if err := decodeFile(patchPath, &ops); err != nil {
		return fmt.Errorf("读取补丁失败: %w", err)
	}

	report, err := ApplyPatch(doc, ops)
	if err != nil {
		// 失败：不写任何输出文件，原文档与已有输出文件保持原样
		return err
	}

	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if reportPath == "" {
		fmt.Println(string(reportJSON))
	} else if err := os.WriteFile(reportPath, reportJSON, 0o644); err != nil {
		return fmt.Errorf("写入报告失败: %w", err)
	}

	if outPath != "" {
		finalJSON, err := json.MarshalIndent(report.Final, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(outPath, finalJSON, 0o644); err != nil {
			return fmt.Errorf("写入输出失败: %w", err)
		}
	}
	return nil
}

func readJSON(path string) (any, error) {
	var v any
	if err := decodeFile(path, &v); err != nil {
		return nil, err
	}
	return v, nil
}

func decodeFile(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
