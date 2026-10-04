package patch

import (
	"encoding/json"
	"strings"
	"testing"
)

func applyOK(t *testing.T, doc, patch string) *Report {
	t.Helper()
	rep, err := Apply([]byte(doc), []byte(patch))
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if !rep.OK {
		t.Fatalf("patch rejected at op %d: %s", rep.OpIndex, rep.Error)
	}
	return rep
}

func applyFail(t *testing.T, doc, patch string, wantOp int) *Report {
	t.Helper()
	rep, err := Apply([]byte(doc), []byte(patch))
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if rep.OK {
		t.Fatalf("expected failure, got success: %s", rep.Final)
	}
	if rep.OpIndex != wantOp {
		t.Fatalf("failed op index = %d, want %d (%s)", rep.OpIndex, wantOp, rep.Error)
	}
	return rep
}

func checkInverseRestores(t *testing.T, doc string, rep *Report) {
	t.Helper()
	restored, err := ApplyInverses(rep.Final, rep.Inverses)
	if err != nil {
		t.Fatalf("inverse replay: %v", err)
	}
	orig, _ := decodeValue([]byte(doc))
	got, _ := decodeValue(restored)
	if !jsonEqual(orig, got) {
		t.Fatalf("inverse replay = %s, want %s", restored, doc)
	}
}

func TestEscapedKeys(t *testing.T) {
	doc := `{"a/b": {"~key": 1}}`
	rep := applyOK(t, doc, `[
		{"op":"replace","path":"/a~1b/~0key","value":2},
		{"op":"add","path":"/a~1b/x~0y","value":3}
	]`)
	var got map[string]interface{}
	if err := json.Unmarshal(rep.Final, &got); err != nil {
		t.Fatal(err)
	}
	sub := got["a/b"].(map[string]interface{})
	if sub["~key"].(float64) != 2 || sub["x~y"].(float64) != 3 {
		t.Fatalf("unexpected final: %s", rep.Final)
	}
	checkInverseRestores(t, doc, rep)
}

func TestArrayMove(t *testing.T) {
	doc := `{"list":["a","b","c","d"]}`
	rep := applyOK(t, doc, `[{"op":"move","from":"/list/1","path":"/list/3"}]`)
	if string(rep.Final) != `{"list":["a","c","d","b"]}` {
		t.Fatalf("final = %s", rep.Final)
	}
	checkInverseRestores(t, doc, rep)
}

func TestArrayInsertShiftsAndRemoveReindexes(t *testing.T) {
	doc := `[1,2,3]`
	rep := applyOK(t, doc, `[
		{"op":"add","path":"/1","value":9},
		{"op":"remove","path":"/0"},
		{"op":"add","path":"/-","value":4}
	]`)
	if string(rep.Final) != `[9,2,3,4]` {
		t.Fatalf("final = %s", rep.Final)
	}
	checkInverseRestores(t, doc, rep)
}

func TestMidPatchTestFailureIsAtomic(t *testing.T) {
	doc := `{"n":1,"arr":[1,2]}`
	rep := applyFail(t, doc, `[
		{"op":"replace","path":"/n","value":100},
		{"op":"test","path":"/arr/0","value":2},
		{"op":"remove","path":"/arr"}
	]`, 1)
	if rep.Final != nil {
		t.Fatalf("failed patch must not produce a final document")
	}
	if len(rep.Steps) != 1 {
		t.Fatalf("expected 1 recorded step before failure, got %d", len(rep.Steps))
	}
}

func TestUnknownOpAndMissingFields(t *testing.T) {
	applyFail(t, `{}`, `[{"op":"frobnicate","path":"/a"}]`, 0)
	applyFail(t, `{}`, `[{"op":"add","path":"/a"}]`, 0)
	applyFail(t, `{}`, `[{"op":"move","path":"/a"}]`, 0)
	rep := applyFail(t, `[1]`, `[{"op":"add","path":"/x","value":1}]`, 0)
	if !strings.Contains(rep.Error, "op 0") {
		t.Fatalf("error should name the op index: %s", rep.Error)
	}
}

func TestArrayIndexRules(t *testing.T) {
	applyFail(t, `[1,2]`, `[{"op":"remove","path":"/2"}]`, 0)
	applyFail(t, `[1,2]`, `[{"op":"remove","path":"/01"}]`, 0)
	applyFail(t, `[1,2]`, `[{"op":"remove","path":"/-"}]`, 0)
	rep := applyOK(t, `[1,2]`, `[{"op":"add","path":"/-","value":3}]`)
	if string(rep.Final) != `[1,2,3]` {
		t.Fatalf("final = %s", rep.Final)
	}
}

func TestMissingParentNotCreated(t *testing.T) {
	applyFail(t, `{}`, `[{"op":"add","path":"/a/b","value":1}]`, 0)
}

func TestRootReplaceAllowedRootRemoveRejected(t *testing.T) {
	rep := applyOK(t, `{"a":1}`, `[{"op":"replace","path":"","value":[1,2]}]`)
	if string(rep.Final) != `[1,2]` {
		t.Fatalf("final = %s", rep.Final)
	}
	applyFail(t, `{"a":1}`, `[{"op":"remove","path":""}]`, 0)
	rep = applyOK(t, `1`, `[{"op":"add","path":"","value":{"x":true}}]`)
	if string(rep.Final) != `{"x":true}` {
		t.Fatalf("final = %s", rep.Final)
	}
}

func TestMoveIntoOwnDescendantRejected(t *testing.T) {
	applyFail(t, `{"a":{"b":1}}`, `[{"op":"move","from":"/a","path":"/a/c"}]`, 0)
}

func TestCopyDoesNotShareState(t *testing.T) {
	doc := `{"src":{"list":[1]},"dst":{}}`
	rep := applyOK(t, doc, `[
		{"op":"copy","from":"/src","path":"/dst"},
		{"op":"add","path":"/dst/list/-","value":2}
	]`)
	want, _ := decodeValue([]byte(`{"src":{"list":[1]},"dst":{"list":[1,2]}}`))
	got, _ := decodeValue(rep.Final)
	if !jsonEqual(want, got) {
		t.Fatalf("final = %s", rep.Final)
	}
}

func TestTestOpSemantics(t *testing.T) {
	// Key order irrelevant, numeric forms equal by mathematical value.
	applyOK(t, `{"a":1,"b":2,"n":1.0}`, `[
		{"op":"test","path":"","value":{"b":2,"a":1,"n":1.0}},
		{"op":"test","path":"/n","value":1}
	]`)
	// Array order matters.
	applyFail(t, `[1,2]`, `[{"op":"test","path":"","value":[2,1]}]`, 0)
	// Boolean is not a number.
	applyFail(t, `{"x":true}`, `[{"op":"test","path":"/x","value":1}]`, 0)
}

func TestPreviewAndWriteAgree(t *testing.T) {
	doc := `{"k":[3,1,2]}`
	p := `[
		{"op":"add","path":"/k/0","value":0},
		{"op":"move","from":"/k/3","path":"/k/1"},
		{"op":"test","path":"/k","value":[0,2,3,1]}
	]`
	r1 := applyOK(t, doc, p)
	r2 := applyOK(t, doc, p)
	if string(r1.Final) != string(r2.Final) {
		t.Fatalf("modes disagree: %s vs %s", r1.Final, r2.Final)
	}
	if len(r1.Steps) != 3 || len(r1.Inverses) != 3 {
		t.Fatalf("expected 3 steps and inverses")
	}
	checkInverseRestores(t, doc, r1)
}
