package format

import (
	"testing"

	"github.com/centroid-is/stc/pkg/parser"
	"github.com/stretchr/testify/require"
)

// TestPhase20DeclRoundTrip parses each Phase 20 declaration-level construct,
// checks the formatted text still contains it, that the output re-parses
// clean, and that formatting is idempotent.
func TestPhase20DeclRoundTrip(t *testing.T) {
	prog := func(vars string) string {
		return "PROGRAM P\nVAR\n" + vars + "\nEND_VAR\nEND_PROGRAM\n"
	}
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{"struct init", prog("fbTime : FB_LocalSystemTime := (bEnable := TRUE, dwCycle := 1);"),
			[]string{"(bEnable := TRUE, dwCycle := 1)"}},
		{"paren init", prog("x : INT := (1 + 2);"), []string{":= (1 + 2);"}},
		{"array of struct", prog("d : ARRAY[1..EcDiagParam.MAX_EC_SLAVES] OF ST_EcSlaveInfo := [\n(p_stat_sName := 'A', p_stat_nPhysAddr := 1001),\n(p_stat_sName := 'B', p_stat_nPhysAddr := 1002)\n];"),
			[]string{"ARRAY[1..EcDiagParam.MAX_EC_SLAVES]", "[(p_stat_sName := 'A', p_stat_nPhysAddr := 1001), (p_stat_sName := 'B', p_stat_nPhysAddr := 1002)]"}},
		{"repetition", prog("a : ARRAY[0..9] OF INT := [3(0), 1, 2(5)];"), []string{"[3(0), 1, 2(5)]"}},
		{"empty repetition", prog("a : ARRAY[0..9] OF ST := [3()];"), []string{"[3()]"}},
		{"simple list", prog("b : ARRAY[1..10] OF BOOL := [TRUE, FALSE, TRUE];"), []string{"[TRUE, FALSE, TRUE]"}},
		{"nested arrays", prog("m : ARRAY[0..1, 0..1] OF INT := [[1, 2], [3, 4]];"), []string{"[[1, 2], [3, 4]]"}},
		{"nested struct", prog("s : ST := (inner := (a := 1), arr := [1, 2]);"), []string{"(inner := (a := 1), arr := [1, 2])"}},
		{"bit access init", prog("x : BOOL := i_uStatusWord.0;"), []string{":= i_uStatusWord.0;"}},
		{"struct member init", "TYPE S :\nSTRUCT\n m : ST_X := (a := 1);\nEND_STRUCT\nEND_TYPE\n", []string{"m : ST_X := (a := 1);"}},
		{"global array init", "VAR_GLOBAL\n arr : ARRAY[1..2] OF ST := [(a := 1), (a := 2)];\nEND_VAR\n", []string{"[(a := 1), (a := 2)]"}},
		{"typed based literal", "PROGRAM P\nx := SHL(BYTE#16#10, nPort);\nEND_PROGRAM\n", []string{"SHL(BYTE#16#10, nPort)"}},
		{"enum base type", "TYPE E : (a := 0, b := 1) UINT; END_TYPE\n", []string{") UINT;"}},
		{"enum default", "TYPE E : (a, b) := b; END_TYPE\n", []string{") := b;"}},
		{"enum base and default", "TYPE E : (a, b) USINT := b; END_TYPE\n", []string{") USINT := b;"}},
		{"type array default", "TYPE A : ARRAY[0..2] OF INT := [1, 2, 3]; END_TYPE\n", []string{":= [1, 2, 3];"}},
		{"inline enum base", prog("x : (a, b) INT;"), []string{"(a, b) INT;"}},
		{"namespace type", prog("ec : Tc2_EtherCAT.ST_EcSlaveState;\np : POINTER TO Lib.T;"),
			[]string{"ec : Tc2_EtherCAT.ST_EcSlaveState;", "POINTER TO Lib.T;"}},
		{"stray semicolons dropped", prog("rDropPoint : REAL;;"), []string{"rDropPoint : REAL;\n"}},
	}
	opts := DefaultFormatOptions()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := parser.Parse("t.st", tc.src)
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
