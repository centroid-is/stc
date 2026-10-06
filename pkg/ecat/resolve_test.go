package ecat

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
)

func demoTopology(t *testing.T) *Topology {
	t.Helper()
	topo, err := LoadProject(filepath.Join(fixtureDir, "Demo Device 1.xml"), filepath.Join(fixtureDir, "Demo Device 2.xml"))
	if err != nil {
		t.Fatal(err)
	}
	return topo
}

func TestResolveDemo(t *testing.T) {
	topo := demoTopology(t)
	vars, _ := CollectLinks(parseFixtures(t, "demo_types.st", "demo_ect.st"))
	bindings, diags := Resolve(topo, vars)
	if len(diags) != 0 {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if len(bindings) != len(vars) {
		t.Fatalf("got %d bindings for %d vars", len(bindings), len(vars))
	}
	if !sort.SliceIsSorted(bindings, func(i, j int) bool { return bindingLess(bindings[i], bindings[j]) }) {
		t.Error("bindings not sorted by path then link")
	}
	byPath := map[string]Binding{}
	for _, b := range bindings {
		byPath[b.Var.Path] = b
	}
	checks := map[string]string{
		"ECT.A1_01.I1":        "Device 1 (EtherCAT) in 0.0 1",
		"ECT.A1_03.p_Current": "Device 1 (EtherCAT) in 10.0 16",
		"ECT.CN01.q_uCMD":     "Device 1 (EtherCAT) out 10.0 16",
		"ECT.CN01.amsaddr":    "Device 1 (EtherCAT) in 88.0 64",
		"ECT.Dev1_AmsNetId":   "Device 1 (EtherCAT) in 137.0 48",
		"ECT.D2_I1":           "Device 2 (EtherCAT) in 25.0 1",
		"ECT.V1_C1":           "Device 1 (EtherCAT) out 15.0 8",
	}
	for path, want := range checks {
		b, ok := byPath[path]
		if !ok {
			t.Errorf("%s not bound", path)
			continue
		}
		got := fmt.Sprintf("%s %s %d.%d %d", b.Slot.Master, b.Slot.Dir, b.Slot.Byte, b.Slot.Bit, b.Slot.BitLen)
		if got != want {
			t.Errorf("%s: got %s want %s", path, got, want)
		}
	}
}

func TestResolveBad(t *testing.T) {
	topo := demoTopology(t)
	vars, _ := CollectLinks(parseFixtures(t, "demo_types.st", "demo_bad.st"))
	bindings, diags := Resolve(topo, vars)
	var got []string
	for _, d := range diags {
		got = append(got, fmt.Sprintf("%d %s %s", d.Pos.Line, d.Severity, d.Code))
	}
	want := []string{"5 error ECAT001", "11 error ECAT003", "14 error ECAT004", "19 warning ECAT005"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("diags = %v, want %v", got, want)
	}
	msg := diags[0].Message
	for _, s := range []string{"DEMO.A1.01 (EL1008)\"", `"Channel 1"`, `"Channel 2"`, `"Channel 3"`, "did you mean"} {
		if !strings.Contains(msg, s) {
			t.Errorf("ECAT001 message %q lacks %q", msg, s)
		}
	}
	if strings.Contains(msg, `"Channel 4"`) {
		t.Errorf("ECAT001 lists more than 3 suggestions: %q", msg)
	}
	if !strings.Contains(diags[1].Message, "16") || !strings.Contains(diags[1].Message, "1-bit") {
		t.Errorf("ECAT003 message %q", diags[1].Message)
	}
	if !strings.Contains(diags[3].Message, "DupA") {
		t.Errorf("ECAT005 message %q", diags[3].Message)
	}
	var paths []string
	for _, b := range bindings {
		paths = append(paths, b.Var.Path)
	}
	if strings.Join(paths, ",") != "demo_bad.DupA,demo_bad.DupB" {
		t.Errorf("bindings = %v", paths)
	}
}

func TestResolveRules(t *testing.T) {
	topo := demoTopology(t)
	d1 := "TIID^Device 1 (EtherCAT)"
	vars := []LinkedVar{
		{Path: "G.netIdStr", Link: d1 + "^InfoData^AmsNetId", TypeName: "STRING", HasAT: true},
		{Path: "G.badStr", Link: d1 + "^Inputs^DevState", TypeName: "STRING", HasAT: true},
		{Path: "G.unknown", Link: d1 + "^Inputs^SlaveCount", TypeName: "T_Lib", HasAT: true},
		{Path: "G.noAt", Link: d1 + "^DEMO.A1.00 (EK1200)^DEMO.A1.02 (EL2008)^Channel 1^Output", BitWidth: 1, HasAT: false},
		{Path: "G.inOnOut", Link: d1 + "^DEMO.A1.00 (EK1200)^DEMO.A1.02 (EL2008)^Channel 2^Output", BitWidth: 1, Dir: DirIn, HasAT: true},
		{Path: "G.badMaster", Link: "TIID^Device 9", BitWidth: 1},
	}
	bindings, diags := Resolve(topo, vars)
	var got []string
	for _, d := range diags {
		got = append(got, d.Code+" "+d.Message)
	}
	if len(diags) != 3 || diags[0].Code != CodeSizeMismatch || diags[1].Code != CodeDirMismatch || diags[2].Code != CodeUnresolved {
		t.Fatalf("diags = %v", got)
	}
	if !strings.Contains(diags[2].Message, `"TIID"`) || !strings.Contains(diags[2].Message, `"Device 1 (EtherCAT)"`) {
		t.Errorf("ECAT001 root suggestion: %q", diags[2].Message)
	}
	var paths []string
	for _, b := range bindings {
		paths = append(paths, b.Var.Path)
	}
	if strings.Join(paths, ",") != "G.netIdStr,G.noAt,G.unknown" {
		t.Errorf("bindings = %v", paths)
	}
}

func TestBindingLessTieBreak(t *testing.T) {
	a := Binding{Var: LinkedVar{Path: "X", Link: "TIID^a"}}
	b := Binding{Var: LinkedVar{Path: "X", Link: "TIID^b"}}
	if !bindingLess(a, b) || bindingLess(b, a) {
		t.Error("tie-break by link failed")
	}
}

func TestSortDiagnostics(t *testing.T) {
	mk := func(file string, line, col int, code string) diag.Diagnostic {
		return diag.Diagnostic{Pos: source.Pos{File: file, Line: line, Col: col}, Code: code}
	}
	ds := []diag.Diagnostic{mk("b.st", 1, 1, "A"), mk("a.st", 2, 5, "B"), mk("a.st", 2, 1, "C"), mk("a.st", 1, 9, "D"), mk("a.st", 2, 1, "E")}
	SortDiagnostics(ds)
	var got []string
	for _, d := range ds {
		got = append(got, d.Code)
	}
	if strings.Join(got, "") != "DCEBA" {
		t.Errorf("order = %v", got)
	}
}
