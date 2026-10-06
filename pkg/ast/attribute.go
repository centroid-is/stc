package ast

import "strings"

// Attribute represents a TwinCAT/CODESYS attribute pragma such as
// {attribute 'qualified_only'} or {attribute 'OPC.UA.DA' := '1'}.
// Value holds the unescaped text; HasValue distinguishes a valueless
// attribute from one with an empty value.
type Attribute struct {
	NodeBase
	Name     string `json:"name"`
	Value    string `json:"value,omitempty"`
	HasValue bool   `json:"-"`
}

// Children returns nil (attributes are leaf nodes).
func (n *Attribute) Children() []Node { return nil }

// String renders the attribute in canonical form. Name and value are always
// single-quoted and every quote character is doubled (two quote characters),
// so the output cannot close the quoted string early. The lexer ends a
// pragma at the first closing brace regardless of quotes, so a closing brace
// in a programmatically built name or value is replaced by a closing
// parenthesis; otherwise the output would re-parse as a truncated pragma
// followed by stray tokens. A parsed attribute never contains one.
func (n *Attribute) String() string {
	var b strings.Builder
	b.WriteString("{attribute ")
	b.WriteString(quoteAttr(n.Name))
	if n.HasValue {
		b.WriteString(" := ")
		b.WriteString(quoteAttr(n.Value))
	}
	b.WriteString("}")
	return b.String()
}

var attrEscaper = strings.NewReplacer("'", "''", "}", ")")

func quoteAttr(s string) string {
	return "'" + attrEscaper.Replace(s) + "'"
}

// HasAttribute reports whether attrs contains an attribute with the given
// name, compared case-insensitively.
func HasAttribute(attrs []*Attribute, name string) bool {
	for _, a := range attrs {
		if a != nil && strings.EqualFold(a.Name, name) {
			return true
		}
	}
	return false
}

// GVLDecl represents a global variable list: the VAR_GLOBAL blocks found at
// the top level of one file, aggregated under a single name.
type GVLDecl struct {
	NodeBase
	Name       *Ident        `json:"name"`
	Blocks     []*VarBlock   `json:"blocks,omitempty"`
	Attributes []*Attribute  `json:"attributes,omitempty"`
	Pragmas    []*PragmaNode `json:"pragmas,omitempty"`
	// NameDerived is true when Name came from the file basename rather than
	// from an explicit name (SetGVLName). A derived name that clashes with a
	// POU only warns, so the variables are still registered.
	NameDerived bool `json:"-"`
}

// Children lists attributes and pragmas first, then the name and blocks.
func (n *GVLDecl) Children() []Node {
	var nodes []Node
	nodes = appendAttrs(nodes, n.Attributes, n.Pragmas)
	if n.Name != nil {
		nodes = append(nodes, n.Name)
	}
	for _, b := range n.Blocks {
		nodes = append(nodes, b)
	}
	return nodes
}
func (n *GVLDecl) declNode() {}

// appendAttrs appends attributes then pragmas to nodes. Every node carrying
// attributes calls this first in Children() so comments preceding an
// attribute attach to the Attribute node.
func appendAttrs(nodes []Node, attrs []*Attribute, pragmas []*PragmaNode) []Node {
	for _, a := range attrs {
		nodes = append(nodes, a)
	}
	for _, p := range pragmas {
		nodes = append(nodes, p)
	}
	return nodes
}

// SanitizeGVLName turns a file basename into a valid identifier: characters
// outside [A-Za-z0-9_] become '_', a leading digit gets a '_' prefix, and an
// empty name becomes "GVL". Hostile filenames therefore cannot inject syntax.
func SanitizeGVLName(base string) string {
	if base == "" {
		return "GVL"
	}
	var b strings.Builder
	for _, r := range base {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out[0] >= '0' && out[0] <= '9' {
		out = "_" + out
	}
	return out
}

// SetGVLName renames every GVLDecl in file to name and reports whether any
// GVLDecl was present.
func SetGVLName(file *SourceFile, name string) bool {
	if file == nil {
		return false
	}
	found := false
	for _, d := range file.Declarations {
		g, ok := d.(*GVLDecl)
		if !ok {
			continue
		}
		found = true
		if g.Name == nil {
			g.Name = &Ident{NodeBase: NodeBase{NodeKind: KindIdent, NodeSpan: g.NodeSpan}}
		}
		g.Name.Name = name
		g.NameDerived = false
	}
	return found
}
