package symtree

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/centroid-is/stc/pkg/types"
)

// Children returns the child nodes in declaration order. For a bounded
// array it synthesises the elements Low..High on each call; an unbounded
// or multi-dimensional array has no children.
func (n *Node) Children() []*Node {
	if n.Kind != KindArray {
		return n.children
	}
	if !n.Bounded || n.elem == nil || n.High < n.Low {
		return nil
	}
	out := make([]*Node, 0, n.High-n.Low+1)
	for i := n.Low; i <= n.High; i++ {
		out = append(out, n.element(i))
	}
	return out
}

// element builds array element i without checking the bounds.
func (n *Node) element(i int) *Node {
	return n.elem("[" + strconv.Itoa(i) + "]")
}

// Lookup resolves a path (case-insensitive) to its node. Array elements
// and bits are synthesised on demand after their bounds are checked.
func (t *Tree) Lookup(path string) (*Node, error) {
	segs, err := ParsePath(path)
	if err != nil {
		return nil, err
	}
	var n *Node
	for _, r := range t.Roots {
		if strings.EqualFold(r.Name, segs[0].Name) {
			n = r
			break
		}
	}
	if n == nil {
		return nil, fmt.Errorf("unknown root %q", segs[0].Name)
	}
	for _, s := range segs[1:] {
		switch {
		case s.IsIndex:
			if n.Kind != KindArray {
				return nil, fmt.Errorf("%s is not an array", n.Path)
			}
			if !n.Bounded {
				return nil, fmt.Errorf("%s has no constant one-dimensional bounds", n.Path)
			}
			if s.Index < n.Low || s.Index > n.High {
				return nil, fmt.Errorf("index %d out of bounds %d..%d for %s", s.Index, n.Low, n.High, n.Path)
			}
			n = n.element(s.Index)
		case s.IsBit:
			width := bitWidth(n)
			if width == 0 {
				return nil, fmt.Errorf("bit access needs an integer or bit-string variable: %s", n.Path)
			}
			if s.Bit >= width {
				return nil, fmt.Errorf("bit %d out of range for %s %s (0..%d)", s.Bit, n.TypeName, n.Path, width-1)
			}
			name := strconv.Itoa(s.Bit)
			n = &Node{
				Name: name, Path: n.Path + "." + name, Kind: KindScalar,
				Type: &types.PrimitiveType{Kind_: types.KindBOOL}, TypeName: "BOOL",
				Section: n.Section, Constant: n.Constant, Retain: n.Retain, Persistent: n.Persistent, Pos: n.Pos,
			}
		default:
			if n.Kind == KindArray {
				return nil, fmt.Errorf("%s is an array; index it before accessing %s", n.Path, s.Name)
			}
			var next *Node
			for _, c := range n.children {
				if strings.EqualFold(c.Name, s.Name) {
					next = c
					break
				}
			}
			if next == nil {
				return nil, fmt.Errorf("unknown member %q of %s", s.Name, n.Path)
			}
			n = next
		}
	}
	return n, nil
}

// bitWidth returns the bit count of an integer or bit-string scalar, 0 for
// anything else (enums, reals, BOOL, structs).
func bitWidth(n *Node) int {
	if n.Kind != KindScalar || n.Type == nil {
		return 0
	}
	switch n.Type.Kind() {
	case types.KindBYTE, types.KindSINT, types.KindUSINT:
		return 8
	case types.KindWORD, types.KindINT, types.KindUINT:
		return 16
	case types.KindDWORD, types.KindDINT, types.KindUDINT:
		return 32
	case types.KindLWORD, types.KindLINT, types.KindULINT:
		return 64
	}
	return 0
}

// WalkOpts controls Tree.Walk.
type WalkOpts struct {
	// ExpandArrays visits array elements; by default arrays are visited
	// but their elements are not.
	ExpandArrays bool
}

// Walk visits every node depth-first in declaration order, roots first.
// When fn returns false the node's subtree is skipped.
func (t *Tree) Walk(fn func(*Node) bool, opts WalkOpts) {
	for _, r := range t.Roots {
		walk(r, fn, opts)
	}
}

func walk(n *Node, fn func(*Node) bool, opts WalkOpts) {
	if !fn(n) {
		return
	}
	if n.Kind == KindArray && !opts.ExpandArrays {
		return
	}
	for _, c := range n.Children() {
		walk(c, fn, opts)
	}
}
