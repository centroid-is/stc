// Package symtree builds the static symbol tree of an analysed project:
// every GVL and PROGRAM as a root, with FB instances, struct members and
// array elements below them. Each node carries its IEC type from the symbol
// table, the declared type name, enum strings and the verbatim attributes
// from the AST (type-level first, then instance-level).
//
// The tree is static: it never reads live values and does not depend on
// pkg/interp. Children are in declaration order. Array elements are
// synthesised lazily from the declared bounds.
//
// # Path grammar
//
// A path addresses a node from a root:
//
//	path    = ident { '.' ident | '[' int ']' | '.' digits }
//	ident   = (letter | '_') { letter | digit | '_' }
//	int     = [ '-' ] digit { digit }
//
// "GVL.fb[2].HMI.p_stat_State" names a struct member of an FB inside an
// array. A trailing ".3" after an integer or bit-string scalar addresses
// bit 3. Multi-dimensional indices ("a[1,2]") are rejected. Paths longer
// than MaxPathLen bytes are rejected. Matching is case-insensitive; Node.Path
// always uses the declared case. Array indices use the declared bounds.
// pkg/interp mirrors this grammar for live Get/Set and must not import this
// package.
package symtree

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/centroid-is/stc/pkg/types"
)

// Kind classifies a node of the symbol tree.
type Kind int

// Node kinds.
const (
	KindGVL        Kind = iota // a global variable list root
	KindProgram                // a PROGRAM root
	KindFBInstance             // a function block instance
	KindStruct                 // a STRUCT value
	KindArray                  // an ARRAY; elements are synthesised lazily
	KindScalar                 // an elementary value (INT, REAL, STRING, ...)
	KindEnum                   // an enum-typed value
	KindReference              // a REFERENCE TO; never followed
	KindPointer                // a POINTER TO; never followed
)

var kindNames = [...]string{
	KindGVL:        "gvl",
	KindProgram:    "program",
	KindFBInstance: "fb_instance",
	KindStruct:     "struct",
	KindArray:      "array",
	KindScalar:     "scalar",
	KindEnum:       "enum",
	KindReference:  "reference",
	KindPointer:    "pointer",
}

// String returns the lower-case kind name used in JSON output.
func (k Kind) String() string {
	if k >= 0 && int(k) < len(kindNames) {
		return kindNames[k]
	}
	return "unknown"
}

// Node is one entry of the symbol tree.
type Node struct {
	// Name is the declared name; "[3]" for an array element, "3" for a bit.
	Name string
	// Path is the full dotted path in declared case, e.g.
	// "GVL.fb[2].HMI.p_stat_State".
	Path string
	Kind Kind
	// Type is the resolved IEC type from the symbol table; nil for roots
	// and for variables whose type did not resolve.
	Type types.Type
	// TypeName is the declared type as written, e.g. "FB_Drive",
	// "ARRAY[1..3] OF FB_Drive", "POINTER TO INT".
	TypeName string
	// Attributes are the type-level attributes (FB, STRUCT or TYPE
	// declaration) followed by the instance-level ones (variable or struct
	// member declaration). Roots carry their GVL or PROGRAM attributes.
	Attributes []*ast.Attribute
	// EnumStrings maps ordinal to value name for enum-typed nodes.
	EnumStrings map[int64]string
	// Section is the VAR section the variable is declared in.
	Section ast.VarSection
	// Constant, Retain and Persistent mirror the declaring VAR block.
	Constant, Retain, Persistent bool
	// Pos is the declaration position of the name.
	Pos ast.Pos
	// Low and High are the declared bounds of a one-dimensional array with
	// constant bounds. Bounded reports whether they are known; an array
	// with unknown or multi-dimensional bounds has no elements.
	Low, High int
	Bounded   bool

	children []*Node
	// elem builds the array element with the given label ("[2]", "[*]").
	elem func(label string) *Node
}

// Tree is the symbol tree of a project.
type Tree struct {
	// Roots holds the GVLs (library files first, then user files, in
	// source order) followed by the PROGRAMs in source order.
	Roots []*Node
}

// builder holds the declaration indexes used while building.
type builder struct {
	table    *symbols.Table
	typeDecl map[string]*ast.TypeDecl
	fbDecl   map[string]*ast.FunctionBlockDecl
}

// Build creates the symbol tree from an analysis result. It needs
// res.Symbols for types and res.Files / res.LibraryFiles for declaration
// order and attributes.
func Build(res analyzer.AnalysisResult) (*Tree, error) {
	if res.Symbols == nil {
		return nil, errors.New("symtree: analysis result has no symbol table")
	}
	b := &builder{
		table:    res.Symbols,
		typeDecl: map[string]*ast.TypeDecl{},
		fbDecl:   map[string]*ast.FunctionBlockDecl{},
	}
	all := append(append([]*ast.SourceFile{}, res.LibraryFiles...), res.Files...)
	userGVL := map[string]bool{}
	for _, f := range res.Files {
		for _, d := range f.Declarations {
			if g, ok := d.(*ast.GVLDecl); ok && g.Name != nil {
				userGVL[strings.ToUpper(g.Name.Name)] = true
			}
		}
	}
	var gvls []*ast.GVLDecl
	var progs []*ast.ProgramDecl
	for i, f := range all {
		if f == nil {
			continue
		}
		library := i < len(res.LibraryFiles)
		for _, d := range f.Declarations {
			switch d := d.(type) {
			case *ast.TypeDecl:
				if d.Name != nil {
					b.typeDecl[strings.ToUpper(d.Name.Name)] = d
				}
			case *ast.FunctionBlockDecl:
				if d.Name != nil {
					b.fbDecl[strings.ToUpper(d.Name.Name)] = d
				}
			case *ast.GVLDecl:
				// A user GVL replaces a library GVL of the same name.
				if d.Name != nil && !(library && userGVL[strings.ToUpper(d.Name.Name)]) {
					gvls = append(gvls, d)
				}
			case *ast.ProgramDecl:
				if d.Name != nil {
					progs = append(progs, d)
				}
			}
		}
	}

	t := &Tree{}
	for _, g := range gvls {
		t.Roots = append(t.Roots, b.gvlRoot(g))
	}
	for _, p := range progs {
		t.Roots = append(t.Roots, b.programRoot(p))
	}
	return t, nil
}

// gvlRoot builds a GVL root; member types come from the GVL symbol's
// struct type, or from the bare global symbol when the GVL has none.
func (b *builder) gvlRoot(g *ast.GVLDecl) *Node {
	root := &Node{
		Name: g.Name.Name, Path: g.Name.Name, Kind: KindGVL,
		Attributes: g.Attributes, Pos: g.Span().Start,
	}
	var members []types.StructMember
	if sym := b.table.LookupGlobal(g.Name.Name); sym != nil && sym.Kind == symbols.KindGVL {
		if st, ok := sym.Type.(*types.StructType); ok {
			members = st.Members
		}
	}
	lookup := func(name string) types.Type {
		for _, m := range members {
			if strings.EqualFold(m.Name, name) {
				return m.Type
			}
		}
		if sym := b.table.LookupGlobal(name); sym != nil && sym.Kind == symbols.KindVariable {
			return asType(sym.Type)
		}
		return nil
	}
	root.children = b.varBlocks(root.Path, g.Blocks, lookup, nil)
	return root
}

// programRoot builds a PROGRAM root with its variables in declaration order.
func (b *builder) programRoot(p *ast.ProgramDecl) *Node {
	root := &Node{
		Name: p.Name.Name, Path: p.Name.Name, Kind: KindProgram,
		Attributes: p.Attributes, Pos: p.Name.Span().Start,
	}
	root.children = b.varBlocks(root.Path, p.VarBlocks, b.scopeLookup(p.Name.Name), nil)
	return root
}

// scopeLookup returns a variable-type lookup into a POU scope.
func (b *builder) scopeLookup(pou string) func(string) types.Type {
	scope := b.table.LookupPOU(pou)
	return func(name string) types.Type {
		if scope == nil {
			return nil
		}
		if sym := scope.LookupLocal(name); sym != nil {
			return asType(sym.Type)
		}
		return nil
	}
}

// varBlocks expands the declarations of blocks under parent. VAR_TEMP and
// VAR_EXTERNAL are skipped: they hold no instance state of their own.
func (b *builder) varBlocks(parent string, blocks []*ast.VarBlock, lookup func(string) types.Type, stack []string) []*Node {
	var out []*Node
	for _, vb := range blocks {
		if vb == nil || vb.Section == ast.VarTemp || vb.Section == ast.VarExternal {
			continue
		}
		for _, vd := range vb.Declarations {
			for _, id := range vd.Names {
				typ := lookup(id.Name)
				if typ == nil {
					typ = b.specType(vd.Type)
				}
				n := b.node(id.Name, parent+"."+id.Name, vd.Type, typ, stack)
				n.Attributes = mergeAttrs(n.Attributes, vd.Attributes)
				n.Section = vb.Section
				n.Constant, n.Retain, n.Persistent = vb.IsConstant, vb.IsRetain, vb.IsPersistent
				n.Pos = id.Span().Start
				out = append(out, n)
			}
		}
	}
	return out
}

// node classifies typ and builds the node with its type-level attributes
// and children. stack holds the upper-case FB and STRUCT type names on the
// current path; a repeat yields a childless node (cycle guard).
func (b *builder) node(name, path string, spec ast.TypeSpec, typ types.Type, stack []string) *Node {
	n := &Node{Name: name, Path: path, Type: typ, Kind: KindScalar}
	n.TypeName = typeName(spec, typ)
	n.Attributes = b.typeAttrs(spec)

	switch t := typ.(type) {
	case *types.ArrayType:
		b.array(n, spec, t, stack)
	case *types.FunctionBlockType:
		n.Kind = KindFBInstance
		if inStack(stack, t.Name) {
			return n
		}
		stack = push(stack, t.Name)
		if decl := b.fbDecl[strings.ToUpper(t.Name)]; decl != nil {
			n.Attributes = mergeAttrs(decl.Attributes, n.Attributes)
			n.children = b.fbChildren(path, decl, stack)
		} else {
			n.children = b.stdFBChildren(path, t)
		}
	case *types.StructType:
		n.Kind = KindStruct
		if inStack(stack, t.Name) {
			return n
		}
		stack = push(stack, t.Name)
		if decl := b.typeDecl[strings.ToUpper(t.Name)]; decl != nil {
			if !sameDecl(spec, t.Name) {
				n.Attributes = mergeAttrs(decl.Attributes, n.Attributes)
			}
			if st, ok := decl.Type.(*ast.StructType); ok {
				n.children = b.structMembers(path, st, t, stack)
			}
		}
	case *types.EnumType:
		n.Kind = KindEnum
		n.EnumStrings = make(map[int64]string, len(t.Values))
		for i, v := range t.Values {
			if i < len(t.Ordinals) {
				n.EnumStrings[t.Ordinals[i]] = v
			}
		}
		if decl := b.typeDecl[strings.ToUpper(t.Name)]; decl != nil && !sameDecl(spec, t.Name) {
			n.Attributes = mergeAttrs(decl.Attributes, n.Attributes)
		}
	case *types.PointerType:
		n.Kind = KindPointer
	case *types.ReferenceType:
		n.Kind = KindReference
	}
	return n
}

// array sets the bounds and the lazy element factory of an array node.
func (b *builder) array(n *Node, spec ast.TypeSpec, t *types.ArrayType, stack []string) {
	n.Kind = KindArray
	var elemSpec ast.TypeSpec
	if as, ok := spec.(*ast.ArrayType); ok {
		elemSpec = as.ElementType
	}
	if len(t.Dimensions) == 1 && t.Dimensions[0].Known {
		n.Low, n.High, n.Bounded = t.Dimensions[0].Low, t.Dimensions[0].High, true
	}
	base := n.Path
	elemType := t.ElementType
	n.elem = func(label string) *Node {
		e := b.node(label, base+label, elemSpec, elemType, stack)
		e.Section, e.Constant, e.Retain, e.Persistent = n.Section, n.Constant, n.Retain, n.Persistent
		e.Pos = n.Pos
		return e
	}
}

// fbChildren lists the variables of an FB along its EXTENDS chain, base
// first. A derived variable with a base variable's name replaces it in place.
func (b *builder) fbChildren(path string, decl *ast.FunctionBlockDecl, stack []string) []*Node {
	var chain []*ast.FunctionBlockDecl
	seen := map[*ast.FunctionBlockDecl]bool{}
	for d := decl; d != nil && !seen[d]; {
		seen[d] = true
		chain = append([]*ast.FunctionBlockDecl{d}, chain...)
		if d.Extends == nil {
			break
		}
		d = b.fbDecl[strings.ToUpper(d.Extends.Name)]
	}
	var out []*Node
	index := map[string]int{}
	for _, d := range chain {
		for _, c := range b.varBlocks(path, d.VarBlocks, b.scopeLookup(d.Name.Name), stack) {
			key := strings.ToUpper(c.Name)
			if i, ok := index[key]; ok {
				out[i] = c
				continue
			}
			index[key] = len(out)
			out = append(out, c)
		}
	}
	return out
}

// stdFBChildren lists the parameters of a standard FB (TON, CTU, ...).
func (b *builder) stdFBChildren(path string, t *types.FunctionBlockType) []*Node {
	var out []*Node
	add := func(params []types.Parameter, sec ast.VarSection) {
		for _, p := range params {
			n := b.node(p.Name, path+"."+p.Name, nil, p.Type, nil)
			n.Section = sec
			out = append(out, n)
		}
	}
	add(t.Inputs, ast.VarInput)
	add(t.Outputs, ast.VarOutput)
	add(t.InOuts, ast.VarInOut)
	return out
}

// structMembers lists struct members in declaration order with types from
// the resolved struct type.
func (b *builder) structMembers(path string, decl *ast.StructType, t *types.StructType, stack []string) []*Node {
	var out []*Node
	for _, m := range decl.Members {
		if m == nil || m.Name == nil {
			continue
		}
		var typ types.Type
		for _, sm := range t.Members {
			if strings.EqualFold(sm.Name, m.Name.Name) {
				typ = sm.Type
				break
			}
		}
		if typ == nil {
			typ = b.specType(m.Type)
		}
		n := b.node(m.Name.Name, path+"."+m.Name.Name, m.Type, typ, stack)
		n.Attributes = mergeAttrs(n.Attributes, m.Attributes)
		n.Pos = m.Name.Span().Start
		out = append(out, n)
	}
	return out
}

// specType resolves a named type spec through the global scope; it is the
// fallback when the symbol table has no type for a variable.
func (b *builder) specType(spec ast.TypeSpec) types.Type {
	nt, ok := spec.(*ast.NamedType)
	if !ok || nt.Name == nil {
		return nil
	}
	if sym := b.table.LookupGlobal(nt.Name.Name); sym != nil {
		return asType(sym.Type)
	}
	return nil
}

// typeAttrs returns the attributes of the TYPE or FB declaration that spec
// names directly (an alias keeps its own attributes).
func (b *builder) typeAttrs(spec ast.TypeSpec) []*ast.Attribute {
	nt, ok := spec.(*ast.NamedType)
	if !ok || nt.Name == nil {
		return nil
	}
	if d := b.typeDecl[strings.ToUpper(nt.Name.Name)]; d != nil {
		return d.Attributes
	}
	return nil
}

// sameDecl reports whether spec names the type name itself, so its
// declaration attributes were already added by typeAttrs.
func sameDecl(spec ast.TypeSpec, name string) bool {
	nt, ok := spec.(*ast.NamedType)
	return ok && nt.Name != nil && strings.EqualFold(nt.Name.Name, name)
}

// mergeAttrs returns a followed by c in a new slice.
func mergeAttrs(a, c []*ast.Attribute) []*ast.Attribute {
	if len(a) == 0 && len(c) == 0 {
		return nil
	}
	out := make([]*ast.Attribute, 0, len(a)+len(c))
	return append(append(out, a...), c...)
}

func asType(v any) types.Type {
	t, _ := v.(types.Type)
	return t
}

func inStack(stack []string, name string) bool {
	for _, s := range stack {
		if strings.EqualFold(s, name) {
			return true
		}
	}
	return false
}

// push returns stack with name appended without sharing the backing array.
func push(stack []string, name string) []string {
	return append(append(make([]string, 0, len(stack)+1), stack...), name)
}

// typeName renders the declared type: from the spec when there is one, else
// from the resolved type.
func typeName(spec ast.TypeSpec, typ types.Type) string {
	switch s := spec.(type) {
	case *ast.NamedType:
		if s.Name != nil {
			return s.Name.Name
		}
	case *ast.ArrayType:
		if at, ok := typ.(*types.ArrayType); ok {
			return arrayName(at, s.ElementType)
		}
	case *ast.PointerType:
		return "POINTER TO " + typeName(s.BaseType, baseOf(typ))
	case *ast.ReferenceType:
		return "REFERENCE TO " + typeName(s.BaseType, baseOf(typ))
	case *ast.StringType:
		kw := "STRING"
		if s.IsWide {
			kw = "WSTRING"
		}
		if s.Length == nil {
			return kw
		}
		if n, ok := ast.ConstIntValue(s.Length, nil); ok {
			return kw + "(" + strconv.FormatInt(n, 10) + ")"
		}
		return kw
	}
	if at, ok := typ.(*types.ArrayType); ok {
		return arrayName(at, nil)
	}
	if typ == nil {
		return ""
	}
	return typ.String()
}

// arrayName renders "ARRAY[1..3, 0..N] OF elem" from the resolved bounds.
func arrayName(at *types.ArrayType, elemSpec ast.TypeSpec) string {
	dims := make([]string, len(at.Dimensions))
	for i, d := range at.Dimensions {
		if d.Known {
			dims[i] = fmt.Sprintf("%d..%d", d.Low, d.High)
		} else {
			dims[i] = d.Text
		}
	}
	return "ARRAY[" + strings.Join(dims, ", ") + "] OF " + typeName(elemSpec, at.ElementType)
}

func baseOf(typ types.Type) types.Type {
	switch t := typ.(type) {
	case *types.PointerType:
		return t.BaseType
	case *types.ReferenceType:
		return t.BaseType
	}
	return nil
}

func itoa(i int) string { return strconv.Itoa(i) }
