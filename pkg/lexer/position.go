package lexer

import "fmt"

// String returns the position in "file:line:col" format.
func (p Pos) String() string {
	return fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Col)
}

// Span represents a range between two positions in a source file.
type Span struct {
	Start Pos `json:"start"`
	End   Pos `json:"end"`
}

// SpanFrom creates a Span from start and end positions.
func SpanFrom(start, end Pos) Span {
	return Span{Start: start, End: end}
}
