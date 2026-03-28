// Package emit produces Structured Text source code from an AST,
// with vendor-specific transformations for Beckhoff, Schneider, and
// Portable targets.
package emit

import "strings"

// Target identifies the vendor emission target.
type Target string

const (
	// TargetBeckhoff emits full CODESYS-compatible ST with all OOP constructs.
	TargetBeckhoff Target = "beckhoff"
	// TargetSchneider emits ST without OOP constructs (METHOD, PROPERTY, INTERFACE, POINTER TO, REFERENCE TO).
	TargetSchneider Target = "schneider"
	// TargetPortable emits the portable subset: no OOP, no pointers/references, no 64-bit types.
	TargetPortable Target = "portable"
)

// Options controls emission behavior.
type Options struct {
	Target            Target
	Indent            string // Default: "    " (4 spaces)
	UppercaseKeywords bool   // Default: true
}

// DefaultOptions returns emission options with sensible defaults (Beckhoff target).
func DefaultOptions() Options {
	return Options{
		Target:            TargetBeckhoff,
		Indent:            "    ",
		UppercaseKeywords: true,
	}
}

// LookupTarget normalizes a target name string to a Target constant.
// Returns TargetBeckhoff for unknown names.
func LookupTarget(name string) Target {
	switch strings.ToLower(name) {
	case "beckhoff":
		return TargetBeckhoff
	case "schneider":
		return TargetSchneider
	case "portable":
		return TargetPortable
	default:
		return TargetBeckhoff
	}
}

// supportsOOP returns true if the target supports OOP constructs.
func (t Target) supportsOOP() bool {
	return t == TargetBeckhoff
}

// supportsPointerTo returns true if the target supports POINTER TO.
func (t Target) supportsPointerTo() bool {
	return t == TargetBeckhoff
}

// supportsReferenceTo returns true if the target supports REFERENCE TO.
func (t Target) supportsReferenceTo() bool {
	return t == TargetBeckhoff
}

// supports64Bit returns true if the target supports 64-bit types.
func (t Target) supports64Bit() bool {
	return t != TargetPortable
}

// is64BitType returns true if the given type name is a 64-bit IEC type.
func is64BitType(name string) bool {
	switch strings.ToUpper(name) {
	case "LINT", "LREAL", "LWORD", "ULINT":
		return true
	}
	return false
}
