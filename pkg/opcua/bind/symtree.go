// Package bind connects pkg/opcua to a real project: it adapts the
// pkg/symtree symbol tree to opcua.SymbolNode and the interp Runtime to
// opcua.NodeSource. It lives outside pkg/opcua so the server package stays
// free of the analyzer and the interpreter.
package bind

import (
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/opcua"
	"github.com/centroid-is/stc/pkg/symtree"
	"github.com/centroid-is/stc/pkg/types"
)

// symNode adapts one symtree.Node. name overrides the node's own name for
// array elements, which symtree labels "[i]" and opcua expects "arr[i]".
type symNode struct {
	n    *symtree.Node
	name string
}

var _ opcua.SymbolNode = symNode{}

// rootNode is the synthetic KindRoot above the tree roots.
type rootNode struct{ t *symtree.Tree }

var _ opcua.SymbolNode = rootNode{}

// Root returns a KindRoot SymbolNode whose Children are t's GVL and
// PROGRAM roots in tree order. A nil tree yields a root without children.
func Root(t *symtree.Tree) opcua.SymbolNode { return rootNode{t: t} }

func (rootNode) Path() string                  { return "" }
func (rootNode) Name() string                  { return "" }
func (rootNode) Kind() opcua.Kind              { return opcua.KindRoot }
func (rootNode) Type() types.Type              { return nil }
func (rootNode) TypeName() string              { return "" }
func (rootNode) Attributes() []ast.Attribute   { return nil }
func (rootNode) EnumStrings() map[int64]string { return nil }
func (r rootNode) Children() []opcua.SymbolNode {
	if r.t == nil {
		return nil
	}
	return wrap(r.t.Roots, nil)
}

// wrap adapts nodes; parent is the array the nodes are elements of, or nil.
func wrap(nodes []*symtree.Node, parent *symtree.Node) []opcua.SymbolNode {
	out := make([]opcua.SymbolNode, 0, len(nodes))
	for _, c := range nodes {
		if c == nil {
			continue
		}
		name := c.Name
		if parent != nil {
			name = parent.Name + c.Name
		}
		out = append(out, symNode{n: c, name: name})
	}
	return out
}

func (s symNode) Path() string                  { return s.n.Path }
func (s symNode) Name() string                  { return s.name }
func (s symNode) Kind() opcua.Kind              { return kindOf(s.n.Kind) }
func (s symNode) TypeName() string              { return s.n.TypeName }
func (s symNode) EnumStrings() map[int64]string { return s.n.EnumStrings }

// Type returns the resolved IEC type; nil for GVL and PROGRAM roots.
func (s symNode) Type() types.Type { return s.n.Type }

// Attributes returns copies in symtree order: type-level first, then the
// instance declaration, so the last occurrence of a name wins.
func (s symNode) Attributes() []ast.Attribute {
	if len(s.n.Attributes) == 0 {
		return nil
	}
	out := make([]ast.Attribute, 0, len(s.n.Attributes))
	for _, a := range s.n.Attributes {
		if a != nil {
			out = append(out, *a)
		}
	}
	return out
}

// Children returns the members in declaration order; for an array the
// elements Low..High, each named "<array>[i]".
func (s symNode) Children() []opcua.SymbolNode {
	if s.n.Kind == symtree.KindArray {
		return wrap(s.n.Children(), s.n)
	}
	return wrap(s.n.Children(), nil)
}

// kindOf maps the symtree kinds onto the opcua kinds one to one.
func kindOf(k symtree.Kind) opcua.Kind {
	switch k {
	case symtree.KindGVL:
		return opcua.KindGVL
	case symtree.KindProgram:
		return opcua.KindProgram
	case symtree.KindFBInstance:
		return opcua.KindFBInstance
	case symtree.KindStruct:
		return opcua.KindStruct
	case symtree.KindArray:
		return opcua.KindArray
	case symtree.KindEnum:
		return opcua.KindEnum
	case symtree.KindReference:
		return opcua.KindReference
	case symtree.KindPointer:
		return opcua.KindPointer
	default:
		return opcua.KindScalar
	}
}
