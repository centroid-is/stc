package format

import (
	"testing"

	"github.com/centroid-is/stc/pkg/parser"
	"github.com/stretchr/testify/require"
)

// TestPhase20ExprRoundTrip parses each Phase 20 expression-level construct,
// checks the formatted text still contains it, that the output re-parses
// clean, and that formatting is idempotent.
func TestPhase20ExprRoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		stmts string
		want  []string
	}{
		{"bit read", "b := w.3;", []string{"b := w.3;"}},
		{"bit write over index", "Modbus.arr[0].3 := TRUE;", []string{"Modbus.arr[0].3 := TRUE;"}},
		{"bit member chain", "x := ECT.X.q_wDigitalInputs.0 OR w.1;", []string{"ECT.X.q_wDigitalInputs.0 OR w.1"}},
		{"named args", "n := F_X(a := 1, b := 2);", []string{"F_X(a := 1, b := 2)"}},
		{"mixed args", "n := f(1, b := 2, c => y);", []string{"f(1, b := 2, c => y)"}},
		{"positional after named", "n := f(a := 1, x, c := 3);", []string{"f(a := 1, x, c := 3)"}},
		{"trailing comma dropped", "n := f(a := 1, q => y,\n);", []string{"f(a := 1, q => y)"}},
		{"ref assign", "r REF= arr[i].x;", []string{"r REF= arr[i].x;"}},
		{"this member", "THIS^.x := 1;", []string{"THIS^.x := 1;"}},
		{"this method", "s := THIS^.SendBytes(pData := ADR(str), nLen := LEN(str));", []string{"THIS^.SendBytes(pData := ADR(str), nLen := LEN(str))"}},
		{"super calls", "SUPER^.M();\nSUPER^();", []string{"SUPER^.M();", "SUPER^();"}},
		{"qualified case labels", "CASE e OF\nlft_e.no_fault:\n  n := 1;\nE.a, E.b:\n  n := 2;\nE.a..E.c:\n  n := 3;\nEND_CASE", []string{"lft_e.no_fault:", "E.a, E.b:", "E.a..E.c:"}},
	}
	opts := DefaultFormatOptions()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := "PROGRAM P\n" + tc.stmts + "\nEND_PROGRAM\n"
			r := parser.Parse("t.st", src)
			require.Empty(t, r.Diags, "source must parse clean")
			once := Format(r.File, opts)
			for _, w := range tc.want {
				require.Contains(t, once, w)
			}
			r2 := parser.Parse("t.st", once)
			require.Empty(t, r2.Diags, "formatted output must re-parse clean:\n%s", once)
			require.Equal(t, once, Format(r2.File, opts), "fmt must be idempotent")
		})
	}
}
