package checker

import (
	"fmt"
	"sort"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/iomap"
	"github.com/centroid-is/stc/pkg/source"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/centroid-is/stc/pkg/types"
)

// Checker performs Pass 2 of semantic analysis: type-checking expressions,
// assignments, and control flow within POU bodies. It assumes Pass 1
// (Resolver.CollectDeclarations) has already populated the symbol table.
type Checker struct {
	table               *symbols.Table
	diags               *diag.Collector
	currentReturnType   types.Type
	currentFunctionName string
	currentScope        *symbols.Scope
	// currentFB is the FUNCTION_BLOCK whose body or action is being
	// checked; THIS and SUPER refer to it. nil outside an FB.
	currentFB *ast.FunctionBlockDecl
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
					c.checkATAddresses(d.VarBlocks, "PROGRAM")
					c.checkVarInitializers(d.VarBlocks, c.table.LookupPOU(d.Name.Name))
					c.checkPOUBody(d.Name.Name, d.Body)
					c.checkActionBodies(d.Name.Name, d.Actions)
				}
			case *ast.FunctionBlockDecl:
				if d.Name != nil {
					c.checkATAddresses(d.VarBlocks, "FUNCTION_BLOCK")
					c.checkVarInitializers(d.VarBlocks, c.table.LookupPOU(d.Name.Name))
					c.currentFB = d
					c.checkPOUBody(d.Name.Name, d.Body)
					c.checkActionBodies(d.Name.Name, d.Actions)
					c.currentFB = nil
				}
			case *ast.TypeDecl:
				if st, ok := d.Type.(*ast.StructType); ok {
					c.checkStructATAddresses(st)
					if d.Name != nil {
						c.checkStructMemberInitializers(d, st)
					}
				}
			case *ast.GVLDecl:
				c.checkATAddresses(d.Blocks, "GVL")
				c.checkGVLInitializers(d)
			case *ast.FunctionDecl:
				if d.Name != nil {
					c.checkATAddresses(d.VarBlocks, "FUNCTION")
					c.checkVarInitializers(d.VarBlocks, c.table.LookupPOU(d.Name.Name))
					// Set return type for RETURN checks
					if sym := c.table.LookupGlobal(d.Name.Name); sym != nil {
						if fnType, ok := sym.Type.(*types.FunctionType); ok {
							c.currentReturnType = fnType.ReturnType
						}
					}
					c.currentFunctionName = d.Name.Name
					c.checkPOUBody(d.Name.Name, d.Body)
					c.currentReturnType = nil
					c.currentFunctionName = ""
				}
			}
		}
	}
}

// checkStructATAddresses validates AT addresses on STRUCT members.
// Wildcards are silent; explicit addresses warn because every instance of the
// struct would share the same I/O location.
func (c *Checker) checkStructATAddresses(st *ast.StructType) {
	for _, m := range st.Members {
		if m.AtAddress == nil {
			continue
		}
		pos := astPosToSource(m.AtAddress.Span().Start)
		addr, err := iomap.ParseAddress(m.AtAddress.Name)
		if err != nil {
			c.diags.Errorf(pos, CodeInvalidATAddress,
				"invalid I/O address %q: %s", m.AtAddress.Name, err)
			continue
		}
		if !addr.IsWildcard {
			c.diags.Warnf(pos, CodeATNotAllowedHere,
				"explicit AT address on STRUCT member %q: all instances share the same address; use %%I* / %%Q*",
				m.Name.Name)
		}
	}
}

// atBinding pairs a variable name with its parsed AT address for overlap detection.
type atBinding struct {
	varName string
	addr    iomap.IOAddress
}

// checkATAddresses validates AT address declarations in var blocks.
// It checks format validity, POU type restrictions, and address overlap.
func (c *Checker) checkATAddresses(varBlocks []*ast.VarBlock, pouType string) {
	var bindings []atBinding

	for _, vb := range varBlocks {
		for _, vd := range vb.Declarations {
			if vd.AtAddress == nil {
				continue
			}

			pos := astPosToSource(vd.AtAddress.Span().Start)

			// Validate address format
			addr, err := iomap.ParseAddress(vd.AtAddress.Name)
			if err != nil {
				c.diags.Errorf(pos, CodeInvalidATAddress,
					"invalid I/O address %q: %s", vd.AtAddress.Name, err)
				continue
			}

			// Check POU type restriction. Wildcards (%I*, %Q*, %M*) are
			// linked by the IDE per instance and are valid in FBs.
			if pouType != "PROGRAM" && pouType != "GVL" && !addr.IsWildcard {
				c.diags.Warnf(pos, CodeATNotAllowedHere,
					"AT address declarations are only valid in PROGRAM blocks, not %s", pouType)
			}

			// Collect for overlap detection (skip wildcards)
			if !addr.IsWildcard {
				varName := ""
				if len(vd.Names) > 0 {
					varName = vd.Names[0].Name
				}
				bindings = append(bindings, atBinding{varName: varName, addr: addr})
			}
		}
	}

	// Check for overlapping addresses within the same area
	for i := 0; i < len(bindings); i++ {
		for j := i + 1; j < len(bindings); j++ {
			a := bindings[i]
			b := bindings[j]

			// Only check within the same area
			if a.addr.Area != b.addr.Area {
				continue
			}

			offA, lenA := a.addr.ByteSpan()
			offB, lenB := b.addr.ByteSpan()

			endA := offA + lenA
			endB := offB + lenB

			// Check byte range overlap: [offA, endA) intersects [offB, endB)
			if offA < endB && offB < endA {
				// For two bit addresses in the same byte but different bits: no overlap
				if a.addr.Size == iomap.SizeBit && b.addr.Size == iomap.SizeBit &&
					a.addr.ByteOffset == b.addr.ByteOffset &&
					a.addr.BitOffset != b.addr.BitOffset {
					continue
				}

				overlapStart := offA
				if offB > overlapStart {
					overlapStart = offB
				}
				overlapEnd := endA
				if endB < overlapEnd {
					overlapEnd = endB
				}

				c.diags.Warnf(astPosToSource(ast.Pos{}), CodeATOverlap,
					"AT addresses %s (%s) and %s (%s) overlap in byte range [%d..%d]",
					a.addr.String(), a.varName,
					b.addr.String(), b.varName,
					overlapStart, overlapEnd-1)
			}
		}
	}
}

// checkActionBodies checks each action body in the owning POU's scope, so
// variables used only inside actions are marked used.
func (c *Checker) checkActionBodies(pouName string, actions []*ast.ActionDecl) {
	for _, a := range actions {
		c.checkPOUBody(pouName, a.Body)
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
	case *ast.RefAssignStmt:
		c.checkRefAssignStmt(s)
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
	if s.Value != nil {
		c.checkConstantTarget(s.Target)
	}
	targetType := c.checkExpr(s.Target)
	valueType := c.checkExpr(s.Value)

	// IEC 61131-3: a FUNCTION returns its value by assigning to its own
	// name. The identifier resolves to the function type, so substitute
	// the declared return type before checking compatibility.
	if c.currentReturnType != nil && c.currentFunctionName != "" {
		if id, ok := s.Target.(*ast.Ident); ok && strings.EqualFold(id.Name, c.currentFunctionName) {
			targetType = c.currentReturnType
		}
	}

	if targetType == types.Invalid || valueType == types.Invalid {
		return
	}

	// A reference reads and writes through to its target (research
	// Pitfall 10); a reference assigned to a reference keeps both types.
	if !(isReference(targetType) && isReference(valueType)) {
		targetType, valueType = derefRef(targetType), derefRef(valueType)
	}

	if involvesEnum(valueType, targetType) {
		c.checkEnumAssign(s, valueType, targetType)
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
				c.caseLabelCompatible(l, exprType, c.checkExpr(l.Value))
			case *ast.CaseLabelRange:
				c.caseLabelCompatible(l, exprType, c.checkExpr(l.Low))
				c.caseLabelCompatible(l, exprType, c.checkExpr(l.High))
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
		// The callee is already reported (undeclared name or type). Still
		// check the argument values so their variables count as used.
		for _, arg := range s.Args {
			if arg.Value != nil {
				c.checkExpr(arg.Value)
			}
		}
		return
	}

	if fnType, ok := calleeType.(*types.FunctionType); ok {
		// A FUNCTION, METHOD or ACTION called as a statement with formal
		// arguments: M(a := x);
		c.checkFuncCallStmtArgs(s, fnType)
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
	bound := make(map[string]bool)
	for _, arg := range s.Args {
		if arg.Name == nil {
			continue
		}
		argName := strings.ToUpper(arg.Name.Name)
		// An alias (CTU.R) and its canonical input (RESET) set the same
		// field, so binding both is a double binding.
		canon := argName
		if c, ok := fbType.ParamAliases[argName]; ok && !arg.IsOutput {
			canon = c
		}
		if bound[canon] {
			c.diags.Errorf(astPosToSource(arg.Span().Start), CodeNoMember,
				"parameter %q of %s is bound more than once", arg.Name.Name, fbType.Name)
		}
		bound[canon] = true

		// Find the parameter in the FB type
		var paramType types.Type
		found, inOut := false, false
		if arg.IsOutput {
			for _, out := range fbType.Outputs {
				if strings.ToUpper(out.Name) == argName {
					paramType = out.Type
					found = true
					break
				}
			}
		} else {
			// name := value binds a VAR_INPUT or a VAR_IN_OUT.
			for _, in := range append(fbType.Inputs[:len(fbType.Inputs):len(fbType.Inputs)], fbType.InOuts...) {
				if strings.ToUpper(in.Name) == argName {
					paramType = in.Type
					found = true
					inOut = in.Direction == types.DirInOut
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
			if inOut && !c.checkInOutArg(fbType.Name, types.Parameter{Name: arg.Name.Name}, arg.Value) {
				continue
			}
			argType := c.checkExpr(arg.Value)
			if argType != types.Invalid && paramType != nil && involvesEnum(argType, paramType) {
				from, to := argType, paramType
				if arg.IsOutput {
					from, to = paramType, argType
				}
				c.checkEnumArg(arg.Value, from, to, func() {
					c.diags.Errorf(astPosToSource(arg.Value.Span().Start), CodeWrongArgType,
						"cannot pass %s as %s parameter %q (expected %s)",
						argType, paramDirStr(arg.IsOutput), arg.Name.Name, paramType)
				})
				continue
			}
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

// checkFuncCallStmtArgs checks the formal arguments of a function-like call
// statement (F(a := 1);) with the same binding rules as a call expression.
func (c *Checker) checkFuncCallStmtArgs(s *ast.CallStmt, fnType *types.FunctionType) {
	c.bindCallArgs(s, fnType, s.Args)
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
	case *ast.BitAccessExpr:
		return c.checkBitAccessExpr(e)
	case *ast.ThisExpr:
		return c.checkThisExpr(e)
	case *ast.SuperExpr:
		return c.checkSuperExpr(e)
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
		if !c.reportQualifiedEnumValue(pos, e.Name) {
			c.reportUndeclared(pos, e.Name)
		}
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

// reportUndeclared reports a name that resolves to nothing. When the name is
// a variable of one or more qualified_only GVLs the error is SEMA033 and
// points at the qualified form; otherwise it is the usual SEMA010.
func (c *Checker) reportUndeclared(pos source.Pos, name string) {
	gvls := c.qualifiedOnlyGVLs(name)
	if len(gvls) == 0 {
		c.diags.Errorf(pos, CodeUndeclared, "undeclared identifier %q", name)
		return
	}
	msg := fmt.Sprintf("GVL '%s' is qualified_only; use %s.%s", gvls[0], gvls[0], name)
	if len(gvls) > 1 {
		msg += fmt.Sprintf(" (also declared in %s)", strings.Join(gvls[1:], ", "))
	}
	c.diags.Errorf(pos, CodeGVLQualifiedOnly, "%s", msg)
}

// qualifiedOnlyGVLs returns the sorted names of qualified_only GVLs that
// declare a variable called name.
func (c *Checker) qualifiedOnlyGVLs(name string) []string {
	key := strings.ToUpper(name)
	var out []string
	for _, sym := range c.table.GlobalScope().Symbols() {
		if sym.Kind == symbols.KindGVL && sym.GVL.QualifiedOnly && sym.GVL.Vars[key] {
			out = append(out, sym.Name)
		}
	}
	sort.Strings(out)
	return out
}

// checkConstantTarget reports SEMA034 when an assignment writes a
// VAR_GLOBAL CONSTANT member, either bare (x := ...) or qualified (G.x := ...).
func (c *Checker) checkConstantTarget(target ast.Expr) {
	var name string
	constant := false
	// Writing a bit writes the variable that holds it.
	target = c.assignRoot(target)
	switch t := target.(type) {
	case *ast.Ident:
		sym := c.currentScope.Lookup(t.Name)
		name = t.Name
		constant = sym != nil && sym.IsConstant
	case *ast.MemberAccessExpr:
		obj, ok := t.Object.(*ast.Ident)
		if !ok || t.Member == nil {
			return
		}
		sym := c.currentScope.Lookup(obj.Name)
		name = t.Member.Name
		constant = sym != nil && sym.Kind == symbols.KindGVL && sym.GVL.Constants[strings.ToUpper(name)]
	}
	if constant {
		c.diags.Errorf(astPosToSource(target.Span().Start), CodeAssignToConstant,
			"cannot assign to constant '%s'", name)
	}
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
	left := derefRef(c.checkExpr(e.Left))
	right := derefRef(c.checkExpr(e.Right))

	if left == types.Invalid || right == types.Invalid {
		return types.Invalid // propagate errors, don't cascade
	}

	op := strings.ToUpper(e.Op.Text)
	left, right, result, done, ok := c.checkEnumBinary(e, op, left, right)
	if !ok {
		return types.Invalid
	}
	if done {
		return result
	}
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
	operandType := derefRef(c.checkExpr(e.Operand))
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
		// Member callees (inst.M(), THIS^.M(), GVL.fb()) are accepted
		// unchecked, arguments included (research Pitfall 11): the callee
		// type of a member call is not resolved yet. The instance they are
		// called on still counts as used, and THIS^ / SUPER^ are validated.
		c.checkMemberCalleeRoot(e.Callee)
		return types.Invalid
	}

	// A METHOD or ACTION of the current POU (own or inherited) binds
	// before a built-in function of the same name.
	if c.currentScope != nil {
		if sym := lookupInPOUChain(c.currentScope, calleeName); sym != nil {
			if fnType, ok := sym.Type.(*types.FunctionType); ok {
				sym.MarkUsed()
				return c.checkUserFuncCall(e, fnType)
			}
		}
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
			c.reportUndeclared(astPosToSource(e.Callee.Span().Start), calleeName)
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

	// Type-check arguments. A conversion function takes any enum as its
	// base integer type; other built-ins take only non-strict enums that
	// way, and a strict enum passed to a typed parameter is SEMA036.
	conversion := isConversionBuiltin(fnType.Name)
	argTypes := make([]types.Type, len(e.Args))
	for i, arg := range e.Args {
		argTypes[i] = c.checkExpr(arg)
		if et := asEnum(argTypes[i]); et != nil {
			switch {
			case conversion || !et.Strict:
				argTypes[i] = enumBase(et)
			case fnType.Params[i].Type != nil:
				c.diags.Errorf(astPosToSource(arg.Span().Start), CodeEnumRule,
					"strict enum %s passed to %s; use an explicit conversion", et.Name, fnType.Name)
				return types.Invalid
			}
		}
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

// checkUserFuncCall checks a call of a user FUNCTION, METHOD or ACTION and
// returns its return type.
func (c *Checker) checkUserFuncCall(e *ast.CallExpr, fnType *types.FunctionType) types.Type {
	if !c.checkCallArgs(e, fnType) {
		return types.Invalid
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

	// v.cBit on an integer value with a CONSTANT index is bit access.
	if typ, ok := c.checkConstBitIndex(e, objType); ok {
		return typ
	}
	objType = derefRef(objType)
	if typ, ok := c.selfMember(e.Object, objType, e.Member); ok {
		return typ
	}

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
	objType := derefRef(c.checkExpr(e.Object))
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

// markRootUsed marks the variable at the root of a member-access chain
// (inst in inst.A1 or a.b.c) as used. Unknown roots are left alone: the
// call is accepted without checking, so no diagnostic is added here.
func (c *Checker) markRootUsed(e ast.Expr) {
	for walking := true; walking; {
		switch x := e.(type) {
		case *ast.MemberAccessExpr:
			e = x.Object
		case *ast.BitAccessExpr:
			e = x.Target
		default:
			walking = false
		}
	}
	id, ok := e.(*ast.Ident)
	if !ok || c.currentScope == nil {
		return
	}
	if sym := c.currentScope.Lookup(id.Name); sym != nil {
		sym.MarkUsed()
	}
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
