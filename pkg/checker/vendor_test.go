package checker

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
	"github.com/centroid-is/stc/pkg/symbols"
)

func TestBeckhoffProfile(t *testing.T) {
	p := Beckhoff
	if !p.SupportsOOP {
		t.Error("Beckhoff should support OOP")
	}
	if !p.SupportsPointerTo {
		t.Error("Beckhoff should support POINTER TO")
	}
	if !p.SupportsReferenceTo {
		t.Error("Beckhoff should support REFERENCE TO")
	}
	if !p.Supports64Bit {
		t.Error("Beckhoff should support 64-bit types")
	}
	if !p.SupportsWString {
		t.Error("Beckhoff should support WSTRING")
	}
	if p.MaxStringLen != 0 {
		t.Errorf("Beckhoff MaxStringLen should be 0 (unlimited), got %d", p.MaxStringLen)
	}
}

func TestSchneiderProfile(t *testing.T) {
	p := Schneider
	if p.SupportsOOP {
		t.Error("Schneider should not support OOP")
	}
	if p.SupportsPointerTo {
		t.Error("Schneider should not support POINTER TO")
	}
	if p.SupportsReferenceTo {
		t.Error("Schneider should not support REFERENCE TO")
	}
	if !p.Supports64Bit {
		t.Error("Schneider should support 64-bit types")
	}
	if !p.SupportsWString {
		t.Error("Schneider should support WSTRING")
	}
	if p.MaxStringLen != 254 {
		t.Errorf("Schneider MaxStringLen should be 254, got %d", p.MaxStringLen)
	}
}

func TestPortableProfile(t *testing.T) {
	p := Portable
	if p.SupportsOOP {
		t.Error("Portable should not support OOP")
	}
	if p.SupportsPointerTo {
		t.Error("Portable should not support POINTER TO")
	}
	if p.SupportsReferenceTo {
		t.Error("Portable should not support REFERENCE TO")
	}
	if p.Supports64Bit {
		t.Error("Portable should not support 64-bit types")
	}
	if p.SupportsWString {
		t.Error("Portable should not support WSTRING")
	}
	if p.MaxStringLen != 254 {
		t.Errorf("Portable MaxStringLen should be 254, got %d", p.MaxStringLen)
	}
}

func TestLookupVendor(t *testing.T) {
	tests := []struct {
		name     string
		expected *VendorProfile
	}{
		{"beckhoff", Beckhoff},
		{"Beckhoff", Beckhoff},
		{"BECKHOFF", Beckhoff},
		{"schneider", Schneider},
		{"Schneider", Schneider},
		{"portable", Portable},
		{"Portable", Portable},
		{"unknown", nil},
		{"", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LookupVendor(tt.name)
			if got != tt.expected {
				t.Errorf("LookupVendor(%q) = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

// Helper to build a minimal AST with the given declarations.
func makeSourceFile(decls ...ast.Declaration) *ast.SourceFile {
	return &ast.SourceFile{
		NodeBase:     ast.NodeBase{NodeKind: ast.KindSourceFile},
		Declarations: decls,
	}
}

func pos(line, col int) source.Pos {
	return source.Pos{File: "test.st", Line: line, Col: col}
}

func TestVendorCheckOOP(t *testing.T) {
	// FB with METHOD checked against schneider profile emits VEND001 warning
	method := &ast.MethodDecl{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindMethodDecl,
			NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 3, Col: 1}},
		},
		Name: &ast.Ident{Name: "DoWork"},
	}
	fb := &ast.FunctionBlockDecl{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindFunctionBlockDecl,
			NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 1, Col: 1}},
		},
		Name:    &ast.Ident{Name: "MyFB"},
		Methods: []*ast.MethodDecl{method},
	}
	file := makeSourceFile(fb)
	table := symbols.NewTable()
	collector := diag.NewCollector()

	CheckVendorCompat([]*ast.SourceFile{file}, table, Schneider, collector)

	diags := collector.All()
	if len(diags) == 0 {
		t.Fatal("expected VEND001 warning for METHOD on schneider")
	}
	found := false
	for _, d := range diags {
		if d.Code == "VEND001" {
			found = true
			if d.Severity != diag.Warning {
				t.Errorf("VEND001 should be Warning, got %v", d.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected VEND001 diagnostic, got %v", diags)
	}
}

func TestVendorCheckPointer(t *testing.T) {
	// POINTER TO type against schneider emits VEND002
	varDecl := &ast.VarDecl{
		Names: []*ast.Ident{{Name: "pData"}},
		Type: &ast.PointerType{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindPointerType,
				NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 3, Col: 5}},
			},
			BaseType: &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
		},
	}
	varBlock := &ast.VarBlock{
		Section:      ast.VarLocal,
		Declarations: []*ast.VarDecl{varDecl},
	}
	prog := &ast.ProgramDecl{
		NodeBase:  ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:      &ast.Ident{Name: "Main"},
		VarBlocks: []*ast.VarBlock{varBlock},
	}
	file := makeSourceFile(prog)
	table := symbols.NewTable()
	collector := diag.NewCollector()

	CheckVendorCompat([]*ast.SourceFile{file}, table, Schneider, collector)

	diags := collector.All()
	found := false
	for _, d := range diags {
		if d.Code == "VEND002" {
			found = true
			if d.Severity != diag.Warning {
				t.Errorf("VEND002 should be Warning, got %v", d.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected VEND002 diagnostic, got %v", diags)
	}
}

func TestVendorCheckReference(t *testing.T) {
	// REFERENCE TO type against schneider emits VEND003
	varDecl := &ast.VarDecl{
		Names: []*ast.Ident{{Name: "refData"}},
		Type: &ast.ReferenceType{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindReferenceType,
				NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 3, Col: 5}},
			},
			BaseType: &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
		},
	}
	varBlock := &ast.VarBlock{
		Section:      ast.VarLocal,
		Declarations: []*ast.VarDecl{varDecl},
	}
	prog := &ast.ProgramDecl{
		NodeBase:  ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:      &ast.Ident{Name: "Main"},
		VarBlocks: []*ast.VarBlock{varBlock},
	}
	file := makeSourceFile(prog)
	table := symbols.NewTable()
	collector := diag.NewCollector()

	CheckVendorCompat([]*ast.SourceFile{file}, table, Schneider, collector)

	diags := collector.All()
	found := false
	for _, d := range diags {
		if d.Code == "VEND003" {
			found = true
			if d.Severity != diag.Warning {
				t.Errorf("VEND003 should be Warning, got %v", d.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected VEND003 diagnostic, got %v", diags)
	}
}

func TestVendorCheck64Bit(t *testing.T) {
	// LINT type against a profile with Supports64Bit=false emits VEND005
	varDecl := &ast.VarDecl{
		Names: []*ast.Ident{{Name: "bigNum"}},
		Type: &ast.NamedType{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindNamedType,
				NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 3, Col: 15}},
			},
			Name: &ast.Ident{Name: "LINT"},
		},
	}
	varBlock := &ast.VarBlock{
		Section:      ast.VarLocal,
		Declarations: []*ast.VarDecl{varDecl},
	}
	prog := &ast.ProgramDecl{
		NodeBase:  ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:      &ast.Ident{Name: "Main"},
		VarBlocks: []*ast.VarBlock{varBlock},
	}
	file := makeSourceFile(prog)
	table := symbols.NewTable()
	collector := diag.NewCollector()

	CheckVendorCompat([]*ast.SourceFile{file}, table, Portable, collector)

	diags := collector.All()
	found := false
	for _, d := range diags {
		if d.Code == "VEND005" {
			found = true
			if d.Severity != diag.Warning {
				t.Errorf("VEND005 should be Warning, got %v", d.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected VEND005 diagnostic, got %v", diags)
	}
}

func TestVendorCheckWString(t *testing.T) {
	// WSTRING type against portable emits VEND006
	varDecl := &ast.VarDecl{
		Names: []*ast.Ident{{Name: "ws"}},
		Type: &ast.StringType{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindStringType,
				NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 3, Col: 15}},
			},
			IsWide: true,
		},
	}
	varBlock := &ast.VarBlock{
		Section:      ast.VarLocal,
		Declarations: []*ast.VarDecl{varDecl},
	}
	prog := &ast.ProgramDecl{
		NodeBase:  ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:      &ast.Ident{Name: "Main"},
		VarBlocks: []*ast.VarBlock{varBlock},
	}
	file := makeSourceFile(prog)
	table := symbols.NewTable()
	collector := diag.NewCollector()

	CheckVendorCompat([]*ast.SourceFile{file}, table, Portable, collector)

	diags := collector.All()
	found := false
	for _, d := range diags {
		if d.Code == "VEND006" {
			found = true
			if d.Severity != diag.Warning {
				t.Errorf("VEND006 should be Warning, got %v", d.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected VEND006 diagnostic, got %v", diags)
	}
}

func TestVendorCheckStringLen(t *testing.T) {
	// STRING(300) against schneider (max 254) emits VEND004
	varDecl := &ast.VarDecl{
		Names: []*ast.Ident{{Name: "longStr"}},
		Type: &ast.StringType{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindStringType,
				NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 3, Col: 15}},
			},
			IsWide: false,
			Length: &ast.Literal{
				NodeBase: ast.NodeBase{NodeKind: ast.KindLiteral},
				Value:    "300",
				LitKind:  ast.LitInt,
			},
		},
	}
	varBlock := &ast.VarBlock{
		Section:      ast.VarLocal,
		Declarations: []*ast.VarDecl{varDecl},
	}
	prog := &ast.ProgramDecl{
		NodeBase:  ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:      &ast.Ident{Name: "Main"},
		VarBlocks: []*ast.VarBlock{varBlock},
	}
	file := makeSourceFile(prog)
	table := symbols.NewTable()
	collector := diag.NewCollector()

	CheckVendorCompat([]*ast.SourceFile{file}, table, Schneider, collector)

	diags := collector.All()
	found := false
	for _, d := range diags {
		if d.Code == "VEND004" {
			found = true
			if d.Severity != diag.Warning {
				t.Errorf("VEND004 should be Warning, got %v", d.Severity)
			}
		}
	}
	if !found {
		t.Errorf("expected VEND004 diagnostic, got %v", diags)
	}
}

func TestVendorDiagsSeverity(t *testing.T) {
	// Create a file with OOP, POINTER TO, REFERENCE TO, 64-bit, and WSTRING
	method := &ast.MethodDecl{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindMethodDecl,
			NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 5, Col: 1}},
		},
		Name: &ast.Ident{Name: "DoWork"},
	}
	fb := &ast.FunctionBlockDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindFunctionBlockDecl},
		Name:     &ast.Ident{Name: "MyFB"},
		Methods:  []*ast.MethodDecl{method},
		VarBlocks: []*ast.VarBlock{
			{
				Section: ast.VarLocal,
				Declarations: []*ast.VarDecl{
					{
						Names: []*ast.Ident{{Name: "p"}},
						Type: &ast.PointerType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindPointerType,
								NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 3, Col: 5}}},
							BaseType: &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
						},
					},
					{
						Names: []*ast.Ident{{Name: "ws"}},
						Type: &ast.StringType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindStringType,
								NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 4, Col: 5}}},
							IsWide: true,
						},
					},
				},
			},
		},
	}
	file := makeSourceFile(fb)
	table := symbols.NewTable()
	collector := diag.NewCollector()

	CheckVendorCompat([]*ast.SourceFile{file}, table, Portable, collector)

	for _, d := range collector.All() {
		if d.Severity != diag.Warning {
			t.Errorf("vendor diagnostic %s should be Warning, got %v", d.Code, d.Severity)
		}
	}
}

func TestBeckhoffNoWarnings(t *testing.T) {
	// All constructs pass without warnings on beckhoff profile
	method := &ast.MethodDecl{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindMethodDecl,
			NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 5, Col: 1}},
		},
		Name: &ast.Ident{Name: "DoWork"},
	}
	fb := &ast.FunctionBlockDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindFunctionBlockDecl},
		Name:     &ast.Ident{Name: "MyFB"},
		Methods:  []*ast.MethodDecl{method},
		VarBlocks: []*ast.VarBlock{
			{
				Section: ast.VarLocal,
				Declarations: []*ast.VarDecl{
					{
						Names: []*ast.Ident{{Name: "p"}},
						Type: &ast.PointerType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindPointerType,
								NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 3, Col: 5}}},
							BaseType: &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
						},
					},
					{
						Names: []*ast.Ident{{Name: "r"}},
						Type: &ast.ReferenceType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindReferenceType,
								NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 4, Col: 5}}},
							BaseType: &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
						},
					},
					{
						Names: []*ast.Ident{{Name: "big"}},
						Type: &ast.NamedType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindNamedType,
								NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 5, Col: 15}}},
							Name: &ast.Ident{Name: "LINT"},
						},
					},
					{
						Names: []*ast.Ident{{Name: "ws"}},
						Type: &ast.StringType{
							NodeBase: ast.NodeBase{NodeKind: ast.KindStringType,
								NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 6, Col: 5}}},
							IsWide: true,
						},
					},
				},
			},
		},
	}
	file := makeSourceFile(fb)
	table := symbols.NewTable()
	collector := diag.NewCollector()

	CheckVendorCompat([]*ast.SourceFile{file}, table, Beckhoff, collector)

	if len(collector.All()) != 0 {
		for _, d := range collector.All() {
			t.Errorf("unexpected diagnostic on Beckhoff: %s: %s", d.Code, d.Message)
		}
	}
}

// TestVendorCheckInterfaceDecl verifies INTERFACE triggers VEND001
func TestVendorCheckInterfaceDecl(t *testing.T) {
	iface := &ast.InterfaceDecl{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindInterfaceDecl,
			NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 1, Col: 1}},
		},
		Name: &ast.Ident{Name: "IRunnable"},
	}
	file := makeSourceFile(iface)
	table := symbols.NewTable()
	collector := diag.NewCollector()

	CheckVendorCompat([]*ast.SourceFile{file}, table, Schneider, collector)

	found := false
	for _, d := range collector.All() {
		if d.Code == "VEND001" && strings.Contains(d.Message, "INTERFACE") {
			found = true
		}
	}
	if !found {
		t.Error("expected VEND001 for INTERFACE on schneider")
	}
}
