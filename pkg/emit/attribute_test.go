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
		"    // c1\n    {attribute \"q\" := \"it's\"} // t\n    {warning disable C0001}\n    v : BOOL;\n")
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

// TestEmitEndPragmasKeepTheirAnchor mirrors the format test: pragmas before
// END_VAR, END_STRUCT and an enum's ")" are emitted in place.
func TestEmitEndPragmasKeepTheirAnchor(t *testing.T) {
	cases := []struct {
		name, src string
		order     []string
	}{
		{"var block", "PROGRAM P\nVAR\n    {warning disable C0195}\n    x : INT;\n    {warning restore C0195}\nEND_VAR\nEND_PROGRAM\n",
			[]string{"VAR\n", "{warning disable C0195}", "x : INT;", "{warning restore C0195}", "END_VAR"}},
		{"struct", "TYPE S :\nSTRUCT\n    a : INT;\n    {endregion}\nEND_STRUCT\nEND_TYPE\n",
			[]string{"STRUCT\n", "a : INT;", "{endregion}", "END_STRUCT"}},
		{"enum", "TYPE E :\n(\n    a,\n    b\n    {endregion}\n);\nEND_TYPE\n",
			[]string{"(\n", "a,", "b\n", "{endregion}", ");"}},
		{"inline struct in a var", "PROGRAM P\nVAR\n    s : STRUCT\n        a : INT;\n        {endregion}\n    END_STRUCT;\nEND_VAR\nEND_PROGRAM\n",
			[]string{"STRUCT\n", "a : INT;", "{endregion}", "END_STRUCT"}},
		{"inline enum in a var", "PROGRAM P\nVAR\n    e : (a, b {attribute 'tail'});\nEND_VAR\nEND_PROGRAM\n",
			[]string{"(a, b {attribute 'tail'})"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := emitClean(t, tc.src, DefaultOptions())
			assertEmitInOrder(t, out, tc.order...)
			if again := emitClean(t, out, DefaultOptions()); again != out {
				t.Fatalf("emit is not idempotent.\nfirst:\n%s\nsecond:\n%s", out, again)
			}
		})
	}
}

func TestEmitInterfaceSignatures(t *testing.T) {
	src := "INTERFACE I_X\n{attribute 'TcRpcEnable'}\nMETHOD M : BOOL\nVAR_INPUT\n    a : INT;\nEND_VAR\nEND_METHOD\n{attribute 'monitoring' := 'call'}\nPROPERTY P : INT\nEND_PROPERTY\n{warning restore C0195}\nEND_INTERFACE\n"
	out := emitClean(t, src, DefaultOptions())
	assertEmitInOrder(t, out,
		"INTERFACE I_X\n",
		"{attribute 'TcRpcEnable'}\n", "METHOD M : BOOL\n", "VAR_INPUT\n", "a : INT;", "END_VAR\n", "END_METHOD\n",
		"{attribute 'monitoring' := 'call'}\n", "PROPERTY P : INT\n", "END_PROPERTY\n",
		"{warning restore C0195}\n", "END_INTERFACE\n")
	if again := emitClean(t, out, DefaultOptions()); again != out {
		t.Fatalf("emit is not idempotent.\nfirst:\n%s\nsecond:\n%s", out, again)
	}
}
