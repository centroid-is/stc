package ecat

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/iomap"
	"github.com/centroid-is/stc/pkg/source"
	"github.com/centroid-is/stc/pkg/types"
)

// Diagnostic codes for EtherCAT link validation. They live here rather
// than in pkg/checker so link validation stays independent of the checker.
const (
	CodeUnresolved        = "ECAT001" // link target not found in the topology
	CodeMemberNotDeclared = "ECAT002" // TcLinkTo member not declared by the type
	CodeSizeMismatch      = "ECAT003" // variable width differs from entry BitLen
	CodeDirMismatch       = "ECAT004" // %Q* linked to an input or %I* to an output
	CodeDuplicate         = "ECAT005" // two variables bound to one slot (warning)
	CodeBadLink           = "ECAT006" // malformed TcLinkTo value
	CodeNoAT              = "ECAT007" // linked leaf has no AT %I*/%Q* (warning)
)

// maxMemberDepth caps member paths and type-alias chains (T-24-03).
const maxMemberDepth = 16

// LinkedVar is one linked leaf variable found in the ST sources.
type LinkedVar struct {
	Path     string     // declared-case dotted path, e.g. "ECT.A1_01.I1"
	Steps    []string   // upper-cased path segments for the interp binder
	Link     string     // normalised TcLinkTo target
	TypeName string     // declared leaf type name, e.g. "BOOL", "AMSADDR"
	Type     types.Type // elementary type, nil for composites
	BitWidth int        // declared bit width, 0 when unknown
	Dir      Dir        // from the leaf's AT address
	HasAT    bool       // leaf carries AT %I* or AT %Q*
	Pos      source.Pos // TcLinkTo attribute position
	EndPos   source.Pos
}

// typeIndex holds the type and FB declarations of all files by upper name.
type typeIndex struct {
	types map[string]*ast.TypeDecl
	fbs   map[string]*ast.FunctionBlockDecl
}

func newTypeIndex(files []*ast.SourceFile) *typeIndex {
	idx := &typeIndex{types: map[string]*ast.TypeDecl{}, fbs: map[string]*ast.FunctionBlockDecl{}}
	for _, f := range files {
		if f == nil {
			continue
		}
		for _, d := range f.Declarations {
			switch n := d.(type) {
			case *ast.TypeDecl:
				if n.Name != nil {
					idx.types[strings.ToUpper(n.Name.Name)] = n
				}
			case *ast.FunctionBlockDecl:
				if n.Name != nil {
					idx.fbs[strings.ToUpper(n.Name.Name)] = n
				}
			}
		}
	}
	return idx
}

// CollectLinks walks every GVL and PROGRAM in files and returns one
// LinkedVar per TcLinkTo-linked leaf in source order. It walks the AST
// rather than the checker so projects whose library types are not loaded
// can still be validated.
func CollectLinks(files []*ast.SourceFile) ([]LinkedVar, []diag.Diagnostic) {
	idx := newTypeIndex(files)
	c := &collector{idx: idx}
	for _, f := range files {
		if f == nil {
			continue
		}
		for _, d := range f.Declarations {
			switch n := d.(type) {
			case *ast.GVLDecl:
				c.blocks(identName(n.Name), n.Blocks)
			case *ast.ProgramDecl:
				c.blocks(identName(n.Name), n.VarBlocks)
			}
		}
	}
	return c.vars, c.diags
}

type collector struct {
	idx   *typeIndex
	vars  []LinkedVar
	diags []diag.Diagnostic
}

func identName(id *ast.Ident) string {
	if id == nil {
		return ""
	}
	return id.Name
}

func (c *collector) blocks(scope string, blocks []*ast.VarBlock) {
	for _, b := range blocks {
		for _, vd := range b.Declarations {
			for _, a := range vd.Attributes {
				if a == nil || !strings.EqualFold(a.Name, "TcLinkTo") {
					continue
				}
				for _, name := range vd.Names {
					c.decl(scope, name.Name, vd, a)
				}
			}
		}
	}
}

func (c *collector) addDiag(sev diag.Severity, a *ast.Attribute, code, format string, args ...any) {
	sp := a.Span()
	c.diags = append(c.diags, diag.Diagnostic{
		Severity: sev,
		Pos:      source.Pos(sp.Start),
		EndPos:   source.Pos(sp.End),
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
	})
}

func (c *collector) decl(scope, name string, vd *ast.VarDecl, a *ast.Attribute) {
	links, err := ParseTcLinkTo(a.Value)
	if err != nil {
		c.addDiag(diag.Error, a, CodeBadLink, "malformed TcLinkTo on %s.%s: %v", scope, name, err)
		return
	}
	for _, l := range links {
		path := []string{scope, name}
		spec, at := vd.Type, vd.AtAddress
		if l.Member != "" {
			segs := strings.Split(l.Member, ".")
			if len(segs) > maxMemberDepth {
				c.addDiag(diag.Error, a, CodeMemberNotDeclared, "member path .%s on %s.%s is too deep (max %d)", l.Member, scope, name, maxMemberDepth)
				continue
			}
			ok := true
			for _, seg := range segs {
				m, ok2 := c.idx.member(spec, seg)
				if !ok2 {
					c.addDiag(diag.Error, a, CodeMemberNotDeclared, "member %q of .%s is not declared by %s (%s)", seg, l.Member, strings.Join(path, "."), typeName(spec))
					ok = false
					break
				}
				path = append(path, m.name)
				spec, at = m.spec, m.at
			}
			if !ok {
				continue
			}
		}
		lv := LinkedVar{
			Path:     strings.Join(path, "."),
			Link:     l.Target,
			TypeName: typeName(spec),
			BitWidth: c.idx.width(spec, 0),
			Pos:      source.Pos(a.Span().Start),
			EndPos:   source.Pos(a.Span().End),
		}
		for _, p := range path {
			lv.Steps = append(lv.Steps, strings.ToUpper(p))
		}
		if t, ok := types.LookupElementaryType(lv.TypeName); ok {
			lv.Type = t
		}
		if at != nil {
			if addr, err := iomap.ParseAddress(at.Name); err == nil {
				switch addr.Area {
				case iomap.AreaInput:
					lv.Dir, lv.HasAT = DirIn, true
				case iomap.AreaOutput:
					lv.Dir, lv.HasAT = DirOut, true
				}
			}
		}
		if !lv.HasAT {
			c.addDiag(diag.Warning, a, CodeNoAT, "linked variable %s has no AT %%I*/%%Q* declaration", lv.Path)
		}
		c.vars = append(c.vars, lv)
	}
}

// memberInfo is a resolved member of a struct or FB.
type memberInfo struct {
	name string
	spec ast.TypeSpec
	at   *ast.Ident
}

// member finds seg (case-insensitive) in the struct or FB that spec names,
// following type aliases and the FB EXTENDS chain.
func (idx *typeIndex) member(spec ast.TypeSpec, seg string) (memberInfo, bool) {
	for depth := 0; depth <= maxMemberDepth; depth++ {
		switch s := spec.(type) {
		case *ast.StructType:
			for _, m := range s.Members {
				if m.Name != nil && strings.EqualFold(m.Name.Name, seg) {
					return memberInfo{m.Name.Name, m.Type, m.AtAddress}, true
				}
			}
			return memberInfo{}, false
		case *ast.NamedType:
			key := strings.ToUpper(identName(s.Name))
			if td, ok := idx.types[key]; ok {
				spec = td.Type
				continue
			}
			if fb, ok := idx.fbs[key]; ok {
				return idx.fbMember(fb, seg)
			}
			return memberInfo{}, false
		default:
			return memberInfo{}, false
		}
	}
	return memberInfo{}, false
}

func (idx *typeIndex) fbMember(fb *ast.FunctionBlockDecl, seg string) (memberInfo, bool) {
	visited := map[*ast.FunctionBlockDecl]bool{}
	for fb != nil && !visited[fb] {
		visited[fb] = true
		for _, b := range fb.VarBlocks {
			for _, vd := range b.Declarations {
				for _, n := range vd.Names {
					if strings.EqualFold(n.Name, seg) {
						return memberInfo{n.Name, vd.Type, vd.AtAddress}, true
					}
				}
			}
		}
		if fb.Extends == nil {
			break
		}
		fb = idx.fbs[strings.ToUpper(fb.Extends.Name)]
	}
	return memberInfo{}, false
}

// typeName is the declared name of a leaf type for display and lookup.
func typeName(spec ast.TypeSpec) string {
	switch s := spec.(type) {
	case *ast.NamedType:
		return identName(s.Name)
	case *ast.StringType:
		if s.IsWide {
			return "WSTRING"
		}
		return "STRING"
	case *ast.ArrayType:
		return "ARRAY"
	case *ast.StructType:
		return "STRUCT"
	case *ast.EnumType:
		return "ENUM"
	}
	return ""
}

// fixedWidths are the EtherCAT link widths of elementary and Tc2 types.
var fixedWidths = map[string]int{
	"BOOL": 1, "BIT": 1,
	"BYTE": 8, "SINT": 8, "USINT": 8,
	"INT": 16, "UINT": 16, "WORD": 16,
	"DINT": 32, "UDINT": 32, "DWORD": 32, "REAL": 32,
	"LINT": 64, "ULINT": 64, "LWORD": 64, "LREAL": 64,
	"T_AMSNETIDARR": 48, "AMSNETID": 48, "T_AMSNETID": 48,
	"AMSADDR": 64,
}

// namedWidth returns the bit width of a named type, 0 when unknown.
func (idx *typeIndex) namedWidth(name string, depth int) int {
	key := strings.ToUpper(name)
	if w, ok := fixedWidths[key]; ok {
		return w
	}
	if td, ok := idx.types[key]; ok {
		return idx.width(td.Type, depth+1)
	}
	return 0
}

// width returns the bit width of spec, 0 when unknown or not linkable.
func (idx *typeIndex) width(spec ast.TypeSpec, depth int) int {
	if depth > maxMemberDepth {
		return 0
	}
	switch s := spec.(type) {
	case *ast.NamedType:
		return idx.namedWidth(identName(s.Name), depth)
	case *ast.SubrangeType:
		return idx.width(s.BaseType, depth+1)
	case *ast.EnumType:
		if s.BaseType == nil {
			return 16
		}
		return idx.width(s.BaseType, depth+1)
	case *ast.StructType:
		total := 0
		for _, m := range s.Members {
			w := idx.width(m.Type, depth+1)
			if w == 0 {
				return 0
			}
			total += w
		}
		return total
	case *ast.ArrayType:
		count := 1
		for _, r := range s.Ranges {
			lo, ok1 := constInt(r.Low)
			hi, ok2 := constInt(r.High)
			if !ok1 || !ok2 || hi < lo {
				return 0
			}
			count *= int(hi - lo + 1)
		}
		return count * idx.width(s.ElementType, depth+1)
	}
	return 0
}

// constInt evaluates an integer literal, optionally negated.
func constInt(e ast.Expr) (int64, bool) {
	switch n := e.(type) {
	case *ast.Literal:
		if n.LitKind != ast.LitInt {
			return 0, false
		}
		v := strings.ReplaceAll(n.Value, "_", "")
		base := 10
		if b, digits, ok := strings.Cut(v, "#"); ok {
			p, err := strconv.Atoi(b)
			if err != nil {
				return 0, false
			}
			base, v = p, digits
		}
		i, err := strconv.ParseInt(v, base, 64)
		return i, err == nil
	case *ast.UnaryExpr:
		if n.Op.Text == "-" {
			i, ok := constInt(n.Operand)
			return -i, ok
		}
	}
	return 0, false
}
