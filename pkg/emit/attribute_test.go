package emit

import (
	"os"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/parser"
)

func emitClean(t *testing.T, src string, opts Options) string {
	t.Helper()
	r := parser.Parse("test.st", src)
	if len(r.Diags) > 0 {
		t.Fatalf("unexpected diagnostics: %v", r.Diags)
	}
	return Emit(r.File, opts)
}

func assertEmitInOrder(t *testing.T, out string, needles ...string) {
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

func TestEmitAttributeStructAndEnum(t *testing.T) {
	for _, name := range []string{"structpragma.st", "enum_attr.st"} {
		data, err := os.ReadFile("../../tests/twincat_probes/" + name)
		if err != nil {
			t.Fatal(err)
		}
		out := emitClean(t, string(data), DefaultOptions())
		again := emitClean(t, out, DefaultOptions())
		if again != out {
			t.Fatalf("%s: emit is not stable.\nfirst:\n%s\nsecond:\n%s", name, out, again)
		}
		switch name {
		case "structpragma.st":
			assertEmitInOrder(t, out, "STRUCT\n", "    {attribute 'OPC.UA.DA.Access' := '1'}\n    I1 : BOOL;\n")
		case "enum_attr.st":
			assertEmitInOrder(t, out,
				"{attribute 'qualified_only'}\n{attribute 'strict'}\nTYPE E_State :\n",
				"    {attribute 'OPC.UA.DA.Description' := 'Ready'}\n    rdy := 2,\n")
		}
	}
}

func TestEmitAttributeCommentAndQuoting(t *testing.T) {
	src := "PROGRAM P\nVAR\n    // c1\n    {attribute \"q\" := 'it''s'} // t\n    {warning disable C0001}\n    v : BOOL;\nEND_VAR\nEND_PROGRAM\n"
	out := emitClean(t, src, DefaultOptions())
	assertEmitInOrder(t, out,
		"VAR\n",
		"    // c1\n    {attribute 'q' := 'it''s'} // t\n    {warning disable C0001}\n    v : BOOL;\n")
}

func TestEmitAttributeAllOwners(t *testing.T) {
	src := `{attribute 'fb'}
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
END_VAR
END_PROGRAM
`
	out := emitClean(t, src, DefaultOptions())
	assertEmitInOrder(t, out,
		"{attribute 'fb'}\nFUNCTION_BLOCK FB\n",
		"{attribute 'blk'}\nVAR_INPUT\n",
		"        {attribute 'm1'}\n        a : INT;\n",
		"e : ({attribute 'ev'} e1, {region} e2);",
		"{attribute 'm'}\nMETHOD M : BOOL\n",
		"{attribute 'p'}\nPROPERTY P : INT\n",
		"{attribute 'f'}\nFUNCTION F : INT\n",
		"{attribute 'i'}\nINTERFACE I\n",
		"{attribute 'prog'}\nPROGRAM PR\n",
	)

	// Without OOP support methods, properties and interfaces are dropped
	// together with their attributes.
	opts := DefaultOptions()
	opts.Target = TargetSchneider
	out = emitClean(t, src, opts)
	for _, gone := range []string{"{attribute 'm'}", "{attribute 'p'}", "{attribute 'i'}"} {
		if strings.Contains(out, gone) {
			t.Errorf("schneider output should not contain %s:\n%s", gone, out)
		}
	}
	if !strings.Contains(out, "{attribute 'fb'}\nFUNCTION_BLOCK FB\n") {
		t.Errorf("schneider output lost FB attribute:\n%s", out)
	}
}
