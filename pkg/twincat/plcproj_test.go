package twincat

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
)

const demoPlcproj = "testdata/sln/Demo/Demo/Demo.plcproj"

func TestPlcprojItems(t *testing.T) {
	info, ds, err := ReadPlcproj(demoPlcproj)
	if err != nil {
		t.Fatal(err)
	}
	if info.Path != abs(t, demoPlcproj) {
		t.Errorf("path = %q", info.Path)
	}
	var got []string
	for _, it := range info.Items {
		got = append(got, it.RelPath+"|"+string(it.Kind))
		if it.AbsPath != filepath.Join(filepath.Dir(info.Path), filepath.FromSlash(it.RelPath)) {
			t.Errorf("abs path %q for %q", it.AbsPath, it.RelPath)
		}
		if it.Line == 0 || it.Ext == "" {
			t.Errorf("item %+v missing line or ext", it)
		}
	}
	want := []string{
		"GVLs/GVL_Main.TcGVL|gvl", "GVLs/GVL_Quoted.TcGVL|gvl", "DUTs/ST_Point.TcDUT|dut", "DUTs/E_Mode.TcDUT|dut",
		"POUs/MAIN.TcPOU|pou", "POUs/FB_Motor.TcPOU|pou", "POUs/I_Motor.TcIO|itf", "POUs/F_Add.TcPOU|pou",
		"PlcTask.TcTTO|task",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("items =\n%v\nwant\n%v", got, want)
	}
	if info.Items[0].Line != 7 {
		t.Errorf("first item line = %d, want 7", info.Items[0].Line)
	}
	d := findCode(ds, CodeUnknownItem)
	if d == nil || d.Severity != diag.Warning || d.Pos.Line != 34 || !strings.Contains(d.Message, "Screen.TcVIS") {
		t.Errorf("want VEND021 at line 34, got %v", ds)
	}
}

func TestPlcprojRefs(t *testing.T) {
	info, _, err := ReadPlcproj(demoPlcproj)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, r := range info.Refs {
		names = append(names, r.Name)
		if r.Pos.File != info.Path {
			t.Errorf("ref %s pos file %q", r.Name, r.Pos.File)
		}
	}
	want := []string{"DemoLib", "Tc2_EtherCAT", "Tc2_Standard", "Tc2_System", "Tc3_Module", "Tc2_SerialCom", "Tc2_Missing"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("refs = %v", names)
	}
	r := info.Refs[1]
	if r.DefaultResolution != "Tc2_EtherCAT, * (Beckhoff Automation GmbH)" || r.Namespace != "Tc2_EtherCAT" ||
		r.Pos != (source.Pos{File: info.Path, Line: 43, Col: 5}) {
		t.Errorf("ref = %+v", r)
	}
}

func TestPlcprojCRLFBOM(t *testing.T) {
	dir := t.TempDir()
	copyTreeCRLF(t, "testdata/sln", dir)
	a, ads, err := ReadPlcproj(demoPlcproj)
	if err != nil {
		t.Fatal(err)
	}
	b, bds, err := ReadPlcproj(filepath.Join(dir, "Demo", "Demo", "Demo.plcproj"))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Items) != len(b.Items) || len(a.Refs) != len(b.Refs) || len(ads) != len(bds) {
		t.Fatalf("counts differ")
	}
	for i := range a.Items {
		if a.Items[i].RelPath != b.Items[i].RelPath || a.Items[i].Line != b.Items[i].Line || a.Items[i].Kind != b.Items[i].Kind {
			t.Errorf("item %d differs: %+v vs %+v", i, a.Items[i], b.Items[i])
		}
	}
	for i := range a.Refs {
		if a.Refs[i].Name != b.Refs[i].Name || a.Refs[i].Pos.Line != b.Refs[i].Pos.Line ||
			a.Refs[i].DefaultResolution != b.Refs[i].DefaultResolution {
			t.Errorf("ref %d differs", i)
		}
	}
	if ads[0].Pos.Line != bds[0].Pos.Line {
		t.Errorf("diag line differs")
	}
}

func TestPlcprojErrors(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := ReadPlcproj(filepath.Join(dir, "gone.plcproj")); err == nil || !strings.Contains(err.Error(), "gone.plcproj") {
		t.Errorf("missing: %v", err)
	}
	bad := filepath.Join(dir, "bad.plcproj")
	writeFile(t, bad, "<Project><ItemGroup><Compile Include=\"a.TcPOU\">")
	var xe *BadXMLError
	if _, _, err := ReadPlcproj(bad); !errors.As(err, &xe) {
		t.Errorf("truncated: %v", err)
	}
}

func TestTcTTO(t *testing.T) {
	task, err := ReadTcTTO("testdata/sln/Demo/Demo/PlcTask.TcTTO")
	if err != nil {
		t.Fatal(err)
	}
	want := Task{Name: "PlcTask", CycleTime: time.Millisecond, CycleNs: 1000000, Priority: 20, Programs: []string{"MAIN"}}
	if !reflect.DeepEqual(task, want) {
		t.Errorf("task = %+v", task)
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "t.TcTTO")
	if err := writeBytes(p, crlfBOM(mustRead(t, "testdata/sln/Demo/Demo/PlcTask.TcTTO"))); err != nil {
		t.Fatal(err)
	}
	if task2, err := ReadTcTTO(p); err != nil || !reflect.DeepEqual(task2, want) {
		t.Errorf("CRLF+BOM task = %+v, %v", task2, err)
	}
}

func TestTcTTOErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := ReadTcTTO(filepath.Join(dir, "x.TcTTO")); err == nil {
		t.Error("missing TcTTO: want error")
	}
	bad := filepath.Join(dir, "bad.TcTTO")
	writeFile(t, bad, "<TcPlcObject><Task")
	var xe *BadXMLError
	if _, err := ReadTcTTO(bad); !errors.As(err, &xe) {
		t.Errorf("bad: %v", err)
	}
	noTask := filepath.Join(dir, "none.TcTTO")
	writeFile(t, noTask, "<TcPlcObject></TcPlcObject>")
	if _, err := ReadTcTTO(noTask); err == nil || !strings.Contains(err.Error(), "no Task") {
		t.Errorf("no task: %v", err)
	}
}

func TestMergeTasks(t *testing.T) {
	ts := []Task{{Name: "PlcTask", CycleTime: time.Millisecond, CycleNs: 1e6, Priority: 20}}
	tto := []ttoFile{{Path: "/p/PlcTask.TcTTO", Task: Task{Name: "plctask", CycleTime: time.Millisecond, CycleNs: 1e6, Priority: 20, Programs: []string{"MAIN"}}}}

	got, ds := mergeTasks(ts, tto, "/p/x.plcproj")
	if len(ds) != 0 || len(got) != 1 || !reflect.DeepEqual(got[0].Programs, []string{"MAIN"}) || got[0].CycleTime != time.Millisecond {
		t.Errorf("agree: %+v %v", got, ds)
	}
	if ts[0].Programs != nil {
		t.Error("mergeTasks mutated its input")
	}
}

func TestMergeTasksMismatch(t *testing.T) {
	ts := []Task{{Name: "PlcTask", CycleTime: time.Millisecond, CycleNs: 1e6, Priority: 20}}
	tto := []ttoFile{{Path: "/p/PlcTask.TcTTO", Task: Task{Name: "PlcTask", CycleTime: 10 * time.Millisecond, CycleNs: 1e7, Programs: []string{"MAIN"}}}}
	got, ds := mergeTasks(ts, tto, "/p/x.plcproj")
	if got[0].CycleTime != time.Millisecond || !reflect.DeepEqual(got[0].Programs, []string{"MAIN"}) {
		t.Errorf("tsproj cycle must win: %+v", got)
	}
	d := findCode(ds, CodeCycleMismatch)
	if d == nil || d.Severity != diag.Warning || d.Pos.File != "/p/PlcTask.TcTTO" {
		t.Errorf("want VEND025: %v", ds)
	}
}

func TestMergeTasksBarePlcproj(t *testing.T) {
	tto := []ttoFile{{Path: "/p/T.TcTTO", Task: Task{Name: "T", CycleTime: 2 * time.Millisecond, CycleNs: 2e6, Programs: []string{"P"}}}}
	got, ds := mergeTasks(nil, tto, "/p/x.plcproj")
	if len(ds) != 0 || len(got) != 1 || got[0].CycleTime != 2*time.Millisecond || got[0].Name != "T" {
		t.Errorf("bare+tto: %+v %v", got, ds)
	}

	got, ds = mergeTasks(nil, nil, "/p/x.plcproj")
	if len(got) != 1 || got[0].CycleTime != 10*time.Millisecond || got[0].CycleNs != 1e7 || got[0].Name != "PlcTask" {
		t.Errorf("default task: %+v", got)
	}
	if d := findCode(ds, CodeDefaultCycle); d == nil || d.Pos.File != "/p/x.plcproj" || d.Severity != diag.Warning {
		t.Errorf("want VEND024: %v", ds)
	}

	// tsproj task with no matching TcTTO keeps an empty program list.
	got, ds = mergeTasks([]Task{{Name: "Other", CycleTime: time.Millisecond, CycleNs: 1e6}}, tto, "/p/x.plcproj")
	if len(ds) != 0 || len(got[0].Programs) != 0 {
		t.Errorf("unmatched: %+v %v", got, ds)
	}
}
