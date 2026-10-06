package emit

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/parser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const refOOPSource = `VAR_GLOBAL
    gr : REFERENCE TO INT;
    g : INT;
END_VAR
FUNCTION_BLOCK FB_Base
VAR
    v : INT;
END_VAR
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_D EXTENDS FB_Base
VAR
    r : REFERENCE TO INT;
    x : INT;
END_VAR
r REF= v;
THIS^.v := 1;
SUPER^();
x := r + 1;
IF x > 0 THEN
    x := gr;
    x := 2;
END_IF;
IF THIS^.v > 0 THEN
    x := 3;
END_IF;
gr REF= g;
x := 4;
END_FUNCTION_BLOCK
PROGRAM P
VAR
    r : INT;
END_VAR
r := 5;
END_PROGRAM
`

// TestEmitUnsupportedStatements covers review ME-04: on targets without OOP
// or references, statements that use REF=, THIS^, SUPER^ or a variable whose
// declaration was dropped are replaced by a comment, so the output never
// names an undeclared variable or an unsupported construct.
func TestEmitUnsupportedStatements(t *testing.T) {
	res := parser.Parse("ref.st", refOOPSource)
	require.Empty(t, res.Diags)

	for _, target := range []Target{TargetSchneider, TargetPortable} {
		t.Run(string(target), func(t *testing.T) {
			out := Emit(res.File, Options{Target: target, UppercaseKeywords: true})
			var code []string
			for _, line := range strings.Split(out, "\n") {
				if !strings.Contains(line, "(* stc emit:") {
					code = append(code, line)
				}
			}
			for _, banned := range []string{"REF=", "THIS", "SUPER", "REFERENCE TO", "x := r", "x := gr", "gr :", "x := 3"} {
				assert.NotContains(t, strings.Join(code, "\n"), banned)
			}
			for _, kept := range []string{"x := 2;", "x := 4;", "r := 5;", "g : INT;", "x : INT;", "IF x > 0 THEN"} {
				assert.Contains(t, out, kept)
			}
			assert.Contains(t, out, "(* stc emit: statement removed, REF= is not supported by the "+string(target)+" target *)")
			assert.Contains(t, out, "(* stc emit: statement removed, THIS^ is not supported by the "+string(target)+" target *)")
			assert.Contains(t, out, "(* stc emit: statement removed, SUPER^ is not supported by the "+string(target)+" target *)")
			assert.Contains(t, out, "(* stc emit: statement removed, r is not declared for the "+string(target)+" target *)")
			assert.Contains(t, out, "(* stc emit: statement removed, gr is not declared for the "+string(target)+" target *)")
			assert.Equal(t, 7, strings.Count(out, "stc emit: statement removed"), out)
		})
	}

	t.Run("beckhoff keeps everything", func(t *testing.T) {
		out := Emit(res.File, DefaultOptions())
		for _, kept := range []string{"r REF= v;", "THIS^.v := 1;", "SUPER^();", "x := r + 1;", "gr REF= g;"} {
			assert.Contains(t, out, kept)
		}
		assert.NotContains(t, out, "statement removed")
	})
}

// TestEmitUnsupportedStatementKinds checks that every statement kind's own
// expressions are inspected for a dropped variable on a reduced target.
func TestEmitUnsupportedStatementKinds(t *testing.T) {
	src := `PROGRAM P
VAR
    p : POINTER TO INT;
    big : LINT;
    i : INT;
    fb : TON;
END_VAR
CASE i OF
    1: i := 2;
END_CASE;
CASE p^ OF
    1: i := 2;
END_CASE;
FOR i := 0 TO big DO
    i := i;
END_FOR;
FOR i := 0 TO 3 BY 1 DO
    i := i;
END_FOR;
WHILE big > 0 DO
    i := 1;
END_WHILE;
REPEAT
    i := 1;
UNTIL big > 0
END_REPEAT;
fb(IN := big > 0);
fb(IN := TRUE);
IF i = 0 THEN
    i := 1;
ELSIF p^ = 1 THEN
    i := 2;
END_IF;
RETURN;
END_PROGRAM
`
	res := parser.Parse("kinds.st", src)
	require.Empty(t, res.Diags)
	out := Emit(res.File, Options{Target: TargetPortable, UppercaseKeywords: true})
	assert.Equal(t, 2, strings.Count(out, "p is not declared for the portable target"), out)
	assert.Equal(t, 4, strings.Count(out, "big is not declared for the portable target"), out)
	for _, kept := range []string{"CASE i OF", "FOR i := 0 TO 3 BY 1 DO", "fb(IN := TRUE);", "RETURN;"} {
		assert.Contains(t, out, kept)
	}
	// Schneider keeps 64-bit types, so only the pointer statements go.
	out = Emit(res.File, Options{Target: TargetSchneider, UppercaseKeywords: true})
	assert.Equal(t, 2, strings.Count(out, "statement removed"), out)
}
