package checker

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
)

// --- allConcreteForConstraint ---

func TestAllConcreteForConstraint(t *testing.T) {
	// Should return types satisfying IsAnyInt
	ints := allConcreteForConstraint(types.IsAnyInt)
	assert.NotEmpty(t, ints, "expected at least some integer types")

	// All results should satisfy the constraint
	for _, k := range ints {
		assert.True(t, types.IsAnyInt(k), "expected integer type, got %v", k)
	}
}

func TestAllConcreteForConstraint_AllTypes(t *testing.T) {
	// Accept everything - should return up to maxCandidates
	all := allConcreteForConstraint(func(k types.TypeKind) bool { return true })
	assert.LessOrEqual(t, len(all), maxCandidates)
	assert.NotEmpty(t, all)
}

func TestAllConcreteForConstraint_NoMatch(t *testing.T) {
	none := allConcreteForConstraint(func(k types.TypeKind) bool { return false })
	assert.Empty(t, none)
}

// --- ResolveCandidates ---

func TestResolveCandidates_NilFnParams(t *testing.T) {
	// nil params
	fn := &types.FunctionType{ReturnType: types.TypeBOOL}
	ret, _, ok := ResolveCandidates(fn, nil)
	assert.True(t, ok)
	assert.Equal(t, types.TypeBOOL, ret)
}

func TestResolveCandidates_GenericConstraintMismatch(t *testing.T) {
	fn := &types.FunctionType{
		ReturnType: nil,
		Params: []types.Parameter{
			{Name: "IN", GenericConstraint: types.IsAnyInt},
		},
	}
	// Pass a BOOL which doesn't satisfy IsAnyInt
	_, _, ok := ResolveCandidates(fn, []types.Type{types.TypeBOOL})
	assert.False(t, ok)
}

func TestResolveCandidates_GenericSuccess(t *testing.T) {
	fn := &types.FunctionType{
		ReturnType: nil,
		Params: []types.Parameter{
			{Name: "IN", GenericConstraint: types.IsAnyNum},
		},
	}
	ret, _, ok := ResolveCandidates(fn, []types.Type{types.TypeDINT})
	assert.True(t, ok)
	assert.NotNil(t, ret)
}

func TestResolveCandidates_NilArgType(t *testing.T) {
	fn := &types.FunctionType{
		ReturnType: types.TypeDINT,
		Params: []types.Parameter{
			{Name: "IN", Type: types.TypeDINT},
		},
	}
	ret, _, ok := ResolveCandidates(fn, []types.Type{nil})
	assert.True(t, ok)
	assert.Equal(t, types.TypeDINT, ret)
}

func TestResolveCandidates_InvalidArgType(t *testing.T) {
	fn := &types.FunctionType{
		ReturnType: types.TypeDINT,
		Params: []types.Parameter{
			{Name: "IN", Type: types.TypeDINT},
		},
	}
	ret, _, ok := ResolveCandidates(fn, []types.Type{types.Invalid})
	assert.True(t, ok)
	assert.Equal(t, types.TypeDINT, ret)
}

func TestResolveCandidates_MultipleGenericIncompatible(t *testing.T) {
	fn := &types.FunctionType{
		ReturnType: nil,
		Params: []types.Parameter{
			{Name: "IN1", GenericConstraint: types.IsAnyInt},
			{Name: "IN2", GenericConstraint: types.IsAnyInt},
		},
	}
	// Pass INT and STRING - STRING doesn't satisfy IsAnyInt
	_, _, ok := ResolveCandidates(fn, []types.Type{types.TypeDINT, types.TypeSTRING})
	assert.False(t, ok)
}

func TestResolveCandidates_ANYParam(t *testing.T) {
	fn := &types.FunctionType{
		ReturnType: nil,
		Params: []types.Parameter{
			{Name: "IN", Type: nil}, // ANY
		},
	}
	ret, _, ok := ResolveCandidates(fn, []types.Type{types.TypeSTRING})
	assert.True(t, ok)
	// With no generic, return type should be first arg type since ReturnType is nil
	assert.Equal(t, types.TypeSTRING, ret)
}

func TestResolveCandidates_NilRetNoArgs(t *testing.T) {
	fn := &types.FunctionType{
		ReturnType: nil,
		Params: []types.Parameter{
			{Name: "IN", Type: nil},
		},
	}
	ret, _, ok := ResolveCandidates(fn, nil)
	assert.True(t, ok)
	assert.Equal(t, types.Invalid, ret)
}

// --- checkLiteral: all literal types ---

func TestChecker_LitDate(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	c := NewChecker(table, diags)
	r := c.checkLiteral(&ast.Literal{LitKind: ast.LitDate, Value: "D#2024-01-01"})
	assert.Equal(t, types.KindDATE, r.Kind())
}

func TestChecker_LitDateTime(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	c := NewChecker(table, diags)
	r := c.checkLiteral(&ast.Literal{LitKind: ast.LitDateTime, Value: "DT#2024-01-01-00:00:00"})
	assert.Equal(t, types.KindDT, r.Kind())
}

func TestChecker_LitTod(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	c := NewChecker(table, diags)
	r := c.checkLiteral(&ast.Literal{LitKind: ast.LitTod, Value: "TOD#12:00:00"})
	assert.Equal(t, types.KindTOD, r.Kind())
}

func TestChecker_LitWString(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	c := NewChecker(table, diags)
	r := c.checkLiteral(&ast.Literal{LitKind: ast.LitWString, Value: `"hello"`})
	assert.Equal(t, types.KindWSTRING, r.Kind())
}

func TestChecker_LitTyped_UnknownPrefix(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	c := NewChecker(table, diags)
	r := c.checkLiteral(&ast.Literal{LitKind: ast.LitTyped, TypePrefix: "UNKNOWN_TYPE", Value: "1"})
	// Should fall back to DINT
	assert.Equal(t, types.KindDINT, r.Kind())
}

// --- checkUserFuncCall ---

func TestChecker_UserFuncCall(t *testing.T) {
	src := `FUNCTION MyAdd : DINT
	VAR_INPUT a : DINT; b : DINT; END_VAR
	END_FUNCTION
	PROGRAM P
	VAR result : DINT; END_VAR
		result := MyAdd(1, 2);
	END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestChecker_UserFuncCall_WrongArgCount(t *testing.T) {
	src := `FUNCTION MyFunc : DINT
	VAR_INPUT a : DINT; END_VAR
	MyFunc := a;
	END_FUNCTION
	PROGRAM P
	VAR result : DINT; END_VAR
		result := MyFunc(1, 2);
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeWrongArgCount {
			found = true
		}
	}
	assert.True(t, found, "expected wrong arg count error")
}

func TestChecker_UserFuncCall_WrongArgType(t *testing.T) {
	src := `FUNCTION MyFunc : DINT
	VAR_INPUT a : DINT; END_VAR
	MyFunc := a;
	END_FUNCTION
	PROGRAM P
	VAR s : STRING; result : DINT; END_VAR
		result := MyFunc(s);
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeWrongArgType {
			found = true
		}
	}
	assert.True(t, found, "expected wrong arg type error")
}

// --- checkMemberAccessExpr: enum and FB ---

func TestChecker_MemberAccess_EnumNotFound(t *testing.T) {
	src := `TYPE Color : (Red, Green, Blue); END_TYPE
	PROGRAM P
	VAR c : Color; END_VAR
		c := Color.Purple;
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeNoMember {
			found = true
		}
	}
	assert.True(t, found, "expected no-member error for enum")
}

// --- resolveInterface ---

func TestResolver_Interface(t *testing.T) {
	src := `INTERFACE IMotor
	METHOD Start
	END_METHOD
	END_INTERFACE`
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	sym := table.LookupGlobal("IMotor")
	assert.NotNil(t, sym, "expected IMotor to be registered")
	assert.Equal(t, symbols.KindInterface, sym.Kind)
}

func TestResolver_Interface_Redeclared(t *testing.T) {
	src := `INTERFACE IMotor
	END_INTERFACE
	INTERFACE IMotor
	END_INTERFACE`
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	assert.True(t, diags.HasErrors(), "expected redeclaration error")
}

// --- checkIfStmt: with else ---

func TestChecker_IfWithElse(t *testing.T) {
	src := `PROGRAM P
	VAR b : BOOL; x : DINT; END_VAR
		IF b THEN
			x := 1;
		ELSIF b THEN
			x := 2;
		ELSE
			x := 3;
		END_IF;
	END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

// --- resolveTypeSpec: more types ---

func TestResolver_PointerType(t *testing.T) {
	src := `TYPE PtrInt : POINTER TO INT; END_TYPE
	PROGRAM P
	END_PROGRAM`
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	sym := table.LookupGlobal("PtrInt")
	assert.NotNil(t, sym)
}

func TestResolver_ReferenceType(t *testing.T) {
	src := `TYPE RefInt : REFERENCE TO INT; END_TYPE
	PROGRAM P
	END_PROGRAM`
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	sym := table.LookupGlobal("RefInt")
	assert.NotNil(t, sym)
}

func TestResolver_StringType(t *testing.T) {
	src := `PROGRAM P
	VAR s : STRING; w : WSTRING; END_VAR
	END_PROGRAM`
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	// Just ensure no crash
	assert.False(t, diags.HasErrors())
}

// --- checkUnreachableDecl: MethodDecl ---

func TestUnreachable_MethodDecl(t *testing.T) {
	src := `FUNCTION_BLOCK MyFB
	METHOD DoStuff
		RETURN;
		x := 1;
	END_METHOD
	END_FUNCTION_BLOCK`
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	CheckUsage([]*ast.SourceFile{file}, table, diags)
	found := false
	for _, d := range diags.All() {
		if d.Code == CodeUnreachableCode {
			found = true
		}
	}
	assert.True(t, found, "expected unreachable code warning in method")
}

// --- checkBuiltinCall: wrong arg count ---

func TestChecker_BuiltinCall_WrongArgCount(t *testing.T) {
	src := `PROGRAM P
	VAR x : LREAL; END_VAR
		x := ABS(1, 2);
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeWrongArgCount {
			found = true
		}
	}
	assert.True(t, found, "expected wrong arg count for ABS(1,2)")
}

// --- checkBinaryExpr: boolean op with non-bool ---

func TestChecker_BooleanOp_NonBool(t *testing.T) {
	src := `PROGRAM P
	VAR x : DINT; y : DINT; r : BOOL; END_VAR
		r := x AND y;
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeTypeMismatch {
			found = true
		}
	}
	assert.True(t, found, "expected type mismatch for INT AND INT")
}

// --- checkBinaryExpr: incompatible types ---

func TestChecker_BinaryExpr_IncompatibleTypes(t *testing.T) {
	src := `PROGRAM P
	VAR x : DINT; s : STRING; r : DINT; END_VAR
		r := x + s;
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeIncompatibleOp {
			found = true
		}
	}
	assert.True(t, found, "expected incompatible op error")
}

// --- checkCallStmt: output arg type mismatch ---

func TestChecker_CallStmt_OutputArgTypeMismatch(t *testing.T) {
	src := `FUNCTION_BLOCK MyFB
	VAR_INPUT cmd : DINT; END_VAR
	VAR_OUTPUT result : DINT; END_VAR
	END_FUNCTION_BLOCK
	PROGRAM P
	VAR fb : MyFB; s : STRING; END_VAR
		fb(cmd := s);
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeWrongArgType {
			found = true
		}
	}
	assert.True(t, found, "expected wrong arg type for FB call")
}

// --- checkIdent with scope = nil ---

func TestChecker_CheckIdent_NilScope(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	c := NewChecker(table, diags)
	c.currentScope = nil
	result := c.checkIdent(&ast.Ident{Name: "x"})
	assert.Equal(t, types.Invalid, result)
}

// --- resolveTypeSpec: ErrorNode ---

func TestResolver_SubrangeType(t *testing.T) {
	src := `TYPE MyRange : INT(1..100); END_TYPE
	PROGRAM P
	END_PROGRAM`
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	sym := table.LookupGlobal("MyRange")
	assert.NotNil(t, sym)
}

// --- checkCallExpr: nil callee name ---

func TestChecker_CallExpr_NilCallee(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	c := NewChecker(table, diags)
	result := c.checkCallExpr(&ast.CallExpr{
		Callee: &ast.Literal{Value: "1"},
	})
	assert.Equal(t, types.Invalid, result)
}

// --- checkForStmt: non-integer BY ---

// --- checkMemberAccessExpr: struct member, FB member, nil member ---

func TestChecker_MemberAccess_StructMember(t *testing.T) {
	src := `TYPE MyStruct : STRUCT x : DINT; y : DINT; END_STRUCT; END_TYPE
	PROGRAM P
	VAR s : MyStruct; r : DINT; END_VAR
		r := s.x;
	END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestChecker_MemberAccess_StructNotFound(t *testing.T) {
	src := `TYPE MyStruct : STRUCT x : DINT; END_STRUCT; END_TYPE
	PROGRAM P
	VAR s : MyStruct; r : DINT; END_VAR
		r := s.z;
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeNoMember {
			found = true
		}
	}
	assert.True(t, found, "expected no-member error for struct")
}

func TestChecker_MemberAccess_FBOutput(t *testing.T) {
	src := `FUNCTION_BLOCK MyFB
	VAR_OUTPUT result : DINT; END_VAR
	END_FUNCTION_BLOCK
	PROGRAM P
	VAR fb : MyFB; r : DINT; END_VAR
		r := fb.result;
	END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestChecker_MemberAccess_FBNotFound(t *testing.T) {
	src := `FUNCTION_BLOCK MyFB
	VAR_INPUT x : DINT; END_VAR
	END_FUNCTION_BLOCK
	PROGRAM P
	VAR fb : MyFB; r : DINT; END_VAR
		r := fb.nonexistent;
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeNoMember {
			found = true
		}
	}
	assert.True(t, found, "expected no-member error for FB")
}

func TestChecker_MemberAccess_NilMember(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	c := NewChecker(table, diags)
	scope := table.RegisterPOU("P", symbols.KindProgram, source.Pos{})
	scope.Insert(&symbols.Symbol{Name: "s", Kind: symbols.KindVariable, Type: types.TypeSTRING})
	c.currentScope = scope
	result := c.checkMemberAccessExpr(&ast.MemberAccessExpr{
		Object: &ast.Ident{Name: "s"},
		Member: nil,
	})
	assert.Equal(t, types.Invalid, result)
}

// --- checkDerefExpr: pointer type ---

func TestChecker_DerefExpr_PointerType(t *testing.T) {
	src := `TYPE PtrInt : POINTER TO DINT; END_TYPE
	PROGRAM P
	VAR p : PtrInt; r : DINT; END_VAR
		r := p^;
	END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

// --- checkCallExpr: undeclared function ---

func TestChecker_CallExpr_UndeclaredFunc(t *testing.T) {
	src := `PROGRAM P
	VAR r : DINT; END_VAR
		r := UnknownFunc(1);
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeUndeclared {
			found = true
		}
	}
	assert.True(t, found, "expected undeclared error")
}

// --- checkBuiltinCall: generic constraint fail ---

func TestChecker_BuiltinCall_GenericConstraintFail(t *testing.T) {
	src := `PROGRAM P
	VAR r : DINT; s : STRING; END_VAR
		r := ABS(s);
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeWrongArgType {
			found = true
		}
	}
	assert.True(t, found, "expected type error for ABS(STRING)")
}

// --- checkUnaryExpr: unary minus on non-numeric ---

func TestChecker_UnaryMinus_String(t *testing.T) {
	src := `PROGRAM P
	VAR s : STRING; r : STRING; END_VAR
		r := -s;
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeTypeMismatch {
			found = true
		}
	}
	assert.True(t, found, "expected type mismatch for -STRING")
}

// --- resolveFunctionBlock: VAR_IN_OUT ---

func TestResolver_FunctionBlock_InOut(t *testing.T) {
	src := `FUNCTION_BLOCK MyFB
	VAR_IN_OUT x : DINT; END_VAR
	END_FUNCTION_BLOCK
	PROGRAM P
	END_PROGRAM`
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	sym := table.LookupGlobal("MyFB")
	assert.NotNil(t, sym)
}

// --- resolveFunction: VAR_IN_OUT, VAR_OUTPUT ---

func TestResolver_Function_InOut(t *testing.T) {
	src := `FUNCTION MyFunc : DINT
	VAR_INPUT a : DINT; END_VAR
	VAR_OUTPUT b : DINT; END_VAR
	VAR_IN_OUT c : DINT; END_VAR
	END_FUNCTION
	PROGRAM P
	END_PROGRAM`
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	sym := table.LookupGlobal("MyFunc")
	assert.NotNil(t, sym)
}

// --- resolveTypeDecl: enum type ---

func TestResolver_EnumType_Values(t *testing.T) {
	src := `TYPE Color : (Red, Green, Blue); END_TYPE
	PROGRAM P
	END_PROGRAM`
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	// Enum values should be in global scope
	red := table.LookupGlobal("Red")
	assert.NotNil(t, red)
	assert.Equal(t, symbols.KindEnumValue, red.Kind)
}

// --- checkBinaryExpr: comparison incompatible ---

func TestChecker_BinaryExpr_CompareIncompatible(t *testing.T) {
	src := `PROGRAM P
	VAR s : STRING; x : DINT; r : BOOL; END_VAR
		r := s = x;
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeIncompatibleOp {
			found = true
		}
	}
	assert.True(t, found, "expected incompatible comparison")
}

// --- checkWhileStmt: valid bool condition ---

func TestChecker_WhileStmt_ValidBool(t *testing.T) {
	src := `PROGRAM P
	VAR b : BOOL; END_VAR
		WHILE b DO
			b := FALSE;
		END_WHILE;
	END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

// --- checkRepeatStmt: valid bool condition ---

func TestChecker_RepeatStmt_ValidBool(t *testing.T) {
	src := `PROGRAM P
	VAR b : BOOL; END_VAR
		REPEAT
			b := TRUE;
		UNTIL b
		END_REPEAT;
	END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

// --- checkCallExpr: non-function symbol ---

func TestChecker_CallExpr_NonFunction(t *testing.T) {
	src := `PROGRAM P
	VAR x : DINT; r : DINT; END_VAR
		r := x(1);
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeNotCallable {
			found = true
		}
	}
	assert.True(t, found, "expected not-callable error")
}

// --- checkLiteral: all remaining literal types ---

func TestChecker_LitTyped_ValidPrefix(t *testing.T) {
	src := `PROGRAM P
	VAR x : INT; END_VAR
		x := INT#42;
	END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

// --- checkIdent: ident with no type ---

func TestChecker_Ident_NoType(t *testing.T) {
	table := symbols.NewTable()
	diags := diag.NewCollector()
	c := NewChecker(table, diags)
	scope := table.RegisterPOU("P", symbols.KindProgram, source.Pos{})
	scope.Insert(&symbols.Symbol{Name: "x", Kind: symbols.KindVariable, Type: nil})
	c.currentScope = scope
	result := c.checkIdent(&ast.Ident{Name: "x"})
	assert.Equal(t, types.Invalid, result)
}

// --- checkCallStmt: input arg with literal compatibility ---

func TestChecker_CallStmt_InputArgLiteralCompatible(t *testing.T) {
	src := `FUNCTION_BLOCK MyFB
	VAR_INPUT cmd : INT; END_VAR
	END_FUNCTION_BLOCK
	PROGRAM P
	VAR fb : MyFB; END_VAR
		fb(cmd := 42);
	END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

// --- resolveVarBlocksInScope: duplicate variable ---

func TestResolver_DuplicateVariable(t *testing.T) {
	src := `PROGRAM P
	VAR x : INT; x : DINT; END_VAR
	END_PROGRAM`
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	assert.True(t, diags.HasErrors(), "expected redeclaration error")
}

// --- resolveFunction: redeclaration ---

func TestResolver_Function_Redeclared(t *testing.T) {
	src := `FUNCTION MyFunc : DINT
	END_FUNCTION
	FUNCTION MyFunc : DINT
	END_FUNCTION`
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	assert.True(t, diags.HasErrors(), "expected redeclaration error")
}

// --- resolveFunctionBlock: redeclaration ---

func TestResolver_FunctionBlock_Redeclared(t *testing.T) {
	src := `FUNCTION_BLOCK MyFB
	END_FUNCTION_BLOCK
	FUNCTION_BLOCK MyFB
	END_FUNCTION_BLOCK`
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	assert.True(t, diags.HasErrors(), "expected redeclaration error")
}

// --- checkCallStmt: with output binding ---

func TestChecker_CallStmt_WithOutputBinding(t *testing.T) {
	src := `FUNCTION_BLOCK MyFB
	VAR_INPUT cmd : DINT; END_VAR
	VAR_OUTPUT result : DINT; END_VAR
	END_FUNCTION_BLOCK
	PROGRAM P
	VAR fb : MyFB; r : DINT; END_VAR
		fb(cmd := 1, result => r);
	END_PROGRAM`
	allDiags := runChecker(src)
	for _, d := range allDiags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

// --- resolveTypeDecl: redeclaration ---

func TestResolver_TypeDecl_Redeclared(t *testing.T) {
	src := `TYPE Color : (Red, Green); END_TYPE
	TYPE Color : (Blue); END_TYPE`
	file := parseFile(src)
	table := symbols.NewTable()
	diags := diag.NewCollector()
	resolver := NewResolver(table, diags)
	resolver.CollectDeclarations([]*ast.SourceFile{file})
	assert.True(t, diags.HasErrors(), "expected redeclaration error")
}

func TestChecker_ForStmt_NonIntBy(t *testing.T) {
	src := `PROGRAM P
	VAR i : DINT; x : LREAL; END_VAR
		FOR i := 1 TO 10 BY x DO
		END_FOR;
	END_PROGRAM`
	allDiags := runChecker(src)
	found := false
	for _, d := range allDiags {
		if d.Code == CodeTypeMismatch {
			found = true
		}
	}
	assert.True(t, found, "expected type mismatch for REAL BY value")
}
