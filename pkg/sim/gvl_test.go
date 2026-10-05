package sim

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
)

func TestEngine_GVL(t *testing.T) {
	res := parser.Parse("G.st", `
VAR_GLOBAL gain : INT := 7; END_VAR
PROGRAM P
VAR_OUTPUT OUT : INT; END_VAR
OUT := G.gain;
gain := gain + 1;
END_PROGRAM
`)
	var prog *ast.ProgramDecl
	var gvls []*ast.GVLDecl
	for _, d := range res.File.Declarations {
		switch d := d.(type) {
		case *ast.ProgramDecl:
			prog = d
		case *ast.GVLDecl:
			gvls = append(gvls, d)
		}
	}
	eng := NewSimulationEngine(SimConfig{Program: prog, GVLs: gvls, NumCycles: 3, CycleDt: 10 * time.Millisecond})
	result, err := eng.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	for i, want := range []int64{7, 8, 9} {
		if got := result.Cycles[i].Outputs["OUT"].Int; got != want {
			t.Errorf("cycle %d: OUT = %d, want %d", i, got, want)
		}
	}
}
