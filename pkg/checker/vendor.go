package checker

import (
	"strconv"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
	"github.com/centroid-is/stc/pkg/symbols"
)

// VendorProfile describes the feature capabilities of a target PLC vendor.
type VendorProfile struct {
	Name               string
	SupportsOOP        bool // METHOD, INTERFACE, PROPERTY
	SupportsPointerTo  bool // POINTER TO
	SupportsReferenceTo bool // REFERENCE TO
	Supports64Bit      bool // LINT, LREAL, LWORD, ULINT
	SupportsWString    bool // WSTRING
	MaxStringLen       int  // 0 = unlimited
}

// Built-in vendor profiles.
var (
	// Beckhoff supports all CODESYS extensions.
	Beckhoff = &VendorProfile{
		Name:               "beckhoff",
		SupportsOOP:        true,
		SupportsPointerTo:  true,
		SupportsReferenceTo: true,
		Supports64Bit:      true,
		SupportsWString:    true,
		MaxStringLen:       0,
	}

	// Schneider lacks OOP, POINTER TO, and REFERENCE TO; string max 254.
	Schneider = &VendorProfile{
		Name:               "schneider",
		SupportsOOP:        false,
		SupportsPointerTo:  false,
		SupportsReferenceTo: false,
		Supports64Bit:      true,
		SupportsWString:    true,
		MaxStringLen:       254,
	}

	// Portable is the intersection of all vendors: no OOP, no pointers,
	// no references, no 64-bit, no WSTRING, max string 254.
	Portable = &VendorProfile{
		Name:               "portable",
		SupportsOOP:        false,
		SupportsPointerTo:  false,
		SupportsReferenceTo: false,
		Supports64Bit:      false,
		SupportsWString:    false,
		MaxStringLen:       254,
	}
)

// vendorRegistry maps lowercase vendor names to profiles.
var vendorRegistry = map[string]*VendorProfile{
	"beckhoff":  Beckhoff,
	"schneider": Schneider,
	"portable":  Portable,
}

// LookupVendor returns the vendor profile for the given name (case-insensitive).
// Returns nil for unknown vendor names.
func LookupVendor(name string) *VendorProfile {
	return vendorRegistry[strings.ToLower(name)]
}

// is64BitType returns true if the type name represents a 64-bit IEC type.
func is64BitType(name string) bool {
	switch strings.ToUpper(name) {
	case "LINT", "LREAL", "LWORD", "ULINT":
		return true
	}
	return false
}

// CheckVendorCompat walks the AST and emits warnings for constructs that
// are not supported by the given vendor profile. All diagnostics are warnings.
func CheckVendorCompat(files []*ast.SourceFile, _ *symbols.Table, profile *VendorProfile, diags *diag.Collector) {
	if profile == nil {
		return
	}
	for _, file := range files {
		for _, decl := range file.Declarations {
			checkVendorDecl(decl, profile, diags)
		}
	}
}

// checkVendorDecl checks a single declaration for vendor compatibility.
func checkVendorDecl(decl ast.Declaration, profile *VendorProfile, diags *diag.Collector) {
	switch d := decl.(type) {
	case *ast.FunctionBlockDecl:
		// Check methods and properties for OOP
		if !profile.SupportsOOP {
			for _, m := range d.Methods {
				emitVendorWarn(diags, spanPos(m.Span()), CodeVendorOOP,
					"METHOD '%s' not supported by %s (no OOP support)", m.Name.Name, profile.Name)
			}
			for _, p := range d.Properties {
				emitVendorWarn(diags, spanPos(p.Span()), CodeVendorOOP,
					"PROPERTY '%s' not supported by %s (no OOP support)", p.Name.Name, profile.Name)
			}
		}
		// Check var blocks
		for _, vb := range d.VarBlocks {
			checkVendorVarBlock(vb, profile, diags)
		}

	case *ast.InterfaceDecl:
		if !profile.SupportsOOP {
			emitVendorWarn(diags, spanPos(d.Span()), CodeVendorOOP,
				"INTERFACE '%s' not supported by %s (no OOP support)", d.Name.Name, profile.Name)
		}

	case *ast.MethodDecl:
		if !profile.SupportsOOP {
			emitVendorWarn(diags, spanPos(d.Span()), CodeVendorOOP,
				"METHOD '%s' not supported by %s (no OOP support)", d.Name.Name, profile.Name)
		}
		for _, vb := range d.VarBlocks {
			checkVendorVarBlock(vb, profile, diags)
		}

	case *ast.PropertyDecl:
		if !profile.SupportsOOP {
			emitVendorWarn(diags, spanPos(d.Span()), CodeVendorOOP,
				"PROPERTY '%s' not supported by %s (no OOP support)", d.Name.Name, profile.Name)
		}

	case *ast.ProgramDecl:
		for _, vb := range d.VarBlocks {
			checkVendorVarBlock(vb, profile, diags)
		}

	case *ast.FunctionDecl:
		for _, vb := range d.VarBlocks {
			checkVendorVarBlock(vb, profile, diags)
		}
	}
}

// checkVendorVarBlock checks variable declarations for vendor compatibility.
func checkVendorVarBlock(vb *ast.VarBlock, profile *VendorProfile, diags *diag.Collector) {
	for _, vd := range vb.Declarations {
		checkVendorTypeSpec(vd.Type, profile, diags)
	}
}

// checkVendorTypeSpec checks a type specifier for vendor compatibility.
func checkVendorTypeSpec(ts ast.TypeSpec, profile *VendorProfile, diags *diag.Collector) {
	if ts == nil {
		return
	}
	switch t := ts.(type) {
	case *ast.PointerType:
		if !profile.SupportsPointerTo {
			emitVendorWarn(diags, spanPos(t.Span()), CodeVendorPointer,
				"POINTER TO not supported by %s", profile.Name)
		}

	case *ast.ReferenceType:
		if !profile.SupportsReferenceTo {
			emitVendorWarn(diags, spanPos(t.Span()), CodeVendorReference,
				"REFERENCE TO not supported by %s", profile.Name)
		}

	case *ast.NamedType:
		if !profile.Supports64Bit && t.Name != nil && is64BitType(t.Name.Name) {
			emitVendorWarn(diags, spanPos(t.Span()), CodeVendor64Bit,
				"64-bit type %s not supported by %s", strings.ToUpper(t.Name.Name), profile.Name)
		}

	case *ast.StringType:
		if t.IsWide && !profile.SupportsWString {
			emitVendorWarn(diags, spanPos(t.Span()), CodeVendorWString,
				"WSTRING not supported by %s", profile.Name)
		}
		if t.Length != nil && profile.MaxStringLen > 0 {
			if lit, ok := t.Length.(*ast.Literal); ok {
				if length, err := strconv.Atoi(lit.Value); err == nil {
					if length > profile.MaxStringLen {
						emitVendorWarn(diags, spanPos(t.Span()), CodeVendorStringLen,
							"string length %d exceeds %s limit of %d", length, profile.Name, profile.MaxStringLen)
					}
				}
			}
		}

	case *ast.ArrayType:
		checkVendorTypeSpec(t.ElementType, profile, diags)
	}
}

// spanPos extracts the start position from an AST span as a source.Pos.
func spanPos(s ast.Span) source.Pos {
	return source.Pos{
		File:   s.Start.File,
		Line:   s.Start.Line,
		Col:    s.Start.Col,
		Offset: s.Start.Offset,
	}
}

// emitVendorWarn emits a vendor warning diagnostic.
func emitVendorWarn(diags *diag.Collector, pos source.Pos, code, format string, args ...any) {
	diags.Warnf(pos, code, format, args...)
}
