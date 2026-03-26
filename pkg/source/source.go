// Package source provides source file abstractions and position tracking
// for the STC compiler toolchain.
package source

import (
	"fmt"
	"sort"
	"strings"
)

// Pos represents a position in a source file.
type Pos struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Col    int    `json:"col"`
	Offset int    `json:"offset"`
}

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

// SourceFile represents a source file with content and line offset tracking.
type SourceFile struct {
	Name    string `json:"name"`
	Content string `json:"content"`
	lines   []int  // byte offsets of line starts, computed lazily
}

// New creates a new SourceFile with the given name and content.
func New(name, content string) *SourceFile {
	return &SourceFile{
		Name:    name,
		Content: content,
	}
}

// computeLines scans the content for newlines and builds a table of
// byte offsets where each line starts.
func (sf *SourceFile) computeLines() {
	if sf.lines != nil {
		return
	}
	sf.lines = []int{0} // line 1 starts at offset 0
	for i, b := range []byte(sf.Content) {
		if b == '\n' {
			sf.lines = append(sf.lines, i+1)
		}
	}
}

// PosFromOffset converts a byte offset to a Pos with line and column numbers.
// Lines and columns are 1-based.
func (sf *SourceFile) PosFromOffset(offset int) Pos {
	sf.computeLines()

	// Binary search for the line containing this offset.
	line := sort.Search(len(sf.lines), func(i int) bool {
		return sf.lines[i] > offset
	})
	// line is now the index of the first line start AFTER offset,
	// so the actual line is line-1 (but 1-based, so line number = line).
	col := offset - sf.lines[line-1] + 1

	return Pos{
		File:   sf.Name,
		Line:   line,
		Col:    col,
		Offset: offset,
	}
}

// LineContent returns the content of the given 1-based line number.
// Returns an empty string if the line number is out of range.
func (sf *SourceFile) LineContent(line int) string {
	sf.computeLines()

	if line < 1 || line > len(sf.lines) {
		return ""
	}

	start := sf.lines[line-1]
	var end int
	if line < len(sf.lines) {
		end = sf.lines[line] - 1 // exclude the newline
	} else {
		end = len(sf.Content)
	}

	if start > len(sf.Content) {
		return ""
	}
	if end > len(sf.Content) {
		end = len(sf.Content)
	}

	return strings.TrimRight(sf.Content[start:end], "\r")
}
