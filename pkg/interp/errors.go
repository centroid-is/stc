package interp

import (
	"fmt"

	"github.com/centroid-is/stc/pkg/ast"
)

// RuntimeError represents an error that occurs during interpretation.
type RuntimeError struct {
	Msg string
	Pos ast.Pos // Optional source position
}

// Error implements the error interface.
func (e *RuntimeError) Error() string {
	if e.Pos.Line > 0 {
		return fmt.Sprintf("%s:%d:%d: runtime error: %s", e.Pos.File, e.Pos.Line, e.Pos.Col, e.Msg)
	}
	return fmt.Sprintf("runtime error: %s", e.Msg)
}

// Control flow signal errors. These are not real errors -- they are used
// to unwind the call stack for RETURN, EXIT, and CONTINUE statements.

// ErrReturn is a control flow signal for the RETURN statement.
type ErrReturn struct{}

func (e *ErrReturn) Error() string { return "RETURN" }

// ErrExit is a control flow signal for the EXIT statement (break loop).
type ErrExit struct{}

func (e *ErrExit) Error() string { return "EXIT" }

// ErrContinue is a control flow signal for the CONTINUE statement.
type ErrContinue struct{}

func (e *ErrContinue) Error() string { return "CONTINUE" }

// Sentinel error values for common runtime errors.
var (
	errDivisionByZero = &RuntimeError{Msg: "division by zero"}
)
