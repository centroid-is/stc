package checker

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
	"github.com/centroid-is/stc/pkg/symbols"
)

// Helper to build a scope with variables for usage tests.
func setupUsageScope(t *testing.T, table *symbols.Table, pouName string, vars []struct {
	name    string
	section ast.VarSection
	used    bool
}) *symbols.Scope {
	t.Helper()
	scope := table.RegisterPOU(pouName, symbols.KindProgram, source.Pos{File: "test.st", Line: 1, Col: 1})
	for i, v := range vars {
		sym := &symbols.Symbol{
			Name:     v.name,
			Kind:     symbols.KindVariable,
			Pos:      source.Pos{File: "test.st", Line: i + 3, Col: 5},
			ParamDir: v.section,
			Used:     v.used,
		}
		if err := scope.Insert(sym); err != nil {
			t.Fatalf("inserting %s: %v", v.name, err)
		}
	}
	return scope
}

func TestUnusedVarDetected(t *testing.T) {
	// PROGRAM with declared but unused variable emits SEMA012 warning
	table := symbols.NewTable()
	setupUsageScope(t, table, "Main", []struct {
		name    string
		section ast.VarSection
		used    bool
	}{
		{"unusedVar", ast.VarLocal, false},
		{"usedVar", ast.VarLocal, true},
	})

	collector := diag.NewCollector()
	CheckUsage(nil, table, collector)

	diags := collector.All()
	found := false
	for _, d := range diags {
		if d.Code == CodeUnusedVar {
			found = true
			if d.Severity != diag.Warning {
				t.Errorf("SEMA012 should be Warning, got %v", d.Severity)
			}
		}
	}
	if !found {
		t.Error("expected SEMA012 for unused variable")
	}
}

func TestUsedVarNoWarning(t *testing.T) {
	// PROGRAM with declared and used variable emits no SEMA012
	table := symbols.NewTable()
	setupUsageScope(t, table, "Main", []struct {
		name    string
		section ast.VarSection
		used    bool
	}{
		{"usedVar", ast.VarLocal, true},
	})

	collector := diag.NewCollector()
	CheckUsage(nil, table, collector)

	for _, d := range collector.All() {
		if d.Code == CodeUnusedVar {
			t.Errorf("unexpected SEMA012 for used variable: %s", d.Message)
		}
	}
}

func TestUnusedVarInput(t *testing.T) {
	// VAR_INPUT variables that are unused should NOT be warned
	table := symbols.NewTable()
	setupUsageScope(t, table, "Main", []struct {
		name    string
		section ast.VarSection
		used    bool
	}{
		{"inputVar", ast.VarInput, false},
	})

	collector := diag.NewCollector()
	CheckUsage(nil, table, collector)

	for _, d := range collector.All() {
		if d.Code == CodeUnusedVar {
			t.Errorf("VAR_INPUT should not trigger SEMA012: %s", d.Message)
		}
	}
}

func TestUnusedVarOutput(t *testing.T) {
	// VAR_OUTPUT variables that are unused should NOT be warned
	table := symbols.NewTable()
	setupUsageScope(t, table, "Main", []struct {
		name    string
		section ast.VarSection
		used    bool
	}{
		{"outputVar", ast.VarOutput, false},
	})

	collector := diag.NewCollector()
	CheckUsage(nil, table, collector)

	for _, d := range collector.All() {
		if d.Code == CodeUnusedVar {
			t.Errorf("VAR_OUTPUT should not trigger SEMA012: %s", d.Message)
		}
	}
}

func TestUnusedVarInOut(t *testing.T) {
	// VAR_IN_OUT variables that are unused should NOT be warned
	table := symbols.NewTable()
	setupUsageScope(t, table, "Main", []struct {
		name    string
		section ast.VarSection
		used    bool
	}{
		{"inOutVar", ast.VarInOut, false},
	})

	collector := diag.NewCollector()
	CheckUsage(nil, table, collector)

	for _, d := range collector.All() {
		if d.Code == CodeUnusedVar {
			t.Errorf("VAR_IN_OUT should not trigger SEMA012: %s", d.Message)
		}
	}
}

func TestUnusedVarGlobal(t *testing.T) {
	// VAR_GLOBAL variables that are unused should NOT be warned
	table := symbols.NewTable()
	setupUsageScope(t, table, "Main", []struct {
		name    string
		section ast.VarSection
		used    bool
	}{
		{"globalVar", ast.VarGlobal, false},
	})

	collector := diag.NewCollector()
	CheckUsage(nil, table, collector)

	for _, d := range collector.All() {
		if d.Code == CodeUnusedVar {
			t.Errorf("VAR_GLOBAL should not trigger SEMA012: %s", d.Message)
		}
	}
}

func TestUnreachableAfterReturn(t *testing.T) {
	// Code after RETURN statement emits SEMA013 warning
	body := []ast.Statement{
		&ast.AssignStmt{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindAssignStmt,
				NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 4, Col: 1}},
			},
			Target: &ast.Ident{Name: "x"},
			Value:  &ast.Literal{Value: "1", LitKind: ast.LitInt},
		},
		&ast.ReturnStmt{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindReturnStmt,
				NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 5, Col: 1}},
			},
		},
		&ast.AssignStmt{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindAssignStmt,
				NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 6, Col: 1}},
			},
			Target: &ast.Ident{Name: "y"},
			Value:  &ast.Literal{Value: "2", LitKind: ast.LitInt},
		},
	}

	prog := &ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     &ast.Ident{Name: "Main"},
		Body:     body,
	}
	file := makeSourceFile(prog)

	table := symbols.NewTable()
	collector := diag.NewCollector()
	CheckUsage([]*ast.SourceFile{file}, table, collector)

	found := false
	for _, d := range collector.All() {
		if d.Code == CodeUnreachableCode {
			found = true
			if d.Severity != diag.Warning {
				t.Errorf("SEMA013 should be Warning, got %v", d.Severity)
			}
		}
	}
	if !found {
		t.Error("expected SEMA013 for unreachable code after RETURN")
	}
}

func TestUnreachableAfterExit(t *testing.T) {
	// Code after EXIT statement in a loop emits SEMA013 warning
	loopBody := []ast.Statement{
		&ast.ExitStmt{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindExitStmt,
				NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 5, Col: 5}},
			},
		},
		&ast.AssignStmt{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindAssignStmt,
				NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 6, Col: 5}},
			},
			Target: &ast.Ident{Name: "x"},
			Value:  &ast.Literal{Value: "1", LitKind: ast.LitInt},
		},
	}
	forStmt := &ast.ForStmt{
		NodeBase: ast.NodeBase{
			NodeKind: ast.KindForStmt,
			NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 4, Col: 1}},
		},
		Variable: &ast.Ident{Name: "i"},
		From:     &ast.Literal{Value: "0", LitKind: ast.LitInt},
		To:       &ast.Literal{Value: "10", LitKind: ast.LitInt},
		Body:     loopBody,
	}

	prog := &ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     &ast.Ident{Name: "Main"},
		Body:     []ast.Statement{forStmt},
	}
	file := makeSourceFile(prog)

	table := symbols.NewTable()
	collector := diag.NewCollector()
	CheckUsage([]*ast.SourceFile{file}, table, collector)

	found := false
	for _, d := range collector.All() {
		if d.Code == CodeUnreachableCode {
			found = true
			if d.Severity != diag.Warning {
				t.Errorf("SEMA013 should be Warning, got %v", d.Severity)
			}
		}
	}
	if !found {
		t.Error("expected SEMA013 for unreachable code after EXIT")
	}
}

func TestNoUnreachableIfLastStmt(t *testing.T) {
	// RETURN as last statement in body emits no warning
	body := []ast.Statement{
		&ast.AssignStmt{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindAssignStmt,
				NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 4, Col: 1}},
			},
			Target: &ast.Ident{Name: "x"},
			Value:  &ast.Literal{Value: "1", LitKind: ast.LitInt},
		},
		&ast.ReturnStmt{
			NodeBase: ast.NodeBase{
				NodeKind: ast.KindReturnStmt,
				NodeSpan: ast.Span{Start: ast.Pos{File: "test.st", Line: 5, Col: 1}},
			},
		},
	}

	prog := &ast.ProgramDecl{
		NodeBase: ast.NodeBase{NodeKind: ast.KindProgramDecl},
		Name:     &ast.Ident{Name: "Main"},
		Body:     body,
	}
	file := makeSourceFile(prog)

	table := symbols.NewTable()
	collector := diag.NewCollector()
	CheckUsage([]*ast.SourceFile{file}, table, collector)

	for _, d := range collector.All() {
		if d.Code == CodeUnreachableCode {
			t.Errorf("unexpected SEMA013 when RETURN is last statement: %s", d.Message)
		}
	}
}
