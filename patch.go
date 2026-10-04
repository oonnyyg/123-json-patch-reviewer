package patch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Operation is a single RFC 6902 patch operation.
type Operation struct {
	Op    string          `json:"op"`
	Path  string          `json:"path"`
	From  string          `json:"from"`
	Value json.RawMessage `json:"value"`
}

// Step describes one applied operation in the review report.
type Step struct {
	Index   int             `json:"index"`
	Op      string          `json:"op"`
	Path    string          `json:"path"`
	After   json.RawMessage `json:"after"`
	Inverse Operation       `json:"inverse"`
}

// Report is the JSON review report produced by Apply.
type Report struct {
	OK       bool            `json:"ok"`
	Error    string          `json:"error,omitempty"`
	OpIndex  int             `json:"failedOpIndex,omitempty"`
	Steps    []Step          `json:"steps,omitempty"`
	Final    json.RawMessage `json:"final,omitempty"`
	Inverses []Operation     `json:"inverseOps,omitempty"`
}

// decodeValue decodes a JSON value using json.Number for numbers.
func decodeValue(raw []byte) (interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v interface{}
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

func encodeValue(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		return json.RawMessage("null")
	}
	return b
}

// Apply runs the patch operations in order against docJSON. It is atomic:
// on any failure the returned report carries the failing operation index and
// no document. On success it returns per-step snapshots, the final document,
// and inverse operations that restore the original when applied in reverse.
func Apply(docJSON, patchJSON []byte) (*Report, error) {
	doc, err := decodeValue(docJSON)
	if err != nil {
		return nil, fmt.Errorf("invalid JSON document: %v", err)
	}
	var ops []Operation
	dec := json.NewDecoder(bytes.NewReader(patchJSON))
	dec.UseNumber()
	if err := dec.Decode(&ops); err != nil {
		return nil, fmt.Errorf("invalid patch: %v", err)
	}
	rep := &Report{}
	for i, op := range ops {
		inv, newDoc, err := applyOne(doc, op, i)
		if err != nil {
			rep.Error = err.Error()
			rep.OpIndex = i
			return rep, nil
		}
		doc = newDoc
		rep.Steps = append(rep.Steps, Step{
			Index:   i,
			Op:      op.Op,
			Path:    op.Path,
			After:   encodeValue(doc),
			Inverse: inv,
		})
		rep.Inverses = append(rep.Inverses, inv)
	}
	rep.OK = true
	rep.Final = encodeValue(doc)
	return rep, nil
}

func missing(i int, field string) error {
	return fmt.Errorf("op %d: missing required %q", i, field)
}

func applyOne(doc interface{}, op Operation, i int) (Operation, interface{}, error) {
	none := Operation{}
	path, err := ParsePointer(op.Path)
	if err != nil {
		return none, nil, fmt.Errorf("op %d: %v", i, err)
	}
	switch op.Op {
	case "add", "replace":
		if op.Value == nil {
			return none, nil, missing(i, "value")
		}
		val, err := decodeValue(op.Value)
		if err != nil {
			return none, nil, fmt.Errorf("op %d: invalid value: %v", i, err)
		}
		if op.Op == "replace" {
			old, err := get(doc, path)
			if err != nil {
				return none, nil, fmt.Errorf("op %d: replace target: %v", i, err)
			}
			newDoc, err := add(doc, path, val)
			if err != nil {
				return none, nil, fmt.Errorf("op %d: %v", i, err)
			}
			return Operation{Op: "replace", Path: op.Path, Value: encodeValue(old)}, newDoc, nil
		}
		// Inverse is "remove" unless add overwrote an existing object member.
		replaced := false
		var old interface{}
		if len(path) > 0 {
			if parent, _, err := parentOf(doc, path); err == nil {
				if _, isObj := parent.(map[string]interface{}); isObj {
					if v, err := get(doc, path); err == nil {
						old, replaced = v, true
					}
				}
			}
		}
		newDoc, err := add(doc, path, val)
		if err != nil {
			return none, nil, fmt.Errorf("op %d: %v", i, err)
		}
		if replaced {
			return Operation{Op: "replace", Path: op.Path, Value: encodeValue(old)}, newDoc, nil
		}
		invPath := op.Path
		if len(path) > 0 && path[len(path)-1] == "-" {
			// Resolve "-" to the actual append index for the inverse.
			if parent, err := get(newDoc, path[:len(path)-1]); err == nil {
				if arr, ok := parent.([]interface{}); ok {
					segs := append(Pointer{}, path[:len(path)-1]...)
					segs = append(segs, strconv.Itoa(len(arr)-1))
					invPath = "/" + strings.Join(escapeSegments(segs), "/")
				}
			}
		}
		return Operation{Op: "remove", Path: invPath}, newDoc, nil
	case "remove":
		newDoc, old, err := remove(doc, path)
		if err != nil {
			return none, nil, fmt.Errorf("op %d: %v", i, err)
		}
		return Operation{Op: "add", Path: op.Path, Value: encodeValue(old)}, newDoc, nil
	case "move", "copy":
		if op.From == "" {
			return none, nil, missing(i, "from")
		}
		from, err := ParsePointer(op.From)
		if err != nil {
			return none, nil, fmt.Errorf("op %d: from: %v", i, err)
		}
		if op.Op == "move" && isPrefixPath(from, path) && len(path) > len(from) {
			return none, nil, fmt.Errorf("op %d: cannot move a value into its own descendant", i)
		}
		src, err := get(doc, from)
		if err != nil {
			return none, nil, fmt.Errorf("op %d: from: %v", i, err)
		}
		if op.Op == "copy" {
			newDoc, err := add(doc, path, deepCopy(src))
			if err != nil {
				return none, nil, fmt.Errorf("op %d: %v", i, err)
			}
			return Operation{Op: "remove", Path: op.Path}, newDoc, nil
		}
		// move: remove from source first, then add at target.
		interim, old, err := remove(doc, from)
		if err != nil {
			return none, nil, fmt.Errorf("op %d: %v", i, err)
		}
		newDoc, err := add(interim, path, old)
		if err != nil {
			return none, nil, fmt.Errorf("op %d: %v", i, err)
		}
		inv := Operation{Op: "move", Path: op.From, From: op.Path}
		return inv, newDoc, nil
	case "test":
		if op.Value == nil {
			return none, nil, missing(i, "value")
		}
		want, err := decodeValue(op.Value)
		if err != nil {
			return none, nil, fmt.Errorf("op %d: invalid value: %v", i, err)
		}
		got, err := get(doc, path)
		if err != nil {
			return none, nil, fmt.Errorf("op %d: test target: %v", i, err)
		}
		if !jsonEqual(got, want) {
			return none, nil, fmt.Errorf("op %d: test failed at %q", i, op.Path)
		}
		return Operation{Op: "test", Path: op.Path, Value: op.Value}, doc, nil
	default:
		return none, nil, fmt.Errorf("op %d: unknown operation %q", i, op.Op)
	}
}

// isPrefixPath reports whether a is a strict-or-equal prefix of b.
func isPrefixPath(a, b Pointer) bool {
	if len(a) > len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ApplyInverses applies inverse operations in reverse order to finalJSON and
// returns the restored document. Used to verify round-trip restoration.
func ApplyInverses(finalJSON []byte, inverses []Operation) ([]byte, error) {
	rev := make([]Operation, len(inverses))
	for i, op := range inverses {
		rev[len(inverses)-1-i] = op
	}
	patch, err := json.Marshal(rev)
	if err != nil {
		return nil, err
	}
	rep, err := Apply(finalJSON, patch)
	if err != nil {
		return nil, err
	}
	if !rep.OK {
		return nil, fmt.Errorf("inverse replay failed at op %d: %s", rep.OpIndex, rep.Error)
	}
	return rep.Final, nil
}
