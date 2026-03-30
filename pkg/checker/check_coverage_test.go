package checker

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/stretchr/testify/assert"
)

// --- Helper function tests ---

func TestIsArithmeticOp(t *testing.T) {
	assert.True(t, isArithmeticOp("+"))
	assert.True(t, isArithmeticOp("-"))
	assert.True(t, isArithmeticOp("*"))
	assert.True(t, isArithmeticOp("/"))
	assert.True(t, isArithmeticOp("MOD"))
	assert.True(t, isArithmeticOp("**"))
	assert.False(t, isArithmeticOp("AND"))
	assert.False(t, isArithmeticOp("="))
}

func TestIsComparisonOp(t *testing.T) {
	assert.True(t, isComparisonOp("="))
	assert.True(t, isComparisonOp("<>"))
	assert.True(t, isComparisonOp("<"))
	assert.True(t, isComparisonOp(">"))
	assert.True(t, isComparisonOp("<="))
	assert.True(t, isComparisonOp(">="))
	assert.False(t, isComparisonOp("+"))
	assert.False(t, isComparisonOp("AND"))
}

func TestIsBooleanOp(t *testing.T) {
	assert.True(t, isBooleanOp("AND"))
	assert.True(t, isBooleanOp("OR"))
	assert.True(t, isBooleanOp("XOR"))
	assert.True(t, isBooleanOp("&"))
	assert.False(t, isBooleanOp("+"))
	assert.False(t, isBooleanOp("="))
}

func TestExprName(t *testing.T) {
	assert.Equal(t, "x", exprName(&ast.Ident{Name: "x"}))
	assert.Equal(t, "", exprName(&ast.Literal{Value: "42"}))
	assert.Equal(t, "", exprName(nil))
}

func TestParamDirStr(t *testing.T) {
	assert.Equal(t, "output", paramDirStr(true))
	assert.Equal(t, "input", paramDirStr(false))
}

func TestIsLiteralExpr(t *testing.T) {
	assert.True(t, isLiteralExpr(&ast.Literal{Value: "42"}))
	assert.True(t, isLiteralExpr(&ast.UnaryExpr{
		Op:      ast.Token{Text: "-"},
		Operand: &ast.Literal{Value: "42"},
	}))
	assert.False(t, isLiteralExpr(&ast.Ident{Name: "x"}))
	assert.False(t, isLiteralExpr(&ast.UnaryExpr{
		Op:      ast.Token{Text: "-"},
		Operand: &ast.Ident{Name: "x"},
	}))
}

func TestIsLiteralCompatible(t *testing.T) {
	// Import types indirectly via the checker's usage
	src := `PROGRAM P
	VAR x : INT; END_VAR
		x := 42;
	END_PROGRAM`
	allDiags := runChecker(src)
	// No errors for literal INT assigned to INT
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

// --- Checker body tests for uncovered paths ---

func TestChecker_NilStmt(t *testing.T) {
	// Ensure nil stmt doesn't panic
	table := symbols.NewTable()
	diags := diag.NewCollector()
	c := NewChecker(table, diags)
	c.checkStmt(nil)
	assert.False(t, diags.HasErrors())
}

func TestChecker_NilExpr(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	c := NewChecker(table, diags)
	result := c.checkExpr(nil)
	assert.NotNil(t, result) // Should be types.Invalid
}

func TestChecker_WhileCondNotBool(t *testing.T) {
	src := `PROGRAM P
	VAR x : INT; END_VAR
		WHILE x DO
		END_WHILE;
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeTypeMismatch && d.Severity == diag.Error {
			found = true
		}
	}
	assert.True(t, found, "expected type error for INT WHILE condition")
}

func TestChecker_RepeatCondNotBool(t *testing.T) {
	src := `PROGRAM P
	VAR x : INT; END_VAR
		REPEAT
		UNTIL x
		END_REPEAT;
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeTypeMismatch && d.Severity == diag.Error {
			found = true
		}
	}
	assert.True(t, found, "expected type error for INT REPEAT condition")
}

func TestChecker_ElsifCondNotBool(t *testing.T) {
	src := `PROGRAM P
	VAR x : INT; b : BOOL; END_VAR
		IF b THEN
		ELSIF x THEN
		END_IF;
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeTypeMismatch && d.Severity == diag.Error {
			found = true
		}
	}
	assert.True(t, found, "expected type error for INT ELSIF condition")
}

func TestChecker_UnaryNot_NonBool(t *testing.T) {
	src := `PROGRAM P
	VAR x : INT; b : BOOL; END_VAR
		b := NOT x;
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeTypeMismatch {
			found = true
		}
	}
	assert.True(t, found, "expected error for NOT on INT")
}

func TestChecker_UnaryMinus_NonNumeric(t *testing.T) {
	src := `PROGRAM P
	VAR s : STRING; x : INT; END_VAR
		x := -s;
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeTypeMismatch {
			found = true
		}
	}
	assert.True(t, found, "expected error for unary minus on STRING")
}

func TestChecker_NotCallable(t *testing.T) {
	src := `PROGRAM P
	VAR x : INT; y : INT; END_VAR
		y := x(1, 2);
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeNotCallable {
			found = true
		}
	}
	assert.True(t, found, "expected not callable error")
}

func TestChecker_NotIndexable(t *testing.T) {
	src := `PROGRAM P
	VAR x : INT; y : INT; END_VAR
		y := x[0];
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeNotIndexable {
			found = true
		}
	}
	assert.True(t, found, "expected not indexable error")
}

func TestChecker_MemberAccessOnPrimitive(t *testing.T) {
	src := `PROGRAM P
	VAR x : INT; y : INT; END_VAR
		y := x.field;
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeNoMember {
			found = true
		}
	}
	assert.True(t, found, "expected no member error on primitive type")
}

func TestChecker_DerefExpr(t *testing.T) {
	src := `PROGRAM P
	VAR p : POINTER TO INT; x : INT; END_VAR
		x := p^;
	END_PROGRAM`
	allDiags := runChecker(src)
	// Should work without errors (deref on pointer type)
	for _, d := range allDiags {
		if d.Severity == diag.Error && d.Code != CodeUndeclared {
			t.Logf("diagnostic: %s (code=%s)", d.Message, d.Code)
		}
	}
}

func TestChecker_ParenExpr(t *testing.T) {
	src := `PROGRAM P
	VAR x : DINT; y : DINT; END_VAR
		y := (x + 1);
	END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestChecker_CaseStmt_IncompatibleLabel(t *testing.T) {
	src := `PROGRAM P
	VAR x : INT; END_VAR
		CASE x OF
			'hello':
				x := 1;
		END_CASE;
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeTypeMismatch {
			found = true
		}
	}
	assert.True(t, found, "expected type mismatch for STRING case label on INT selector")
}

func TestChecker_CaseStmt_WithRange(t *testing.T) {
	src := `PROGRAM P
	VAR x : INT; y : INT; END_VAR
		CASE x OF
			1..10:
				y := 1;
		ELSE
			y := 0;
		END_CASE;
	END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestChecker_FunctionDecl_Body(t *testing.T) {
	// Function return assignment "Add := expr" may trigger type errors
	// depending on how the resolver handles function name as a variable.
	// This test ensures the function body is checked at all.
	src := `FUNCTION Add : DINT
	VAR_INPUT a : DINT; b : DINT; END_VAR
		Add := a + b;
	END_FUNCTION`
	allDiags := runChecker(src)
	// Just verify it doesn't panic; function return assignment may
	// have specific handling requirements
	_ = allDiags
}

func TestChecker_FunctionBlock_Body(t *testing.T) {
	src := `FUNCTION_BLOCK FB_Counter
	VAR count : DINT; END_VAR
		count := count + 1;
	END_FUNCTION_BLOCK`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestChecker_LiteralTypes(t *testing.T) {
	tests := []struct {
		name string
		src  string
	}{
		{
			"real literal",
			`PROGRAM P VAR x : REAL; END_VAR x := 3.14; END_PROGRAM`,
		},
		{
			"bool literal",
			`PROGRAM P VAR x : BOOL; END_VAR x := TRUE; END_PROGRAM`,
		},
		{
			"string literal",
			`PROGRAM P VAR x : STRING; END_VAR x := 'hello'; END_PROGRAM`,
		},
		{
			"time literal",
			`PROGRAM P VAR x : TIME; END_VAR x := T#5s; END_PROGRAM`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allDiags := runChecker(tt.src)
			for _, d := range allDiags {
				if d.Severity == diag.Error {
					t.Errorf("unexpected error: %s", d.Message)
				}
			}
		})
	}
}

func TestChecker_AssignLiteralWidening(t *testing.T) {
	// Integer literal 42 (defaults to DINT) assigned to INT should work via literal compat
	src := `PROGRAM P VAR x : INT; END_VAR x := 42; END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestChecker_ForStmt_AllParts(t *testing.T) {
	src := `PROGRAM P
	VAR i : INT; s : STRING; END_VAR
		FOR i := s TO s BY s DO
		END_FOR;
	END_PROGRAM`
	allDiags := runChecker(src)
	// Should have type errors for FROM, TO, and BY (STRING not integer)
	errorCount := 0
	for _, d := range allDiags {
		if d.Code == CodeTypeMismatch && d.Severity == diag.Error {
			errorCount++
		}
	}
	assert.GreaterOrEqual(t, errorCount, 3, "expected at least 3 type errors for STRING in FOR parts")
}

func TestChecker_CallStmt_NilCallee(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	c := NewChecker(table, diags)
	c.checkCallStmt(&ast.CallStmt{Callee: nil})
	assert.False(t, diags.HasErrors())
}

func TestChecker_CallStmt_NonFBCallee(t *testing.T) {
	src := `PROGRAM P
	VAR x : INT; END_VAR
	END_PROGRAM`
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})

	checker := NewChecker(table, diags)
	pouScope := table.LookupPOU("P")
	if pouScope == nil {
		t.Fatal("expected POU scope")
	}
	checker.currentScope = pouScope

	callStmt := &ast.CallStmt{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCallStmt},
		Callee:   &ast.Ident{NodeBase: ast.NodeBase{NodeKind: ast.KindIdent}, Name: "x"},
	}
	checker.checkCallStmt(callStmt)
	found := false
	for _, d := range diags.All() {
		if d.Code == CodeNotCallable {
			found = true
		}
	}
	assert.True(t, found, "expected not callable for INT variable")
}

func TestChecker_CallStmt_WrongOutputParam(t *testing.T) {
	src := `
FUNCTION_BLOCK FB_Timer
VAR_INPUT
    enable : BOOL;
END_VAR
VAR_OUTPUT
    done : BOOL;
END_VAR
END_FUNCTION_BLOCK

PROGRAM Main
VAR
    timer1 : FB_Timer;
END_VAR
END_PROGRAM
`
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})

	checker := NewChecker(table, diags)
	pouScope := table.LookupPOU("Main")
	if pouScope == nil {
		t.Fatal("expected POU scope")
	}
	checker.currentScope = pouScope

	// Call with an output param that doesn't exist
	callStmt := &ast.CallStmt{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCallStmt},
		Callee:   &ast.Ident{NodeBase: ast.NodeBase{NodeKind: ast.KindIdent}, Name: "timer1"},
		Args: []*ast.CallArg{
			{
				Name:     &ast.Ident{NodeBase: ast.NodeBase{NodeKind: ast.KindIdent}, Name: "nonexistent"},
				Value:    &ast.Literal{NodeBase: ast.NodeBase{NodeKind: ast.KindLiteral}, LitKind: ast.LitBool, Value: "TRUE"},
				IsOutput: true,
			},
		},
	}
	checker.checkCallStmt(callStmt)
	found := false
	for _, d := range diags.All() {
		if d.Code == CodeNoMember {
			found = true
		}
	}
	assert.True(t, found, "expected no-member error for nonexistent output param")
}

// --- Vendor checks: additional coverage ---

func TestVendorCheck_NilProfile(t *testing.T) {
	file := makeSourceFile(&ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     &ast.Ident{Name: "Main"},
	})
	table := symbols.NewTable()
	collector := diag.NewCollector()
	CheckVendorCompat([]*ast.SourceFile{file}, table, nil, collector)
	assert.Len(t, collector.All(), 0, "nil profile should produce no diagnostics")
}

func TestVendorCheck_PropertyDecl(t *testing.T) {
	prop := &ast.PropertyDecl{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindPropertyDecl,
			NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 1, Col: 1}},
		},
		Name: &ast.Ident{Name: "MyProp"},
	}
	file := makeSourceFile(prop)
	table := symbols.NewTable()
	collector := diag.NewCollector()
	CheckVendorCompat([]*ast.SourceFile{file}, table, Schneider, collector)
	found := false
	for _, d := range collector.All() {
		if d.Code == CodeVendorOOP {
			found = true
		}
	}
	assert.True(t, found, "expected VEND001 for PROPERTY on schneider")
}

func TestVendorCheck_MethodDecl_Standalone(t *testing.T) {
	method := &ast.MethodDecl{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindMethodDecl,
			NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 1, Col: 1}},
		},
		Name: &ast.Ident{Name: "DoWork"},
		VarBlocks: []*ast.VarBlock{
			{
				Section: ast.VarLocal,
				Declarations: []*ast.VarDecl{
					{
						Names: []*ast.Ident{{Name: "p"}},
						Type: &ast.PointerType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindPointerType},
							BaseType: &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
						},
					},
				},
			},
		},
	}
	file := makeSourceFile(method)
	table := symbols.NewTable()
	collector := diag.NewCollector()
	CheckVendorCompat([]*ast.SourceFile{file}, table, Portable, collector)
	// Should get VEND001 for METHOD and VEND002 for POINTER TO
	codes := make(map[string]bool)
	for _, d := range collector.All() {
		codes[d.Code] = true
	}
	assert.True(t, codes[CodeVendorOOP], "expected VEND001")
	assert.True(t, codes[CodeVendorPointer], "expected VEND002")
}

func TestVendorCheck_FunctionDecl_VarBlocks(t *testing.T) {
	fn := &ast.FunctionDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindFunctionDecl},
		Name:     &ast.Ident{Name: "MyFunc"},
		VarBlocks: []*ast.VarBlock{
			{
				Section: ast.VarLocal,
				Declarations: []*ast.VarDecl{
					{
						Names: []*ast.Ident{{Name: "big"}},
						Type: &ast.NamedType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType},
							Name:     &ast.Ident{Name: "LREAL"},
						},
					},
				},
			},
		},
	}
	file := makeSourceFile(fn)
	table := symbols.NewTable()
	collector := diag.NewCollector()
	CheckVendorCompat([]*ast.SourceFile{file}, table, Portable, collector)
	found := false
	for _, d := range collector.All() {
		if d.Code == CodeVendor64Bit {
			found = true
		}
	}
	assert.True(t, found, "expected VEND005 for LREAL on portable")
}

func TestVendorCheck_ArrayOfPointer(t *testing.T) {
	varDecl := &ast.VarDecl{
		Names: []*ast.Ident{{Name: "ptrs"}},
		Type: &ast.ArrayType{
			NodeBase: ast.NodeBase{NodeKind: ast.KindArrayType},
			Ranges: []*ast.SubrangeSpec{{
				Low:  &ast.Literal{Value: "0"},
				High: &ast.Literal{Value: "9"},
			}},
			ElementType: &ast.PointerType{
				NodeBase: ast.NodeBase{NodeKind: ast.KindPointerType},
				BaseType: &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
			},
		},
	}
	prog := &ast.ProgramDecl{
		NodeBase:  ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:      &ast.Ident{Name: "P"},
		VarBlocks: []*ast.VarBlock{{Section: ast.VarLocal, Declarations: []*ast.VarDecl{varDecl}}},
	}
	file := makeSourceFile(prog)
	table := symbols.NewTable()
	collector := diag.NewCollector()
	CheckVendorCompat([]*ast.SourceFile{file}, table, Schneider, collector)
	found := false
	for _, d := range collector.All() {
		if d.Code == CodeVendorPointer {
			found = true
		}
	}
	assert.True(t, found, "expected VEND002 for ARRAY OF POINTER TO on schneider")
}

func TestVendorCheck_StringLenWithinLimit(t *testing.T) {
	varDecl := &ast.VarDecl{
		Names: []*ast.Ident{{Name: "s"}},
		Type: &ast.StringType{
			NodeBase: ast.NodeBase{NodeKind: ast.KindStringType},
			Length: &ast.Literal{
				NodeBase: ast.NodeBase{NodeKind: ast.KindLiteral},
				Value:    "100",
				LitKind:  ast.LitInt,
			},
		},
	}
	prog := &ast.ProgramDecl{
		NodeBase:  ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:      &ast.Ident{Name: "P"},
		VarBlocks: []*ast.VarBlock{{Section: ast.VarLocal, Declarations: []*ast.VarDecl{varDecl}}},
	}
	file := makeSourceFile(prog)
	table := symbols.NewTable()
	collector := diag.NewCollector()
	CheckVendorCompat([]*ast.SourceFile{file}, table, Schneider, collector)
	for _, d := range collector.All() {
		if d.Code == CodeVendorStringLen {
			t.Error("unexpected VEND004 for string length within limit")
		}
	}
}

func TestIs64BitType(t *testing.T) {
	assert.True(t, is64BitType("LINT"))
	assert.True(t, is64BitType("LREAL"))
	assert.True(t, is64BitType("LWORD"))
	assert.True(t, is64BitType("ULINT"))
	assert.True(t, is64BitType("lint"))  // case insensitive
	assert.False(t, is64BitType("INT"))
	assert.False(t, is64BitType("REAL"))
	assert.False(t, is64BitType("DINT"))
}

// --- Usage analysis additional coverage ---

func TestIsInterfaceVar(t *testing.T) {
	assert.True(t, isInterfaceVar(ast.VarInput))
	assert.True(t, isInterfaceVar(ast.VarOutput))
	assert.True(t, isInterfaceVar(ast.VarInOut))
	assert.True(t, isInterfaceVar(ast.VarGlobal))
	assert.True(t, isInterfaceVar(ast.VarExternal))
	assert.False(t, isInterfaceVar(ast.VarLocal))
	assert.False(t, isInterfaceVar(ast.VarTemp))
}

func TestUnreachableNested_IfStmt(t *testing.T) {
	body := []ast.Statement{
		&ast.IfStmt{
			NodeBase:  ast.NodeBase{NodeKind: ast.KindIfStmt},
			Condition: &ast.Ident{Name: "flag"},
			Then: []ast.Statement{
				&ast.ReturnStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindReturnStmt,
					NodeSpan: ast.Span{Start: ast.Pos{Line: 5, Col: 1}}}},
				&ast.AssignStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindAssignStmt,
					NodeSpan: ast.Span{Start: ast.Pos{Line: 6, Col: 1}}}},
			},
			ElsIfs: []*ast.ElsIf{{
				Condition: &ast.Ident{Name: "other"},
				Body: []ast.Statement{
					&ast.ExitStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindExitStmt,
						NodeSpan: ast.Span{Start: ast.Pos{Line: 8, Col: 1}}}},
					&ast.AssignStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindAssignStmt,
						NodeSpan: ast.Span{Start: ast.Pos{Line: 9, Col: 1}}}},
				},
			}},
			Else: []ast.Statement{
				&ast.ReturnStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindReturnStmt,
					NodeSpan: ast.Span{Start: ast.Pos{Line: 11, Col: 1}}}},
				&ast.AssignStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindAssignStmt,
					NodeSpan: ast.Span{Start: ast.Pos{Line: 12, Col: 1}}}},
			},
		},
	}

	prog := &ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     &ast.Ident{Name: "Main"},
		Body:     body,
	}
	file := makeSourceFile(prog)
	table := symbols.NewTable()
	collector := diag.NewCollector()
	CheckUsage([]*ast.SourceFile{file}, table, collector)

	unreachCount := 0
	for _, d := range collector.All() {
		if d.Code == CodeUnreachableCode {
			unreachCount++
		}
	}
	assert.Equal(t, 3, unreachCount, "expected 3 unreachable code warnings (THEN, ELSIF, ELSE)")
}

func TestUnreachable_CaseStmt(t *testing.T) {
	body := []ast.Statement{
		&ast.CaseStmt{
			NodeBase: ast.NodeBase{NodeKind: ast.KindCaseStmt},
			Expr:     &ast.Ident{Name: "x"},
			Branches: []*ast.CaseBranch{{
				Body: []ast.Statement{
					&ast.ReturnStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindReturnStmt,
						NodeSpan: ast.Span{Start: ast.Pos{Line: 5, Col: 1}}}},
					&ast.AssignStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindAssignStmt,
						NodeSpan: ast.Span{Start: ast.Pos{Line: 6, Col: 1}}}},
				},
			}},
			ElseBranch: []ast.Statement{
				&ast.ExitStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindExitStmt,
					NodeSpan: ast.Span{Start: ast.Pos{Line: 8, Col: 1}}}},
				&ast.AssignStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindAssignStmt,
					NodeSpan: ast.Span{Start: ast.Pos{Line: 9, Col: 1}}}},
			},
		},
	}

	prog := &ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     &ast.Ident{Name: "Main"},
		Body:     body,
	}
	file := makeSourceFile(prog)
	table := symbols.NewTable()
	collector := diag.NewCollector()
	CheckUsage([]*ast.SourceFile{file}, table, collector)

	unreachCount := 0
	for _, d := range collector.All() {
		if d.Code == CodeUnreachableCode {
			unreachCount++
		}
	}
	assert.Equal(t, 2, unreachCount, "expected 2 unreachable code warnings (branch + else)")
}

func TestUnreachable_WhileRepeat(t *testing.T) {
	whileBody := []ast.Statement{
		&ast.WhileStmt{
			NodeBase: ast.NodeBase{NodeKind: ast.KindWhileStmt},
			Condition: &ast.Ident{Name: "flag"},
			Body: []ast.Statement{
				&ast.ExitStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindExitStmt,
					NodeSpan: ast.Span{Start: ast.Pos{Line: 5, Col: 1}}}},
				&ast.AssignStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindAssignStmt,
					NodeSpan: ast.Span{Start: ast.Pos{Line: 6, Col: 1}}}},
			},
		},
		&ast.RepeatStmt{
			NodeBase: ast.NodeBase{NodeKind: ast.KindRepeatStmt},
			Body: []ast.Statement{
				&ast.ReturnStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindReturnStmt,
					NodeSpan: ast.Span{Start: ast.Pos{Line: 8, Col: 1}}}},
				&ast.AssignStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindAssignStmt,
					NodeSpan: ast.Span{Start: ast.Pos{Line: 9, Col: 1}}}},
			},
			Condition: &ast.Ident{Name: "done"},
		},
	}

	prog := &ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     &ast.Ident{Name: "Main"},
		Body:     whileBody,
	}
	file := makeSourceFile(prog)
	table := symbols.NewTable()
	collector := diag.NewCollector()
	CheckUsage([]*ast.SourceFile{file}, table, collector)

	unreachCount := 0
	for _, d := range collector.All() {
		if d.Code == CodeUnreachableCode {
			unreachCount++
		}
	}
	assert.Equal(t, 2, unreachCount)
}

func TestUnreachable_FunctionBlockMethods(t *testing.T) {
	fb := &ast.FunctionBlockDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindFunctionBlockDecl},
		Name:     &ast.Ident{Name: "FB"},
		Methods: []*ast.MethodDecl{{
			NodeBase: ast.NodeBase{NodeKind: ast.KindMethodDecl},
			Name:     &ast.Ident{Name: "M"},
			Body: []ast.Statement{
				&ast.ReturnStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindReturnStmt,
					NodeSpan: ast.Span{Start: ast.Pos{Line: 3, Col: 1}}}},
				&ast.AssignStmt{NodeBase: ast.NodeBase{NodeKind: ast.KindAssignStmt,
					NodeSpan: ast.Span{Start: ast.Pos{Line: 4, Col: 1}}}},
			},
		}},
	}
	file := makeSourceFile(fb)
	table := symbols.NewTable()
	collector := diag.NewCollector()
	CheckUsage([]*ast.SourceFile{file}, table, collector)

	found := false
	for _, d := range collector.All() {
		if d.Code == CodeUnreachableCode {
			found = true
		}
	}
	assert.True(t, found, "expected unreachable code in FB method")
}

// --- Resolver additional coverage ---

func TestResolveTypeSpec_Nil(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	r := NewResolver(table, diags)
	result := r.resolveTypeSpec(nil)
	assert.NotNil(t, result) // Should be types.Invalid
}

func TestResolveTypeSpec_NamedType_NilName(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	r := NewResolver(table, diags)
	result := r.resolveTypeSpec(&ast.NamedType{Name: nil})
	assert.NotNil(t, result) // Should be types.Invalid
}

func TestResolveTypeSpec_ErrorNode(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	r := NewResolver(table, diags)
	result := r.resolveTypeSpec(&ast.ErrorNode{})
	assert.NotNil(t, result) // Should be types.Invalid
}

func TestEvalConstInt(t *testing.T) {
	assert.Equal(t, 42, evalConstInt(&ast.Literal{LitKind: ast.LitInt, Value: "42"}))
	assert.Equal(t, 0, evalConstInt(&ast.Literal{LitKind: ast.LitString, Value: "hello"}))
	assert.Equal(t, 0, evalConstInt(nil))
	assert.Equal(t, 0, evalConstInt(&ast.Ident{Name: "x"}))
}

func TestResolverProgramNilName(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	r := NewResolver(table, diags)
	r.resolveProgram(&ast.ProgramDecl{Name: nil}, false)
	assert.False(t, diags.HasErrors())
}

func TestResolverFunctionBlockNilName(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	r := NewResolver(table, diags)
	r.resolveFunctionBlock(&ast.FunctionBlockDecl{Name: nil}, false)
	assert.False(t, diags.HasErrors())
}

func TestResolverFunctionNilName(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	r := NewResolver(table, diags)
	r.resolveFunction(&ast.FunctionDecl{Name: nil}, false)
	assert.False(t, diags.HasErrors())
}

func TestResolverTypeDeclNilName(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	r := NewResolver(table, diags)
	r.resolveTypeDecl(&ast.TypeDecl{Name: nil}, false)
	assert.False(t, diags.HasErrors())
}

func TestResolverInterfaceNilName(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	r := NewResolver(table, diags)
	r.resolveInterface(&ast.InterfaceDecl{Name: nil}, false)
	assert.False(t, diags.HasErrors())
}

// --- Candidates ---

func TestResolveCandidates_NoParams(t *testing.T) {
	// Function with no params should return return type directly
	src := `FUNCTION NoArgs : BOOL
		NoArgs := TRUE;
	END_FUNCTION

	PROGRAM P
	VAR b : BOOL; END_VAR
		b := NoArgs();
	END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Logf("diagnostic: %s (code=%s)", d.Message, d.Code)
		}
	}
}

func TestResolveCandidates_EmptyParams(t *testing.T) {
	// Use a types import through the checker
	src := `PROGRAM P
	VAR x : INT; END_VAR
		x := ADD(1, 2);
	END_PROGRAM`
	allDiags := runChecker(src)
	// ADD with 2 args should work
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Logf("diagnostic: %s (code=%s)", d.Message, d.Code)
		}
	}
}

func TestVendorCheck_FBVarBlocks(t *testing.T) {
	// FB with var blocks containing vendor-incompatible types
	fb := &ast.FunctionBlockDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindFunctionBlockDecl},
		Name:     &ast.Ident{Name: "FB"},
		VarBlocks: []*ast.VarBlock{{
			Section: ast.VarLocal,
			Declarations: []*ast.VarDecl{{
				Names: []*ast.Ident{{Name: "r"}},
				Type: &ast.ReferenceType{
					NodeBase: ast.NodeBase{NodeKind: ast.KindReferenceType},
					BaseType: &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
				},
			}},
		}},
	}
	file := makeSourceFile(fb)
	table := symbols.NewTable()
	collector := diag.NewCollector()
	CheckVendorCompat([]*ast.SourceFile{file}, table, Schneider, collector)
	found := false
	for _, d := range collector.All() {
		if d.Code == CodeVendorReference {
			found = true
		}
	}
	assert.True(t, found, "expected VEND003 for REFERENCE TO in FB var block")
}

func TestVendorCheck_FBPropertyDecl(t *testing.T) {
	fb := &ast.FunctionBlockDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindFunctionBlockDecl},
		Name:     &ast.Ident{Name: "FB"},
		Properties: []*ast.PropertyDecl{{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindPropertyDecl,
				NodeSpan: ast.Span{Start: ast.Pos{Line: 2, Col: 1}},
			},
			Name: &ast.Ident{Name: "Prop"},
		}},
	}
	file := makeSourceFile(fb)
	table := symbols.NewTable()
	collector := diag.NewCollector()
	CheckVendorCompat([]*ast.SourceFile{file}, table, Schneider, collector)
	found := false
	for _, d := range collector.All() {
		if d.Code == CodeVendorOOP {
			found = true
		}
	}
	assert.True(t, found, "expected VEND001 for PROPERTY in FB on schneider")
}
