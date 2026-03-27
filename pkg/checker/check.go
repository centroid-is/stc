package checker

import (
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/centroid-is/stc/pkg/types"
)

// Checker performs Pass 2 of semantic analysis: type-checking expressions,
// assignments, and control flow within POU bodies. It assumes Pass 1
// (Resolver.CollectDeclarations) has already populated the symbol table.
type Checker struct {
	table             *symbols.Table
	diags             *diag.Collector
	currentReturnType types.Type
	currentScope      *symbols.Scope
}

// NewChecker creates a new Checker using the given symbol table and diagnostics.
func NewChecker(table *symbols.Table, diags *diag.Collector) *Checker {
	return &Checker{table: table, diags: diags}
}

// CheckBodies walks all source files and type-checks POU bodies.
func (c *Checker) CheckBodies(files []*ast.SourceFile) {
	for _, file := range files {
		for _, decl := range file.Declarations {
			switch d := decl.(type) {
			case *ast.ProgramDecl:
				if d.Name != nil {
					c.checkPOUBody(d.Name.Name, d.Body)
				}
			case *ast.FunctionBlockDecl:
				if d.Name != nil {
					c.checkPOUBody(d.Name.Name, d.Body)
				}
			case *ast.FunctionDecl:
				if d.Name != nil {
					// Set return type for RETURN checks
					if sym := c.table.LookupGlobal(d.Name.Name); sym != nil {
						if fnType, ok := sym.Type.(*types.FunctionType); ok {
							c.currentReturnType = fnType.ReturnType
						}
					}
					c.checkPOUBody(d.Name.Name, d.Body)
					c.currentReturnType = nil
				}
			}
		}
	}
}

func (c *Checker) checkPOUBody(name string, body []ast.Statement) {
	pouScope := c.table.LookupPOU(name)
	if pouScope == nil {
		return
	}
	c.currentScope = pouScope
	for _, stmt := range body {
		c.checkStmt(stmt)
	}
	c.currentScope = nil
}

func (c *Checker) checkStmt(stmt ast.Statement) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		c.checkAssignStmt(s)
	case *ast.IfStmt:
		c.checkIfStmt(s)
	case *ast.ForStmt:
		c.checkForStmt(s)
	case *ast.WhileStmt:
		c.checkWhileStmt(s)
	case *ast.RepeatStmt:
		c.checkRepeatStmt(s)
	case *ast.CaseStmt:
		c.checkCaseStmt(s)
	case *ast.CallStmt:
		c.checkCallStmt(s)
	case *ast.ReturnStmt:
		// Nothing to check for bare RETURN
	case *ast.ExitStmt:
		// Nothing to check
	case *ast.ContinueStmt:
		// Nothing to check
	case *ast.EmptyStmt:
		// Nothing to check
	case *ast.ErrorNode:
		// Propagate, don't cascade
	}
}

func (c *Checker) checkAssignStmt(s *ast.AssignStmt) {
	targetType := c.checkExpr(s.Target)
	valueType := c.checkExpr(s.Value)

	if targetType == types.Invalid || valueType == types.Invalid {
		return
	}

	// Check type compatibility: value must widen to target
	if !targetType.Equal(valueType) {
		// Integer literals (default DINT) are compatible with any integer type
		// Real literals (default LREAL) are compatible with any real type
		if isLiteralExpr(s.Value) && isLiteralCompatible(valueType.Kind(), targetType.Kind()) {
			return
		}
		if !types.CanWiden(valueType.Kind(), targetType.Kind()) {
			pos := astPosToSource(s.Span().Start)
			c.diags.Errorf(pos, CodeTypeMismatch,
				"cannot assign %s to %s", valueType, targetType)
		}
	}
}

func (c *Checker) checkIfStmt(s *ast.IfStmt) {
	condType := c.checkExpr(s.Condition)
	if condType != types.Invalid && condType.Kind() != types.KindBOOL {
		pos := astPosToSource(s.Condition.Span().Start)
		c.diags.Errorf(pos, CodeTypeMismatch,
			"IF condition must be BOOL, got %s", condType)
	}

	for _, stmt := range s.Then {
		c.checkStmt(stmt)
	}
	for _, elif := range s.ElsIfs {
		elifCondType := c.checkExpr(elif.Condition)
		if elifCondType != types.Invalid && elifCondType.Kind() != types.KindBOOL {
			pos := astPosToSource(elif.Condition.Span().Start)
			c.diags.Errorf(pos, CodeTypeMismatch,
				"ELSIF condition must be BOOL, got %s", elifCondType)
		}
		for _, stmt := range elif.Body {
			c.checkStmt(stmt)
		}
	}
	for _, stmt := range s.Else {
		c.checkStmt(stmt)
	}
}

func (c *Checker) checkForStmt(s *ast.ForStmt) {
	// Check loop variable type
	if s.Variable != nil {
		varType := c.checkExpr(s.Variable)
		if varType != types.Invalid && !types.IsAnyInt(varType.Kind()) {
			pos := astPosToSource(s.Variable.Span().Start)
			c.diags.Errorf(pos, CodeTypeMismatch,
				"FOR loop variable must be an integer type, got %s", varType)
		}
	}

	// Check FROM, TO, BY expressions are integer-compatible
	if s.From != nil {
		fromType := c.checkExpr(s.From)
		if fromType != types.Invalid && !types.IsAnyInt(fromType.Kind()) {
			pos := astPosToSource(s.From.Span().Start)
			c.diags.Errorf(pos, CodeTypeMismatch,
				"FOR FROM expression must be an integer type, got %s", fromType)
		}
	}
	if s.To != nil {
		toType := c.checkExpr(s.To)
		if toType != types.Invalid && !types.IsAnyInt(toType.Kind()) {
			pos := astPosToSource(s.To.Span().Start)
			c.diags.Errorf(pos, CodeTypeMismatch,
				"FOR TO expression must be an integer type, got %s", toType)
		}
	}
	if s.By != nil {
		byType := c.checkExpr(s.By)
		if byType != types.Invalid && !types.IsAnyInt(byType.Kind()) {
			pos := astPosToSource(s.By.Span().Start)
			c.diags.Errorf(pos, CodeTypeMismatch,
				"FOR BY expression must be an integer type, got %s", byType)
		}
	}

	for _, stmt := range s.Body {
		c.checkStmt(stmt)
	}
}

func (c *Checker) checkWhileStmt(s *ast.WhileStmt) {
	condType := c.checkExpr(s.Condition)
	if condType != types.Invalid && condType.Kind() != types.KindBOOL {
		pos := astPosToSource(s.Condition.Span().Start)
		c.diags.Errorf(pos, CodeTypeMismatch,
			"WHILE condition must be BOOL, got %s", condType)
	}
	for _, stmt := range s.Body {
		c.checkStmt(stmt)
	}
}

func (c *Checker) checkRepeatStmt(s *ast.RepeatStmt) {
	for _, stmt := range s.Body {
		c.checkStmt(stmt)
	}
	condType := c.checkExpr(s.Condition)
	if condType != types.Invalid && condType.Kind() != types.KindBOOL {
		pos := astPosToSource(s.Condition.Span().Start)
		c.diags.Errorf(pos, CodeTypeMismatch,
			"REPEAT UNTIL condition must be BOOL, got %s", condType)
	}
}

func (c *Checker) checkCaseStmt(s *ast.CaseStmt) {
	exprType := c.checkExpr(s.Expr)

	for _, branch := range s.Branches {
		for _, label := range branch.Labels {
			switch l := label.(type) {
			case *ast.CaseLabelValue:
				labelType := c.checkExpr(l.Value)
				if exprType != types.Invalid && labelType != types.Invalid {
					if _, ok := types.CommonType(exprType.Kind(), labelType.Kind()); !ok {
						pos := astPosToSource(l.Span().Start)
						c.diags.Errorf(pos, CodeTypeMismatch,
							"case label type %s incompatible with selector type %s",
							labelType, exprType)
					}
				}
			case *ast.CaseLabelRange:
				c.checkExpr(l.Low)
				c.checkExpr(l.High)
			}
		}
		for _, stmt := range branch.Body {
			c.checkStmt(stmt)
		}
	}
	for _, stmt := range s.ElseBranch {
		c.checkStmt(stmt)
	}
}

func (c *Checker) checkCallStmt(s *ast.CallStmt) {
	if s.Callee == nil {
		return
	}

	// Resolve callee - should be an FB instance
	calleeType := c.checkExpr(s.Callee)
	if calleeType == types.Invalid {
		return
	}

	fbType, ok := calleeType.(*types.FunctionBlockType)
	if !ok {
		pos := astPosToSource(s.Callee.Span().Start)
		c.diags.Errorf(pos, CodeNotCallable,
			"cannot call %s (type %s is not a function block)", exprName(s.Callee), calleeType)
		return
	}

	// Validate named arguments against FB inputs/outputs
	for _, arg := range s.Args {
		if arg.Name == nil {
			continue
		}
		argName := strings.ToUpper(arg.Name.Name)

		// Find the parameter in the FB type
		var paramType types.Type
		found := false
		if arg.IsOutput {
			for _, out := range fbType.Outputs {
				if strings.ToUpper(out.Name) == argName {
					paramType = out.Type
					found = true
					break
				}
			}
		} else {
			for _, in := range fbType.Inputs {
				if strings.ToUpper(in.Name) == argName {
					paramType = in.Type
					found = true
					break
				}
			}
		}

		if !found {
			pos := astPosToSource(arg.Name.Span().Start)
			c.diags.Errorf(pos, CodeNoMember,
				"%s has no %s parameter %q",
				fbType.Name,
				paramDirStr(arg.IsOutput),
				arg.Name.Name)
			continue
		}

		if arg.Value != nil {
			argType := c.checkExpr(arg.Value)
			if argType != types.Invalid && paramType != nil {
				if !paramType.Equal(argType) && !types.CanWiden(argType.Kind(), paramType.Kind()) {
					// Allow literal compatibility (e.g., integer literal 100 passed as INT param)
					if isLiteralExpr(arg.Value) && isLiteralCompatible(argType.Kind(), paramType.Kind()) {
						continue
					}
					pos := astPosToSource(arg.Value.Span().Start)
					c.diags.Errorf(pos, CodeWrongArgType,
						"cannot pass %s as %s parameter %q (expected %s)",
						argType, paramDirStr(arg.IsOutput), arg.Name.Name, paramType)
				}
			}
		}
	}
}

// checkExpr type-checks an expression and returns its resolved type.
func (c *Checker) checkExpr(expr ast.Expr) types.Type {
	if expr == nil {
		return types.Invalid
	}

	switch e := expr.(type) {
	case *ast.Ident:
		return c.checkIdent(e)
	case *ast.Literal:
		return c.checkLiteral(e)
	case *ast.BinaryExpr:
		return c.checkBinaryExpr(e)
	case *ast.UnaryExpr:
		return c.checkUnaryExpr(e)
	case *ast.CallExpr:
		return c.checkCallExpr(e)
	case *ast.MemberAccessExpr:
		return c.checkMemberAccessExpr(e)
	case *ast.IndexExpr:
		return c.checkIndexExpr(e)
	case *ast.DerefExpr:
		return c.checkDerefExpr(e)
	case *ast.ParenExpr:
		return c.checkExpr(e.Inner)
	case *ast.ErrorNode:
		return types.Invalid
	}
	return types.Invalid
}

func (c *Checker) checkIdent(e *ast.Ident) types.Type {
	if c.currentScope == nil {
		return types.Invalid
	}
	sym := c.currentScope.Lookup(e.Name)
	if sym == nil {
		pos := astPosToSource(e.Span().Start)
		c.diags.Errorf(pos, CodeUndeclared,
			"undeclared identifier %q", e.Name)
		return types.Invalid
	}
	sym.MarkUsed()

	if sym.Type != nil {
		if t, ok := sym.Type.(types.Type); ok {
			return t
		}
	}
	return types.Invalid
}

func (c *Checker) checkLiteral(e *ast.Literal) types.Type {
	switch e.LitKind {
	case ast.LitInt:
		return types.TypeDINT // Default integer literal type
	case ast.LitReal:
		return types.TypeLREAL // Default real literal type
	case ast.LitBool:
		return types.TypeBOOL
	case ast.LitString:
		return types.TypeSTRING
	case ast.LitWString:
		return types.TypeWSTRING
	case ast.LitTime:
		return types.TypeTIME
	case ast.LitDate:
		return types.TypeDATE
	case ast.LitDateTime:
		return types.TypeDT
	case ast.LitTod:
		return types.TypeTOD
	case ast.LitTyped:
		// Look up the type prefix
		if e.TypePrefix != "" {
			if t, ok := types.LookupElementaryType(e.TypePrefix); ok {
				return t
			}
		}
		return types.TypeDINT
	}
	return types.Invalid
}

func (c *Checker) checkBinaryExpr(e *ast.BinaryExpr) types.Type {
	left := c.checkExpr(e.Left)
	right := c.checkExpr(e.Right)

	if left == types.Invalid || right == types.Invalid {
		return types.Invalid // propagate errors, don't cascade
	}

	op := strings.ToUpper(e.Op.Text)
	switch {
	case isArithmeticOp(op):
		common, ok := types.CommonType(left.Kind(), right.Kind())
		if !ok {
			pos := astPosToSource(e.Op.Span.Start)
			c.diags.Errorf(pos, CodeIncompatibleOp,
				"operator %s not defined for types %s and %s", e.Op.Text, left, right)
			return types.Invalid
		}
		return &types.PrimitiveType{Kind_: common}

	case isComparisonOp(op):
		_, ok := types.CommonType(left.Kind(), right.Kind())
		if !ok {
			pos := astPosToSource(e.Op.Span.Start)
			c.diags.Errorf(pos, CodeIncompatibleOp,
				"cannot compare %s and %s", left, right)
			return types.Invalid
		}
		return types.TypeBOOL

	case isBooleanOp(op):
		if left.Kind() != types.KindBOOL || right.Kind() != types.KindBOOL {
			pos := astPosToSource(e.Op.Span.Start)
			c.diags.Errorf(pos, CodeTypeMismatch,
				"boolean operator %s requires BOOL operands, got %s and %s",
				e.Op.Text, left, right)
			return types.Invalid
		}
		return types.TypeBOOL
	}

	return types.Invalid
}

func (c *Checker) checkUnaryExpr(e *ast.UnaryExpr) types.Type {
	operandType := c.checkExpr(e.Operand)
	if operandType == types.Invalid {
		return types.Invalid
	}

	op := strings.ToUpper(e.Op.Text)
	switch op {
	case "NOT":
		if operandType.Kind() != types.KindBOOL {
			pos := astPosToSource(e.Op.Span.Start)
			c.diags.Errorf(pos, CodeTypeMismatch,
				"NOT requires BOOL operand, got %s", operandType)
			return types.Invalid
		}
		return types.TypeBOOL
	case "-":
		if !types.IsAnyNum(operandType.Kind()) {
			pos := astPosToSource(e.Op.Span.Start)
			c.diags.Errorf(pos, CodeTypeMismatch,
				"unary minus requires numeric operand, got %s", operandType)
			return types.Invalid
		}
		return operandType
	}
	return operandType
}

func (c *Checker) checkCallExpr(e *ast.CallExpr) types.Type {
	// Resolve callee
	calleeName := exprName(e.Callee)
	if calleeName == "" {
		return types.Invalid
	}

	// Check built-in functions first
	upperName := strings.ToUpper(calleeName)
	if fnType, ok := types.BuiltinFunctions[upperName]; ok {
		return c.checkBuiltinCall(e, fnType)
	}

	// Check user-defined functions
	if c.currentScope != nil {
		sym := c.currentScope.Lookup(calleeName)
		if sym == nil {
			pos := astPosToSource(e.Callee.Span().Start)
			c.diags.Errorf(pos, CodeUndeclared,
				"undeclared identifier %q", calleeName)
			return types.Invalid
		}
		sym.MarkUsed()

		if fnType, ok := sym.Type.(*types.FunctionType); ok {
			return c.checkUserFuncCall(e, fnType)
		}

		pos := astPosToSource(e.Callee.Span().Start)
		c.diags.Errorf(pos, CodeNotCallable,
			"%q is not callable (type %v)", calleeName, sym.Type)
		return types.Invalid
	}
	return types.Invalid
}

func (c *Checker) checkBuiltinCall(e *ast.CallExpr, fnType *types.FunctionType) types.Type {
	// Validate argument count
	if len(e.Args) != len(fnType.Params) {
		pos := astPosToSource(e.Span().Start)
		c.diags.Errorf(pos, CodeWrongArgCount,
			"%s expects %d argument(s), got %d",
			fnType.Name, len(fnType.Params), len(e.Args))
		return types.Invalid
	}

	// Type-check arguments
	argTypes := make([]types.Type, len(e.Args))
	for i, arg := range e.Args {
		argTypes[i] = c.checkExpr(arg)
	}

	// Use candidate resolution for generic functions
	retType, _, ok := ResolveCandidates(fnType, argTypes)
	if !ok {
		pos := astPosToSource(e.Span().Start)
		c.diags.Errorf(pos, CodeWrongArgType,
			"no matching overload for %s with argument types", fnType.Name)
		return types.Invalid
	}

	return retType
}

func (c *Checker) checkUserFuncCall(e *ast.CallExpr, fnType *types.FunctionType) types.Type {
	// Validate argument count
	if len(e.Args) != len(fnType.Params) {
		pos := astPosToSource(e.Span().Start)
		c.diags.Errorf(pos, CodeWrongArgCount,
			"%s expects %d argument(s), got %d",
			fnType.Name, len(fnType.Params), len(e.Args))
		return types.Invalid
	}

	// Type-check arguments
	for i, arg := range e.Args {
		argType := c.checkExpr(arg)
		if argType == types.Invalid || i >= len(fnType.Params) {
			continue
		}
		paramType := fnType.Params[i].Type
		if paramType != nil && !paramType.Equal(argType) {
			if !types.CanWiden(argType.Kind(), paramType.Kind()) {
				pos := astPosToSource(arg.Span().Start)
				c.diags.Errorf(pos, CodeWrongArgType,
					"argument %d: cannot pass %s as %s",
					i+1, argType, paramType)
			}
		}
	}

	return fnType.ReturnType
}

func (c *Checker) checkMemberAccessExpr(e *ast.MemberAccessExpr) types.Type {
	objType := c.checkExpr(e.Object)
	if objType == types.Invalid {
		return types.Invalid
	}
	if e.Member == nil {
		return types.Invalid
	}
	memberName := e.Member.Name

	switch t := objType.(type) {
	case *types.StructType:
		for _, m := range t.Members {
			if strings.EqualFold(m.Name, memberName) {
				return m.Type
			}
		}
		pos := astPosToSource(e.Member.Span().Start)
		c.diags.Errorf(pos, CodeNoMember,
			"type %s has no member %q", t, memberName)
		return types.Invalid

	case *types.FunctionBlockType:
		// Look up inputs and outputs
		for _, in := range t.Inputs {
			if strings.EqualFold(in.Name, memberName) {
				return in.Type
			}
		}
		for _, out := range t.Outputs {
			if strings.EqualFold(out.Name, memberName) {
				return out.Type
			}
		}
		for _, io := range t.InOuts {
			if strings.EqualFold(io.Name, memberName) {
				return io.Type
			}
		}
		pos := astPosToSource(e.Member.Span().Start)
		c.diags.Errorf(pos, CodeNoMember,
			"type %s has no member %q", t, memberName)
		return types.Invalid

	case *types.EnumType:
		for _, val := range t.Values {
			if strings.EqualFold(val, memberName) {
				return t
			}
		}
		pos := astPosToSource(e.Member.Span().Start)
		c.diags.Errorf(pos, CodeNoMember,
			"enum %s has no value %q", t, memberName)
		return types.Invalid
	}

	pos := astPosToSource(e.Member.Span().Start)
	c.diags.Errorf(pos, CodeNoMember,
		"type %s does not support member access", objType)
	return types.Invalid
}

func (c *Checker) checkIndexExpr(e *ast.IndexExpr) types.Type {
	objType := c.checkExpr(e.Object)
	if objType == types.Invalid {
		return types.Invalid
	}

	arrType, ok := objType.(*types.ArrayType)
	if !ok {
		pos := astPosToSource(e.Span().Start)
		c.diags.Errorf(pos, CodeNotIndexable,
			"type %s does not support indexing", objType)
		return types.Invalid
	}

	// Check each index is an integer type
	for _, idx := range e.Indices {
		idxType := c.checkExpr(idx)
		if idxType != types.Invalid && !types.IsAnyInt(idxType.Kind()) {
			pos := astPosToSource(idx.Span().Start)
			c.diags.Errorf(pos, CodeTypeMismatch,
				"array index must be an integer type, got %s", idxType)
		}
	}

	return arrType.ElementType
}

func (c *Checker) checkDerefExpr(e *ast.DerefExpr) types.Type {
	operandType := c.checkExpr(e.Operand)
	if operandType == types.Invalid {
		return types.Invalid
	}

	if pt, ok := operandType.(*types.PointerType); ok {
		return pt.BaseType
	}
	// Per user decision: represent but don't deeply validate
	return operandType
}

// --- Helper functions ---

func isArithmeticOp(op string) bool {
	switch op {
	case "+", "-", "*", "/", "MOD", "**":
		return true
	}
	return false
}

func isComparisonOp(op string) bool {
	switch op {
	case "=", "<>", "<", ">", "<=", ">=":
		return true
	}
	return false
}

func isBooleanOp(op string) bool {
	switch op {
	case "AND", "OR", "XOR", "&":
		return true
	}
	return false
}

func exprName(e ast.Expr) string {
	if e == nil {
		return ""
	}
	switch expr := e.(type) {
	case *ast.Ident:
		return expr.Name
	}
	return ""
}

func paramDirStr(isOutput bool) string {
	if isOutput {
		return "output"
	}
	return "input"
}

// isLiteralExpr checks if an expression is a literal value.
func isLiteralExpr(e ast.Expr) bool {
	switch e.(type) {
	case *ast.Literal:
		return true
	case *ast.UnaryExpr:
		// Negative literals like -42
		ue := e.(*ast.UnaryExpr)
		_, ok := ue.Operand.(*ast.Literal)
		return ok
	}
	return false
}

// isLiteralCompatible checks if a literal type can be used where
// a target type is expected. Integer literals are compatible with any
// integer type, and real literals with any real type.
func isLiteralCompatible(litKind, targetKind types.TypeKind) bool {
	if types.IsAnyInt(litKind) && types.IsAnyInt(targetKind) {
		return true
	}
	if types.IsAnyReal(litKind) && types.IsAnyReal(targetKind) {
		return true
	}
	return false
}
