package checker

import (
	"os"
	"testing"

	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func enumOf(t *testing.T, table *symbols.Table, name string) *types.EnumType {
	t.Helper()
	sym := table.LookupGlobal(name)
	require.NotNil(t, sym, "type %s not registered", name)
	et, ok := sym.Type.(*types.EnumType)
	require.True(t, ok, "type %s is %T, not an enum", name, sym.Type)
	return et
}

func TestEnumResolve(t *testing.T) {
	t.Run("explicit base type and ordinals", func(t *testing.T) {
		ds, table := runGVL(t, []gvlFile{{"t.st", "TYPE E : (a := 0, b := 1) UINT; END_TYPE\n"}})
		assert.Empty(t, errorsOf(ds))
		et := enumOf(t, table, "E")
		assert.Equal(t, types.KindUINT, et.BaseType)
		assert.Equal(t, []string{"a", "b"}, et.Values)
		assert.Equal(t, []int64{0, 1}, et.Ordinals)
		assert.False(t, et.Qualified || et.Strict || et.ToString)
	})

	t.Run("default base type INT and previous+1 numbering", func(t *testing.T) {
		ds, table := runGVL(t, []gvlFile{{"t.st", "TYPE E : (tun := 0, rdy := 2, nst); END_TYPE\n"}})
		assert.Empty(t, errorsOf(ds))
		et := enumOf(t, table, "E")
		assert.Equal(t, types.KindINT, et.BaseType)
		assert.Equal(t, []int64{0, 2, 3}, et.Ordinals)
	})

	t.Run("bit-string base type is accepted", func(t *testing.T) {
		ds, table := runGVL(t, []gvlFile{{"t.st", "TYPE E : (a := 16#FF) BYTE; END_TYPE\n"}})
		assert.Empty(t, errorsOf(ds))
		assert.Equal(t, types.KindBYTE, enumOf(t, table, "E").BaseType)
	})

	t.Run("non-integer base type reports SEMA036 and falls back to INT", func(t *testing.T) {
		ds, table := runGVL(t, []gvlFile{{"t.st", "TYPE E : (a, b) REAL; END_TYPE\n"}})
		errs := diagsWithCode(ds, CodeEnumRule)
		require.Len(t, errs, 1)
		assert.Contains(t, errs[0].Message, "enum base type must be an integer type")
		assert.Equal(t, types.KindINT, enumOf(t, table, "E").BaseType)
	})

	t.Run("ordinal out of base type range reports SEMA036", func(t *testing.T) {
		ds, _ := runGVL(t, []gvlFile{{"t.st", "TYPE E : (a := 300) USINT; END_TYPE\n"}})
		errs := diagsWithCode(ds, CodeEnumRule)
		require.Len(t, errs, 1)
		assert.Contains(t, errs[0].Message, "300")
		assert.Contains(t, errs[0].Message, "USINT")
	})

	t.Run("negative ordinal on unsigned base and large ordinal on signed base", func(t *testing.T) {
		ds, _ := runGVL(t, []gvlFile{{"t.st", "TYPE E : (a := -1) UDINT; END_TYPE\nTYPE F : (x := 40000) INT; END_TYPE\nTYPE G : (y := -129) SINT; END_TYPE\n"}})
		assert.Len(t, diagsWithCode(ds, CodeEnumRule), 3)
	})

	t.Run("ordinals in range for every integer base type", func(t *testing.T) {
		src := ""
		for _, k := range []string{"SINT", "INT", "DINT", "LINT", "USINT", "UINT", "UDINT", "ULINT", "BYTE", "WORD", "DWORD", "LWORD"} {
			src += "TYPE E_" + k + " : (a := 0, b := 127) " + k + "; END_TYPE\n"
		}
		ds, _ := runGVL(t, []gvlFile{{"t.st", src}})
		assert.Empty(t, errorsOf(ds))
	})

	t.Run("attributes set Qualified, Strict and ToString", func(t *testing.T) {
		src := "{attribute 'qualified_only'}\n{attribute 'strict'}\n{attribute 'to_string'}\nTYPE E : (a, b); END_TYPE\n"
		ds, table := runGVL(t, []gvlFile{{"t.st", src}})
		assert.Empty(t, errorsOf(ds))
		et := enumOf(t, table, "E")
		assert.True(t, et.Qualified)
		assert.True(t, et.Strict)
		assert.True(t, et.ToString)
	})

	t.Run("qualified_only values are not global, normal values are", func(t *testing.T) {
		src := "{attribute 'qualified_only'}\nTYPE EQ : (a, b); END_TYPE\nTYPE EN : (c, d); END_TYPE\n"
		_, table := runGVL(t, []gvlFile{{"t.st", src}})
		assert.Nil(t, table.GlobalScope().LookupLocal("a"))
		require.NotNil(t, table.GlobalScope().LookupLocal("c"))
		assert.Equal(t, symbols.KindEnumValue, table.GlobalScope().LookupLocal("c").Kind)
	})

	t.Run("GVL variable may reuse a qualified_only enum value name", func(t *testing.T) {
		src := "{attribute 'qualified_only'}\nTYPE EQ : (a, b); END_TYPE\n"
		gvl := "VAR_GLOBAL\n\ta : INT;\nEND_VAR\n"
		ds, _ := runGVL(t, []gvlFile{{"t.st", src}, {"GVL.st", gvl}})
		assert.Empty(t, errorsOf(ds))
	})

	t.Run("inline VAR enum registers its values in the POU scope", func(t *testing.T) {
		src := `PROGRAM P
VAR
	eStep : (E_IDLE, E_RUN) := E_IDLE;
END_VAR
IF eStep = E_RUN THEN
	eStep := E_IDLE;
END_IF
END_PROGRAM
`
		ds, table := runGVL(t, []gvlFile{{"main.st", src}})
		assert.Empty(t, errorsOf(ds))
		scope := table.LookupPOU("P")
		v := scope.LookupLocal("eStep")
		require.NotNil(t, v)
		et, ok := v.Type.(*types.EnumType)
		require.True(t, ok)
		assert.Equal(t, "P.eStep", et.Name)
		idle := scope.LookupLocal("E_IDLE")
		require.NotNil(t, idle)
		assert.Equal(t, symbols.KindEnumValue, idle.Kind)
		assert.Same(t, et, idle.Type)
		assert.Nil(t, table.GlobalScope().LookupLocal("E_IDLE"))
	})

	t.Run("inline enums in different POUs reuse value names", func(t *testing.T) {
		src := `FUNCTION_BLOCK FB_A
VAR_INPUT
	mode : (IDLE, RUN);
END_VAR
VAR
	s : (OFF, ON);
END_VAR
s := ON;
IF mode = RUN THEN s := OFF; END_IF
END_FUNCTION_BLOCK

FUNCTION F_B : INT
VAR
	s : (ON, OFF, BROKEN);
END_VAR
s := BROKEN;
F_B := 0;
END_FUNCTION
`
		ds, table := runGVL(t, []gvlFile{{"main.st", src}})
		assert.Empty(t, errorsOf(ds))
		fb := table.LookupGlobal("FB_A").Type.(*types.FunctionBlockType)
		require.Len(t, fb.Inputs, 1)
		assert.Equal(t, "FB_A.mode", fb.Inputs[0].Type.String(), "parameter type is the named inline enum")
		assert.Equal(t, []int64{0, 1, 2}, table.LookupPOU("F_B").LookupLocal("s").Type.(*types.EnumType).Ordinals)
	})

	t.Run("inline enum value clashing with a variable is a redeclaration", func(t *testing.T) {
		src := "PROGRAM P\nVAR\n\tRUN : BOOL;\n\te : (IDLE, RUN);\nEND_VAR\nEND_PROGRAM\n"
		ds, _ := runGVL(t, []gvlFile{{"main.st", src}})
		assert.Len(t, diagsWithCode(ds, CodeRedeclared), 1)
	})

	t.Run("inline enum with a bad base type reports once", func(t *testing.T) {
		src := "FUNCTION_BLOCK FB\nVAR_INPUT\n\tm : (A, B) STRING;\nEND_VAR\nEND_FUNCTION_BLOCK\n"
		ds, _ := runGVL(t, []gvlFile{{"main.st", src}})
		assert.Len(t, diagsWithCode(ds, CodeEnumRule), 1)
	})

	t.Run("enum_attr probe has no undeclared enum values", func(t *testing.T) {
		data, err := os.ReadFile("../../tests/twincat_probes/enum_attr.st")
		require.NoError(t, err)
		ds := runChecker(string(data))
		assert.Empty(t, diagsWithCode(ds, CodeUndeclared))
		assert.Empty(t, errorsOf(ds))
	})
}
