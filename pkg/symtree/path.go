package symtree

import (
	"errors"
	"fmt"
	"strconv"
)

// MaxPathLen bounds the length of a path accepted by ParsePath and Lookup.
// Paths arrive from remote clients (OPC UA, MCP), so the cap keeps parsing
// cheap.
const MaxPathLen = 1024

// Segment is one step of a parsed path: a member name, an array index or a
// bit number.
type Segment struct {
	Name    string // member or root name, as written
	Index   int    // array index when IsIndex
	IsIndex bool
	IsBit   bool
	Bit     int // bit number when IsBit
}

// ParsePath splits a path into segments following the grammar in the
// package documentation. It never panics; malformed input yields an error.
func ParsePath(p string) ([]Segment, error) {
	if p == "" {
		return nil, errors.New("empty path")
	}
	if len(p) > MaxPathLen {
		return nil, fmt.Errorf("path longer than %d bytes", MaxPathLen)
	}
	if !identStart(p[0]) {
		return nil, fmt.Errorf("path must start with an identifier: %q", p)
	}
	i := identEnd(p, 0)
	segs := []Segment{{Name: p[:i]}}
	for i < len(p) {
		switch p[i] {
		case '.':
			i++
			switch {
			case i < len(p) && identStart(p[i]):
				j := identEnd(p, i)
				segs = append(segs, Segment{Name: p[i:j]})
				i = j
			case i < len(p) && isDigit(p[i]):
				j := i
				for j < len(p) && isDigit(p[j]) {
					j++
				}
				bit, err := strconv.Atoi(p[i:j])
				if err != nil {
					return nil, fmt.Errorf("invalid bit number %q in %q", p[i:j], p)
				}
				segs = append(segs, Segment{IsBit: true, Bit: bit})
				i = j
			default:
				return nil, fmt.Errorf("expected identifier or bit number after '.' at offset %d in %q", i, p)
			}
		case '[':
			i++
			j := i
			if j < len(p) && p[j] == '-' {
				j++
			}
			for j < len(p) && isDigit(p[j]) {
				j++
			}
			idx, err := strconv.Atoi(p[i:j])
			if err != nil {
				return nil, fmt.Errorf("invalid array index at offset %d in %q", i, p)
			}
			if j < len(p) && p[j] == ',' {
				return nil, fmt.Errorf("multi-dimensional arrays not supported: %q", p)
			}
			if j >= len(p) || p[j] != ']' {
				return nil, fmt.Errorf("expected ']' at offset %d in %q", j, p)
			}
			segs = append(segs, Segment{IsIndex: true, Index: idx})
			i = j + 1
		default:
			return nil, fmt.Errorf("unexpected character %q at offset %d in %q", p[i], i, p)
		}
	}
	return segs, nil
}

func identStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

// identEnd returns the offset just past the identifier starting at i.
func identEnd(p string, i int) int {
	for i < len(p) && (identStart(p[i]) || isDigit(p[i])) {
		i++
	}
	return i
}
