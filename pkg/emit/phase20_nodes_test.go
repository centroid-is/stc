package emit

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
)

// p20 helpers build Phase 20 nodes by hand; the parser does not produce
// them yet (plans 20-02 and 20-03).
func p20Id(name string) *ast.Ident {
	return &ast.Ident{NodeBase: ast.NodeBase{NodeKind: ast.KindIdent}, Name: name}
}

func p20Int(v string) *ast.Literal {
	return &ast.Literal{NodeBase: ast.NodeBase{NodeKind: ast.KindLiteral}, LitKind: ast.LitInt, Value: v}
}

func p20Str(v string) *ast.Literal {
	return &ast.Literal{NodeBase: ast.NodeBase{NodeKind: ast.KindLiteral}, LitKind: ast.LitString, Value: v}
}

func p20Member(obj ast.Expr, member string) *ast.MemberAccessExpr {
	return &ast.MemberAccessExpr{NodeBase: ast.NodeBase{NodeKind: ast.KindMemberAccessExpr}, Object: obj, Member: p20Id(member)}
}

func p20Bit(target ast.Expr, idx string) *ast.BitAccessExpr {
	return &ast.BitAccessExpr{NodeBase: ast.NodeBase{NodeKind: ast.KindBitAccessExpr}, Target: target, Index: p20Int(idx)}
}

func p20Assign(target, value ast.Expr) *ast.AssignStmt {
	return &ast.AssignStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindAssignStmt}, Target: target, Value: value}
}

func p20Named(name string, v ast.Expr, out bool) *ast.CallArg {
	a := &ast.CallArg{NodeBase: ast.NodeBase{NodeKind: ast.KindCallArg}, Value: v, IsOutput: out}
	if name != "" {
		a.Name = p20Id(name)
	}
	return a
}

func p20Field(name string, v ast.Expr) *ast.FieldInit {
	return &ast.FieldInit{NodeBase: ast.NodeBase{NodeKind: ast.KindFieldInit}, Name: p20Id(name), Value: v}
}

func p20StructInit(fields ...*ast.FieldInit) *ast.StructInit {
	return &ast.StructInit{NodeBase: ast.NodeBase{NodeKind: ast.KindStructInit}, Fields: fields}
}

func p20Elem(count ast.Expr, v ast.Expr) *ast.ArrayInitElem {
	return &ast.ArrayInitElem{NodeBase: ast.NodeBase{NodeKind: ast.KindArrayInitElem}, Count: count, Value: v}
}

func p20ArrayInit(elems ...*ast.ArrayInitElem) *ast.ArrayInit {
	return &ast.ArrayInit{NodeBase: ast.NodeBase{NodeKind: ast.KindArrayInit}, Elements: elems}
}

func p20Named_T(name string) *ast.NamedType {
	return &ast.NamedType{NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType}, Name: p20Id(name)}
}

func p20Var(name string, typ ast.TypeSpec, init ast.Expr) *ast.VarDecl {
	return &ast.VarDecl{NodeBase: ast.NodeBase{NodeKind: ast.KindVarDecl}, Names: []*ast.Ident{p20Id(name)}, Type: typ, InitValue: init}
}

func p20Enum(base ast.TypeSpec, vals ...*ast.EnumValue) *ast.EnumType {
	return &ast.EnumType{NodeBase: ast.NodeBase{NodeKind: ast.KindEnumType}, BaseType: base, Values: vals}
}

func p20EnumVal(name string, v ast.Expr) *ast.EnumValue {
	return &ast.EnumValue{Name: p20Id(name), Value: v}
}

// p20File builds one source file exercising every Phase 20 node.
func p20File() *ast.SourceFile {
	this := &ast.DerefExpr{NodeBase: ast.NodeBase{NodeKind: ast.KindDerefExpr},
		Operand: &ast.ThisExpr{NodeBase: ast.NodeBase{NodeKind: ast.KindThisExpr}}}
	super := &ast.DerefExpr{NodeBase: ast.NodeBase{NodeKind: ast.KindDerefExpr},
		Operand: &ast.SuperExpr{NodeBase: ast.NodeBase{NodeKind: ast.KindSuperExpr}}}
	arrIdx := &ast.IndexExpr{NodeBase: ast.NodeBase{NodeKind: ast.KindIndexExpr}, Object: p20Id("arr"), Indices: []ast.Expr{p20Int("0")}}
	call := &ast.CallExpr{NodeBase: ast.NodeBase{NodeKind: ast.KindCallExpr}, Callee: p20Id("F"),
		Args:      []ast.Expr{p20Int("1")},
		NamedArgs: []*ast.CallArg{p20Named("b", p20Int("2"), false), p20Named("c", p20Id("x"), true)}}
	emptyArg := &ast.CallExpr{NodeBase: ast.NodeBase{NodeKind: ast.KindCallExpr}, Callee: p20Id("G"),
		NamedArgs: []*ast.CallArg{p20Named("a", nil, false), p20Named("", p20Int("7"), false)}}
	superCall := &ast.CallExpr{NodeBase: ast.NodeBase{NodeKind: ast.KindCallExpr}, Callee: p20Member(super, "M")}
	refAssign := &ast.RefAssignStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindRefAssignStmt,
		TrailingTrivia: []ast.Trivia{{Kind: ast.TriviaLineComment, Text: "// rebind"}}},
		Target: p20Id("r"), Value: p20Id("x")}
	nsType := &ast.NamedType{NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType}, Namespace: p20Id("Tc2_EtherCAT"), Name: p20Id("ST_EcSlaveState")}
	arrInit := p20ArrayInit(
		p20Elem(nil, p20Int("1")),
		p20Elem(nil, p20Int("2")),
		p20Elem(p20Int("3"), p20Int("0")),
		p20Elem(nil, p20StructInit(p20Field("a", p20Int("1")))),
	)
	nested := p20ArrayInit(
		p20Elem(nil, p20ArrayInit(p20Elem(nil, p20Int("1")), p20Elem(nil, p20Int("2")))),
		p20Elem(nil, p20ArrayInit(p20Elem(nil, p20Int("3")), p20Elem(nil, p20Int("4")))),
	)
	prog := &ast.ProgramDecl{NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl}, Name: p20Id("Main"),
		VarBlocks: []*ast.VarBlock{{NodeBase: ast.NodeBase{NodeKind: ast.KindVarBlock}, Section: ast.VarLocal,
			Declarations: []*ast.VarDecl{
				p20Var("s", p20Named_T("ST_X"), p20StructInit(p20Field("a", p20Int("1")), p20Field("b", p20Str("'x'")))),
				p20Var("arr", p20Named_T("ARR_T"), arrInit),
				p20Var("m", p20Named_T("MAT_T"), nested),
				p20Var("ec", nsType, nil),
				p20Var("e", p20Enum(p20Named_T("UINT"), p20EnumVal("a", p20Int("0")), p20EnumVal("b", p20Int("1"))), nil),
			}}},
		Body: []ast.Statement{
			p20Assign(p20Id("q"), p20Bit(p20Id("w"), "3")),
			p20Assign(p20Bit(p20Member(p20Id("a"), "b"), "0"), p20Bit(arrIdx, "3")),
			p20Assign(p20Id("n"), call),
			p20Assign(p20Id("k"), emptyArg),
			refAssign,
			p20Assign(p20Member(this, "x"), p20Int("1")),
			p20Assign(superCall, nil),
		},
	}
	enumDecl := &ast.TypeDecl{NodeBase: ast.NodeBase{NodeKind: ast.KindTypeDecl}, Name: p20Id("E"),
		Type:      p20Enum(p20Named_T("UINT"), p20EnumVal("a", p20Int("0")), p20EnumVal("b", nil)),
		InitValue: p20Id("b")}
	plainEnumDecl := &ast.TypeDecl{NodeBase: ast.NodeBase{NodeKind: ast.KindTypeDecl}, Name: p20Id("E2"),
		Type:      p20Enum(nil, p20EnumVal("a", nil), p20EnumVal("b", nil)),
		InitValue: p20Id("b")}
	aliasDecl := &ast.TypeDecl{NodeBase: ast.NodeBase{NodeKind: ast.KindTypeDecl}, Name: p20Id("T_Count"),
		Type: p20Named_T("INT"), InitValue: p20Int("5")}
	return &ast.SourceFile{NodeBase: ast.NodeBase{NodeKind: ast.KindSourceFile},
		Declarations: []ast.Declaration{enumDecl, plainEnumDecl, aliasDecl, prog}}
}

// p20Want lists the exact printed substrings for every Phase 20 node.
var p20Want = []string{
	"TYPE E :", "a := 0,", "b", ") UINT := b;", "END_TYPE",
	"TYPE E2 :", ") := b;",
	"TYPE T_Count :", "INT := 5;",
	"s : ST_X := (a := 1, b := 'x');",
	"arr : ARR_T := [1, 2, 3(0), (a := 1)];",
	"m : MAT_T := [[1, 2], [3, 4]];",
	"ec : Tc2_EtherCAT.ST_EcSlaveState;",
	"e : (a := 0, b := 1) UINT;",
	"q := w.3;",
	"a.b.0 := arr[0].3;",
	"n := F(1, b := 2, c => x);",
	"k := G(a :=, 7);",
	"r REF= x; // rebind",
	"THIS^.x := 1;",
	"SUPER^.M();",
}

func TestPhase20Nodes(t *testing.T) {
	// The RefAssign trailing comment is not printed by emit.
	want := make([]string, 0, len(p20Want))
	for _, w := range p20Want {
		want = append(want, strings.TrimSuffix(w, " // rebind"))
	}
	beckhoff := Emit(p20File(), DefaultOptions())
	for _, target := range []Target{TargetBeckhoff, TargetSchneider, TargetPortable} {
		t.Run(string(target), func(t *testing.T) {
			opts := DefaultOptions()
			opts.Target = target
			out := Emit(p20File(), opts)
			for _, w := range want {
				if !strings.Contains(out, w) {
					t.Errorf("missing %q in:\n%s", w, out)
				}
			}
			assertEmitInOrder(t, out, want...)
			if out != beckhoff {
				t.Errorf("%s output differs from beckhoff:\n%s\n---\n%s", target, out, beckhoff)
			}
		})
	}
	t.Run("lowercase keywords", func(t *testing.T) {
		opts := DefaultOptions()
		opts.UppercaseKeywords = false
		low := Emit(p20File(), opts)
		for _, w := range []string{"r ref= x;", "this^.x := 1;", "super^.M();"} {
			if !strings.Contains(low, w) {
				t.Errorf("missing %q in:\n%s", w, low)
			}
		}
	})
}
