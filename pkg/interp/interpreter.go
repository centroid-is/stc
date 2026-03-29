package interp

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
)

// Interpreter evaluates typed ASTs via tree-walking.
type Interpreter struct {
	// MaxLoopIterations is a safety limit for while/repeat loops.
	MaxLoopIterations int

	// dt is the current scan cycle delta time, passed to FB Execute calls.
	dt time.Duration

	// LocalFunctions provides per-interpreter-instance function overrides.
	// These take priority over global StdlibFunctions during evalCall.
	// Used for test-specific functions (assertions, ADVANCE_TIME) to avoid
	// global state mutation between test cases.
	LocalFunctions map[string]func(args []Value, pos ast.Pos) (Value, error)

	// Collector gathers assertion results during test execution.
	// Each test case gets its own collector via RegisterAssertions.
	Collector *AssertionCollector
}

// New creates a new Interpreter with default settings.
func New() *Interpreter {
	return &Interpreter{
		MaxLoopIterations: 1_000_000,
	}
}

// SetDt sets the current scan cycle delta time on the interpreter.
// Used by the test runner for ADVANCE_TIME support.
func (interp *Interpreter) SetDt(dt time.Duration) {
	interp.dt = dt
}

// EvalExpr is the exported wrapper around evalExpr for use by the test runner.
func (interp *Interpreter) EvalExpr(env *Env, expr ast.Expr) (Value, error) {
	return interp.evalExpr(env, expr)
}

// --- Expression evaluation ---

// evalExpr evaluates an expression AST node and returns its runtime Value.
func (interp *Interpreter) evalExpr(env *Env, expr ast.Expr) (Value, error) {
	switch e := expr.(type) {
	case *ast.Literal:
		return interp.evalLiteral(e)
	case *ast.Ident:
		return interp.evalIdent(env, e)
	case *ast.BinaryExpr:
		return interp.evalBinary(env, e)
	case *ast.UnaryExpr:
		return interp.evalUnary(env, e)
	case *ast.ParenExpr:
		return interp.evalExpr(env, e.Inner)
	case *ast.IndexExpr:
		return interp.evalIndex(env, e)
	case *ast.CallExpr:
		return interp.evalCall(env, e)
	case *ast.MemberAccessExpr:
		return interp.evalMemberAccess(env, e)
	case *ast.DerefExpr:
		return Value{}, &RuntimeError{Msg: "pointer dereference not yet implemented"}
	case *ast.ErrorNode:
		return Value{}, &RuntimeError{Msg: "cannot evaluate error node"}
	default:
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("unsupported expression type: %T", expr)}
	}
}

// evalLiteral parses a literal's string value into a typed Value.
func (interp *Interpreter) evalLiteral(lit *ast.Literal) (Value, error) {
	switch lit.LitKind {
	case ast.LitInt:
		return interp.parseLitInt(lit.Value)
	case ast.LitReal:
		return interp.parseLitReal(lit.Value)
	case ast.LitBool:
		return interp.parseLitBool(lit.Value)
	case ast.LitString:
		return interp.parseLitString(lit.Value)
	case ast.LitTime:
		return interp.parseLitTime(lit.Value)
	case ast.LitTyped:
		return interp.parseLitTyped(lit.Value, lit.TypePrefix)
	case ast.LitWString:
		return interp.parseLitString(lit.Value)
	default:
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("unsupported literal kind: %v", lit.LitKind)}
	}
}

func (interp *Interpreter) parseLitInt(s string) (Value, error) {
	// Remove underscores (IEC allows 1_000)
	s = strings.ReplaceAll(s, "_", "")

	// Check for base-prefixed literals: 16#FF, 2#1010, 8#77
	if idx := strings.Index(s, "#"); idx > 0 {
		baseStr := s[:idx]
		digits := s[idx+1:]
		base, err := strconv.Atoi(baseStr)
		if err != nil {
			return Value{}, &RuntimeError{Msg: fmt.Sprintf("invalid integer base: %s", baseStr)}
		}
		n, err := strconv.ParseInt(digits, base, 64)
		if err != nil {
			return Value{}, &RuntimeError{Msg: fmt.Sprintf("invalid integer literal: %s", s)}
		}
		return Value{Kind: ValInt, Int: n, IECType: types.KindDINT}, nil
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("invalid integer literal: %s", s)}
	}
	return Value{Kind: ValInt, Int: n, IECType: types.KindDINT}, nil
}

func (interp *Interpreter) parseLitReal(s string) (Value, error) {
	s = strings.ReplaceAll(s, "_", "")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("invalid real literal: %s", s)}
	}
	return Value{Kind: ValReal, Real: f, IECType: types.KindLREAL}, nil
}

func (interp *Interpreter) parseLitBool(s string) (Value, error) {
	b := strings.EqualFold(s, "TRUE")
	return Value{Kind: ValBool, Bool: b, IECType: types.KindBOOL}, nil
}

func (interp *Interpreter) parseLitString(s string) (Value, error) {
	// Strip surrounding quotes if present
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			s = s[1 : len(s)-1]
		}
	}
	return Value{Kind: ValString, Str: s, IECType: types.KindSTRING}, nil
}

// timePartRegex matches time components like "1h", "30m", "5s", "100ms", "500d"
var timePartRegex = regexp.MustCompile(`(\d+(?:\.\d+)?)(d|h|m(?:s)?|s)`)

func (interp *Interpreter) parseLitTime(s string) (Value, error) {
	// Strip T# or t# prefix
	upper := strings.ToUpper(s)
	if strings.HasPrefix(upper, "T#") {
		s = s[2:]
	} else if strings.HasPrefix(upper, "TIME#") {
		s = s[5:]
	}

	// Remove underscores
	s = strings.ReplaceAll(s, "_", "")

	var total time.Duration
	matches := timePartRegex.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("invalid time literal: %s", s)}
	}

	for _, m := range matches {
		numStr := m[1]
		unit := m[2]
		num, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return Value{}, &RuntimeError{Msg: fmt.Sprintf("invalid time component: %s", m[0])}
		}
		switch unit {
		case "d":
			total += time.Duration(num * float64(24*time.Hour))
		case "h":
			total += time.Duration(num * float64(time.Hour))
		case "ms":
			total += time.Duration(num * float64(time.Millisecond))
		case "m":
			total += time.Duration(num * float64(time.Minute))
		case "s":
			total += time.Duration(num * float64(time.Second))
		}
	}
	return Value{Kind: ValTime, Time: total, IECType: types.KindTIME}, nil
}

func (interp *Interpreter) parseLitTyped(value string, prefix string) (Value, error) {
	// Typed literals like INT#5 or UINT#10
	// The prefix is the type, the value may contain base#digits
	upper := strings.ToUpper(prefix)

	// Try to parse as int first
	switch upper {
	case "INT", "SINT", "DINT", "LINT", "USINT", "UINT", "UDINT", "ULINT",
		"BYTE", "WORD", "DWORD", "LWORD":
		v, err := interp.parseLitInt(value)
		if err != nil {
			return Value{}, err
		}
		// Override IECType based on prefix
		if t, ok := types.LookupElementaryType(upper); ok {
			v.IECType = t.Kind()
		}
		return v, nil
	case "REAL", "LREAL":
		v, err := interp.parseLitReal(value)
		if err != nil {
			return Value{}, err
		}
		if t, ok := types.LookupElementaryType(upper); ok {
			v.IECType = t.Kind()
		}
		return v, nil
	case "BOOL":
		return interp.parseLitBool(value)
	default:
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("unsupported typed literal prefix: %s", prefix)}
	}
}

// evalIdent resolves an identifier in the environment.
func (interp *Interpreter) evalIdent(env *Env, id *ast.Ident) (Value, error) {
	v, ok := env.Get(id.Name)
	if !ok {
		return Value{}, &RuntimeError{
			Msg: fmt.Sprintf("undefined variable: %s", id.Name),
			Pos: id.Span().Start,
		}
	}
	return v, nil
}

// evalBinary evaluates a binary expression.
func (interp *Interpreter) evalBinary(env *Env, e *ast.BinaryExpr) (Value, error) {
	left, err := interp.evalExpr(env, e.Left)
	if err != nil {
		return Value{}, err
	}
	right, err := interp.evalExpr(env, e.Right)
	if err != nil {
		return Value{}, err
	}

	op := strings.ToUpper(e.Op.Text)

	// Boolean operators
	switch op {
	case "AND":
		return BoolValue(left.IsTruthy() && right.IsTruthy()), nil
	case "OR":
		return BoolValue(left.IsTruthy() || right.IsTruthy()), nil
	case "XOR":
		return BoolValue(left.IsTruthy() != right.IsTruthy()), nil
	}

	// String concatenation via +
	if left.Kind == ValString && right.Kind == ValString {
		switch op {
		case "+":
			return StringValue(left.Str + right.Str), nil
		case "=":
			return BoolValue(left.Str == right.Str), nil
		case "<>":
			return BoolValue(left.Str != right.Str), nil
		case "<":
			return BoolValue(left.Str < right.Str), nil
		case ">":
			return BoolValue(left.Str > right.Str), nil
		case "<=":
			return BoolValue(left.Str <= right.Str), nil
		case ">=":
			return BoolValue(left.Str >= right.Str), nil
		}
	}

	// Bool equality
	if left.Kind == ValBool && right.Kind == ValBool {
		switch op {
		case "=":
			return BoolValue(left.Bool == right.Bool), nil
		case "<>":
			return BoolValue(left.Bool != right.Bool), nil
		}
	}

	// Time arithmetic
	if left.Kind == ValTime && right.Kind == ValTime {
		switch op {
		case "+":
			return TimeValue(left.Time + right.Time), nil
		case "-":
			return TimeValue(left.Time - right.Time), nil
		case "=":
			return BoolValue(left.Time == right.Time), nil
		case "<>":
			return BoolValue(left.Time != right.Time), nil
		case "<":
			return BoolValue(left.Time < right.Time), nil
		case ">":
			return BoolValue(left.Time > right.Time), nil
		case "<=":
			return BoolValue(left.Time <= right.Time), nil
		case ">=":
			return BoolValue(left.Time >= right.Time), nil
		}
	}

	// Power always returns real
	if op == "**" {
		lf := toFloat(left)
		rf := toFloat(right)
		return RealValue(math.Pow(lf, rf)), nil
	}

	// Numeric: promote to real if either operand is real
	if left.Kind == ValReal || right.Kind == ValReal {
		lf := toFloat(left)
		rf := toFloat(right)
		return interp.evalBinaryReal(lf, op, rf)
	}

	// Both are int
	if left.Kind == ValInt && right.Kind == ValInt {
		return interp.evalBinaryInt(left.Int, op, right.Int)
	}

	return Value{}, &RuntimeError{
		Msg: fmt.Sprintf("unsupported binary operation: %s %s %s", left.Kind, op, right.Kind),
	}
}

func (interp *Interpreter) evalBinaryInt(l int64, op string, r int64) (Value, error) {
	switch op {
	case "+":
		return IntValue(l + r), nil
	case "-":
		return IntValue(l - r), nil
	case "*":
		return IntValue(l * r), nil
	case "/":
		if r == 0 {
			return Value{}, &RuntimeError{Msg: "division by zero"}
		}
		return IntValue(l / r), nil
	case "MOD":
		if r == 0 {
			return Value{}, &RuntimeError{Msg: "division by zero"}
		}
		return IntValue(l % r), nil
	case "=":
		return BoolValue(l == r), nil
	case "<>":
		return BoolValue(l != r), nil
	case "<":
		return BoolValue(l < r), nil
	case ">":
		return BoolValue(l > r), nil
	case "<=":
		return BoolValue(l <= r), nil
	case ">=":
		return BoolValue(l >= r), nil
	default:
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("unsupported int operator: %s", op)}
	}
}

func (interp *Interpreter) evalBinaryReal(l float64, op string, r float64) (Value, error) {
	switch op {
	case "+":
		return RealValue(l + r), nil
	case "-":
		return RealValue(l - r), nil
	case "*":
		return RealValue(l * r), nil
	case "/":
		if r == 0 {
			return Value{}, &RuntimeError{Msg: "division by zero"}
		}
		return RealValue(l / r), nil
	case "=":
		return BoolValue(l == r), nil
	case "<>":
		return BoolValue(l != r), nil
	case "<":
		return BoolValue(l < r), nil
	case ">":
		return BoolValue(l > r), nil
	case "<=":
		return BoolValue(l <= r), nil
	case ">=":
		return BoolValue(l >= r), nil
	default:
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("unsupported real operator: %s", op)}
	}
}

// evalUnary evaluates a unary expression.
func (interp *Interpreter) evalUnary(env *Env, e *ast.UnaryExpr) (Value, error) {
	operand, err := interp.evalExpr(env, e.Operand)
	if err != nil {
		return Value{}, err
	}

	op := strings.ToUpper(e.Op.Text)
	switch op {
	case "NOT":
		return BoolValue(!operand.IsTruthy()), nil
	case "-":
		switch operand.Kind {
		case ValInt:
			return IntValue(-operand.Int), nil
		case ValReal:
			return RealValue(-operand.Real), nil
		default:
			return Value{}, &RuntimeError{Msg: fmt.Sprintf("cannot negate %s", operand.Kind)}
		}
	case "+":
		return operand, nil
	default:
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("unsupported unary operator: %s", op)}
	}
}

// evalIndex evaluates an array index expression.
func (interp *Interpreter) evalIndex(env *Env, e *ast.IndexExpr) (Value, error) {
	obj, err := interp.evalExpr(env, e.Object)
	if err != nil {
		return Value{}, err
	}
	if obj.Kind != ValArray {
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("cannot index %s", obj.Kind)}
	}
	if len(e.Indices) == 0 {
		return Value{}, &RuntimeError{Msg: "missing array index"}
	}

	idx, err := interp.evalExpr(env, e.Indices[0])
	if err != nil {
		return Value{}, err
	}
	if idx.Kind != ValInt {
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("array index must be integer, got %s", idx.Kind)}
	}
	i := int(idx.Int)
	if i < 0 || i >= len(obj.Array) {
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("array index out of bounds: %d (length %d)", i, len(obj.Array))}
	}
	return obj.Array[i], nil
}

// --- Statement execution ---

// ExecStatements is the exported wrapper around execStatements for use by
// the test runner and other external packages that need to execute statement lists.
func (interp *Interpreter) ExecStatements(env *Env, stmts []ast.Statement) error {
	return interp.execStatements(env, stmts)
}

// execStatements executes a list of statements sequentially.
func (interp *Interpreter) execStatements(env *Env, stmts []ast.Statement) error {
	for _, stmt := range stmts {
		if err := interp.execStmt(env, stmt); err != nil {
			return err
		}
	}
	return nil
}

// execStmt dispatches a single statement for execution.
func (interp *Interpreter) execStmt(env *Env, stmt ast.Statement) error {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		return interp.execAssign(env, s)
	case *ast.IfStmt:
		return interp.execIf(env, s)
	case *ast.CaseStmt:
		return interp.execCase(env, s)
	case *ast.ForStmt:
		return interp.execFor(env, s)
	case *ast.WhileStmt:
		return interp.execWhile(env, s)
	case *ast.RepeatStmt:
		return interp.execRepeat(env, s)
	case *ast.ReturnStmt:
		return &ErrReturn{}
	case *ast.ExitStmt:
		return &ErrExit{}
	case *ast.ContinueStmt:
		return &ErrContinue{}
	case *ast.EmptyStmt:
		return nil
	case *ast.CallStmt:
		return interp.execCallStmt(env, s)
	case *ast.ErrorNode:
		return &RuntimeError{Msg: fmt.Sprintf("cannot execute error node: %s", s.Message)}
	default:
		return &RuntimeError{Msg: fmt.Sprintf("unsupported statement type: %T", stmt)}
	}
}

// execAssign executes an assignment statement.
// If Value is nil, this is an expression statement (e.g., a function call
// used as a statement). In that case, just evaluate Target for side effects.
func (interp *Interpreter) execAssign(env *Env, s *ast.AssignStmt) error {
	if s.Value == nil {
		// Expression statement: evaluate target for side effects (e.g., assertion calls)
		_, err := interp.evalExpr(env, s.Target)
		return err
	}

	val, err := interp.evalExpr(env, s.Value)
	if err != nil {
		return err
	}

	switch target := s.Target.(type) {
	case *ast.Ident:
		if !env.Set(target.Name, val) {
			// If variable doesn't exist, define it (for compatibility)
			env.Define(target.Name, val)
		}
		return nil
	case *ast.IndexExpr:
		return interp.execAssignIndex(env, target, val)
	case *ast.MemberAccessExpr:
		return interp.execAssignMember(env, target, val)
	default:
		return &RuntimeError{Msg: fmt.Sprintf("unsupported assignment target: %T", s.Target)}
	}
}

// execAssignIndex handles assignment to array elements: arr[i] := val
func (interp *Interpreter) execAssignIndex(env *Env, target *ast.IndexExpr, val Value) error {
	// Get the array variable name
	id, ok := target.Object.(*ast.Ident)
	if !ok {
		return &RuntimeError{Msg: "array assignment requires identifier as base"}
	}

	arr, found := env.Get(id.Name)
	if !found {
		return &RuntimeError{Msg: fmt.Sprintf("undefined variable: %s", id.Name)}
	}
	if arr.Kind != ValArray {
		return &RuntimeError{Msg: fmt.Sprintf("cannot index %s", arr.Kind)}
	}
	if len(target.Indices) == 0 {
		return &RuntimeError{Msg: "missing array index"}
	}

	idx, err := interp.evalExpr(env, target.Indices[0])
	if err != nil {
		return err
	}
	i := int(idx.Int)
	if i < 0 || i >= len(arr.Array) {
		return &RuntimeError{Msg: fmt.Sprintf("array index out of bounds: %d", i)}
	}

	arr.Array[i] = val
	env.Set(id.Name, arr)
	return nil
}

// execIf executes an IF/ELSIF/ELSE statement.
func (interp *Interpreter) execIf(env *Env, s *ast.IfStmt) error {
	cond, err := interp.evalExpr(env, s.Condition)
	if err != nil {
		return err
	}

	if cond.IsTruthy() {
		return interp.execStatements(env, s.Then)
	}

	// Check ELSIF branches
	for _, elsif := range s.ElsIfs {
		c, err := interp.evalExpr(env, elsif.Condition)
		if err != nil {
			return err
		}
		if c.IsTruthy() {
			return interp.execStatements(env, elsif.Body)
		}
	}

	// ELSE branch
	if len(s.Else) > 0 {
		return interp.execStatements(env, s.Else)
	}
	return nil
}

// execCase executes a CASE statement.
func (interp *Interpreter) execCase(env *Env, s *ast.CaseStmt) error {
	selector, err := interp.evalExpr(env, s.Expr)
	if err != nil {
		return err
	}

	for _, branch := range s.Branches {
		for _, label := range branch.Labels {
			match, err := interp.matchCaseLabel(env, selector, label)
			if err != nil {
				return err
			}
			if match {
				return interp.execStatements(env, branch.Body)
			}
		}
	}

	// ELSE branch
	if len(s.ElseBranch) > 0 {
		return interp.execStatements(env, s.ElseBranch)
	}
	return nil
}

// matchCaseLabel checks whether a selector value matches a case label.
func (interp *Interpreter) matchCaseLabel(env *Env, selector Value, label ast.CaseLabel) (bool, error) {
	switch l := label.(type) {
	case *ast.CaseLabelValue:
		val, err := interp.evalExpr(env, l.Value)
		if err != nil {
			return false, err
		}
		return valuesEqual(selector, val), nil
	case *ast.CaseLabelRange:
		low, err := interp.evalExpr(env, l.Low)
		if err != nil {
			return false, err
		}
		high, err := interp.evalExpr(env, l.High)
		if err != nil {
			return false, err
		}
		return valuesInRange(selector, low, high), nil
	default:
		return false, &RuntimeError{Msg: fmt.Sprintf("unsupported case label type: %T", label)}
	}
}

// execFor executes a FOR loop.
func (interp *Interpreter) execFor(env *Env, s *ast.ForStmt) error {
	from, err := interp.evalExpr(env, s.From)
	if err != nil {
		return err
	}
	to, err := interp.evalExpr(env, s.To)
	if err != nil {
		return err
	}

	by := IntValue(1)
	if s.By != nil {
		by, err = interp.evalExpr(env, s.By)
		if err != nil {
			return err
		}
	}

	varName := s.Variable.Name

	// Define or set the loop variable
	if !env.Set(varName, from) {
		env.Define(varName, from)
	}

	step := by.Int
	if step == 0 {
		return &RuntimeError{Msg: "FOR loop step cannot be zero"}
	}

	for i := 0; i < interp.MaxLoopIterations; i++ {
		current, _ := env.Get(varName)

		// Check termination condition
		if step > 0 && current.Int > to.Int {
			break
		}
		if step < 0 && current.Int < to.Int {
			break
		}

		// Execute body
		err := interp.execStatements(env, s.Body)
		if err != nil {
			if _, ok := err.(*ErrExit); ok {
				break
			}
			if _, ok := err.(*ErrContinue); ok {
				// Continue to next iteration
			} else {
				return err
			}
		}

		// Increment loop variable
		current, _ = env.Get(varName)
		env.Set(varName, IntValue(current.Int+step))
	}
	return nil
}

// execWhile executes a WHILE loop.
func (interp *Interpreter) execWhile(env *Env, s *ast.WhileStmt) error {
	for i := 0; i < interp.MaxLoopIterations; i++ {
		cond, err := interp.evalExpr(env, s.Condition)
		if err != nil {
			return err
		}
		if !cond.IsTruthy() {
			break
		}

		err = interp.execStatements(env, s.Body)
		if err != nil {
			if _, ok := err.(*ErrExit); ok {
				break
			}
			if _, ok := err.(*ErrContinue); ok {
				continue
			}
			return err
		}
	}
	return nil
}

// execRepeat executes a REPEAT...UNTIL loop.
func (interp *Interpreter) execRepeat(env *Env, s *ast.RepeatStmt) error {
	for i := 0; i < interp.MaxLoopIterations; i++ {
		err := interp.execStatements(env, s.Body)
		if err != nil {
			if _, ok := err.(*ErrExit); ok {
				break
			}
			if _, ok := err.(*ErrContinue); ok {
				// Fall through to check condition
			} else {
				return err
			}
		}

		cond, err := interp.evalExpr(env, s.Condition)
		if err != nil {
			return err
		}
		if cond.IsTruthy() {
			break
		}
	}
	return nil
}

// --- Utility functions ---

// toFloat converts a Value to float64 for mixed arithmetic.
func toFloat(v Value) float64 {
	switch v.Kind {
	case ValInt:
		return float64(v.Int)
	case ValReal:
		return v.Real
	case ValBool:
		if v.Bool {
			return 1.0
		}
		return 0.0
	default:
		return 0.0
	}
}

// valuesEqual compares two Values for equality.
func valuesEqual(a, b Value) bool {
	if a.Kind != b.Kind {
		// Allow int/real comparison
		if (a.Kind == ValInt || a.Kind == ValReal) && (b.Kind == ValInt || b.Kind == ValReal) {
			return toFloat(a) == toFloat(b)
		}
		return false
	}
	switch a.Kind {
	case ValBool:
		return a.Bool == b.Bool
	case ValInt:
		return a.Int == b.Int
	case ValReal:
		return a.Real == b.Real
	case ValString:
		return a.Str == b.Str
	case ValTime:
		return a.Time == b.Time
	default:
		return false
	}
}

// valuesInRange checks whether low <= v <= high for numeric values.
func valuesInRange(v, low, high Value) bool {
	vf := toFloat(v)
	lf := toFloat(low)
	hf := toFloat(high)
	return vf >= lf && vf <= hf
}

// --- FB call and member access ---

// execCallStmt handles function block call statements: fbInst(IN := val, ...)
// It resolves the callee in the environment, sets inputs from named args,
// executes the FB, and copies output args back to the env.
func (interp *Interpreter) execCallStmt(env *Env, s *ast.CallStmt) error {
	// Resolve callee
	calleeIdent, ok := s.Callee.(*ast.Ident)
	if !ok {
		return &RuntimeError{Msg: fmt.Sprintf("unsupported call target: %T", s.Callee)}
	}

	v, found := env.Get(calleeIdent.Name)
	if !found {
		return &RuntimeError{Msg: fmt.Sprintf("undefined: %s", calleeIdent.Name)}
	}
	if v.Kind != ValFBInstance || v.FBRef == nil {
		return &RuntimeError{Msg: fmt.Sprintf("%s is not a function block instance", calleeIdent.Name)}
	}

	fbInst := v.FBRef

	// Set input args
	for _, arg := range s.Args {
		if arg.IsOutput {
			// Output binding (=>) — skip during input phase
			continue
		}
		if arg.Name == nil {
			continue
		}
		argVal, err := interp.evalExpr(env, arg.Value)
		if err != nil {
			return err
		}
		fbInst.SetInput(arg.Name.Name, argVal)
	}

	// Execute the FB
	fbInst.Execute(interp.dt, interp)

	// Copy output args back (=> bindings)
	for _, arg := range s.Args {
		if !arg.IsOutput {
			continue
		}
		if arg.Name == nil || arg.Value == nil {
			continue
		}
		outVal := fbInst.GetOutput(arg.Name.Name)
		// The value expression should be an identifier to assign to
		if targetIdent, ok := arg.Value.(*ast.Ident); ok {
			if !env.Set(targetIdent.Name, outVal) {
				env.Define(targetIdent.Name, outVal)
			}
		}
	}

	return nil
}

// evalMemberAccess evaluates obj.member where obj may be an FB instance or struct.
func (interp *Interpreter) evalMemberAccess(env *Env, e *ast.MemberAccessExpr) (Value, error) {
	obj, err := interp.evalExpr(env, e.Object)
	if err != nil {
		return Value{}, err
	}

	memberName := e.Member.Name

	switch obj.Kind {
	case ValFBInstance:
		if obj.FBRef == nil {
			return Value{}, &RuntimeError{Msg: "nil FB instance reference"}
		}
		fbInst := obj.FBRef
		return fbInst.GetMember(memberName), nil
	case ValStruct:
		if obj.Struct != nil {
			key := strings.ToUpper(memberName)
			if v, ok := obj.Struct[key]; ok {
				return v, nil
			}
		}
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("struct has no member '%s'", memberName)}
	default:
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("cannot access member '%s' on %s", memberName, obj.Kind)}
	}
}

// execAssignMember handles assignment to a member: obj.member := val
func (interp *Interpreter) execAssignMember(env *Env, target *ast.MemberAccessExpr, val Value) error {
	obj, err := interp.evalExpr(env, target.Object)
	if err != nil {
		return err
	}

	memberName := target.Member.Name

	switch obj.Kind {
	case ValFBInstance:
		if obj.FBRef == nil {
			return &RuntimeError{Msg: "nil FB instance reference"}
		}
		fbInst := obj.FBRef
		fbInst.SetInput(memberName, val)
		return nil
	case ValStruct:
		if obj.Struct != nil {
			key := strings.ToUpper(memberName)
			obj.Struct[key] = val
			// Write back the struct to the env
			if objIdent, ok := target.Object.(*ast.Ident); ok {
				env.Set(objIdent.Name, obj)
			}
			return nil
		}
		return &RuntimeError{Msg: fmt.Sprintf("struct has no member '%s'", memberName)}
	default:
		return &RuntimeError{Msg: fmt.Sprintf("cannot assign member '%s' on %s", memberName, obj.Kind)}
	}
}

// evalCall evaluates a function call expression.
// It dispatches to StdlibFunctions first, then falls back to user-defined functions.
func (interp *Interpreter) evalCall(env *Env, e *ast.CallExpr) (Value, error) {
	// Handle method calls: fb.Method()
	if memberAccess, ok := e.Callee.(*ast.MemberAccessExpr); ok {
		return interp.evalMethodCall(env, memberAccess, e.Args)
	}

	// Resolve callee name
	calleeName := ""
	switch c := e.Callee.(type) {
	case *ast.Ident:
		calleeName = strings.ToUpper(c.Name)
	default:
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("unsupported call target: %T", e.Callee)}
	}

	// Check LocalFunctions first (per-instance overrides for test assertions, etc.)
	if interp.LocalFunctions != nil {
		if fn, ok := interp.LocalFunctions[calleeName]; ok {
			args := make([]Value, 0, len(e.Args))
			for _, argExpr := range e.Args {
				v, err := interp.evalExpr(env, argExpr)
				if err != nil {
					return Value{}, err
				}
				args = append(args, v)
			}
			return fn(args, e.Span().Start)
		}
	}

	// Check StdlibFunctions (math, string, conversion)
	if fn, ok := StdlibFunctions[calleeName]; ok {
		args := make([]Value, 0, len(e.Args))
		for _, argExpr := range e.Args {
			v, err := interp.evalExpr(env, argExpr)
			if err != nil {
				return Value{}, err
			}
			args = append(args, v)
		}
		return fn(args)
	}

	return Value{}, &RuntimeError{Msg: fmt.Sprintf("undefined function: %s", calleeName)}
}

// evalMethodCall evaluates a method call on an object (e.g., fb.GetValue()).
// It resolves the object, finds the method declaration, and executes it.
func (interp *Interpreter) evalMethodCall(env *Env, memberAccess *ast.MemberAccessExpr, argExprs []ast.Expr) (Value, error) {
	obj, err := interp.evalExpr(env, memberAccess.Object)
	if err != nil {
		return Value{}, err
	}

	methodName := memberAccess.Member.Name

	if obj.Kind != ValFBInstance || obj.FBRef == nil {
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("cannot call method '%s' on %s", methodName, obj.Kind)}
	}

	fbInst := obj.FBRef

	// Find the method in the FB declaration (including inherited methods)
	method := findMethod(fbInst, methodName)
	if method == nil {
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("method '%s' not found on FB '%s'", methodName, fbInst.TypeName)}
	}

	// Create method environment with access to FB instance variables
	methodEnv := NewEnv(fbInst.Env)

	// Define return variable (method name holds the return value)
	retVal := ZeroFromTypeSpec(method.ReturnType)
	methodEnv.Define(method.Name.Name, retVal)

	// Map arguments to VAR_INPUT parameters
	args := make([]Value, 0, len(argExprs))
	for _, argExpr := range argExprs {
		v, err := interp.evalExpr(env, argExpr)
		if err != nil {
			return Value{}, err
		}
		args = append(args, v)
	}

	argIdx := 0
	for _, vb := range method.VarBlocks {
		if vb.Section == ast.VarInput {
			for _, vd := range vb.Declarations {
				for _, n := range vd.Names {
					if argIdx < len(args) {
						methodEnv.Define(n.Name, args[argIdx])
						argIdx++
					} else {
						methodEnv.Define(n.Name, ZeroFromTypeSpec(vd.Type))
					}
				}
			}
		} else {
			for _, vd := range vb.Declarations {
				val := ZeroFromTypeSpec(vd.Type)
				if vd.InitValue != nil {
					if iv, err := interp.evalExpr(methodEnv, vd.InitValue); err == nil {
						val = iv
					}
				}
				for _, n := range vd.Names {
					methodEnv.Define(n.Name, val)
				}
			}
		}
	}

	// Execute method body
	err = interp.execStatements(methodEnv, method.Body)
	if err != nil {
		if _, ok := err.(*ErrReturn); !ok {
			return Value{}, err
		}
	}

	// Read return value
	if method.Name != nil {
		if v, ok := methodEnv.Get(method.Name.Name); ok {
			return v, nil
		}
	}

	return retVal, nil
}

// findMethod looks up a method by name in the FB declaration hierarchy.
// It checks the FB's own methods first, then walks up the EXTENDS chain
// if parent declarations are available.
func findMethod(inst *FBInstance, name string) *ast.MethodDecl {
	if inst.Decl == nil {
		return nil
	}
	upperName := strings.ToUpper(name)

	// Search in the FB's own methods
	for _, m := range inst.Decl.Methods {
		if m.Name != nil && strings.ToUpper(m.Name.Name) == upperName {
			return m
		}
	}

	// If the FB extends another, search parent's methods via ParentDecl
	if inst.Decl.Extends != nil && inst.ParentDecl != nil {
		parentInst := &FBInstance{
			TypeName:   inst.Decl.Extends.Name,
			Decl:       inst.ParentDecl,
			Env:        inst.Env,
			ParentDecl: inst.ParentDecl, // propagate further up the chain
		}
		return findMethod(parentInst, name)
	}

	return nil
}
