// Package lint provides a rule-based linting engine for IEC 61131-3 Structured Text.
// It checks PLCopen coding guidelines and configurable naming conventions.
package lint

import (
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
)

// Lint diagnostic codes.
const (
	CodeMagicNumber    = "LINT001" // magic number in expression
	CodeDeepNesting    = "LINT002" // deeply nested control flow (>3 levels)
	CodeLongPOU        = "LINT003" // POU body exceeds 200 statements
	CodeMissingReturnType = "LINT004" // FUNCTION without return type
	CodeNamingPOU      = "LINT005" // POU name not PascalCase
	CodeNamingVar      = "LINT006" // variable name not lower_snake_case
	CodeNamingConstant = "LINT007" // constant not UPPER_SNAKE_CASE
)

// Rule is the interface for a lint rule.
type Rule interface {
	Name() string
	Check(file *ast.SourceFile, opts LintOptions) []diag.Diagnostic
}

// LintOptions configures the linter behavior.
type LintOptions struct {
	NamingConvention string // "plcopen" (default), "none" to disable naming checks
}

// DefaultLintOptions returns options with PLCopen naming convention.
func DefaultLintOptions() LintOptions {
	return LintOptions{NamingConvention: "plcopen"}
}
