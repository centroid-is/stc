// Package diag provides diagnostic types for compiler errors, warnings,
// and informational messages with source position tracking.
package diag

import (
	"encoding/json"
	"fmt"

	"github.com/centroid-is/stc/pkg/source"
)

// Severity indicates the severity level of a diagnostic.
type Severity int

const (
	// Error indicates a compilation error that prevents successful compilation.
	Error Severity = iota
	// Warning indicates a potential issue that does not prevent compilation.
	Warning
	// Info indicates an informational message.
	Info
	// Hint indicates a suggestion for improvement.
	Hint
)

// String returns the string representation of the severity.
func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	case Info:
		return "info"
	case Hint:
		return "hint"
	default:
		return "unknown"
	}
}

// MarshalJSON returns the JSON encoding of the severity as a string.
func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// Diagnostic represents a compiler diagnostic with position, severity,
// and a human-readable message.
type Diagnostic struct {
	Severity Severity   `json:"severity"`
	Pos      source.Pos `json:"pos"`
	EndPos   source.Pos `json:"end_pos"`
	Code     string     `json:"code"`
	Message  string     `json:"message"`
}

// String returns the diagnostic in "file:line:col: severity: message" format.
func (d Diagnostic) String() string {
	return fmt.Sprintf("%s: %s: %s", d.Pos.String(), d.Severity.String(), d.Message)
}
