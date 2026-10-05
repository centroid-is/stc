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
	want := "{attribute 'qualified_only'}\n{attribute 'd' := 'it''s'}\n{warning disable C0001}\nPROGRAM P\nEND_PROGRAM\n"
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
GET
P := 1;
END_GET
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
		"\n{attribute 'prog'}\n{attribute 'tail'}\nVAR\n",
	)
	assertIdempotent(t, out)
}
