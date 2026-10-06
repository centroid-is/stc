package format

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
)

const emptyArgSrc = "PROGRAM P\nVAR\n\tb : BOOL;\n\tt : TON;\nEND_VAR\nt(IN := b, PT := , Q => , ET => );\nEND_PROGRAM\n"

func TestFormatStructAT(t *testing.T) {
	data := readProbe(t, "structat.st")
	out := formatClean(t, data)
	if strings.Count(out, "I1 AT %I* : BOOL;") != 1 {
		t.Fatalf("expected struct member AT in output:\n%s", out)
	}
	assertInOrder(t, out, "{attribute 'OPC.UA.DA.Access' := '1'}", "I1 AT %I* : BOOL;", "I2 : BOOL;")
	assertIdempotent(t, out)

	out = formatClean(t, "TYPE S :\nSTRUCT\n\ta AT %Q* : INT := 5;\nEND_STRUCT\nEND_TYPE\n")
	if !strings.Contains(out, "a AT %Q* : INT := 5;") {
		t.Fatalf("AT with init value lost:\n%s", out)
	}
	assertIdempotent(t, out)
}

func TestFormatEmptyArg(t *testing.T) {
	out := formatClean(t, emptyArgSrc)
	if !strings.Contains(out, "t(IN := b, PT :=, Q =>, ET =>);") {
		t.Fatalf("empty args not printed:\n%s", out)
	}
	assertIdempotent(t, out)

	// Re-parsing the printed text gives the same AST shape.
	r := parser.Parse("again.st", out)
	if len(r.Diags) > 0 {
		t.Fatalf("reparse diagnostics: %v", r.Diags)
	}
	cs := r.File.Declarations[0].(*ast.ProgramDecl).Body[0].(*ast.CallStmt)
	if len(cs.Args) != 4 || cs.Args[0].Value == nil {
		t.Fatalf("unexpected args after reparse: %+v", cs.Args)
	}
	for _, a := range cs.Args[1:] {
		if a.Value != nil {
			t.Fatalf("arg %s should stay empty", a.Name.Name)
		}
	}
}
