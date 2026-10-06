package checker

import (
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sema037 returns the SEMA037 diagnostics in ds.
func sema037(ds []diag.Diagnostic) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, d := range ds {
		if d.Code == CodeUndeclaredType {
			out = append(out, d)
		}
	}
	return out
}

func TestUndeclaredType(t *testing.T) {
	t.Run("PROGRAM variable, no cascade on uses", func(t *testing.T) {
		ds := runAction(t, "PROGRAM P\nVAR\n\tx : FB_DoesNotExist;\n\tn : INT;\nEND_VAR\nx.y := 1;\nx(a := n);\nn := x.z;\nEND_PROGRAM\n")
		errs := errorsOf(ds)
		require.Len(t, errs, 1, "%v", errs)
		assert.Equal(t, CodeUndeclaredType, errs[0].Code)
		assert.Equal(t, "undeclared type 'FB_DoesNotExist'", errs[0].Message)
		assert.Equal(t, 3, errs[0].Pos.Line)
		assert.Empty(t, ds[1:], "no unused-variable cascade either: %v", ds)
	})

	t.Run("FB and FUNCTION inputs report once", func(t *testing.T) {
		src := "FUNCTION_BLOCK FB_A\nVAR_INPUT\n\tx : FB_DoesNotExist;\nEND_VAR\nEND_FUNCTION_BLOCK\n" +
			"FUNCTION F : INT\nVAR_INPUT\n\ty : FB_Missing2;\nEND_VAR\nF := 1;\nEND_FUNCTION\n" +
			"FUNCTION_BLOCK FB_M\nMETHOD M : FB_Missing3\nVAR_INPUT\n\tz : FB_Missing4;\nEND_VAR\nEND_METHOD\nEND_FUNCTION_BLOCK\n"
		got := sema037(runAction(t, src))
		require.Len(t, got, 4, "%v", got)
		assert.Equal(t, "undeclared type 'FB_DoesNotExist'", got[0].Message)
		assert.Equal(t, "undeclared type 'FB_Missing2'", got[1].Message)
	})

	t.Run("self-referential and mutually recursive aliases", func(t *testing.T) {
		got := sema037(runAction(t, "TYPE T : T; END_TYPE\n"))
		require.Len(t, got, 1)
		assert.Equal(t, "undeclared type 'T'", got[0].Message)

		got = sema037(runAction(t, "TYPE U : V; END_TYPE\nTYPE V : U; END_TYPE\n"))
		assert.Len(t, got, 1)
	})

	t.Run("alias of an undeclared type reports at the alias only", func(t *testing.T) {
		src := "PROGRAM P\nVAR\n\ta : A;\n\tb : B;\nEND_VAR\na := b;\nEND_PROGRAM\n" +
			"TYPE A : Missing; END_TYPE\nTYPE B : ARRAY[0..1] OF Missing; END_TYPE\n"
		ds := errorsOf(runAction(t, src))
		require.Len(t, ds, 2, "%v", ds)
		for _, d := range ds {
			assert.Equal(t, "undeclared type 'Missing'", d.Message)
		}
	})

	t.Run("interfaces are declared types", func(t *testing.T) {
		src := "INTERFACE I_Before\nEND_INTERFACE\nPROGRAM P\nVAR\n\ti : I_Motor;\n\tj : I_Before;\nEND_VAR\nEND_PROGRAM\nINTERFACE I_Motor\nEND_INTERFACE\n"
		assert.Empty(t, sema037(runAction(t, src)))
	})

	t.Run("namespace-qualified type", func(t *testing.T) {
		decl := "TYPE ST_EcSlaveState :\nSTRUCT\n\tdeviceState : BYTE;\nEND_STRUCT\nEND_TYPE\n"
		prog := "PROGRAM P\nVAR\n\tec : Tc2_EtherCAT.ST_EcSlaveState;\n\tb : BYTE;\n\tt : Tc2_Standard.TON;\nEND_VAR\nb := ec.deviceState;\nt(IN := TRUE);\nEND_PROGRAM\n"

		ds := runAction(t, prog+decl)
		assert.Empty(t, errorsOf(ds), "user declaration")

		ds, _ = runGVL(t, []gvlFile{{"main.st", prog}}, ResolveOpts{LibraryFiles: parseGVLFiles(t, []gvlFile{{"lib.st", decl}})})
		assert.Empty(t, errorsOf(ds), "library declaration")

		got := sema037(runAction(t, prog))
		require.Len(t, got, 1)
		assert.Equal(t, "undeclared type 'Tc2_EtherCAT.ST_EcSlaveState'", got[0].Message)
	})

	t.Run("library-supplied types never report", func(t *testing.T) {
		lib := "FUNCTION_BLOCK ADSREAD\nVAR_INPUT\n\tNETID : T_AmsNetId;\n\tother : T_NotInAnyStub;\nEND_VAR\nEND_FUNCTION_BLOCK\n" +
			"TYPE T_AmsNetId : STRING(23); END_TYPE\n"
		prog := "PROGRAM P\nVAR\n\tr : ADSREAD;\n\tid : T_AmsNetId;\nEND_VAR\nr(NETID := id);\nEND_PROGRAM\n"
		ds, _ := runGVL(t, []gvlFile{{"main.st", prog}}, ResolveOpts{LibraryFiles: parseGVLFiles(t, []gvlFile{{"lib.st", lib}})})
		assert.Empty(t, errorsOf(ds), "names inside library stubs are not reported either")
	})

	t.Run("POINTER TO, REFERENCE TO and EXTENDS", func(t *testing.T) {
		src := "PROGRAM P\nVAR\n\tp : POINTER TO Missing1;\n\tr : REFERENCE TO Missing2;\nEND_VAR\nEND_PROGRAM\n" +
			"FUNCTION_BLOCK FB_Derived EXTENDS FB_Missing\nEND_FUNCTION_BLOCK\n"
		got := sema037(runAction(t, src))
		require.Len(t, got, 3, "%v", got)
		assert.Equal(t, "undeclared type 'Missing1'", got[0].Message)
		assert.Equal(t, "undeclared type 'Missing2'", got[1].Message)
		assert.Equal(t, "undeclared type 'FB_Missing'", got[2].Message)
	})

	t.Run("elementary and standard FB names", func(t *testing.T) {
		src := "PROGRAM P\nVAR\n\ta : LTIME;\n\tb : LDATE;\n\tc : LTOD;\n\td : LTIME_OF_DAY;\n\te : LDT;\n\tf : LDATE_AND_TIME;\n" +
			"\tt1 : TON;\n\tt2 : TOF;\n\tt3 : TP;\n\tc1 : CTU;\n\tc2 : CTD;\n\tc3 : CTUD;\n\tr1 : R_TRIG;\n\tr2 : F_TRIG;\n\ts1 : SR;\n\ts2 : RS;\nEND_VAR\nEND_PROGRAM\n" +
			"FUNCTION F : VOID\nEND_FUNCTION\n"
		assert.Empty(t, sema037(runAction(t, src)))
		for name, kind := range map[string]types.TypeKind{
			"LTIME": types.KindTIME, "LDATE": types.KindDATE, "LTOD": types.KindTOD, "LTIME_OF_DAY": types.KindTOD,
			"LDT": types.KindDT, "LDATE_AND_TIME": types.KindDT, "VOID": types.KindVoid,
		} {
			typ, ok := types.LookupElementaryType(name)
			require.True(t, ok, name)
			assert.Equal(t, kind, typ.Kind(), name)
		}
	})

	t.Run("call with an undeclared callee type still checks argument values", func(t *testing.T) {
		ds := runAction(t, "PROGRAM P\nVAR\n\tx : FB_DoesNotExist;\n\tn : INT;\nEND_VAR\nx(a := n, b := zz);\nEND_PROGRAM\n")
		assert.ElementsMatch(t, []string{CodeUndeclaredType, CodeUndeclared}, codesOf(ds))
	})
}
