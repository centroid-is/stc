package checker

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runChecker parses source, runs both passes, returns diagnostics.
func runChecker(src string) []diag.Diagnostic {
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	checker := NewChecker(table, diags)
	checker.CheckBodies([]*ast.SourceFile{file})
	return diags.All()
}

func TestTypeMismatch(t *testing.T) {
	file := parseTestdata(t, "type_mismatch.st")

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	checker := NewChecker(table, diags)
	checker.CheckBodies([]*ast.SourceFile{file})

	require.True(t, diags.HasErrors(), "expected type mismatch error")
	errors := diags.Errors()
	found := false
	for _, e := range errors {
		if e.Code == CodeTypeMismatch {
			found = true
			assert.Contains(t, e.Message, "STRING")
			assert.Contains(t, e.Message, "INT")
			assert.Greater(t, e.Pos.Line, 0, "diagnostic should have line position")
		}
	}
	assert.True(t, found, "expected SEMA001 type mismatch diagnostic")
}

func TestUndeclaredVar(t *testing.T) {
	file := parseTestdata(t, "undeclared.st")

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	checker := NewChecker(table, diags)
	checker.CheckBodies([]*ast.SourceFile{file})

	require.True(t, diags.HasErrors(), "expected undeclared var error")
	errors := diags.Errors()
	found := false
	for _, e := range errors {
		if e.Code == CodeUndeclared {
			found = true
			assert.Contains(t, e.Message, "undeclared_var")
		}
	}
	assert.True(t, found, "expected SEMA010 undeclared identifier diagnostic")
}

func TestValidProgram(t *testing.T) {
	file := parseTestdata(t, "valid_program.st")

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	checker := NewChecker(table, diags)
	checker.CheckBodies([]*ast.SourceFile{file})

	assert.False(t, diags.HasErrors(), "expected no errors, got: %v", diags.All())
}

func TestBinaryExprTypes(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr bool
		errCode string
	}{
		{
			name: "INT + INT = INT (ok)",
			src: `PROGRAM P VAR a : INT; b : INT; c : INT; END_VAR
				c := a + b;
			END_PROGRAM`,
			wantErr: false,
		},
		{
			name: "INT + DINT = DINT (widening ok)",
			src: `PROGRAM P VAR a : INT; b : DINT; c : DINT; END_VAR
				c := a + b;
			END_PROGRAM`,
			wantErr: false,
		},
		{
			name: "INT + STRING = error",
			src: `PROGRAM P VAR a : INT; b : STRING; c : INT; END_VAR
				c := a + b;
			END_PROGRAM`,
			wantErr: true,
			errCode: CodeIncompatibleOp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allDiags := runChecker(tt.src)
			if tt.wantErr {
				hasErr := false
				for _, d := range allDiags {
					if d.Code == tt.errCode {
						hasErr = true
					}
				}
				assert.True(t, hasErr, "expected error code %s", tt.errCode)
			} else {
				for _, d := range allDiags {
					if d.Severity == diag.Error {
						t.Errorf("unexpected error: %s", d)
					}
				}
			}
		})
	}
}

func TestComparisonReturnsBool(t *testing.T) {
	src := `PROGRAM P
	VAR a : INT; b : INT; c : BOOL; END_VAR
		c := a > b;
	END_PROGRAM`

	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d)
		}
	}
}

func TestBooleanOpsRequireBool(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		wantErr bool
	}{
		{
			name: "TRUE AND FALSE ok",
			src: `PROGRAM P VAR a : BOOL; b : BOOL; c : BOOL; END_VAR
				c := a AND b;
			END_PROGRAM`,
			wantErr: false,
		},
		{
			name: "INT AND INT = error",
			src: `PROGRAM P VAR a : INT; b : INT; c : INT; END_VAR
				c := a AND b;
			END_PROGRAM`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allDiags := runChecker(tt.src)
			hasErr := false
			for _, d := range allDiags {
				if d.Severity == diag.Error && d.Code == CodeTypeMismatch {
					hasErr = true
				}
			}
			if tt.wantErr {
				assert.True(t, hasErr, "expected type mismatch error for boolean op")
			} else {
				assert.False(t, hasErr, "expected no type mismatch error")
			}
		})
	}
}

func TestFBInstanceMemberAccess(t *testing.T) {
	// Test FB instance output member access (timer1.done, timer1.elapsed)
	file := parseTestdata(t, "fb_instance.st")

	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	checker := NewChecker(table, diags)
	checker.CheckBodies([]*ast.SourceFile{file})

	// Should have no type errors
	for _, d := range diags.All() {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s (code=%s)", d.Message, d.Code)
		}
	}
}

func TestFBInstanceCall(t *testing.T) {
	// Manually construct a CallStmt to test FB call checking,
	// since the parser's FB call syntax requires specific handling.
	src := `
FUNCTION_BLOCK FB_Timer
VAR_INPUT
    enable : BOOL;
    preset : INT;
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

	// Manually create a CallStmt and check it
	callStmt := &ast.CallStmt{
		NodeBase: ast.NodeBase{NodeKind: ast.KindCallStmt},
		Callee:   &ast.Ident{NodeBase: ast.NodeBase{NodeKind: ast.KindIdent}, Name: "timer1"},
		Args: []*ast.CallArg{
			{
				Name:  &ast.Ident{NodeBase: ast.NodeBase{NodeKind: ast.KindIdent}, Name: "enable"},
				Value: &ast.Literal{NodeBase: ast.NodeBase{NodeKind: ast.KindLiteral}, LitKind: ast.LitBool, Value: "TRUE"},
			},
			{
				Name:  &ast.Ident{NodeBase: ast.NodeBase{NodeKind: ast.KindIdent}, Name: "preset"},
				Value: &ast.Literal{NodeBase: ast.NodeBase{NodeKind: ast.KindLiteral}, LitKind: ast.LitInt, Value: "100"},
			},
		},
	}

	checker := NewChecker(table, diags)
	pouScope := table.LookupPOU("Main")
	require.NotNil(t, pouScope)
	checker.currentScope = pouScope
	checker.checkCallStmt(callStmt)

	// Should have no type errors for valid FB call
	for _, d := range diags.All() {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s (code=%s)", d.Message, d.Code)
		}
	}
}

func TestArrayIndexing(t *testing.T) {
	// First define S_Point type that array_struct.st needs
	src := `
TYPE S_Point :
STRUCT
    x : REAL;
    y : REAL;
END_STRUCT;
END_TYPE

PROGRAM Main
VAR
    arr : ARRAY[0..9] OF INT;
    idx : INT;
    val : INT;
    pt : S_Point;
    px : REAL;
END_VAR
    val := arr[idx];
    px := pt.x;
END_PROGRAM
`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s (code=%s)", d.Message, d.Code)
		}
	}
}

func TestArrayStringIndex(t *testing.T) {
	src := `PROGRAM P
	VAR arr : ARRAY[0..9] OF INT; s : STRING; END_VAR
		arr[s] := 1;
	END_PROGRAM`

	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeTypeMismatch && d.Severity == diag.Error {
			found = true
		}
	}
	assert.True(t, found, "expected type mismatch for STRING array index")
}

func TestStructMemberAccess(t *testing.T) {
	src := `
TYPE S_Point :
STRUCT
    x : REAL;
    y : REAL;
END_STRUCT;
END_TYPE

PROGRAM P
VAR
    pt : S_Point;
    px : REAL;
END_VAR
    px := pt.x;
END_PROGRAM
`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s (code=%s)", d.Message, d.Code)
		}
	}
}

func TestStructMemberNotFound(t *testing.T) {
	src := `
TYPE S_Point :
STRUCT
    x : REAL;
    y : REAL;
END_STRUCT;
END_TYPE

PROGRAM P
VAR
    pt : S_Point;
    val : REAL;
END_VAR
    val := pt.nonexistent;
END_PROGRAM
`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeNoMember {
			found = true
			assert.Contains(t, d.Message, "nonexistent")
		}
	}
	assert.True(t, found, "expected SEMA024 for nonexistent struct member")
}

func TestFunctionCallArgCount(t *testing.T) {
	// ADD expects 2 args
	src := `PROGRAM P
	VAR a : INT; END_VAR
		a := ADD(1);
	END_PROGRAM`

	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeWrongArgCount {
			found = true
		}
	}
	assert.True(t, found, "expected SEMA020 for wrong argument count")
}

func TestForLoopVarType(t *testing.T) {
	src := `PROGRAM P
	VAR r : REAL; END_VAR
		FOR r := 1 TO 10 DO
		END_FOR;
	END_PROGRAM`

	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeTypeMismatch && d.Severity == diag.Error {
			found = true
		}
	}
	assert.True(t, found, "expected type error for REAL loop variable")
}

func TestIfConditionType(t *testing.T) {
	src := `PROGRAM P
	VAR x : INT; END_VAR
		IF x THEN
		END_IF;
	END_PROGRAM`

	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeTypeMismatch {
			found = true
			assert.Contains(t, d.Message, "BOOL")
		}
	}
	assert.True(t, found, "expected type error for non-BOOL IF condition")
}

// --- AT address validation tests ---

func TestATAddressValidInProgram(t *testing.T) {
	src := `PROGRAM P
VAR
    x AT %IX0.0 : BOOL;
END_VAR
END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error || d.Code == CodeATNotAllowedHere {
			t.Errorf("unexpected diagnostic for AT in PROGRAM: %s (code=%s)", d.Message, d.Code)
		}
	}
}

func TestATAddressNotAllowedInFunctionBlock(t *testing.T) {
	src := `FUNCTION_BLOCK FB_Test
VAR
    x AT %IX0.0 : BOOL;
END_VAR
END_FUNCTION_BLOCK`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeATNotAllowedHere {
			found = true
			assert.Contains(t, d.Message, "FUNCTION_BLOCK")
		}
	}
	assert.True(t, found, "expected ATNotAllowedHere warning for AT in FUNCTION_BLOCK")
}

func TestATAddressNotAllowedInFunction(t *testing.T) {
	src := `FUNCTION F : INT
VAR
    x AT %IX0.0 : BOOL;
END_VAR
    F := 0;
END_FUNCTION`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeATNotAllowedHere {
			found = true
			assert.Contains(t, d.Message, "FUNCTION")
		}
	}
	assert.True(t, found, "expected ATNotAllowedHere warning for AT in FUNCTION")
}

func TestATAddressInvalidFormat(t *testing.T) {
	src := `PROGRAM P
VAR
    x AT %ZZ0 : BOOL;
END_VAR
END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeInvalidATAddress {
			found = true
		}
	}
	assert.True(t, found, "expected InvalidATAddress error for %%ZZ0")
}

func TestATOverlapWordAndBit(t *testing.T) {
	// %IW0 covers bytes 0-1, %IX0.3 touches byte 0 -- should overlap
	src := `PROGRAM P
VAR
    a AT %IW0 : INT;
    b AT %IX0.3 : BOOL;
END_VAR
END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeATOverlap {
			found = true
		}
	}
	assert.True(t, found, "expected ATOverlap warning for %%IW0 and %%IX0.3")
}

func TestATNoOverlapAdjacent(t *testing.T) {
	// %IW0 covers bytes 0-1, %IW2 covers bytes 2-3 -- adjacent, no overlap
	src := `PROGRAM P
VAR
    a AT %IW0 : INT;
    b AT %IW2 : INT;
END_VAR
END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Code == CodeATOverlap {
			t.Errorf("unexpected overlap warning for adjacent addresses: %s", d.Message)
		}
	}
}

func TestATOverlapDWordAndWord(t *testing.T) {
	// %ID0 covers bytes 0-3, %IW2 covers bytes 2-3 -- should overlap
	src := `PROGRAM P
VAR
    a AT %ID0 : DINT;
    b AT %IW2 : INT;
END_VAR
END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeATOverlap {
			found = true
		}
	}
	assert.True(t, found, "expected ATOverlap warning for %%ID0 and %%IW2")
}

func TestATNoOverlapDifferentAreas(t *testing.T) {
	// %IW0 (input) and %QW0 (output) -- different areas, no overlap
	src := `PROGRAM P
VAR
    a AT %IW0 : INT;
    b AT %QW0 : INT;
END_VAR
END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Code == CodeATOverlap {
			t.Errorf("unexpected overlap warning for different areas: %s", d.Message)
		}
	}
}

func TestATWildcardNoOverlap(t *testing.T) {
	// Wildcard addresses should not produce overlap warnings
	src := `PROGRAM P
VAR
    a AT %I* : BOOL;
    b AT %IX0.0 : BOOL;
END_VAR
END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Code == CodeATOverlap {
			t.Errorf("unexpected overlap warning with wildcard: %s", d.Message)
		}
	}
}
