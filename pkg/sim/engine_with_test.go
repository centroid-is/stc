package sim

import (
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/parser"
)

func TestNewSimulationEngineWith(t *testing.T) {
	src := `PROGRAM P
VAR_INPUT x : INT; END_VAR
VAR_OUTPUT y : INT; END_VAR
VAR k : INT := 1; END_VAR
y := x + k;
END_PROGRAM
`
	res := parser.Parse("p.st", src)
	rt, err := interp.NewRuntime([]*ast.SourceFile{res.File})
	if err != nil {
		t.Fatal(err)
	}
	if err := rt.Set("P.k", 10); err != nil {
		t.Fatal(err)
	}
	eng := NewSimulationEngineWith(SimConfig{NumCycles: 2, CycleDt: time.Millisecond, Waveforms: []WaveformBinding{{
		InputName: "x", Generator: NewWaveformGenerator(WaveformConfig{Kind: WaveStep, Amplitude: 5}),
	}}}, rt.Engine("P"))
	out, err := eng.Run()
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Cycles) != 2 {
		t.Fatalf("cycles = %d", len(out.Cycles))
	}
	if got := out.Cycles[1].Outputs["Y"]; got.String() != "15" {
		t.Errorf("Y = %v, want 15 (set before cycle 1 is visible)", got)
	}
}
