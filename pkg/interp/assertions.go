package interp

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
)

// AssertionResult records one assertion outcome.
type AssertionResult struct {
	Passed  bool
	Message string
	Pos     ast.Pos
}

// AssertionCollector gathers assertion results during test execution.
// It is NOT a global -- each test case gets its own collector.
type AssertionCollector struct {
	Results []AssertionResult
}

// Record adds an assertion result to the collector.
func (c *AssertionCollector) Record(passed bool, msg string, pos ast.Pos) {
	c.Results = append(c.Results, AssertionResult{Passed: passed, Message: msg, Pos: pos})
}

// HasFailures returns true if any assertion failed.
func (c *AssertionCollector) HasFailures() bool {
	for _, r := range c.Results {
		if !r.Passed {
			return true
		}
	}
	return false
}

// Failures returns only the failed assertion results.
func (c *AssertionCollector) Failures() []AssertionResult {
	var out []AssertionResult
	for _, r := range c.Results {
		if !r.Passed {
			out = append(out, r)
		}
	}
	return out
}

// RegisterAssertions populates LocalFunctions with ASSERT_TRUE, ASSERT_FALSE,
// ASSERT_EQ, and ASSERT_NEAR. Each assertion records pass/fail on the
// collector and returns BoolValue(true) so execution continues after failure.
func (interp *Interpreter) RegisterAssertions(collector *AssertionCollector) {
	if interp.LocalFunctions == nil {
		interp.LocalFunctions = make(map[string]func(args []Value, pos ast.Pos) (Value, error))
	}
	interp.Collector = collector

	interp.LocalFunctions["ASSERT_TRUE"] = func(args []Value, pos ast.Pos) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "ASSERT_TRUE requires at least 1 argument", Pos: pos}
		}
		passed := args[0].IsTruthy()
		msg := ""
		if !passed {
			msg = "expected TRUE, got FALSE"
			if len(args) >= 2 && args[1].Kind == ValString {
				msg = args[1].Str
			}
		}
		collector.Record(passed, msg, pos)
		return BoolValue(true), nil
	}

	interp.LocalFunctions["ASSERT_FALSE"] = func(args []Value, pos ast.Pos) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "ASSERT_FALSE requires at least 1 argument", Pos: pos}
		}
		passed := !args[0].IsTruthy()
		msg := ""
		if !passed {
			msg = "expected FALSE, got TRUE"
			if len(args) >= 2 && args[1].Kind == ValString {
				msg = args[1].Str
			}
		}
		collector.Record(passed, msg, pos)
		return BoolValue(true), nil
	}

	interp.LocalFunctions["ASSERT_EQ"] = func(args []Value, pos ast.Pos) (Value, error) {
		if len(args) < 2 {
			return Value{}, &RuntimeError{Msg: "ASSERT_EQ requires at least 2 arguments", Pos: pos}
		}
		actual := args[0]
		expected := args[1]
		passed := valuesEqual(actual, expected)
		msg := ""
		if !passed {
			msg = fmt.Sprintf("expected %s, got %s", expected.String(), actual.String())
			if len(args) >= 3 && args[2].Kind == ValString {
				msg = args[2].Str
			}
		}
		collector.Record(passed, msg, pos)
		return BoolValue(true), nil
	}

	interp.LocalFunctions["ASSERT_NEAR"] = func(args []Value, pos ast.Pos) (Value, error) {
		if len(args) < 3 {
			return Value{}, &RuntimeError{Msg: "ASSERT_NEAR requires at least 3 arguments (actual, expected, epsilon)", Pos: pos}
		}
		actual := toFloat(args[0])
		expected := toFloat(args[1])
		epsilon := toFloat(args[2])
		passed := math.Abs(actual-expected) <= epsilon
		msg := ""
		if !passed {
			msg = fmt.Sprintf("expected %g +/- %g, got %g", expected, epsilon, actual)
			if len(args) >= 4 && args[3].Kind == ValString {
				msg = args[3].Str
			}
		}
		collector.Record(passed, msg, pos)
		return BoolValue(true), nil
	}
}

// RegisterFunction adds a named function to LocalFunctions.
// This is used by the test runner to register user-defined functions.
func (interp *Interpreter) RegisterFunction(name string, fn func(args []Value, pos ast.Pos) (Value, error)) {
	if interp.LocalFunctions == nil {
		interp.LocalFunctions = make(map[string]func(args []Value, pos ast.Pos) (Value, error))
	}
	interp.LocalFunctions[name] = fn
}

// RegisterEnumType registers an enum type with the interpreter so that
// typed enum literals like Color#Green can be resolved at runtime.
func (interp *Interpreter) RegisterEnumType(typeName string, values map[string]int64) {
	if interp.EnumTypes == nil {
		interp.EnumTypes = make(map[string]map[string]int64)
	}
	interp.EnumTypes[strings.ToUpper(typeName)] = values
}

// RegisterAdvanceTime adds ADVANCE_TIME to LocalFunctions. It expects 1 arg
// of Kind==ValTime and calls the provided tick function with that duration.
func (interp *Interpreter) RegisterAdvanceTime(tickFn func(time.Duration)) {
	if interp.LocalFunctions == nil {
		interp.LocalFunctions = make(map[string]func(args []Value, pos ast.Pos) (Value, error))
	}

	interp.LocalFunctions["ADVANCE_TIME"] = func(args []Value, pos ast.Pos) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "ADVANCE_TIME requires 1 argument", Pos: pos}
		}
		if args[0].Kind != ValTime {
			return Value{}, &RuntimeError{Msg: fmt.Sprintf("ADVANCE_TIME expects TIME argument, got %s", args[0].Kind), Pos: pos}
		}
		tickFn(args[0].Time)
		return BoolValue(true), nil
	}
}
