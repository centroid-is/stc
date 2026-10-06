package format

import (
	"os"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/parser"
)

// formatClean parses src, fails on diagnostics, and formats it.
func formatClean(t *testing.T, src string) string {
	t.Helper()
	r := parser.Parse("test.st", src)
	if len(r.Diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", r.Diags)
	}
	return Format(r.File, DefaultFormatOptions())
}

// assertIdempotent formats out again and requires identical bytes.
func assertIdempotent(t *testing.T, out string) {
	t.Helper()
	again := formatClean(t, out)
	if again != out {
		t.Fatalf("format is not idempotent.\nfirst:\n%s\nsecond:\n%s", out, again)
	}
}

// assertInOrder requires every needle to appear in out, in the given order.
func assertInOrder(t *testing.T, out string, needles ...string) {
	t.Helper()
	pos := 0
	for _, n := range needles {
		i := strings.Index(out[pos:], n)
		if i < 0 {
			t.Fatalf("expected %q after offset %d in:\n%s", n, pos, out)
		}
		pos += i + len(n)
	}
}

func readProbe(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../tests/twincat_probes/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestFormatAttributeStructMember(t *testing.T) {
	out := formatClean(t, readProbe(t, "structpragma.st"))
	want := "TYPE ST_X :\nSTRUCT\n    {attribute 'OPC.UA.DA.Access' := '1'}\n    I1 : BOOL;\n    I2 : BOOL;\nEND_STRUCT\nEND_TYPE\n"
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	assertIdempotent(t, out)
}

func TestFormatAttributeEnum(t *testing.T) {
	out := formatClean(t, readProbe(t, "enum_attr.st"))
	want := "{attribute 'qualified_only'}\n{attribute 'strict'}\nTYPE E_State :\n(\n    {attribute 'OPC.UA.DA.Description' := 'Ready'}\n    rdy := 2,\n    nst := 3\n);\nEND_TYPE\n"
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	assertIdempotent(t, out)
}

func TestFormatAttributeCommentOrder(t *testing.T) {
	src := "PROGRAM P\nVAR\n    // c1\n    {attribute 'x' := 'y'} // t\n    // c2\n    v : BOOL;\nEND_VAR\nEND_PROGRAM\n"
	out := formatClean(t, src)
	want := "PROGRAM P\nVAR\n    // c1\n    {attribute 'x' := 'y'} // t\n    // c2\n    v : BOOL;\nEND_VAR\nEND_PROGRAM\n"
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	assertIdempotent(t, out)
}

func TestFormatAttributeCanonicalQuoting(t *testing.T) {
	src := "{attribute \"qualified_only\"}\n{attribute 'd' := 'it''s'}\n{warning disable C0001}\nPROGRAM P\nEND_PROGRAM\n"
	out := formatClean(t, src)
	// A double-quoted name keeps its double quotes: TwinCAT ignores such
	// attributes, and rewriting the quotes would activate them.
	want := "{attribute \"qualified_only\"}\n{attribute 'd' := 'it''s'}\n{warning disable C0001}\nPROGRAM P\nEND_PROGRAM\n"
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
	assertIdempotent(t, out)
}

func TestFormatAttributeAllOwners(t *testing.T) {
	src := `// top
{attribute 'fb'}
{region}
FUNCTION_BLOCK FB
{attribute 'blk'}
VAR_INPUT
    v : BOOL;
END_VAR
VAR
    s : STRUCT
        {attribute 'm1'}
        a : INT;
    END_STRUCT;
    e : ({attribute 'ev'} e1, {region} e2);
END_VAR
{attribute 'm'}
METHOD M : BOOL
M := TRUE;
END_METHOD
{attribute 'p'}
PROPERTY P : INT
END_PROPERTY
END_FUNCTION_BLOCK

{attribute 'f'}
FUNCTION F : INT
F := 1;
END_FUNCTION

{attribute 'i'}
INTERFACE I
END_INTERFACE

{attribute 'prog'}
PROGRAM PR
VAR
    x : INT;
    {attribute 'tail'}
END_VAR
END_PROGRAM
`
	out := formatClean(t, src)
	assertInOrder(t, out,
		"// top\n{attribute 'fb'}\n{region}\nFUNCTION_BLOCK FB\n",
		"{attribute 'blk'}\nVAR_INPUT\n",
		"        {attribute 'm1'}\n        a : INT;\n",
		"e : ({attribute 'ev'} e1, {region} e2);",
		"\n{attribute 'm'}\nMETHOD M : BOOL\n",
		"\n{attribute 'p'}\nPROPERTY P : INT\n",
		"\n{attribute 'f'}\nFUNCTION F : INT\n",
		"\n{attribute 'i'}\nINTERFACE I\n",
		"\n{attribute 'prog'}\nPROGRAM PR\nVAR\n    x : INT;\n    {attribute 'tail'}\nEND_VAR\n",
	)
	assertIdempotent(t, out)
}

// TestFormatEndPragmasKeepTheirAnchor checks that a pragma written just
// before END_VAR, END_STRUCT or an enum's ")" is printed there again, not
// before VAR or above the last member: moving {warning restore} or
// {endregion} changes which lines it covers.
func TestFormatEndPragmasKeepTheirAnchor(t *testing.T) {
	cases := []struct {
		name, src string
		order     []string
	}{
		{"var block", "PROGRAM P\nVAR\n    {warning disable C0195}\n    x : INT;\n    {warning restore C0195}\nEND_VAR\nEND_PROGRAM\n",
			[]string{"VAR\n", "{warning disable C0195}", "x : INT;", "{warning restore C0195}", "END_VAR"}},
		{"struct", "TYPE S :\nSTRUCT\n    {region 'r'}\n    a : INT;\n    b : INT;\n    {endregion}\nEND_STRUCT\nEND_TYPE\n",
			[]string{"STRUCT\n", "{region 'r'}", "a : INT;", "b : INT;", "{endregion}", "END_STRUCT"}},
		{"enum", "TYPE E :\n(\n    {region}\n    a,\n    b\n    {endregion}\n);\nEND_TYPE\n",
			[]string{"(\n", "{region}", "a,", "b\n", "{endregion}", ");"}},
		{"inline struct in a var", "PROGRAM P\nVAR\n    s : STRUCT\n        a : INT;\n        {endregion}\n    END_STRUCT;\nEND_VAR\nEND_PROGRAM\n",
			[]string{"STRUCT\n", "a : INT;", "{endregion}", "END_STRUCT"}},
		{"inline enum in a var", "PROGRAM P\nVAR\n    e : (a, b {attribute 'tail'});\nEND_VAR\nEND_PROGRAM\n",
			[]string{"(a, b {attribute 'tail'})"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := formatClean(t, tc.src)
			assertInOrder(t, out, tc.order...)
			assertIdempotent(t, out)
		})
	}
}

// TestFormatInterfaceSignatures checks that interface METHOD and PROPERTY
// signatures keep their attributes, var blocks and END_ keywords, so the
// output re-parses and {attribute 'TcRpcEnable'} survives a reformat.
func TestFormatInterfaceSignatures(t *testing.T) {
	src := "INTERFACE I_X\n{attribute 'TcRpcEnable'}\nMETHOD M : BOOL\nVAR_INPUT\n    a : INT;\nEND_VAR\nEND_METHOD\n{attribute 'monitoring' := 'call'}\nPROPERTY P : INT\nEND_PROPERTY\n{warning restore C0195}\nEND_INTERFACE\n"
	out := formatClean(t, src)
	assertInOrder(t, out,
		"INTERFACE I_X\n",
		"{attribute 'TcRpcEnable'}\n", "METHOD M : BOOL\n", "VAR_INPUT\n", "a : INT;", "END_VAR\n", "END_METHOD\n",
		"{attribute 'monitoring' := 'call'}\n", "PROPERTY P : INT\n", "END_PROPERTY\n",
		"{warning restore C0195}\n", "END_INTERFACE\n")
	assertIdempotent(t, out)
}
