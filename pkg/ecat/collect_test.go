package ecat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/pipeline"
)

const fixtureDir = "../../tests/ecat_fixtures"

// parseFixtures parses ST files; demo_ect.st's GVL is renamed ECT.
func parseFixtures(t *testing.T, names ...string) []*ast.SourceFile {
	t.Helper()
	var files []*ast.SourceFile
	for _, n := range names {
		p := filepath.Join(fixtureDir, n)
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		res := pipeline.Parse(p, string(src), nil)
		for _, d := range res.Diags {
			if d.Severity == diag.Error {
				t.Fatalf("parse %s: %s", n, d)
			}
		}
		if n == "demo_ect.st" {
			ast.SetGVLName(res.File, "ECT")
		}
		files = append(files, res.File)
	}
	return files
}

func parseSrc(t *testing.T, src string) []*ast.SourceFile {
	t.Helper()
	res := pipeline.Parse("inline.st", src, nil)
	for _, d := range res.Diags {
		if d.Severity == diag.Error {
			t.Fatalf("parse: %s", d)
		}
	}
	return []*ast.SourceFile{res.File}
}

func TestCollectLinksDemo(t *testing.T) {
	files := parseFixtures(t, "demo_types.st", "demo_ect.st")
	vars, diags := CollectLinks(files)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	var want []string
	for i := 1; i <= 8; i++ {
		want = append(want, fmt.Sprintf("ECT.A1_01.I%d in 1 BOOL", i))
	}
	for i := 1; i <= 8; i++ {
		want = append(want, fmt.Sprintf("ECT.A1_02.O%d out 1 BOOL", i))
	}
	want = append(want,
		"ECT.A1_03.p_stat_Enabled in 1 BOOL",
		"ECT.A1_03.p_stat_Tripped in 1 BOOL",
		"ECT.A1_03.p_Current in 16 UINT",
		"ECT.A1_03.p_cmd_Reset out 1 BOOL",
		"ECT.A1_04_Underrange in 1 BOOL",
		"ECT.CN01.q_uCMD out 16 UINT",
		"ECT.CN01.q_iLFR out 16 INT",
		"ECT.CN01.i_uETA in 16 UINT",
		"ECT.CN01.i_iRFR in 16 INT",
		"ECT.CN01.amsaddr in 64 AMSADDR",
		"ECT.V1_C1 out 8 BYTE",
		"ECT.A1_01_WcState in 1 BOOL",
		"ECT.A1_01_State in 16 WORD",
		"ECT.Dev1_DevState in 16 UINT",
		"ECT.Dev1_SlaveCount in 16 UINT",
		"ECT.Dev1_Frm0State in 16 UINT",
		"ECT.Dev1_Frm0WcState in 16 UINT",
		"ECT.Dev1_AmsNetId in 48 T_AmsNetIdArr",
		"ECT.D2_I1 in 1 BOOL",
	)
	var got []string
	for _, v := range vars {
		got = append(got, fmt.Sprintf("%s %s %d %s", v.Path, v.Dir, v.BitWidth, v.TypeName))
		if !v.HasAT {
			t.Errorf("%s: HasAT false", v.Path)
		}
		if v.Pos.Line == 0 || !strings.HasSuffix(v.Pos.File, "demo_ect.st") {
			t.Errorf("%s: bad pos %+v", v.Path, v.Pos)
		}
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	first := vars[0]
	if strings.Join(first.Steps, ".") != "ECT.A1_01.I1" {
		t.Errorf("steps = %v", first.Steps)
	}
	if first.Link != "TIID^Device 1 (EtherCAT)^DEMO.A1.00 (EK1200)^DEMO.A1.01 (EL1008)^Channel 1^Input" {
		t.Errorf("link = %q", first.Link)
	}
	if first.Pos.Line != 5 || first.Type == nil {
		t.Errorf("first pos line %d type %v", first.Pos.Line, first.Type)
	}
	if vars[len(vars)-1].Steps[0] != "ECT" || vars[21].Steps[2] != "Q_UCMD" {
		t.Errorf("steps not upper-cased: %v %v", vars[len(vars)-1].Steps, vars[21].Steps)
	}
}

func TestCollectLinksBad(t *testing.T) {
	files := parseFixtures(t, "demo_types.st", "demo_bad.st")
	vars, diags := CollectLinks(files)
	var got []string
	for _, d := range diags {
		got = append(got, fmt.Sprintf("%s:%d %s %s", filepath.Base(d.Pos.File), d.Pos.Line, d.Severity, d.Code))
	}
	want := []string{"demo_bad.st:8 error ECAT002", "demo_bad.st:22 error ECAT006"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("diags = %v, want %v", got, want)
	}
	if !strings.Contains(diags[1].Message, `missing ":="`) || !strings.Contains(diags[0].Message, "I9") {
		t.Errorf("messages: %q / %q", diags[0].Message, diags[1].Message)
	}
	if len(vars) != 5 {
		t.Errorf("got %d vars, want 5", len(vars))
	}
}

func TestCollectLinksShapes(t *testing.T) {
	src := `
TYPE E_Mode : (Off, On) DWORD; END_TYPE
TYPE E_Def : (A, B); END_TYPE
TYPE T_Alias : UDINT; END_TYPE
TYPE T_Range : INT(0..10); END_TYPE
TYPE ST_Inner : STRUCT x AT %I* : LREAL; END_STRUCT END_TYPE
TYPE ST_Outer : STRUCT sub : ST_Inner; arr : ARRAY[1..2, -1..0] OF INT; mode : E_Mode; END_STRUCT END_TYPE
TYPE T_OuterAlias : ST_Outer; END_TYPE
FUNCTION_BLOCK FB_Base
VAR baseIn AT %I* : SINT; END_VAR
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_Child EXTENDS FB_Base
VAR childOut AT %Q* : ULINT; END_VAR
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_Loop EXTENDS FB_Loop
END_FUNCTION_BLOCK
PROGRAM MAIN
VAR
	{attribute 'tclinkto' := '.sub.x := TIID^M^a; .arr := TIID^M^b; .mode := TIID^M^c'}
	o : T_OuterAlias;
	{attribute 'TcLinkTo' := '.baseIn := TIID^M^d; .childOut := TIID^M^e; .nope := TIID^M^f'}
	fb : FB_Child;
	{attribute 'TcLinkTo' := 'TIID^M^g'}
	noAt : E_Def;
	{attribute 'TcLinkTo' := 'TIID^M^h'}
	mem AT %MX0.0 : BOOL;
	{attribute 'TcLinkTo' := 'TIID^M^i'}
	s AT %I* : STRING;
	{attribute 'TcLinkTo' := 'TIID^M^j'}
	lib AT %I* : Tc2_Unknown.T_Thing;
	{attribute 'TcLinkTo' := '.x.y := TIID^M^k'}
	scalar AT %I* : BOOL;
	{attribute 'TcLinkTo' := '.zz := TIID^M^l'}
	loop : FB_Loop;
	{attribute 'TcLinkTo' := 'TIID^M^m'}
	a, b AT %I* : T_Range;
	{attribute 'Other' := 'x'}
	plain : INT;
	{attribute 'TcLinkTo' := '.sub := TIID^M^n'}
	o2 : ST_Outer;
END_VAR
END_PROGRAM
`
	vars, diags := CollectLinks(parseSrc(t, src))
	var got []string
	for _, v := range vars {
		got = append(got, fmt.Sprintf("%s %s %d %v", v.Path, v.Dir, v.BitWidth, v.HasAT))
	}
	want := []string{
		"MAIN.o.sub.x in 64 true",
		"MAIN.o.arr in 64 false",
		"MAIN.o.mode in 32 false",
		"MAIN.fb.baseIn in 8 true",
		"MAIN.fb.childOut out 64 true",
		"MAIN.noAt in 16 false",
		"MAIN.mem in 1 false",
		"MAIN.s in 0 true",
		"MAIN.lib in 0 true",
		"MAIN.a in 16 true",
		"MAIN.b in 16 true",
		"MAIN.o2.sub in 64 false",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	var codes []string
	for _, d := range diags {
		codes = append(codes, d.Code+" "+d.Severity.String())
	}
	wantCodes := []string{
		"ECAT007 warning", "ECAT007 warning", // o.arr, o.mode
		"ECAT002 error",   // fb.nope
		"ECAT007 warning", // noAt
		"ECAT007 warning", // mem (%M is not an I/O area)
		"ECAT002 error",   // scalar.x.y
		"ECAT002 error",   // loop.zz (cyclic EXTENDS terminates)
		"ECAT007 warning", // o2.sub
	}
	if strings.Join(codes, "|") != strings.Join(wantCodes, "|") {
		t.Fatalf("codes = %v, want %v", codes, wantCodes)
	}
	if vars[7].TypeName != "STRING" || vars[8].TypeName != "T_Thing" {
		t.Errorf("type names %q %q", vars[7].TypeName, vars[8].TypeName)
	}
}

func TestCollectLinksDepthCap(t *testing.T) {
	member := strings.Repeat(".a", maxMemberDepth+1)
	src := "TYPE ST_R : STRUCT a : ST_R; END_STRUCT END_TYPE\nVAR_GLOBAL\n{attribute 'TcLinkTo' := '" + member + " := TIID^M^x'}\nr : ST_R;\nEND_VAR\n"
	_, diags := CollectLinks(parseSrc(t, src))
	if len(diags) != 1 || diags[0].Code != CodeMemberNotDeclared || !strings.Contains(diags[0].Message, "too deep") {
		t.Fatalf("diags = %v", diags)
	}
}

func TestBitWidth(t *testing.T) {
	src := `
TYPE ST_Self : STRUCT me : ST_Self; END_STRUCT END_TYPE
TYPE T_Arr : ARRAY[0..2] OF BOOL; END_TYPE
TYPE T_BadArr : ARRAY[0..n] OF BYTE; END_TYPE
TYPE T_Hex : ARRAY[16#0..16#3] OF BYTE; END_TYPE
TYPE T_Ptr : POINTER TO INT; END_TYPE
TYPE T_EnumRef : E_Missing; END_TYPE
`
	idx := newTypeIndex(parseSrc(t, src))
	tests := []struct {
		name string
		want int
	}{
		{"BOOL", 1}, {"byte", 8}, {"USINT", 8}, {"WORD", 16}, {"DINT", 32}, {"UDINT", 32},
		{"DWORD", 32}, {"REAL", 32}, {"LINT", 64}, {"LWORD", 64}, {"LREAL", 64},
		{"T_AmsNetIdArr", 48}, {"AMSNETID", 48}, {"T_AmsNetId", 48}, {"AMSADDR", 64},
		{"T_Arr", 3}, {"T_Hex", 32}, {"T_BadArr", 0}, {"T_Ptr", 0}, {"T_EnumRef", 0}, {"ST_Self", 0},
		{"TIME", 0},
	}
	for _, tt := range tests {
		if got := idx.namedWidth(tt.name, 0); got != tt.want {
			t.Errorf("%s: got %d want %d", tt.name, got, tt.want)
		}
	}
	if got := idx.width(nil, 0); got != 0 {
		t.Errorf("nil spec width %d", got)
	}
}

func TestCollectHelpers(t *testing.T) {
	if vars, diags := CollectLinks([]*ast.SourceFile{nil}); vars != nil || diags != nil {
		t.Fatalf("nil file: %v %v", vars, diags)
	}
	if identName(nil) != "" {
		t.Error("identName(nil)")
	}
	cases := map[string]ast.TypeSpec{
		"WSTRING": &ast.StringType{IsWide: true},
		"ARRAY":   &ast.ArrayType{},
		"STRUCT":  &ast.StructType{},
		"ENUM":    &ast.EnumType{},
		"":        &ast.PointerType{},
	}
	for want, spec := range cases {
		if got := typeName(spec); got != want {
			t.Errorf("typeName(%T) = %q want %q", spec, got, want)
		}
	}
	idx := newTypeIndex(parseSrc(t, "TYPE T_A : T_B; END_TYPE\nTYPE T_B : T_A; END_TYPE\n"))
	if _, ok := idx.member(&ast.NamedType{Name: &ast.Ident{Name: "T_A"}}, "x"); ok {
		t.Error("alias cycle resolved a member")
	}
	if _, ok := idx.member(&ast.PointerType{}, "x"); ok {
		t.Error("pointer resolved a member")
	}
	lit := func(k ast.LiteralKind, v string) ast.Expr { return &ast.Literal{LitKind: k, Value: v} }
	for _, e := range []ast.Expr{
		lit(ast.LitReal, "1.0"), lit(ast.LitInt, "x#1"), lit(ast.LitInt, "1_000_000_000_000_000_000_000"),
		&ast.UnaryExpr{Op: ast.Token{Text: "NOT"}, Operand: lit(ast.LitInt, "1")}, nil,
	} {
		if _, ok := constInt(e); ok {
			t.Errorf("constInt(%#v) ok", e)
		}
	}
	if v, ok := constInt(&ast.UnaryExpr{Op: ast.Token{Text: "-"}, Operand: lit(ast.LitInt, "2#101")}); !ok || v != -5 {
		t.Errorf("constInt(-2#101) = %d %v", v, ok)
	}
}

// TestCollectLinksWithLibraries resolves member links through a struct
// declared only in a library (ST301's ECT terminal structs live in
// SVNCoreComponents); links inside the library are not collected.
func TestCollectLinksWithLibraries(t *testing.T) {
	lib := parseSrc(t, `TYPE ST_Term :
STRUCT
	ok AT %I* : BOOL;
END_STRUCT
END_TYPE
VAR_GLOBAL
	{attribute 'TcLinkTo' := 'TIID^Device 1 (EtherCAT)^Lib^In'}
	libVar AT %I* : BOOL;
END_VAR
`)
	files := parseSrc(t, `VAR_GLOBAL
	{attribute 'TcLinkTo' := '.ok:=TIID^Device 1 (EtherCAT)^Term 1^Status^Ok'}
	t1 : ST_Term;
END_VAR
`)
	if _, diags := CollectLinks(files); len(diags) == 0 {
		t.Fatal("without the library the member should be undeclared")
	}
	vars, diags := CollectLinksWithLibraries(files, lib)
	if len(diags) != 0 || len(vars) != 1 {
		t.Fatalf("vars %+v diags %v", vars, diags)
	}
	if vars[0].TypeName != "BOOL" || !strings.Contains(strings.ToLower(vars[0].Path), "t1.ok") {
		t.Errorf("linked var %+v", vars[0])
	}
}
