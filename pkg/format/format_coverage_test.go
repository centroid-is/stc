package format

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
)

func TestFormat_EmptyIndentDefault(t *testing.T) {
	// When Indent is empty, should default to 4 spaces
	input := `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := 42;
END_PROGRAM
`
	opts := FormatOptions{Indent: "", UppercaseKeywords: true}
	got := formatSTWith(input, opts)
	if !strings.Contains(got, "    x : INT;") {
		t.Errorf("expected default 4-space indent, got:\n%s", got)
	}
}

func TestFormat_InterfaceDecl(t *testing.T) {
	input := `INTERFACE IRunnable EXTENDS IBase
    METHOD Run : BOOL;
    PROPERTY Name : STRING;
END_INTERFACE
`
	got := formatST(input)
	if !strings.Contains(got, "INTERFACE IRunnable") {
		t.Errorf("expected INTERFACE, got:\n%s", got)
	}
	if !strings.Contains(got, "END_INTERFACE") {
		t.Errorf("expected END_INTERFACE, got:\n%s", got)
	}
}

func TestFormat_MethodDecl(t *testing.T) {
	input := `FUNCTION_BLOCK FB_Motor
METHOD PUBLIC Start : BOOL
VAR_INPUT
    speed : INT;
END_VAR
    Start := TRUE;
END_METHOD
END_FUNCTION_BLOCK
`
	got := formatST(input)
	if !strings.Contains(got, "METHOD") {
		t.Errorf("expected METHOD, got:\n%s", got)
	}
	if !strings.Contains(got, "END_METHOD") {
		t.Errorf("expected END_METHOD, got:\n%s", got)
	}
}

func TestFormat_PropertyDecl(t *testing.T) {
	input := `FUNCTION_BLOCK FB_Motor
PROPERTY PUBLIC Speed : INT
METHOD Get : INT
    Get := 0;
END_METHOD
METHOD Set
VAR_INPUT
    value : INT;
END_VAR
END_METHOD
END_PROPERTY
END_FUNCTION_BLOCK
`
	got := formatST(input)
	if !strings.Contains(got, "PROPERTY") {
		t.Errorf("expected PROPERTY, got:\n%s", got)
	}
	if !strings.Contains(got, "END_PROPERTY") {
		t.Errorf("expected END_PROPERTY, got:\n%s", got)
	}
}

func TestFormat_ReturnStmt(t *testing.T) {
	input := `FUNCTION Foo : BOOL
    RETURN;
END_FUNCTION
`
	got := formatST(input)
	if !strings.Contains(got, "RETURN;") {
		t.Errorf("expected RETURN;, got:\n%s", got)
	}
}

func TestFormat_ExitStmt(t *testing.T) {
	input := `PROGRAM Main
VAR
    i : INT;
END_VAR
    FOR i := 0 TO 10 DO
        EXIT;
    END_FOR;
END_PROGRAM
`
	got := formatST(input)
	if !strings.Contains(got, "EXIT;") {
		t.Errorf("expected EXIT;, got:\n%s", got)
	}
}

func TestFormat_ContinueStmt(t *testing.T) {
	input := `PROGRAM Main
VAR
    i : INT;
END_VAR
    FOR i := 0 TO 10 DO
        CONTINUE;
    END_FOR;
END_PROGRAM
`
	got := formatST(input)
	if !strings.Contains(got, "CONTINUE;") {
		t.Errorf("expected CONTINUE;, got:\n%s", got)
	}
}

func TestFormat_EmptyStmt(t *testing.T) {
	// Build an AST directly with EmptyStmt since the parser may not produce one
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "Main"},
				Body: []ast.Statement{
					&ast.EmptyStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindEmptyStmt}},
				},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, ";") {
		t.Errorf("expected semicolon, got:\n%s", got)
	}
}

func TestFormat_CallStmt(t *testing.T) {
	// Manually build an AST with a CallStmt to test formatting
	sf := &ast.SourceFile{
		NodeBase: ast.NodeBase{NodeKind: ast.KindSourceFile},
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
				Name:     &ast.Ident{Name: "Main"},
				Body: []ast.Statement{
					&ast.CallStmt{
						NodeBase: ast.NodeBase{NodeKind: ast.KindCallStmt},
						Callee:   &ast.Ident{Name: "timer"},
						Args: []*ast.CallArg{
							{
								Name:  &ast.Ident{Name: "enable"},
								Value: &ast.Literal{LitKind: ast.LitBool, Value: "TRUE"},
							},
							{
								Name:     &ast.Ident{Name: "done"},
								Value:    &ast.Ident{Name: "isDone"},
								IsOutput: true,
							},
						},
					},
				},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "enable := TRUE") {
		t.Errorf("expected input assignment, got:\n%s", got)
	}
	if !strings.Contains(got, "done => isDone") {
		t.Errorf("expected output assignment, got:\n%s", got)
	}
}

func TestFormat_BinaryExpr_WordOps(t *testing.T) {
	tests := []struct {
		op       string
		expected string
	}{
		{"AND", "AND"},
		{"OR", "OR"},
		{"XOR", "XOR"},
		{"MOD", "MOD"},
	}
	for _, tt := range tests {
		t.Run(tt.op, func(t *testing.T) {
			sf := &ast.SourceFile{
				Declarations: []ast.Declaration{
					&ast.ProgramDecl{
						Name: &ast.Ident{Name: "P"},
						Body: []ast.Statement{
							&ast.AssignStmt{
								Target: &ast.Ident{Name: "x"},
								Value: &ast.BinaryExpr{
									Left:  &ast.Ident{Name: "a"},
									Op:    ast.Token{Text: tt.op},
									Right: &ast.Ident{Name: "b"},
								},
							},
						},
					},
				},
			}
			got := Format(sf, DefaultFormatOptions())
			if !strings.Contains(got, tt.expected) {
				t.Errorf("expected %s operator in output, got:\n%s", tt.expected, got)
			}
		})
	}
}

func TestFormat_UnaryExpr_NOT(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				Body: []ast.Statement{
					&ast.AssignStmt{
						Target: &ast.Ident{Name: "x"},
						Value: &ast.UnaryExpr{
							Op:      ast.Token{Text: "NOT"},
							Operand: &ast.Ident{Name: "flag"},
						},
					},
				},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "NOT flag") {
		t.Errorf("expected 'NOT flag', got:\n%s", got)
	}
}

func TestFormat_UnaryExpr_Minus(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				Body: []ast.Statement{
					&ast.AssignStmt{
						Target: &ast.Ident{Name: "x"},
						Value: &ast.UnaryExpr{
							Op:      ast.Token{Text: "-"},
							Operand: &ast.Ident{Name: "y"},
						},
					},
				},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "-y") {
		t.Errorf("expected '-y', got:\n%s", got)
	}
}

func TestFormat_CallExpr(t *testing.T) {
	input := `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := ADD(1, 2);
END_PROGRAM
`
	got := formatST(input)
	if !strings.Contains(got, "ADD(") {
		t.Errorf("expected function call, got:\n%s", got)
	}
}

func TestFormat_MemberAccessExpr(t *testing.T) {
	input := `
TYPE S_Point :
STRUCT
    x : REAL;
    y : REAL;
END_STRUCT;
END_TYPE

PROGRAM Main
VAR
    pt : S_Point;
    px : REAL;
END_VAR
    px := pt.x;
END_PROGRAM
`
	got := formatST(input)
	if !strings.Contains(got, "pt.x") {
		t.Errorf("expected member access, got:\n%s", got)
	}
}

func TestFormat_IndexExpr(t *testing.T) {
	input := `PROGRAM Main
VAR
    arr : ARRAY[0..9] OF INT;
    i : INT;
END_VAR
    i := arr[0];
END_PROGRAM
`
	got := formatST(input)
	if !strings.Contains(got, "arr[0]") {
		t.Errorf("expected index expression, got:\n%s", got)
	}
}

func TestFormat_DerefExpr(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				Body: []ast.Statement{
					&ast.AssignStmt{
						Target: &ast.Ident{Name: "x"},
						Value: &ast.DerefExpr{
							Operand: &ast.Ident{Name: "ptr"},
						},
					},
				},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "ptr^") {
		t.Errorf("expected ptr^, got:\n%s", got)
	}
}

func TestFormat_ParenExpr(t *testing.T) {
	input := `PROGRAM Main
VAR
    x : INT;
END_VAR
    x := (1 + 2) * 3;
END_PROGRAM
`
	got := formatST(input)
	// Should contain parenthesized sub-expr
	if !strings.Contains(got, "(") || !strings.Contains(got, ")") {
		t.Errorf("expected parentheses, got:\n%s", got)
	}
}

func TestFormat_TypedLiteral(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				Body: []ast.Statement{
					&ast.AssignStmt{
						Target: &ast.Ident{Name: "x"},
						Value: &ast.Literal{
							LitKind:    ast.LitTyped,
							Value:      "5",
							TypePrefix: "INT",
						},
					},
				},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "INT#5") {
		t.Errorf("expected INT#5, got:\n%s", got)
	}
}

func TestFormat_PointerType(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				VarBlocks: []*ast.VarBlock{{
					Section: ast.VarLocal,
					Declarations: []*ast.VarDecl{{
						Names: []*ast.Ident{{Name: "p"}},
						Type: &ast.PointerType{
							BaseType: &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
						},
					}},
				}},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "POINTER TO") {
		t.Errorf("expected POINTER TO, got:\n%s", got)
	}
}

func TestFormat_ReferenceType(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				VarBlocks: []*ast.VarBlock{{
					Section: ast.VarLocal,
					Declarations: []*ast.VarDecl{{
						Names: []*ast.Ident{{Name: "r"}},
						Type: &ast.ReferenceType{
							BaseType: &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
						},
					}},
				}},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "REFERENCE TO") {
		t.Errorf("expected REFERENCE TO, got:\n%s", got)
	}
}

func TestFormat_StringType(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				VarBlocks: []*ast.VarBlock{{
					Section: ast.VarLocal,
					Declarations: []*ast.VarDecl{
						{
							Names: []*ast.Ident{{Name: "s"}},
							Type: &ast.StringType{
								Length: &ast.Literal{Value: "255"},
							},
						},
						{
							Names: []*ast.Ident{{Name: "ws"}},
							Type:  &ast.StringType{IsWide: true},
						},
					},
				}},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "STRING(255)") {
		t.Errorf("expected STRING(255), got:\n%s", got)
	}
	if !strings.Contains(got, "WSTRING") {
		t.Errorf("expected WSTRING, got:\n%s", got)
	}
}

func TestFormat_SubrangeType(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				VarBlocks: []*ast.VarBlock{{
					Section: ast.VarLocal,
					Declarations: []*ast.VarDecl{{
						Names: []*ast.Ident{{Name: "x"}},
						Type: &ast.SubrangeType{
							BaseType: &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
							Low:      &ast.Literal{Value: "0"},
							High:     &ast.Literal{Value: "100"},
						},
					}},
				}},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "INT(0..100)") {
		t.Errorf("expected INT(0..100), got:\n%s", got)
	}
}

func TestFormat_EnumType_Inline(t *testing.T) {
	input := `TYPE E_Color : (Red, Green, Blue);
END_TYPE
`
	got := formatST(input)
	if !strings.Contains(got, "Red") {
		t.Errorf("expected enum values, got:\n%s", got)
	}
}

func TestFormat_VarBlock_Flags(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				VarBlocks: []*ast.VarBlock{{
					Section:      ast.VarGlobal,
					IsConstant:   true,
					IsRetain:     true,
					IsPersistent: true,
					Declarations: []*ast.VarDecl{{
						Names: []*ast.Ident{{Name: "x"}},
						Type:  &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
					}},
				}},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "CONSTANT") {
		t.Errorf("expected CONSTANT, got:\n%s", got)
	}
	if !strings.Contains(got, "RETAIN") {
		t.Errorf("expected RETAIN, got:\n%s", got)
	}
	if !strings.Contains(got, "PERSISTENT") {
		t.Errorf("expected PERSISTENT, got:\n%s", got)
	}
}

func TestFormat_VarDecl_MultiNames(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				VarBlocks: []*ast.VarBlock{{
					Section: ast.VarLocal,
					Declarations: []*ast.VarDecl{{
						Names:     []*ast.Ident{{Name: "a"}, {Name: "b"}, {Name: "c"}},
						Type:      &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
						InitValue: &ast.Literal{Value: "0"},
					}},
				}},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "a, b, c") {
		t.Errorf("expected multi-name declaration, got:\n%s", got)
	}
	if !strings.Contains(got, " := 0") {
		t.Errorf("expected init value, got:\n%s", got)
	}
}

func TestFormat_VarDecl_AtAddress(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				VarBlocks: []*ast.VarBlock{{
					Section: ast.VarLocal,
					Declarations: []*ast.VarDecl{{
						Names:     []*ast.Ident{{Name: "x"}},
						Type:      &ast.NamedType{Name: &ast.Ident{Name: "BOOL"}},
						AtAddress: &ast.Ident{Name: "%IX0.0"},
					}},
				}},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "AT %IX0.0") {
		t.Errorf("expected AT address, got:\n%s", got)
	}
}

func TestFormat_ActionDecl(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ActionDecl{
				Name: &ast.Ident{Name: "MyAction"},
				Body: []ast.Statement{
					&ast.AssignStmt{
						Target: &ast.Ident{Name: "x"},
						Value:  &ast.Literal{Value: "1"},
					},
				},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "ACTION MyAction") {
		t.Errorf("expected ACTION, got:\n%s", got)
	}
	if !strings.Contains(got, "END_ACTION") {
		t.Errorf("expected END_ACTION, got:\n%s", got)
	}
}

func TestFormat_TestCaseDecl(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.TestCaseDecl{
				NodeBase: ast.NodeBase{NodeKind: ast.KindTestCaseDecl},
				Name:     "my test",
				Body: []ast.Statement{
					&ast.AssignStmt{
						Target: &ast.Ident{Name: "x"},
						Value:  &ast.Literal{Value: "1"},
					},
				},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "TEST_CASE 'my test'") {
		t.Errorf("expected TEST_CASE, got:\n%s", got)
	}
	if !strings.Contains(got, "END_TEST_CASE") {
		t.Errorf("expected END_TEST_CASE, got:\n%s", got)
	}
}

func TestFormat_CaseLabel_Range(t *testing.T) {
	input := `PROGRAM Main
VAR
    x : INT;
    y : INT;
END_VAR
    CASE x OF
        1..5:
            y := 1;
    END_CASE;
END_PROGRAM
`
	got := formatST(input)
	if !strings.Contains(got, "..") {
		t.Errorf("expected range '..' in case label, got:\n%s", got)
	}
}

func TestFormat_LowercaseOps(t *testing.T) {
	opts := FormatOptions{Indent: "    ", UppercaseKeywords: false}
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				Body: []ast.Statement{
					&ast.AssignStmt{
						Target: &ast.Ident{Name: "x"},
						Value: &ast.BinaryExpr{
							Left:  &ast.Ident{Name: "a"},
							Op:    ast.Token{Text: "AND"},
							Right: &ast.Ident{Name: "b"},
						},
					},
				},
			},
		},
	}
	got := Format(sf, opts)
	if !strings.Contains(got, "and") {
		t.Errorf("expected lowercase 'and', got:\n%s", got)
	}
}

func TestFormat_ErrorNode_Skipped(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ErrorNode{
				NodeBase: ast.NodeBase{NodeKind: ast.KindErrorNode},
				Message:  "test error",
			},
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "Main"},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	// ErrorNode should be skipped, program should still appear
	if !strings.Contains(got, "PROGRAM Main") {
		t.Errorf("expected PROGRAM Main after skipped error, got:\n%s", got)
	}
}

func TestFormat_MultipleDeclarations(t *testing.T) {
	input := `PROGRAM P1
END_PROGRAM

PROGRAM P2
END_PROGRAM
`
	got := formatST(input)
	if !strings.Contains(got, "PROGRAM P1") {
		t.Errorf("expected P1, got:\n%s", got)
	}
	if !strings.Contains(got, "PROGRAM P2") {
		t.Errorf("expected P2, got:\n%s", got)
	}
}

func TestFormat_StructTypeBody(t *testing.T) {
	input := `TYPE S_Point :
STRUCT
    x : REAL := 0.0;
    y : REAL;
END_STRUCT
END_TYPE
`
	got := formatST(input)
	if !strings.Contains(got, ":= 0.0") {
		t.Errorf("expected struct member init value, got:\n%s", got)
	}
}

func TestFormat_EnumTypeBody(t *testing.T) {
	input := `TYPE E_State :
(
    Idle := 0,
    Running,
    Stopped
);
END_TYPE
`
	got := formatST(input)
	if !strings.Contains(got, "Idle") {
		t.Errorf("expected enum values, got:\n%s", got)
	}
}

func TestFormat_NilExpr(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				Body: []ast.Statement{
					&ast.AssignStmt{
						Target: &ast.Ident{Name: "x"},
						Value:  nil, // nil value
					},
				},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	// Should not panic
	if !strings.Contains(got, "PROGRAM P") {
		t.Errorf("expected program, got:\n%s", got)
	}
}

func TestFormat_NilTypeSpec(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				VarBlocks: []*ast.VarBlock{{
					Section: ast.VarLocal,
					Declarations: []*ast.VarDecl{{
						Names: []*ast.Ident{{Name: "x"}},
						Type:  nil, // nil type
					}},
				}},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	// Should not panic
	if !strings.Contains(got, "PROGRAM P") {
		t.Errorf("expected program, got:\n%s", got)
	}
}

func TestFormat_TrailingTriviaOnStmt(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				Body: []ast.Statement{
					&ast.AssignStmt{
						NodeBase: ast.NodeBase{
							NodeKind: ast.KindAssignStmt,
							TrailingTrivia: []ast.Trivia{
								{Kind: ast.TriviaBlockComment, Text: "(* trail *)"},
							},
						},
						Target: &ast.Ident{Name: "x"},
						Value:  &ast.Literal{Value: "42"},
					},
				},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "(* trail *)") {
		t.Errorf("expected trailing trivia, got:\n%s", got)
	}
}

func TestFormat_StructType_AsTypeSpec(t *testing.T) {
	// Test the emitTypeSpec path for StructType (inline, not type body)
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				VarBlocks: []*ast.VarBlock{{
					Section: ast.VarLocal,
					Declarations: []*ast.VarDecl{{
						Names: []*ast.Ident{{Name: "s"}},
						Type: &ast.StructType{
							Members: []*ast.StructMember{
								{Name: &ast.Ident{Name: "x"}, Type: &ast.NamedType{Name: &ast.Ident{Name: "INT"}}},
								{Name: &ast.Ident{Name: "y"}, Type: &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
									InitValue: &ast.Literal{Value: "0"}},
							},
						},
					}},
				}},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "STRUCT") {
		t.Errorf("expected STRUCT, got:\n%s", got)
	}
	if !strings.Contains(got, "END_STRUCT") {
		t.Errorf("expected END_STRUCT, got:\n%s", got)
	}
}

func TestFormat_EnumType_AsTypeSpec(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				VarBlocks: []*ast.VarBlock{{
					Section: ast.VarLocal,
					Declarations: []*ast.VarDecl{{
						Names: []*ast.Ident{{Name: "e"}},
						Type: &ast.EnumType{
							Values: []*ast.EnumValue{
								{Name: &ast.Ident{Name: "A"}},
								{Name: &ast.Ident{Name: "B"}, Value: &ast.Literal{Value: "1"}},
							},
						},
					}},
				}},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "A, B") {
		t.Errorf("expected enum values, got:\n%s", got)
	}
}

func TestFormat_ErrorNode_InExpr(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				Body: []ast.Statement{
					&ast.AssignStmt{
						Target: &ast.Ident{Name: "x"},
						Value:  &ast.ErrorNode{Message: "oops"},
					},
				},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	// ErrorNode in expr should be silently skipped
	if !strings.Contains(got, "x :=") {
		t.Errorf("expected assignment, got:\n%s", got)
	}
}

func TestFormat_ErrorNode_InTypeSpec(t *testing.T) {
	sf := &ast.SourceFile{
		Declarations: []ast.Declaration{
			&ast.ProgramDecl{
				Name: &ast.Ident{Name: "P"},
				VarBlocks: []*ast.VarBlock{{
					Section: ast.VarLocal,
					Declarations: []*ast.VarDecl{{
						Names: []*ast.Ident{{Name: "x"}},
						Type:  &ast.ErrorNode{Message: "type error"},
					}},
				}},
			},
		},
	}
	got := Format(sf, DefaultFormatOptions())
	if !strings.Contains(got, "PROGRAM P") {
		t.Errorf("expected program, got:\n%s", got)
	}
}
