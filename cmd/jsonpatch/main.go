// Command jsonpatch previews or applies an RFC 6902 JSON Patch to a document.
//
// Usage:
//
//	jsonpatch -doc doc.json -patch patch.json [-out result.json] [-report report.json]
//
// Without -out it runs in preview mode: the review report (per-step changes,
// final document, inverse operations) is printed to stdout and nothing is
// written. With -out the patched document is written only if every operation
// succeeds; on failure the original document and any existing output file are
// left untouched.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	patch "example.com/json-patch-reviewer"
)

func main() {
	docPath := flag.String("doc", "", "path to the JSON document")
	patchPath := flag.String("patch", "", "path to the JSON patch array")
	outPath := flag.String("out", "", "output file; omit for preview-only mode")
	reportPath := flag.String("report", "", "optional file for the JSON review report")
	flag.Parse()
	if *docPath == "" || *patchPath == "" {
		fmt.Fprintln(os.Stderr, "usage: jsonpatch -doc doc.json -patch patch.json [-out result.json] [-report report.json]")
		os.Exit(2)
	}
	docJSON, err := os.ReadFile(*docPath)
	if err != nil {
		fatal(err)
	}
	patchJSON, err := os.ReadFile(*patchPath)
	if err != nil {
		fatal(err)
	}
	rep, err := patch.Apply(docJSON, patchJSON)
	if err != nil {
		fatal(err)
	}
	reportJSON, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		fatal(err)
	}
	if *reportPath != "" {
		if err := os.WriteFile(*reportPath, reportJSON, 0o644); err != nil {
			fatal(err)
		}
	}
	if !rep.OK {
		fmt.Fprintf(os.Stderr, "patch rejected at op %d: %s\n", rep.OpIndex, rep.Error)
		if *outPath == "" {
			fmt.Println(string(reportJSON))
		}
		os.Exit(1)
	}
	if *outPath == "" {
		fmt.Println(string(reportJSON))
		return
	}
	pretty, err := json.MarshalIndent(json.RawMessage(rep.Final), "", "  ")
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(*outPath, pretty, 0o644); err != nil {
		fatal(err)
	}
	fmt.Printf("patched document written to %s\n", *outPath)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "jsonpatch:", err)
	os.Exit(2)
}
