package diag

import (
	"fmt"

	"github.com/centroid-is/stc/pkg/source"
)

// Collector accumulates diagnostics during compilation.
type Collector struct {
	diags []Diagnostic
}

// NewCollector creates a new empty Collector.
func NewCollector() *Collector {
	return &Collector{}
}

// Add appends a diagnostic with the given severity, position, code, and message.
func (c *Collector) Add(sev Severity, pos source.Pos, endPos source.Pos, code, msg string) {
	c.diags = append(c.diags, Diagnostic{
		Severity: sev,
		Pos:      pos,
		EndPos:   endPos,
		Code:     code,
		Message:  msg,
	})
}

// Errorf adds an error diagnostic with a formatted message.
func (c *Collector) Errorf(pos source.Pos, code, format string, args ...any) {
	c.Add(Error, pos, source.Pos{}, code, fmt.Sprintf(format, args...))
}

// Warnf adds a warning diagnostic with a formatted message.
func (c *Collector) Warnf(pos source.Pos, code, format string, args ...any) {
	c.Add(Warning, pos, source.Pos{}, code, fmt.Sprintf(format, args...))
}

// All returns all collected diagnostics.
func (c *Collector) All() []Diagnostic {
	return c.diags
}

// HasErrors returns true if any error-level diagnostics have been collected.
func (c *Collector) HasErrors() bool {
	for _, d := range c.diags {
		if d.Severity == Error {
			return true
		}
	}
	return false
}

// Errors returns only the error-level diagnostics.
func (c *Collector) Errors() []Diagnostic {
	var errs []Diagnostic
	for _, d := range c.diags {
		if d.Severity == Error {
			errs = append(errs, d)
		}
	}
	return errs
}
