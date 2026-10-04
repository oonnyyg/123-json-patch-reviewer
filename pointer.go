package patch

import (
	"fmt"
	"strconv"
	"strings"
)

// Pointer is a parsed JSON Pointer (RFC 6901).
type Pointer []string

// ParsePointer parses a JSON Pointer string. The empty string refers to the
// whole document. Segments use ~0 for '~' and ~1 for '/'.
func ParsePointer(s string) (Pointer, error) {
	if s == "" {
		return Pointer{}, nil
	}
	if !strings.HasPrefix(s, "/") {
		return nil, fmt.Errorf("invalid pointer %q: must be empty or start with '/'", s)
	}
	raw := strings.Split(s[1:], "/")
	segs := make(Pointer, len(raw))
	for i, seg := range raw {
		var b strings.Builder
		for j := 0; j < len(seg); j++ {
			if seg[j] == '~' {
				if j+1 >= len(seg) || (seg[j+1] != '0' && seg[j+1] != '1') {
					return nil, fmt.Errorf("invalid pointer %q: bad escape in segment %q", s, seg)
				}
				if seg[j+1] == '0' {
					b.WriteByte('~')
				} else {
					b.WriteByte('/')
				}
				j++
				continue
			}
			b.WriteByte(seg[j])
		}
		segs[i] = b.String()
	}
	return segs, nil
}

// escapeSegments escapes pointer segments (~ -> ~0, / -> ~1).
func escapeSegments(segs []string) []string {
	out := make([]string, len(segs))
	for i, s := range segs {
		s = strings.ReplaceAll(s, "~", "~0")
		s = strings.ReplaceAll(s, "/", "~1")
		out[i] = s
	}
	return out
}

// arrayIndex validates an array index segment: decimal digits, no leading
// zeros (except "0" itself), and within bounds for the given length.
// allowDash permits "-" meaning "one past the end" (append position).
func arrayIndex(seg string, length int, allowDash bool) (int, error) {
	if seg == "-" {
		if allowDash {
			return length, nil
		}
		return 0, fmt.Errorf("'-' is only valid as an add target index")
	}
	if seg == "" {
		return 0, fmt.Errorf("empty array index")
	}
	for i := 0; i < len(seg); i++ {
		if seg[i] < '0' || seg[i] > '9' {
			return 0, fmt.Errorf("invalid array index %q", seg)
		}
	}
	if len(seg) > 1 && seg[0] == '0' {
		return 0, fmt.Errorf("array index %q has a leading zero", seg)
	}
	n, err := strconv.Atoi(seg)
	if err != nil {
		return 0, fmt.Errorf("invalid array index %q", seg)
	}
	limit := length - 1
	if allowDash {
		limit = length
	}
	if n > limit {
		return 0, fmt.Errorf("array index %d out of bounds (length %d)", n, length)
	}
	return n, nil
}
