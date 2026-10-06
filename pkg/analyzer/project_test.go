package analyzer

import (
	"path/filepath"
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/vendor/twincat"
)

const twincatTestdata = "../vendor/twincat/testdata"

func TestAnalyzeProjectDemo(t *testing.T) {
	m, _, err := twincat.Import(filepath.Join(twincatTestdata, "sln", "Demo", "Demo solution.tsproj"), twincat.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res := AnalyzeProject(m, nil, nil)
	if res.Symbols == nil {
		t.Fatal("nil symbol table")
	}
	sema039 := 0
	for _, d := range res.Diags {
		if d.Severity == diag.Error {
			t.Errorf("unexpected error: %v", d)
		}
		if d.Code == "SEMA039" {
			sema039++
			if filepath.Base(d.Pos.File) != "GVL_Quoted.TcGVL" {
				t.Errorf("SEMA039 at %s", d.Pos.File)
			}
		}
	}
	if sema039 != 1 {
		t.Errorf("want one SEMA039, got %d: %v", sema039, res.Diags)
	}
}

func TestAnalyzeProjectBrokenPosition(t *testing.T) {
	m, _, err := twincat.Import(filepath.Join(twincatTestdata, "broken", "Broken.plcproj"), twincat.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res := AnalyzeProject(m, nil, nil)
	want, _ := filepath.Abs(filepath.Join(twincatTestdata, "broken", "POUs", "MAIN.TcPOU"))
	for _, d := range res.Diags {
		if d.Code == "SEMA010" {
			if d.Pos.File != want || d.Pos.Line != 11 || d.Pos.Col != 6 {
				t.Errorf("SEMA010 at %s:%d:%d, want %s:11:6", d.Pos.File, d.Pos.Line, d.Pos.Col, want)
			}
			return
		}
	}
	t.Fatalf("no SEMA010 in %v", res.Diags)
}
