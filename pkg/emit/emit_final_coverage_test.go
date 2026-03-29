package emit

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
)

func TestEmit_InterfaceWithExtendsAndProps(t *testing.T) {
	file := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.InterfaceDecl{
				Name:    &ast.Ident{Name: "IMotor"},
				Extends: []*ast.Ident{{Name: "IBase"}, {Name: "ILoggable"}},
				Methods: []*ast.MethodSignature{
					{Name: &ast.Ident{Name: "Start"}},
				},
				Properties: []*ast.PropertySignature{
					{Name: &ast.Ident{Name: "speed"}, Type: &ast.NamedType{Name: &ast.Ident{Name: "REAL"}}},
				},
			},
		},
	}
	out := Emit(file, Options{Target: TargetBeckhoff})
	if !strings.Contains(strings.ToLower(out), "extends ibase, iloggable") {
		t.Errorf("missing EXTENDS: %s", out)
	}
	if !strings.Contains(strings.ToLower(out), "speed") {
		t.Error("missing property signature")
	}
}

func TestEmit_InterfaceSchneiderSkip(t *testing.T) {
	file := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.InterfaceDecl{Name: &ast.Ident{Name: "IMotor"}},
		},
	}
	out := Emit(file, Options{Target: TargetSchneider})
	if strings.Contains(strings.ToLower(out), "interface") {
		t.Error("Schneider should skip interface")
	}
}

func TestEmit_PropertyWithAccessAndGetSet(t *testing.T) {
	file := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.FunctionBlockDecl{
				Name: &ast.Ident{Name: "FB_Test"},
				Properties: []*ast.PropertyDecl{
					{
						AccessModifier: ast.AccessPublic,
						Name:           &ast.Ident{Name: "Value"},
						Type:           &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
						Getter:         &ast.MethodDecl{Name: &ast.Ident{Name: "GET"}, Body: []ast.Statement{&ast.ReturnStmt{}}},
						Setter:         &ast.MethodDecl{Name: &ast.Ident{Name: "SET"}, Body: []ast.Statement{&ast.ReturnStmt{}}},
					},
				},
			},
		},
	}
	out := Emit(file, Options{Target: TargetBeckhoff})
	if !strings.Contains(strings.ToLower(out), "property public value : int") {
		t.Errorf("missing property decl: %s", out)
	}
}

func TestEmit_PropertyNoType(t *testing.T) {
	file := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.FunctionBlockDecl{
				Name: &ast.Ident{Name: "FB_Test"},
				Properties: []*ast.PropertyDecl{
					{Name: &ast.Ident{Name: "X"}},
				},
			},
		},
	}
	out := Emit(file, Options{Target: TargetBeckhoff})
	if !strings.Contains(strings.ToLower(out), "property x") {
		t.Errorf("missing property: %s", out)
	}
}

func TestEmit_VarDeclNilType(t *testing.T) {
	file := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "Main"},
				VarBlocks: []*ast.VarBlock{{
					Section: ast.VarLocal,
					Declarations:   []*ast.VarDecl{{Names: []*ast.Ident{{Name: "x"}}}},
				}},
			},
		},
	}
	out := Emit(file, Options{Target: TargetPortable})
	if !strings.Contains(strings.ToLower(out), "x") {
		t.Error("should emit var even with nil type")
	}
}

func TestEmit_CallStmtOutputArgs(t *testing.T) {
	file := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "Main"},
				Body: []ast.Statement{
					&ast.CallStmt{
						Callee: &ast.Ident{Name: "timer"},
						Args: []*ast.CallArg{
							{Name: &ast.Ident{Name: "IN"}, Value: &ast.Literal{Value: "TRUE", LitKind: ast.LitBool}},
							{Name: &ast.Ident{Name: "Q"}, Value: &ast.Ident{Name: "done"}, IsOutput: true},
						},
					},
				},
			},
		},
	}
	out := Emit(file, Options{Target: TargetBeckhoff})
	if !strings.Contains(strings.ToLower(out), "q => done") {
		t.Errorf("missing output arg: %s", out)
	}
}

func TestEmit_TypeSpecErrorNode(t *testing.T) {
	file := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "Main"},
				VarBlocks: []*ast.VarBlock{{
					Section: ast.VarLocal,
					Declarations:   []*ast.VarDecl{{Names: []*ast.Ident{{Name: "x"}}, Type: &ast.ErrorNode{}}},
				}},
			},
		},
	}
	out := Emit(file, Options{Target: TargetBeckhoff})
	_ = out // just ensure no panic
}

func TestEmit_WideStringWithLen(t *testing.T) {
	file := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "Main"},
				VarBlocks: []*ast.VarBlock{{
					Section: ast.VarLocal,
					Declarations: []*ast.VarDecl{{
						Names: []*ast.Ident{{Name: "ws"}},
						Type:  &ast.StringType{IsWide: true, Length: &ast.Literal{Value: "100", LitKind: ast.LitInt}},
					}},
				}},
			},
		},
	}
	out := Emit(file, Options{Target: TargetBeckhoff})
	if !strings.Contains(strings.ToLower(out), "wstring(100)") {
		t.Errorf("missing WSTRING: %s", out)
	}
}

func TestEmit_MultiDimArray(t *testing.T) {
	file := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "Main"},
				VarBlocks: []*ast.VarBlock{{
					Section: ast.VarLocal,
					Declarations: []*ast.VarDecl{{
						Names: []*ast.Ident{{Name: "m"}},
						Type: &ast.ArrayType{
							Ranges: []*ast.SubrangeSpec{
								{Low: &ast.Literal{Value: "1", LitKind: ast.LitInt}, High: &ast.Literal{Value: "10", LitKind: ast.LitInt}},
								{Low: &ast.Literal{Value: "1", LitKind: ast.LitInt}, High: &ast.Literal{Value: "5", LitKind: ast.LitInt}},
							},
							ElementType: &ast.NamedType{Name: &ast.Ident{Name: "REAL"}},
						},
					}},
				}},
			},
		},
	}
	out := Emit(file, Options{Target: TargetBeckhoff})
	if !strings.Contains(strings.ToLower(out), "1..10, 1..5") {
		t.Errorf("missing multi-dim: %s", out)
	}
}

func TestEmit_StructWithInitValues(t *testing.T) {
	file := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.TypeDecl{
				Name: &ast.Ident{Name: "S"},
				Type: &ast.StructType{
					Members: []*ast.StructMember{{
						Name:      &ast.Ident{Name: "x"},
						Type:      &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
						InitValue: &ast.Literal{Value: "42", LitKind: ast.LitInt},
					}},
				},
			},
		},
	}
	out := Emit(file, Options{Target: TargetBeckhoff})
	if !strings.Contains(strings.ToLower(out), "x : int := 42") {
		t.Errorf("missing struct init: %s", out)
	}
}

func TestEmit_MethodFinalOverride(t *testing.T) {
	file := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.FunctionBlockDecl{
				Name: &ast.Ident{Name: "FB"},
				Methods: []*ast.MethodDecl{
					{Name: &ast.Ident{Name: "M1"}, IsFinal: true},
					{Name: &ast.Ident{Name: "M2"}, IsOverride: true, ReturnType: &ast.NamedType{Name: &ast.Ident{Name: "BOOL"}}},
				},
			},
		},
	}
	out := Emit(file, Options{Target: TargetBeckhoff})
	if !strings.Contains(strings.ToLower(out), "final") {
		t.Error("missing FINAL")
	}
	if !strings.Contains(strings.ToLower(out), "override") {
		t.Error("missing OVERRIDE")
	}
}

func TestEmit_EnumWithValues(t *testing.T) {
	file := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.TypeDecl{
				Name: &ast.Ident{Name: "Color"},
				Type: &ast.EnumType{
					Values: []*ast.EnumValue{
						{Name: &ast.Ident{Name: "Red"}, Value: &ast.Literal{Value: "0", LitKind: ast.LitInt}},
						{Name: &ast.Ident{Name: "Green"}, Value: &ast.Literal{Value: "1", LitKind: ast.LitInt}},
						{Name: &ast.Ident{Name: "blue"}},
					},
				},
			},
		},
	}
	out := Emit(file, Options{Target: TargetBeckhoff})
	if !strings.Contains(strings.ToLower(out), "red := 0") {
		t.Errorf("missing enum init: %s", out)
	}
	if !strings.Contains(strings.ToLower(out), "blue") {
		t.Error("missing enum value without init")
	}
}
