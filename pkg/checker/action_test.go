package checker

import (
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/require"
)

func runAction(t *testing.T, src string) []diag.Diagnostic {
	t.Helper()
	ds, _ := runGVL(t, []gvlFile{{"main.st", src}})
	return ds
}

func codesOf(ds []diag.Diagnostic) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Code
	}
	return out
}

const fbDrive = `FUNCTION_BLOCK FB_Drive
VAR_INPUT
	i_uETA AT %I* : UINT;
END_VAR
VAR_OUTPUT
	q_uCMD AT %Q* : UINT;
END_VAR
coe();
ACTION coe
q_uCMD := i_uETA;
END_ACTION
ACTION sdo
q_uCMD := 16#000F;
END_ACTION
END_FUNCTION_BLOCK
`

func TestAction(t *testing.T) {
	t.Run("program calls its own actions cleanly", func(t *testing.T) {
		ds := runAction(t, `PROGRAM MAIN
VAR
	x : BOOL;
	n : DINT;
END_VAR
A1();
A2();
ACTION A1
x := NOT x;
END_ACTION
ACTION A2
n := n + 1;
END_ACTION
END_PROGRAM
`)
		require.Empty(t, ds)
	})

	t.Run("action drives a TON and an R_TRIG and reads their outputs", func(t *testing.T) {
		ds := runAction(t, `PROGRAM MAIN
VAR
	x : BOOL;
	b : BOOL;
	done : BOOL;
	edge : BOOL;
	elapsed : TIME;
	t : TON;
	trig : R_TRIG;
END_VAR
A_Timers();
ACTION A_Timers
t(IN := x, PT := T#1s);
trig(CLK := b);
done := t.Q;
elapsed := t.ET;
edge := trig.Q;
END_ACTION
END_PROGRAM
`)
		require.Empty(t, ds)
	})

	t.Run("FB action drives a TON and an R_TRIG with Tc2 input names", func(t *testing.T) {
		ds := runAction(t, `FUNCTION_BLOCK FB_Debounce
VAR_INPUT
	raw : BOOL;
END_VAR
VAR_OUTPUT
	stable : BOOL;
	rising : BOOL;
END_VAR
VAR
	t : TON;
	trig : R_TRIG;
END_VAR
A_Filter();
ACTION A_Filter
t(IN := raw, PT := T#50MS);
trig(CLK := t.Q);
stable := t.Q;
rising := trig.Q;
END_ACTION
END_FUNCTION_BLOCK
PROGRAM MAIN
VAR
	d : FB_Debounce;
	y : BOOL;
END_VAR
d(raw := TRUE);
d.A_Filter();
y := d.stable AND d.rising;
END_PROGRAM
`)
		require.Empty(t, ds)
	})

	t.Run("after-POU actions resolve in the owning POU", func(t *testing.T) {
		ds := runAction(t, "PROGRAM MAIN\nVAR x : BOOL; END_VAR\nA1();\nEND_PROGRAM\nACTION A1\nx := NOT x;\nEND_ACTION\n")
		require.Empty(t, ds)
	})

	t.Run("variable used only inside an action is not unused", func(t *testing.T) {
		ds := runAction(t, "PROGRAM MAIN\nVAR\n\tonlyInAction : INT;\n\tnever : INT;\nEND_VAR\nA1();\nACTION A1\nonlyInAction := 1;\nEND_ACTION\nEND_PROGRAM\n")
		require.Equal(t, []string{CodeUnusedVar}, codesOf(ds))
		require.Contains(t, ds[0].Message, "never")
	})

	t.Run("calling an undefined action is undeclared", func(t *testing.T) {
		ds := runAction(t, "PROGRAM MAIN\nB9();\nACTION A1\nEND_ACTION\nEND_PROGRAM\n")
		require.Equal(t, []string{CodeUndeclared}, codesOf(errorsOf(ds)))
		require.Contains(t, ds[0].Message, `"B9"`)
	})

	t.Run("action named like a variable is a redeclaration", func(t *testing.T) {
		ds := runAction(t, "PROGRAM MAIN\nVAR\n\tA1 : INT;\nEND_VAR\nA1 := 1;\nACTION A1\nEND_ACTION\nEND_PROGRAM\n")
		errs := errorsOf(ds)
		require.Equal(t, []string{CodeRedeclared}, codesOf(errs))
		require.Equal(t, 6, errs[0].Pos.Line)
	})

	t.Run("undeclared identifier in an action body is reported there", func(t *testing.T) {
		ds := runAction(t, "PROGRAM MAIN\nA1();\nACTION A1\nzz := 1;\nEND_ACTION\nEND_PROGRAM\n")
		errs := errorsOf(ds)
		require.Equal(t, []string{CodeUndeclared}, codesOf(errs))
		require.Equal(t, 4, errs[0].Pos.Line)
	})

	t.Run("member call on an unknown or non-identifier root adds nothing", func(t *testing.T) {
		ds := runAction(t, "PROGRAM MAIN\nnobody.coe();\nf().coe();\nEND_PROGRAM\n")
		require.Empty(t, ds)
	})

	t.Run("inst.coe() on an FB instance is accepted", func(t *testing.T) {
		ds := runAction(t, fbDrive+"PROGRAM MAIN\nVAR\n\tinst : FB_Drive;\nEND_VAR\ninst.coe();\ninst.sdo();\nEND_PROGRAM\n")
		require.Empty(t, ds)
	})

	t.Run("FB action calls a method and a method calls an action", func(t *testing.T) {
		ds := runAction(t, `FUNCTION_BLOCK FB
VAR_INPUT
	k : DINT;
END_VAR
VAR_OUTPUT
	q : DINT;
	ok : BOOL;
END_VAR
A();
METHOD M_Reset : BOOL
VAR_INPUT
	v : DINT;
END_VAR
q := v;
A();
M_Reset := TRUE;
END_METHOD
ACTION A
ok := M_Reset(k);
q := q + 1;
END_ACTION
END_FUNCTION_BLOCK
`)
		require.Empty(t, ds)
	})

	t.Run("method called as a statement with formal arguments", func(t *testing.T) {
		ds := runAction(t, "FUNCTION_BLOCK FB\nVAR_INPUT k : DINT; END_VAR\nVAR_OUTPUT q : DINT; END_VAR\n"+
			"M(v := k, r => q);\nM(v := 1);\nM(nope := 1);\n"+
			"METHOD M : BOOL\nVAR_INPUT v : DINT; END_VAR\nVAR_OUTPUT r : DINT; END_VAR\nEND_METHOD\nEND_FUNCTION_BLOCK\n")
		errs := errorsOf(ds)
		require.Equal(t, []string{CodeNoMember}, codesOf(errs))
		require.Contains(t, errs[0].Message, `"nope"`)
		require.Empty(t, ds[1:])
	})

	t.Run("method call arguments are checked like function calls", func(t *testing.T) {
		ds := runAction(t, "FUNCTION_BLOCK FB\nVAR_OUTPUT q : BOOL; END_VAR\nq := M(1, 2);\nMETHOD M : BOOL\nEND_METHOD\nEND_FUNCTION_BLOCK\n")
		require.Equal(t, []string{CodeWrongArgCount}, codesOf(errorsOf(ds)))
	})

	t.Run("method VAR_IN_OUT counts as a call parameter", func(t *testing.T) {
		ds := runAction(t, "FUNCTION_BLOCK FB\nVAR_OUTPUT q : DINT; END_VAR\nM(q);\n"+
			"METHOD M\nVAR_IN_OUT io : DINT; END_VAR\nVAR\n\tlocal : DINT;\nEND_VAR\nEND_METHOD\nEND_FUNCTION_BLOCK\n")
		require.Empty(t, errorsOf(ds))
	})

	t.Run("method named like a variable is a redeclaration", func(t *testing.T) {
		ds := runAction(t, "FUNCTION_BLOCK FB\nVAR_OUTPUT M : BOOL; END_VAR\nM := TRUE;\nMETHOD M : BOOL\nEND_METHOD\nEND_FUNCTION_BLOCK\n")
		require.Equal(t, []string{CodeRedeclared}, codesOf(errorsOf(ds)))
	})

	t.Run("action named like a method is a redeclaration", func(t *testing.T) {
		ds := runAction(t, "FUNCTION_BLOCK FB\nMETHOD A\nEND_METHOD\nACTION A\nEND_ACTION\nEND_FUNCTION_BLOCK\n")
		require.Equal(t, []string{CodeRedeclared}, codesOf(errorsOf(ds)))
	})

	t.Run("unreachable code in an action is reported", func(t *testing.T) {
		ds := runAction(t, "PROGRAM MAIN\nVAR x : INT; END_VAR\nA1();\nACTION A1\nRETURN;\nx := 1;\nEND_ACTION\nEND_PROGRAM\n"+
			"FUNCTION_BLOCK FB\nVAR_OUTPUT q : INT; END_VAR\nACTION B\nRETURN;\nq := 1;\nEND_ACTION\nEND_FUNCTION_BLOCK\n")
		require.Equal(t, []string{CodeUnreachableCode, CodeUnreachableCode}, codesOf(ds))
	})

	t.Run("action symbol is a VOID function in the POU scope", func(t *testing.T) {
		_, table := runGVL(t, []gvlFile{{"main.st", "PROGRAM MAIN\nA1();\nACTION A1\nEND_ACTION\nEND_PROGRAM\n"}})
		sym := table.LookupPOU("MAIN").LookupLocal("A1")
		require.NotNil(t, sym)
		require.Equal(t, symbols.KindAction, sym.Kind)
		require.Equal(t, "Action", sym.Kind.String())
		fn, ok := sym.Type.(*types.FunctionType)
		require.True(t, ok)
		require.Equal(t, types.TypeVOID, fn.ReturnType)
		require.True(t, sym.Used)
	})
}
