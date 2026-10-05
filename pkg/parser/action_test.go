package parser

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/stretchr/testify/require"
)

func actionNames(acts []*ast.ActionDecl) []string {
	out := make([]string, len(acts))
	for i, a := range acts {
		out[i] = a.Name.Name
	}
	return out
}

func hasDiag(diags []diag.Diagnostic, substr string) bool {
	for _, d := range diags {
		if strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}

const orphanMsg = "ACTION without a preceding PROGRAM or FUNCTION_BLOCK"

func TestAction(t *testing.T) {
	t.Run("action.st attaches the after-POU action to MAIN", func(t *testing.T) {
		r := Parse("action.st", readProbe(t, "action.st"))
		require.Empty(t, r.Diags)
		require.Len(t, r.File.Declarations, 1)
		prog := r.File.Declarations[0].(*ast.ProgramDecl)
		require.Equal(t, "MAIN", prog.Name.Name)
		require.Len(t, prog.Body, 2)
		require.Equal(t, []string{"A1"}, actionNames(prog.Actions))
		require.Equal(t, ast.KindActionDecl, prog.Actions[0].Kind())
		require.Len(t, prog.Actions[0].Body, 1)
		_, isAssign := prog.Actions[0].Body[0].(*ast.AssignStmt)
		require.True(t, isAssign)
	})

	t.Run("action_inside.st attaches inside actions to MAIN and FB_Drive", func(t *testing.T) {
		r := Parse("action_inside.st", readProbe(t, "action_inside.st"))
		require.Empty(t, r.Diags)
		require.Len(t, r.File.Declarations, 2)
		prog := r.File.Declarations[0].(*ast.ProgramDecl)
		require.Equal(t, []string{"A050_ModbusCall", "A100_input"}, actionNames(prog.Actions))
		require.Len(t, prog.Body, 2)
		a050 := prog.Actions[0]
		require.Len(t, a050.Pragmas, 1)
		require.Equal(t, "{warning disable C0139}", a050.Pragmas[0].Text)
		require.Len(t, a050.Body, 1)
		require.Len(t, prog.Actions[1].Body, 2)

		fb := r.File.Declarations[1].(*ast.FunctionBlockDecl)
		require.Equal(t, "FB_Drive", fb.Name.Name)
		require.Len(t, fb.Methods, 1)
		require.Equal(t, []string{"coe", "sdo"}, actionNames(fb.Actions))
		require.Len(t, fb.Body, 1)
	})

	t.Run("colon and semicolon after the action name both parse", func(t *testing.T) {
		src := "PROGRAM P\nVAR x : INT; END_VAR\nA1();\nACTION A1:\nx := 1;\nEND_ACTION\nACTION A2;\nx := 2;\nEND_ACTION;\nEND_PROGRAM\n"
		r := Parse("p.st", src)
		require.Empty(t, r.Diags)
		prog := r.File.Declarations[0].(*ast.ProgramDecl)
		require.Equal(t, []string{"A1", "A2"}, actionNames(prog.Actions))
		require.Len(t, prog.Body, 1)
	})

	t.Run("action after END_FUNCTION_BLOCK attaches to the FB", func(t *testing.T) {
		src := "FUNCTION_BLOCK FB\nVAR x : INT; END_VAR\nA();\nEND_FUNCTION_BLOCK\nACTION A\nx := 1;\nEND_ACTION\n"
		r := Parse("fb.st", src)
		require.Empty(t, r.Diags)
		require.Len(t, r.File.Declarations, 1)
		fb := r.File.Declarations[0].(*ast.FunctionBlockDecl)
		require.Equal(t, []string{"A"}, actionNames(fb.Actions))
	})

	t.Run("action at file start is an orphan", func(t *testing.T) {
		r := Parse("o.st", "ACTION A\nx := 1;\nEND_ACTION\n")
		require.True(t, hasDiag(r.Diags, orphanMsg))
		require.Len(t, r.File.Declarations, 1)
		a := r.File.Declarations[0].(*ast.ActionDecl)
		require.Equal(t, "A", a.Name.Name)
		require.Equal(t, 1, r.Diags[0].Pos.Line)
	})

	t.Run("action after a FUNCTION is an orphan", func(t *testing.T) {
		src := "FUNCTION F : INT\nF := 1;\nEND_FUNCTION\nACTION A\nEND_ACTION\n"
		r := Parse("o.st", src)
		require.True(t, hasDiag(r.Diags, orphanMsg))
		require.Len(t, r.File.Declarations, 2)
		_, ok := r.File.Declarations[1].(*ast.ActionDecl)
		require.True(t, ok)
	})

	t.Run("action after a TYPE is an orphan", func(t *testing.T) {
		src := "TYPE E : (a, b); END_TYPE\nACTION A\nEND_ACTION\n"
		r := Parse("o.st", src)
		require.True(t, hasDiag(r.Diags, orphanMsg))
		_, ok := r.File.Declarations[len(r.File.Declarations)-1].(*ast.ActionDecl)
		require.True(t, ok)
	})

	t.Run("two POUs each followed by their own actions", func(t *testing.T) {
		src := "PROGRAM P1\nEND_PROGRAM\nACTION A\nEND_ACTION\nACTION B\nEND_ACTION\n" +
			"FUNCTION_BLOCK F1\nEND_FUNCTION_BLOCK\nACTION C\nEND_ACTION\n"
		r := Parse("two.st", src)
		require.Empty(t, r.Diags)
		require.Len(t, r.File.Declarations, 2)
		require.Equal(t, []string{"A", "B"}, actionNames(r.File.Declarations[0].(*ast.ProgramDecl).Actions))
		require.Equal(t, []string{"C"}, actionNames(r.File.Declarations[1].(*ast.FunctionBlockDecl).Actions))
	})

	t.Run("missing END_ACTION before END_PROGRAM is a diagnostic", func(t *testing.T) {
		src := "PROGRAM P\nVAR x : INT; END_VAR\nACTION A\nx := 1;\nEND_PROGRAM\nPROGRAM Q\nEND_PROGRAM\n"
		r := Parse("m.st", src)
		require.True(t, hasDiag(r.Diags, "expected KwEndAction"))
		require.Len(t, r.File.Declarations, 2)
		prog := r.File.Declarations[0].(*ast.ProgramDecl)
		require.Equal(t, "P", prog.Name.Name)
		require.Equal(t, []string{"A"}, actionNames(prog.Actions))
		require.Len(t, prog.Actions[0].Body, 1)
		require.Equal(t, "Q", r.File.Declarations[1].(*ast.ProgramDecl).Name.Name)
	})

	t.Run("attribute before a top-level action attaches to it", func(t *testing.T) {
		src := "PROGRAM P\nEND_PROGRAM\n{attribute 'hide'}\nACTION A\nEND_ACTION\n"
		r := Parse("a.st", src)
		require.Empty(t, r.Diags)
		prog := r.File.Declarations[0].(*ast.ProgramDecl)
		require.Len(t, prog.Actions, 1)
		require.Equal(t, []string{"hide"}, attrNames(prog.Actions[0].Attributes))
		require.Empty(t, prog.Attributes)
	})

	t.Run("attribute before an inside action attaches to it", func(t *testing.T) {
		src := "PROGRAM P\nA();\n{attribute 'hide'}\nACTION A\nEND_ACTION\nEND_PROGRAM\n" +
			"FUNCTION_BLOCK F\n{attribute 'x'}\nACTION B\nEND_ACTION\nEND_FUNCTION_BLOCK\n"
		r := Parse("a.st", src)
		require.Empty(t, r.Diags)
		prog := r.File.Declarations[0].(*ast.ProgramDecl)
		require.Equal(t, []string{"hide"}, attrNames(prog.Actions[0].Attributes))
		fb := r.File.Declarations[1].(*ast.FunctionBlockDecl)
		require.Equal(t, []string{"x"}, attrNames(fb.Actions[0].Attributes))
	})

	t.Run("statement-level pragmas in a PROGRAM body are dropped", func(t *testing.T) {
		src := "PROGRAM P\nVAR x : INT; END_VAR\nx := 1;\n{warning 'w'}\nx := 2;\nEND_PROGRAM\n"
		r := Parse("p.st", src)
		require.Empty(t, r.Diags)
		prog := r.File.Declarations[0].(*ast.ProgramDecl)
		require.Len(t, prog.Body, 2)
		require.Empty(t, prog.Actions)
	})

	t.Run("pragma inside an action body after statements is skipped", func(t *testing.T) {
		src := "PROGRAM P\nVAR x : INT; END_VAR\nACTION A\nx := 1;\n{warning 'w'}\nx := 2;\nEND_ACTION\nEND_PROGRAM\n"
		r := Parse("p.st", src)
		require.Empty(t, r.Diags)
		prog := r.File.Declarations[0].(*ast.ProgramDecl)
		require.Len(t, prog.Actions[0].Body, 2)
	})

	// Adversarial inputs (T-19-14): each must finish without hanging.
	t.Run("unterminated action at EOF", func(t *testing.T) {
		r := Parse("u.st", "PROGRAM P\nEND_PROGRAM\nACTION A\nx := 1;\n")
		require.True(t, hasDiag(r.Diags, "expected KwEndAction"))
		require.Len(t, r.File.Declarations[0].(*ast.ProgramDecl).Actions, 1)
	})

	t.Run("ACTION ACTION ACTION", func(t *testing.T) {
		r := Parse("u.st", "ACTION ACTION ACTION")
		require.NotEmpty(t, r.Diags)
		for _, d := range r.File.Declarations {
			_, ok := d.(*ast.ActionDecl)
			require.True(t, ok)
		}
	})

	t.Run("ACTION at EOF", func(t *testing.T) {
		r := Parse("u.st", "PROGRAM P\nEND_PROGRAM\nACTION")
		require.NotEmpty(t, r.Diags)
		require.Len(t, r.File.Declarations[0].(*ast.ProgramDecl).Actions, 1)
	})

	t.Run("ACTION ACTION inside a PROGRAM", func(t *testing.T) {
		r := Parse("u.st", "PROGRAM P\nACTION ACTION ACTION\nEND_PROGRAM\n")
		require.NotEmpty(t, r.Diags)
		require.Len(t, r.File.Declarations, 1)
		require.Len(t, r.File.Declarations[0].(*ast.ProgramDecl).Actions, 3)
	})

	t.Run("unterminated action does not swallow the next POU", func(t *testing.T) {
		r := Parse("u.st", "ACTION A\nx := 1;\nFUNCTION F : INT\nEND_FUNCTION\n")
		require.True(t, hasDiag(r.Diags, "expected KwEndAction"))
		require.Len(t, r.File.Declarations, 2)
		_, ok := r.File.Declarations[1].(*ast.FunctionDecl)
		require.True(t, ok)
	})

	t.Run("action span covers ACTION to END_ACTION", func(t *testing.T) {
		r := Parse("action.st", readProbe(t, "action.st"))
		a := r.File.Declarations[0].(*ast.ProgramDecl).Actions[0]
		require.Equal(t, 9, a.Span().Start.Line)
		require.Equal(t, 11, a.Span().End.Line)
	})
}
