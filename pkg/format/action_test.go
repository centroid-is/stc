package format

import (
	"os"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/stretchr/testify/require"
)

func fmtActionSrc(t *testing.T, src string) string {
	t.Helper()
	r := parser.Parse("a.st", src)
	require.Empty(t, r.Diags)
	return Format(r.File, DefaultFormatOptions())
}

func readActionProbe(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("../../tests/twincat_probes/" + name)
	require.NoError(t, err)
	return string(data)
}

func TestAction(t *testing.T) {
	t.Run("action.st prints the action after END_PROGRAM", func(t *testing.T) {
		out := fmtActionSrc(t, readActionProbe(t, "action.st"))
		require.Contains(t, out, "A1();\nEND_PROGRAM\n\nACTION A1\n    x := NOT x;\nEND_ACTION\n")
		require.NotContains(t, out, ":= ;")
	})

	t.Run("action_inside.st prints actions after each POU", func(t *testing.T) {
		out := fmtActionSrc(t, readActionProbe(t, "action_inside.st"))
		require.Equal(t, 4, strings.Count(out, "\nACTION "))
		require.Contains(t, out, "END_PROGRAM\n\nACTION A050_ModbusCall\n    {warning disable C0139}\n    n := n + 1;\nEND_ACTION\n")
		require.Contains(t, out, "END_FUNCTION_BLOCK\n\nACTION coe\n")
		require.Less(t, strings.Index(out, "END_FUNCTION_BLOCK"), strings.Index(out, "ACTION sdo"))
	})

	t.Run("attribute prints above ACTION", func(t *testing.T) {
		out := fmtActionSrc(t, "PROGRAM P\nEND_PROGRAM\n{attribute 'hide'}\nACTION A\nEND_ACTION\n")
		require.Contains(t, out, "END_PROGRAM\n\n{attribute 'hide'}\nACTION A\nEND_ACTION\n")
	})

	t.Run("fmt output is idempotent and keeps action counts", func(t *testing.T) {
		first := fmtActionSrc(t, readActionProbe(t, "action_inside.st"))
		r := parser.Parse("b.st", first)
		require.Empty(t, r.Diags)
		require.Len(t, r.File.Declarations[0].(*ast.ProgramDecl).Actions, 2)
		require.Len(t, r.File.Declarations[1].(*ast.FunctionBlockDecl).Actions, 2)
		require.Equal(t, first, Format(r.File, DefaultFormatOptions()))
	})

	t.Run("expression statement prints without assignment", func(t *testing.T) {
		out := fmtActionSrc(t, "PROGRAM P\nfb.A1();\nEND_PROGRAM\n")
		require.Contains(t, out, "fb.A1();\n")
	})
}
