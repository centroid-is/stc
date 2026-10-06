package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const fbEmptyCall = "FUNCTION_BLOCK FB_T\nVAR_INPUT\n\ta : INT;\n\tb : INT;\nEND_VAR\nVAR_OUTPUT\n\tq : BOOL;\nEND_VAR\nq := a > b;\nEND_FUNCTION_BLOCK\n"

// emptyCallProg wraps body in a PROGRAM with an FB_T instance fb, INT n,
// INT y and BOOL r, all of which are referenced so only the call matters.
func emptyCallProg(vars, body string) string {
	return fbEmptyCall + "PROGRAM P\nVAR\n\tfb : FB_T;\n\tn : INT;\n" + vars + "END_VAR\n" + body + "\nEND_PROGRAM\n"
}

func TestEmptyFBCall(t *testing.T) {
	tests := []struct {
		name  string
		vars  string
		body  string
		codes []string // expected error codes in order
	}{
		{"empty call at top level", "", "fb();\nn := n;", nil},
		{"empty call in CASE arm", "", "CASE n OF\n1: fb();\nEND_CASE", nil},
		{"empty call in IF branch", "", "IF n > 0 THEN\n\tfb();\nEND_IF", nil},
		{"positional first mixed args", "", "fb(1, b := n);", nil},
		{"positional first mixed args unknown name", "", "fb(1, nope := n);", []string{CodeNoMember}},
		{"unknown named input", "", "fb(nope := 1);\nn := n;", []string{CodeNoMember}},
		{"INT variable still not callable", "\tx : INT;\n", "x();\nn := n;", []string{CodeNotCallable}},
		{"undeclared FB type reports only SEMA037", "\tbad : FB_Missing;\n", "bad();\nbad(n);\nn := n;", []string{CodeUndeclaredType}},
		{"FB instance as value still not callable", "\ty : INT;\n", "y := fb();\nn := n;", []string{CodeNotCallable}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ds := runAction(t, emptyCallProg(tc.vars, tc.body))
			assert.Equal(t, tc.codes, nilIfEmpty(codesOf(errorsOf(ds))), "%v", ds)
		})
	}

	t.Run("instance and positional argument count as used", func(t *testing.T) {
		ds := runAction(t, emptyCallProg("\tm : INT;\n", "fb(m, b := n);"))
		require.Empty(t, ds, "%v", ds)
	})
}

func nilIfEmpty(s []string) []string {
	if len(s) == 0 {
		return nil
	}
	return s
}
