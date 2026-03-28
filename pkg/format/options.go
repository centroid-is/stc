// Package format provides ST code formatting with configurable style.
package format

// FormatOptions controls formatting behavior.
type FormatOptions struct {
	Indent            string // Indentation string (default: "    " = 4 spaces)
	UppercaseKeywords bool   // Whether keywords are uppercase (default: true)
}

// DefaultFormatOptions returns formatting options with sensible defaults:
// 4-space indent, uppercase keywords.
func DefaultFormatOptions() FormatOptions {
	return FormatOptions{
		Indent:            "    ",
		UppercaseKeywords: true,
	}
}
