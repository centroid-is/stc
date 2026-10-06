package lint

import (
	"sort"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/stretchr/testify/assert"
)

func p20LintId(name string) *ast.Ident { return &ast.Ident{Name: name} }

func p20LintInt(v string) *ast.Literal { return &ast.Literal{LitKind: ast.LitInt, Value: v} }

// visitedIdents walks a statement and returns the sorted identifier names seen.
func visitedIdents(stmt ast.Statement) []string {
	var names []string
	walkExprsInStmt(stmt, func(e ast.Expr) {
		if id, ok := e.(*ast.Ident); ok {
			names = append(names, id.Name)
		}
	})
	sort.Strings(names)
	return names
}

func TestPhase20Nodes(t *testing.T) {
	t.Run("bit access target and constant index", func(t *testing.T) {
		stmt := &ast.AssignStmt{Target: p20LintId("q"),
			Value: &ast.BitAccessExpr{Target: p20LintId("w"), Index: p20LintId("C_BIT")}}
		assert.Equal(t, []string{"C_BIT", "q", "w"}, visitedIdents(stmt))
	})

	t.Run("bit index literal is not a magic number", func(t *testing.T) {
		var lits []string
		stmt := &ast.AssignStmt{Target: p20LintId("q"),
			Value: &ast.BitAccessExpr{Target: p20LintId("w"), Index: p20LintInt("7")}}
		walkExprsInStmt(stmt, func(e ast.Expr) {
			if l, ok := e.(*ast.Literal); ok {
				lits = append(lits, l.Value)
			}
		})
		assert.Empty(t, lits)
	})

	t.Run("named call args", func(t *testing.T) {
		call := &ast.CallExpr{Callee: p20LintId("F"), Args: []ast.Expr{p20LintId("p")},
			NamedArgs: []*ast.CallArg{
				{Name: p20LintId("a"), Value: p20LintId("v1")},
				{Name: p20LintId("b"), Value: p20LintId("v2"), IsOutput: true},
				{Name: p20LintId("c")}, // empty argument
				nil,
			}}
		stmt := &ast.AssignStmt{Target: p20LintId("n"), Value: call}
		assert.Equal(t, []string{"F", "n", "p", "v1", "v2"}, visitedIdents(stmt))
	})

	t.Run("ref assign", func(t *testing.T) {
		stmt := &ast.RefAssignStmt{Target: p20LintId("r"), Value: p20LintId("x")}
		assert.Equal(t, []string{"r", "x"}, visitedIdents(stmt))
	})

	t.Run("this and super", func(t *testing.T) {
		stmt := &ast.AssignStmt{
			Target: &ast.MemberAccessExpr{Object: &ast.DerefExpr{Operand: &ast.ThisExpr{}}, Member: p20LintId("x")},
			Value:  &ast.CallExpr{Callee: &ast.MemberAccessExpr{Object: &ast.DerefExpr{Operand: &ast.SuperExpr{}}, Member: p20LintId("M")}},
		}
		assert.NotPanics(t, func() { visitedIdents(stmt) })
	})

	t.Run("initialisers", func(t *testing.T) {
		init := &ast.ArrayInit{Elements: []*ast.ArrayInitElem{
			{Value: p20LintId("v1")},
			{Count: p20LintId("N"), Value: p20LintId("v2")},
			{Value: &ast.StructInit{Fields: []*ast.FieldInit{{Name: p20LintId("f"), Value: p20LintId("v3")}, nil}}},
			nil,
		}}
		stmt := &ast.AssignStmt{Target: p20LintId("arr"), Value: init}
		assert.Equal(t, []string{"N", "arr", "v1", "v2", "v3"}, visitedIdents(stmt))
	})

	t.Run("magic numbers inside new nodes are reported", func(t *testing.T) {
		code := `PROGRAM Main
VAR r : REFERENCE TO INT; x : INT; END_VAR
    x := 1;
END_PROGRAM`
		// Parser support lands in 20-02; splice nodes into a parsed program.
		res := parser.Parse("test.st", code)
		assert.Empty(t, res.Diags)
		file := res.File
		prog := file.Declarations[0].(*ast.ProgramDecl)
		prog.Body = append(prog.Body,
			&ast.AssignStmt{Target: p20LintId("x"), Value: &ast.CallExpr{Callee: p20LintId("F"),
				NamedArgs: []*ast.CallArg{{Name: p20LintId("a"), Value: p20LintInt("42")}}}},
			&ast.RefAssignStmt{Target: p20LintId("r"), Value: p20LintInt("43")},
		)
		diags := checkMagicNumbers(file)
		count := 0
		for _, d := range diags {
			if d.Code == CodeMagicNumber {
				count++
			}
		}
		assert.Equal(t, 2, count)
	})
}
