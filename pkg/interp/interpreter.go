package interp

import (
	"fmt"
	"math"
	"regexp"
	"slices"
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
	// clock is virtual time, advanced by ADVANCE_TIME. FB instances use it
	// to work out how long since they themselves last ran.
	clock time.Duration

	// LocalFunctions provides per-interpreter-instance function overrides.
	// These take priority over global StdlibFunctions during evalCall.
	// Used for test-specific functions (assertions, ADVANCE_TIME) to avoid
	// global state mutation between test cases.
	LocalFunctions map[string]func(args []Value, pos ast.Pos) (Value, error)

	// Collector gathers assertion results during test execution.
	// Each test case gets its own collector via RegisterAssertions.
	Collector *AssertionCollector

	// EnumTypes maps uppercase type names to their enum value maps.
	// Each enum value map is uppercase enum member name -> integer value.
	EnumTypes map[string]map[string]int64

	// TypeDecls maps uppercase user-defined type names to their TypeSpec.
	// Zero-value construction consults this so that a named STRUCT or ARRAY
	// used as an array element or struct member resolves to the right shape
	// instead of falling back to an INT zero.
	TypeDecls map[string]ast.TypeSpec

	// FBDecls maps uppercase user-defined function block names to their
	// declarations. FB instantiation consults this so that an FB-typed VAR
	// inside another FB is created as a live nested instance.
	FBDecls map[string]*ast.FunctionBlockDecl

	// gvls holds the registered global variable lists; see RegisterGVL.
	gvls gvlState

	// callDepth counts nested ACTION, METHOD, user FUNCTION and user FB
	// executions; see EnterCall.
	callDepth int
}

// MaxCallDepth bounds nested ACTION, METHOD, FUNCTION and FB calls. A
// self-recursive ACTION would otherwise grow the Go stack until the process
// dies; past this depth the call fails with a RuntimeError instead.
const MaxCallDepth = 256

// EnterCall records entry into a nested ACTION, METHOD, FUNCTION or FB body and
// fails with a RuntimeError once MaxCallDepth is exceeded. Every successful
// EnterCall must be paired with ExitCall.
func (interp *Interpreter) EnterCall(name string, pos ast.Pos) error {
	if interp.callDepth >= MaxCallDepth {
		return &RuntimeError{
			Msg: fmt.Sprintf("maximum call depth %d exceeded calling %s", MaxCallDepth, name),
			Pos: pos,
		}
	}
	interp.callDepth++
	return nil
}

// ExitCall undoes one EnterCall.
func (interp *Interpreter) ExitCall() {
	interp.callDepth--
}

// TypeResolverFunc returns a TypeResolver backed by the interpreter's
// TypeDecls, or nil when no user types are registered.
func (interp *Interpreter) TypeResolverFunc() TypeResolver {
	if interp == nil || len(interp.TypeDecls) == 0 {
		return nil
	}
	return func(upperName string) (ast.TypeSpec, bool) {
		ts, ok := interp.TypeDecls[upperName]
		return ts, ok
	}
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
	interp.clock += dt
}

// Clock returns the interpreter's virtual time, advanced by ADVANCE_TIME.
func (interp *Interpreter) Clock() time.Duration {
	return interp.clock
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
		return interp.evalDeref(env, e)
	case *ast.BitAccessExpr:
		return interp.evalBitAccess(env, e)
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

	// Remove underscores. Units are case-insensitive in IEC 61131-3
	// (T#1D == T#1d), so normalize before matching.
	s = strings.ToLower(strings.ReplaceAll(s, "_", ""))

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
		// Check if this is an enum typed literal (e.g., Color#Green)
		if interp.EnumTypes != nil {
			if enumMap, ok := interp.EnumTypes[upper]; ok {
				upperVal := strings.ToUpper(value)
				if intVal, found := enumMap[upperVal]; found {
					return Value{Kind: ValInt, Int: intVal, IECType: types.KindDINT}, nil
				}
				return Value{}, &RuntimeError{Msg: fmt.Sprintf("unknown enum value '%s' for type '%s'", value, prefix)}
			}
		}
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("unsupported typed literal prefix: %s", prefix)}
	}
}

// evalIdent resolves an identifier in the environment.
// If the value is a REFERENCE TO, it auto-dereferences to the target.
// Identifiers not found in any scope fall back to bare enum member names
// (e.g. `state := RUNNING` for TYPE E_State : (IDLE, RUNNING);).
func (interp *Interpreter) evalIdent(env *Env, id *ast.Ident) (Value, error) {
	v, ok := env.Get(id.Name)
	if !ok {
		if interp.EnumTypes != nil {
			upper := strings.ToUpper(id.Name)
			for _, enumMap := range interp.EnumTypes {
				if intVal, found := enumMap[upper]; found {
					return Value{Kind: ValInt, Int: intVal, IECType: types.KindDINT}, nil
				}
			}
		}
		return Value{}, &RuntimeError{
			Msg: fmt.Sprintf("undefined variable: %s", id.Name),
			Pos: id.Span().Start,
		}
	}
	// Auto-dereference REFERENCE TO values
	if v.Kind == ValReference && v.PtrEnv != nil && v.PtrVar != "" {
		target, found := v.PtrEnv.Get(v.PtrVar)
		if !found {
			return Value{}, &RuntimeError{
				Msg: fmt.Sprintf("dangling reference: variable '%s' no longer exists", v.PtrVar),
				Pos: id.Span().Start,
			}
		}
		return target, nil
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

	// ARRAY and STRUCT are value types in IEC 61131-3: assigning one to another
	// copies the contents, so later writes to the source must not show up
	// through the destination. They are backed by Go slices and maps, so the
	// copy has to be explicit.
	if val.IsAggregate() {
		val = val.Clone()
	}

	return interp.assignToTarget(env, s.Target, val)
}

// assignToTarget stores val into an assignable expression: an identifier, an
// array element, a struct member or a pointer dereference. Shared by ordinary
// assignment and by VAR_IN_OUT write-back once a function block call returns.
// Callers own the value semantics: execAssign clones aggregates before calling.
func (interp *Interpreter) assignToTarget(env *Env, targetExpr ast.Expr, val Value) error {
	switch target := targetExpr.(type) {
	case *ast.Ident:
		// Check if this variable is a REFERENCE TO — if so, write through
		if existing, ok := env.Get(target.Name); ok && existing.Kind == ValReference && existing.PtrEnv != nil && existing.PtrVar != "" {
			// Check if the RHS is also a reference (REF assignment)
			if val.Kind == ValReference {
				// Assigning a new reference target
				env.Set(target.Name, val)
			} else {
				// Write through the reference to the target variable
				existing.PtrEnv.Set(existing.PtrVar, val)
			}
			return nil
		}
		// Check subrange constraints
		if msg := env.CheckSubrange(target.Name, val); msg != "" {
			return &RuntimeError{Msg: msg, Pos: target.Span().Start}
		}
		if !env.Set(target.Name, val) {
			// If variable doesn't exist, define it (for compatibility)
			env.Define(target.Name, val)
		}
		return nil
	case *ast.IndexExpr:
		return interp.execAssignIndex(env, target, val)
	case *ast.MemberAccessExpr:
		return interp.execAssignMember(env, target, val)
	case *ast.DerefExpr:
		return interp.execAssignDeref(env, target, val)
	case *ast.BitAccessExpr:
		return interp.assignBit(env, target, val)
	default:
		return &RuntimeError{Msg: fmt.Sprintf("unsupported assignment target: %T", targetExpr)}
	}
}

// isAssignable reports whether an expression can serve as an assignment target.
// Used to decide whether a VAR_IN_OUT argument has somewhere to be written back
// to: a literal or a computed expression has not, and is skipped silently.
func isAssignable(e ast.Expr) bool {
	switch e.(type) {
	case *ast.Ident, *ast.IndexExpr, *ast.MemberAccessExpr, *ast.DerefExpr, *ast.BitAccessExpr:
		return true
	default:
		return false
	}
}

// execAssignIndex handles assignment to array elements: arr[i] := val.
//
// The base may be any expression that evaluates to an array -- a plain
// identifier, a struct member (s.slots[i]), or a nested index. An array Value
// carries a Go slice, so mutating the element writes through to the same
// backing store the base was read from; no write-back is required.
func (interp *Interpreter) execAssignIndex(env *Env, target *ast.IndexExpr, val Value) error {
	arr, err := interp.evalExpr(env, target.Object)
	if err != nil {
		return err
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

	// ARRAY and STRUCT are value types: storing one copies it.
	if val.IsAggregate() {
		val = val.Clone()
	}
	arr.Array[i] = val
	if id, ok := target.Object.(*ast.Ident); ok {
		env.Set(id.Name, arr)
	}
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
	// Resolve callee: a plain instance name, or a member path such as
	// GVL.timer or s.fb that evaluates to an FB instance.
	var v Value
	var calleeName string
	switch c := s.Callee.(type) {
	case *ast.Ident:
		calleeName = c.Name
		var found bool
		v, found = env.Get(c.Name)
		if !found {
			return &RuntimeError{Msg: fmt.Sprintf("undefined: %s", c.Name)}
		}
	case *ast.MemberAccessExpr:
		calleeName = c.Member.Name
		var err error
		v, err = interp.evalMemberAccess(env, c)
		if err != nil {
			return err
		}
	default:
		return &RuntimeError{Msg: fmt.Sprintf("unsupported call target: %T", s.Callee)}
	}
	if v.Kind != ValFBInstance || v.FBRef == nil {
		return &RuntimeError{Msg: fmt.Sprintf("%s is not a function block instance", calleeName)}
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
		if arg.Value == nil {
			// Empty argument (name := ,): treated as omitted.
			continue
		}
		argVal, err := interp.evalExpr(env, arg.Value)
		if err != nil {
			return err
		}
		fbInst.SetInput(arg.Name.Name, argVal)
	}

	// Execute the FB, with the time elapsed since this instance last ran.
	if err := fbInst.Execute(fbInst.deltaFor(interp.clock, interp.dt), interp); err != nil {
		return err
	}

	// Copy VAR_IN_OUT args back into the caller's variable. IEC 61131-3 passes
	// VAR_IN_OUT by reference, so an assignment inside the FB body has to be
	// visible to the caller once the call returns. The value is copied back
	// rather than aliased: arrays and structs have value semantics elsewhere,
	// so leaving the caller sharing the FB env's backing slice or map would
	// make every later write inside the FB leak out mid-cycle.
	for _, arg := range s.Args {
		if arg.IsOutput || arg.Name == nil || arg.Value == nil {
			continue
		}
		if !isAssignable(arg.Value) {
			// Literal or computed expression: nothing to write back to.
			continue
		}
		inoutVal, ok := fbInst.GetInOut(arg.Name.Name)
		if !ok {
			continue
		}
		if inoutVal.IsAggregate() {
			inoutVal = inoutVal.Clone()
		}
		if err := interp.assignToTarget(env, arg.Value, inoutVal); err != nil {
			return err
		}
	}

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
		switch target := arg.Value.(type) {
		case *ast.Ident:
			if !env.Set(target.Name, outVal) {
				env.Define(target.Name, outVal)
			}
		case *ast.MemberAccessExpr:
			// q => GVL.flag or q => s.member
			if err := interp.execAssignMember(env, target, outVal); err != nil {
				return err
			}
		}
	}

	return nil
}

// evalMemberAccess evaluates obj.member where obj may be an FB instance or struct.
func (interp *Interpreter) evalMemberAccess(env *Env, e *ast.MemberAccessExpr) (Value, error) {
	if g, gvl := interp.gvlRoot(env, e.Object); g != nil {
		return evalGVLMember(g, gvl, e.Member)
	}
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
		// Check for property getter
		if prop := findProperty(fbInst, memberName, interp); prop != nil && prop.Getter != nil {
			return interp.execPropertyGetter(fbInst, prop)
		}
		return fbInst.GetMember(memberName), nil
	case ValStruct:
		if obj.Struct != nil {
			key := strings.ToUpper(memberName)
			if v, ok := obj.Struct[key]; ok {
				return v, nil
			}
		}
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("struct has no member '%s'", memberName)}
	case ValInt:
		// w.cBit with a constant integer cBit is a bit read (ruling A3).
		if n, ok := constBitIndex(env, e); ok {
			return readBit(obj, n, e.Span().Start)
		}
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("cannot access member '%s' on %s", memberName, obj.Kind)}
	default:
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("cannot access member '%s' on %s", memberName, obj.Kind)}
	}
}

// execAssignMember handles assignment to a member: obj.member := val
func (interp *Interpreter) execAssignMember(env *Env, target *ast.MemberAccessExpr, val Value) error {
	if g, gvl := interp.gvlRoot(env, target.Object); g != nil {
		return assignGVLMember(g, gvl, target.Member, val)
	}
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
		// Check for property setter
		if prop := findProperty(fbInst, memberName, interp); prop != nil && prop.Setter != nil {
			return interp.execPropertySetter(fbInst, prop, val)
		}
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
	case ValInt:
		// w.cBit := v with a constant integer cBit is a bit write (ruling
		// A3). obj is the current value of target.Object, so the object
		// expression is evaluated only once.
		if n, ok := constBitIndex(env, target); ok {
			return interp.assignBitAt(env, target.Object, obj, n, val)
		}
		return &RuntimeError{Msg: fmt.Sprintf("cannot assign member '%s' on %s", memberName, obj.Kind)}
	default:
		return &RuntimeError{Msg: fmt.Sprintf("cannot assign member '%s' on %s", memberName, obj.Kind)}
	}
}

// execAssignDeref handles assignment through a pointer dereference: p^ := val
func (interp *Interpreter) execAssignDeref(env *Env, target *ast.DerefExpr, val Value) error {
	ptr, err := interp.evalExpr(env, target.Operand)
	if err != nil {
		return err
	}
	if ptr.Kind != ValPointer {
		return &RuntimeError{Msg: fmt.Sprintf("cannot dereference non-pointer value of type %s", ptr.Kind)}
	}
	if ptr.PtrEnv == nil || ptr.PtrVar == "" {
		return &RuntimeError{Msg: "nil pointer dereference"}
	}
	if !ptr.PtrEnv.Set(ptr.PtrVar, val) {
		return &RuntimeError{Msg: fmt.Sprintf("dangling pointer: variable '%s' no longer exists", ptr.PtrVar)}
	}
	return nil
}

// evalDeref evaluates a pointer dereference expression (p^).
// It resolves the pointer value and reads the target variable from the stored env.
func (interp *Interpreter) evalDeref(env *Env, e *ast.DerefExpr) (Value, error) {
	ptr, err := interp.evalExpr(env, e.Operand)
	if err != nil {
		return Value{}, err
	}
	if ptr.Kind != ValPointer {
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("cannot dereference non-pointer value of type %s", ptr.Kind)}
	}
	if ptr.PtrEnv == nil || ptr.PtrVar == "" {
		return Value{}, &RuntimeError{Msg: "nil pointer dereference"}
	}
	v, ok := ptr.PtrEnv.Get(ptr.PtrVar)
	if !ok {
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("dangling pointer: variable '%s' no longer exists", ptr.PtrVar)}
	}
	return v, nil
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

	// ACTION of the enclosing POU: A1(); runs against the owner's variables.
	if act, owner := env.LookupAction(calleeName); act != nil {
		if len(e.Args) > 0 {
			return Value{}, &RuntimeError{
				Msg: fmt.Sprintf("action %s takes no arguments", act.Name.Name),
				Pos: e.Span().Start,
			}
		}
		return Value{}, interp.execAction(owner, act, e.Span().Start)
	}

	// Handle ADR() specially: it needs the variable reference, not its value
	if calleeName == "ADR" {
		if len(e.Args) != 1 {
			return Value{}, &RuntimeError{Msg: "ADR requires exactly 1 argument"}
		}
		argIdent, ok := e.Args[0].(*ast.Ident)
		if !ok {
			return Value{}, &RuntimeError{Msg: "ADR argument must be a variable name"}
		}
		// Find the env that owns this variable
		targetEnv := env.FindOwner(argIdent.Name)
		if targetEnv == nil {
			return Value{}, &RuntimeError{Msg: fmt.Sprintf("ADR: undefined variable '%s'", argIdent.Name)}
		}
		return Value{
			Kind:   ValPointer,
			PtrEnv: targetEnv,
			PtrVar: strings.ToUpper(argIdent.Name),
		}, nil
	}

	// Handle REF() similarly to ADR() but returns a Reference value
	if calleeName == "REF" {
		if len(e.Args) != 1 {
			return Value{}, &RuntimeError{Msg: "REF requires exactly 1 argument"}
		}
		argIdent, ok := e.Args[0].(*ast.Ident)
		if !ok {
			return Value{}, &RuntimeError{Msg: "REF argument must be a variable name"}
		}
		targetEnv := env.FindOwner(argIdent.Name)
		if targetEnv == nil {
			return Value{}, &RuntimeError{Msg: fmt.Sprintf("REF: undefined variable '%s'", argIdent.Name)}
		}
		return Value{
			Kind:   ValReference,
			PtrEnv: targetEnv,
			PtrVar: strings.ToUpper(argIdent.Name),
		}, nil
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

	// Zero-argument FB instance call written as an expression statement:
	// fb(); runs the instance with its current inputs.
	if len(e.Args) == 0 {
		if v, ok := env.Get(calleeName); ok && v.Kind == ValFBInstance && v.FBRef != nil {
			return Value{}, interp.runFBInstance(v.FBRef)
		}
	}

	return Value{}, &RuntimeError{Msg: fmt.Sprintf("undefined function: %s", calleeName)}
}

// execAction runs an ACTION body in owner, the environment of the POU or FB
// instance that owns the action. RETURN ends only the action.
func (interp *Interpreter) execAction(owner *Env, act *ast.ActionDecl, pos ast.Pos) error {
	if err := interp.EnterCall(act.Name.Name, pos); err != nil {
		return err
	}
	defer interp.ExitCall()
	if err := interp.execStatements(owner, act.Body); err != nil {
		if _, ok := err.(*ErrReturn); !ok {
			return err
		}
	}
	return nil
}

// runFBInstance executes an FB instance without setting any inputs, with the
// time elapsed since the instance last ran.
func (interp *Interpreter) runFBInstance(inst *FBInstance) error {
	return inst.Execute(inst.deltaFor(interp.clock, interp.dt), interp)
}

// fbDeclChain returns inst's declaration followed by the FBs it EXTENDS,
// derived-most first. The FBDecls registry supplies the chain; when it does
// not know a base, the instance's recorded ParentDecl continues the walk (the
// test runner and hand-built instances set only ParentDecl). Each level is
// visited once, so a cycle or a long chain cannot loop.
func fbDeclChain(inst *FBInstance, interp *Interpreter) []*ast.FunctionBlockDecl {
	if inst.Decl == nil {
		return nil
	}
	var out []*ast.FunctionBlockDecl
	add := func(base *ast.FunctionBlockDecl) {
		chain := fbExtendsChain(base, interp)
		for i := len(chain) - 1; i >= 0; i-- {
			if !slices.Contains(out, chain[i]) {
				out = append(out, chain[i])
			}
		}
	}
	add(inst.Decl)
	if inst.ParentDecl != nil && inst.Decl.Extends != nil && !slices.Contains(out, inst.ParentDecl) {
		add(inst.ParentDecl)
	}
	return out
}

// findAction looks up an ACTION by name on an FB instance: the FB's own
// actions, then the EXTENDS chain, then whatever was registered on the
// instance env when it was created.
func findAction(inst *FBInstance, name string, interp *Interpreter) *ast.ActionDecl {
	for _, d := range fbDeclChain(inst, interp) {
		for _, a := range d.Actions {
			if a.Name != nil && strings.EqualFold(a.Name.Name, name) {
				return a
			}
		}
	}
	if inst.Env != nil {
		return inst.Env.localAction(name)
	}
	return nil
}

// evalMethodCall evaluates a method call on an object (e.g., fb.GetValue()).
// It resolves the object, finds the method declaration, and executes it.
func (interp *Interpreter) evalMethodCall(env *Env, memberAccess *ast.MemberAccessExpr, argExprs []ast.Expr) (Value, error) {
	methodName := memberAccess.Member.Name
	pos := memberAccess.Span().Start

	// GVL.fb(); runs an FB instance that lives in a GVL.
	if g, gvl := interp.gvlRoot(env, memberAccess.Object); g != nil {
		v, err := evalGVLMember(g, gvl, memberAccess.Member)
		if err != nil {
			return Value{}, err
		}
		if len(argExprs) == 0 && v.Kind == ValFBInstance && v.FBRef != nil {
			return Value{}, interp.runFBInstance(v.FBRef)
		}
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("%s.%s is not callable", gvl.Name, methodName), Pos: pos}
	}

	obj, err := interp.evalExpr(env, memberAccess.Object)
	if err != nil {
		return Value{}, err
	}

	// s.fb(); runs an FB instance held in a struct member.
	if obj.Kind == ValStruct && len(argExprs) == 0 {
		if v, ok := obj.Struct[strings.ToUpper(methodName)]; ok && v.Kind == ValFBInstance && v.FBRef != nil {
			return Value{}, interp.runFBInstance(v.FBRef)
		}
	}

	if obj.Kind != ValFBInstance || obj.FBRef == nil {
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("cannot call method '%s' on %s", methodName, obj.Kind)}
	}

	fbInst := obj.FBRef

	// Find the method in the FB declaration (including inherited methods)
	method := findMethod(fbInst, methodName, interp)
	if method == nil {
		// inst.A1(); runs the FB's ACTION in the instance env.
		if act := findAction(fbInst, methodName, interp); act != nil {
			if len(argExprs) > 0 {
				return Value{}, &RuntimeError{Msg: fmt.Sprintf("action %s takes no arguments", act.Name.Name), Pos: pos}
			}
			return Value{}, interp.execAction(fbInst.Env, act, pos)
		}
		// outer.inner(); runs a nested FB instance.
		if len(argExprs) == 0 && fbInst.Env != nil {
			if v, ok := fbInst.Env.GetLocal(methodName); ok && v.Kind == ValFBInstance && v.FBRef != nil {
				return Value{}, interp.runFBInstance(v.FBRef)
			}
		}
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("method '%s' not found on FB '%s'", methodName, fbInst.TypeName)}
	}

	if err := interp.EnterCall(method.Name.Name, pos); err != nil {
		return Value{}, err
	}
	defer interp.ExitCall()

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

// findMethod looks up a method by name in the FB declaration hierarchy: the
// FB's own methods first, then each FB up the EXTENDS chain.
func findMethod(inst *FBInstance, name string, interp *Interpreter) *ast.MethodDecl {
	for _, d := range fbDeclChain(inst, interp) {
		for _, m := range d.Methods {
			if m.Name != nil && strings.EqualFold(m.Name.Name, name) {
				return m
			}
		}
	}
	return nil
}

// findProperty looks up a property by name in the FB declaration hierarchy:
// the FB's own properties first, then each FB up the EXTENDS chain.
func findProperty(inst *FBInstance, name string, interp *Interpreter) *ast.PropertyDecl {
	for _, d := range fbDeclChain(inst, interp) {
		for _, p := range d.Properties {
			if p.Name != nil && strings.EqualFold(p.Name.Name, name) {
				return p
			}
		}
	}
	return nil
}

// execPropertyGetter executes a property's GET accessor and returns the result.
func (interp *Interpreter) execPropertyGetter(inst *FBInstance, prop *ast.PropertyDecl) (Value, error) {
	getter := prop.Getter
	getterEnv := NewEnv(inst.Env)

	// Define return variable (getter method name, typically "GET" or the property name)
	retVal := ZeroFromTypeSpec(prop.Type)
	if getter.Name != nil {
		getterEnv.Define(getter.Name.Name, retVal)
	}
	// Also define the property name as the return variable
	if prop.Name != nil {
		getterEnv.Define(prop.Name.Name, retVal)
	}

	// Initialize local variables
	for _, vb := range getter.VarBlocks {
		for _, vd := range vb.Declarations {
			val := ZeroFromTypeSpec(vd.Type)
			if vd.InitValue != nil {
				if iv, err := interp.evalExpr(getterEnv, vd.InitValue); err == nil {
					val = iv
				}
			}
			for _, n := range vd.Names {
				getterEnv.Define(n.Name, val)
			}
		}
	}

	err := interp.execStatements(getterEnv, getter.Body)
	if err != nil {
		if _, ok := err.(*ErrReturn); !ok {
			return Value{}, err
		}
	}

	// Read return value: try getter name first, then property name
	if getter.Name != nil {
		if v, ok := getterEnv.Get(getter.Name.Name); ok {
			return v, nil
		}
	}
	if prop.Name != nil {
		if v, ok := getterEnv.Get(prop.Name.Name); ok {
			return v, nil
		}
	}
	return retVal, nil
}

// execPropertySetter executes a property's SET accessor with the given value.
func (interp *Interpreter) execPropertySetter(inst *FBInstance, prop *ast.PropertyDecl, val Value) error {
	setter := prop.Setter
	setterEnv := NewEnv(inst.Env)

	// Define the property name variable for assignment
	if prop.Name != nil {
		setterEnv.Define(prop.Name.Name, val)
	}

	// Initialize VAR_INPUT parameters (the value being set)
	for _, vb := range setter.VarBlocks {
		if vb.Section == ast.VarInput {
			for _, vd := range vb.Declarations {
				for _, n := range vd.Names {
					setterEnv.Define(n.Name, val)
				}
			}
		} else {
			for _, vd := range vb.Declarations {
				v := ZeroFromTypeSpec(vd.Type)
				if vd.InitValue != nil {
					if iv, err := interp.evalExpr(setterEnv, vd.InitValue); err == nil {
						v = iv
					}
				}
				for _, n := range vd.Names {
					setterEnv.Define(n.Name, v)
				}
			}
		}
	}

	err := interp.execStatements(setterEnv, setter.Body)
	if err != nil {
		if _, ok := err.(*ErrReturn); !ok {
			return err
		}
	}
	return nil
}
