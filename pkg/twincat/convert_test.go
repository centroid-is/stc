package twincat

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/lexer"
	"github.com/centroid-is/stc/pkg/pipeline"
)

const pouDir = "testdata/sln/Demo/Demo/POUs"

func convertOK(t *testing.T, path, rel string, mode Mode) (Converted, []diag.Diagnostic) {
	t.Helper()
	c, ds, err := ConvertFile(path, rel, mode)
	if err != nil {
		t.Fatalf("ConvertFile(%s): %v", path, err)
	}
	return c, ds
}

func parseClean(t *testing.T, path, text string) *ast.SourceFile {
	t.Helper()
	res := pipeline.Parse(path, text, nil)
	if len(res.Diags) != 0 {
		t.Fatalf("parse diags for %s:\n%v\n--- text ---\n%s", path, res.Diags, text)
	}
	return res.File
}

func TestConvertFBLayout(t *testing.T) {
	path := filepath.Join(pouDir, "FB_Motor.TcPOU")
	c, ds := convertOK(t, path, "POUs/FB_Motor.TcPOU", ModeLayout)
	if len(ds) != 0 {
		t.Errorf("unexpected diags %v", ds)
	}
	if c.Kind != KindPOU || c.Name != "FB_Motor" {
		t.Errorf("kind/name = %s/%s", c.Kind, c.Name)
	}
	f := parseClean(t, path, c.Text)
	if len(f.Declarations) != 1 {
		t.Fatalf("want 1 declaration, got %d", len(f.Declarations))
	}
	fb, ok := f.Declarations[0].(*ast.FunctionBlockDecl)
	if !ok {
		t.Fatalf("got %T", f.Declarations[0])
	}
	if len(fb.Methods) != 2 || len(fb.Actions) != 1 || len(fb.Properties) != 1 {
		t.Fatalf("methods/actions/properties = %d/%d/%d", len(fb.Methods), len(fb.Actions), len(fb.Properties))
	}
	if p := fb.Properties[0]; p.Getter == nil || p.Setter == nil {
		t.Errorf("property accessors missing: %+v", p)
	}
	for _, kw := range []string{"END_FUNCTION_BLOCK", "ACTION Reset:", "END_ACTION", "END_METHOD", "GET", "END_GET", "SET", "END_SET", "END_PROPERTY"} {
		if !strings.Contains(c.Text, kw) {
			t.Errorf("missing %q", kw)
		}
	}
}

// TestConvertPositions checks that the first non-blank character of every
// CDATA segment lands at its XML line and column in the converted text.
func TestConvertPositions(t *testing.T) {
	for _, name := range []string{"FB_Motor.TcPOU", "MAIN.TcPOU", "I_Motor.TcIO", "F_Add.TcPOU"} {
		path := filepath.Join(pouDir, name)
		raw := mustRead(t, path)
		c, _ := convertOK(t, path, "POUs/"+name, ModeLayout)
		assertSegmentPositions(t, path, raw, c.Text)
	}
}

func assertSegmentPositions(t *testing.T, path string, raw []byte, text string) {
	t.Helper()
	toks := lexer.Tokenize(path, text)
	at := map[[2]int]string{}
	for _, tk := range toks {
		at[[2]int{tk.Pos.Line, tk.Pos.Col}] = tk.Text
	}
	marker := []byte("<![CDATA[")
	checked := 0
	for i := 0; ; {
		j := bytes.Index(raw[i:], marker)
		if j < 0 {
			break
		}
		start := i + j + len(marker)
		k := start
		for k < len(raw) && (raw[k] == ' ' || raw[k] == '\t' || raw[k] == '\n' || raw[k] == '\r') {
			k++
		}
		i = start
		if bytes.HasPrefix(raw[k:], []byte("]]>")) {
			continue // empty segment
		}
		line, col := lineCol(raw, int64(k))
		tok, ok := at[[2]int{line, col}]
		if !ok || tok == "" || tok[0] != raw[k] {
			t.Errorf("%s: no token %q at %d:%d (got %q)", path, raw[k], line, col, tok)
		}
		checked++
	}
	if checked == 0 {
		t.Errorf("%s: no segments checked", path)
	}
}

func TestConvertEndKeywords(t *testing.T) {
	cases := []struct {
		file, end string
		kind      Kind
	}{
		{"MAIN.TcPOU", "END_PROGRAM", KindPOU},
		{"F_Add.TcPOU", "END_FUNCTION", KindPOU},
		{"I_Motor.TcIO", "END_INTERFACE", KindITF},
	}
	for _, tc := range cases {
		path := filepath.Join(pouDir, tc.file)
		c, _ := convertOK(t, path, "POUs/"+tc.file, ModeLayout)
		parseClean(t, path, c.Text)
		if c.Kind != tc.kind || !strings.Contains(c.Text, tc.end) {
			t.Errorf("%s: kind %s, text:\n%s", tc.file, c.Kind, c.Text)
		}
	}
	c, _ := convertOK(t, filepath.Join(pouDir, "I_Motor.TcIO"), "POUs/I_Motor.TcIO", ModeLayout)
	if !strings.Contains(c.Text, "END_METHOD") || !strings.Contains(c.Text, "END_PROPERTY") || c.Name != "I_Motor" {
		t.Errorf("interface text:\n%s", c.Text)
	}
}

func TestPouEndKeyword(t *testing.T) {
	cases := map[string]string{
		"{attribute 'x'}\n(* c *)\n// l\nFUNCTION_BLOCK FB_A": "END_FUNCTION_BLOCK",
		"PROGRAM P":                   "END_PROGRAM",
		"FUNCTION F : INT":            "END_FUNCTION",
		"INTERFACE I":                 "END_INTERFACE",
		"VAR_GLOBAL x : INT; END_VAR": "",
		"":                            "",
	}
	for decl, want := range cases {
		if got := pouEndKeyword("x.st", decl); got != want {
			t.Errorf("pouEndKeyword(%q) = %q, want %q", decl, got, want)
		}
	}
}

func TestConvertGVLAndDUT(t *testing.T) {
	gvl := "testdata/sln/Demo/Demo/GVLs/GVL_Main.TcGVL"
	c, _ := convertOK(t, gvl, "GVLs/GVL_Main.TcGVL", ModeLayout)
	if c.Kind != KindGVL || c.Name != "GVL_Main" {
		t.Errorf("gvl kind/name %s/%s", c.Kind, c.Name)
	}
	parseClean(t, gvl, c.Text)
	assertSegmentPositions(t, gvl, mustRead(t, gvl), c.Text)

	dut := "testdata/sln/Demo/Demo/DUTs/ST_Point.TcDUT"
	c, _ = convertOK(t, dut, "DUTs/ST_Point.TcDUT", ModeLayout)
	if c.Kind != KindDUT || c.Name != "ST_Point" {
		t.Errorf("dut kind/name %s/%s", c.Kind, c.Name)
	}
	parseClean(t, dut, c.Text)
	if !strings.Contains(c.Text, "TYPE ST_Point :\nSTRUCT\n\tx : INT;\n\ty : INT;\nEND_STRUCT\nEND_TYPE") {
		t.Errorf("DUT text changed:\n%s", c.Text)
	}
}

func TestConvertCompact(t *testing.T) {
	path := filepath.Join(pouDir, "FB_Motor.TcPOU")
	c, _ := convertOK(t, path, "POUs/FB_Motor.TcPOU", ModeCompact)
	if !strings.HasPrefix(c.Text, "// source: POUs/FB_Motor.TcPOU\n{attribute 'reflection'}\n") {
		t.Errorf("compact header:\n%s", c.Text)
	}
	if strings.Contains(c.Text, "\n\n\n") {
		t.Errorf("compact text has layout padding:\n%s", c.Text)
	}
	f := parseClean(t, path, c.Text)
	fb := f.Declarations[0].(*ast.FunctionBlockDecl)
	if len(fb.Methods) != 2 || len(fb.Actions) != 1 || len(fb.Properties) != 1 {
		t.Errorf("compact lost members")
	}
}

func TestConvertDecl(t *testing.T) {
	path := filepath.Join(pouDir, "FB_Motor.TcPOU")
	c, _ := convertOK(t, path, "POUs/FB_Motor.TcPOU", ModeDecl)
	f := parseClean(t, path, c.Text)
	for _, want := range []string{"FUNCTION_BLOCK FB_Motor", "METHOD PUBLIC Start : BOOL\nVAR_INPUT\n\tforce : BOOL;\nEND_VAR", "METHOD Stop", "END_METHOD", "PROPERTY PUBLIC Speed : INT", "GET", "END_GET", "SET", "END_SET", "END_PROPERTY", "END_FUNCTION_BLOCK"} {
		if !strings.Contains(c.Text, want) {
			t.Errorf("decl mode missing %q:\n%s", want, c.Text)
		}
	}
	for _, bad := range []string{"startCount := startCount + 1", "running := enable", "ACTION", "speedSet := Speed", "wasRunning := running"} {
		if strings.Contains(c.Text, bad) {
			t.Errorf("decl mode contains body text %q:\n%s", bad, c.Text)
		}
	}
	fb := f.Declarations[0].(*ast.FunctionBlockDecl)
	if len(fb.Methods) != 2 || len(fb.Actions) != 0 || len(fb.Properties) != 1 || len(fb.Body) != 0 {
		t.Errorf("decl AST: methods %d actions %d props %d body %d", len(fb.Methods), len(fb.Actions), len(fb.Properties), len(fb.Body))
	}
}

func TestConvertNonST(t *testing.T) {
	path := "testdata/convert/FB_Fbd.TcPOU"
	c, ds := convertOK(t, path, "FB_Fbd.TcPOU", ModeLayout)
	d := findCode(ds, CodeSkipped)
	if d == nil || !strings.Contains(d.Message, "FBD") || d.Pos.Line != 10 {
		t.Errorf("want VEND022 for FBD at line 10: %v", ds)
	}
	parseClean(t, path, c.Text)
	if !strings.Contains(c.Text, "FUNCTION_BLOCK FB_Fbd") || !strings.Contains(c.Text, "END_FUNCTION_BLOCK") {
		t.Errorf("declaration not kept:\n%s", c.Text)
	}
}

func TestConvertLayoutOverlap(t *testing.T) {
	// A lone CR inside CDATA becomes a newline after XML decoding, so the
	// decoded text spans more lines than the raw file; the next segment
	// must fall back to the next free line.
	raw := []byte("<TcPlcObject>\n<POU Name=\"P\">\n<Declaration><![CDATA[PROGRAM P\rVAR\rx : INT;\rEND_VAR]]></Declaration>\n<Implementation><ST><![CDATA[x := 1;]]></ST></Implementation>\n</POU>\n</TcPlcObject>\n")
	c, ds, err := Convert("/v/P.TcPOU", "P.TcPOU", raw, ModeLayout)
	if err != nil {
		t.Fatal(err)
	}
	d := findCode(ds, CodeLayoutOverlap)
	if d == nil || d.Severity != diag.Warning || d.Pos.Line != 4 {
		t.Errorf("want VEND023 at line 4: %v", ds)
	}
	parseClean(t, "/v/P.TcPOU", c.Text)
}

func TestLayoutPut(t *testing.T) {
	var l layout
	if !l.put(3, 5, "a\nb") || !l.put(4, 4, "c") || l.put(4, 2, "d") || l.put(2, 1, "e") {
		t.Fatal("unexpected put results")
	}
	if got, want := l.String(), "\n\n    a\nb  c\nd\ne\n"; got != want {
		t.Errorf("layout = %q, want %q", got, want)
	}
}

func TestConvertCRLFBOM(t *testing.T) {
	for _, name := range []string{"FB_Motor.TcPOU", "MAIN.TcPOU", "I_Motor.TcIO"} {
		path := filepath.Join(pouDir, name)
		lf := mustRead(t, path)
		a, _, err := Convert(path, name, lf, ModeLayout)
		if err != nil {
			t.Fatal(err)
		}
		b, ds, err := Convert(path, name, crlfBOM(lf), ModeLayout)
		if err != nil {
			t.Fatal(err)
		}
		if len(ds) != 0 || a.Text != b.Text || a.Kind != b.Kind || a.Name != b.Name {
			t.Errorf("%s: CRLF+BOM differs (diags %v)\n%q\n%q", name, ds, a.Text, b.Text)
		}
		parseClean(t, path, b.Text)
	}
}

func TestConvertErrors(t *testing.T) {
	if _, _, err := ConvertFile(filepath.Join(t.TempDir(), "x.TcPOU"), "x.TcPOU", ModeLayout); err == nil {
		t.Error("missing file: want error")
	}
	var xe *BadXMLError
	if _, _, err := Convert("/v/t.TcPOU", "t.TcPOU", []byte("<TcPlcObject><POU Name=\"A\"><Declaration><![CDATA[PROGRAM A"), ModeLayout); !errors.As(err, &xe) {
		t.Errorf("truncated: %v", err)
	}
	if _, _, err := ConvertFile("testdata/sln/Demo/Demo/PlcTask.TcTTO", "PlcTask.TcTTO", ModeLayout); err == nil || !strings.Contains(err.Error(), "Task") {
		t.Errorf("TcTTO: %v", err)
	}
	if _, _, err := Convert("/v/e.TcPOU", "e.TcPOU", []byte("<TcPlcObject></TcPlcObject>"), ModeLayout); err == nil {
		t.Error("empty object: want error")
	}
}
