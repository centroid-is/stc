package twincat

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/diag"
)

const demoTsproj = "testdata/sln/Demo/Demo solution.tsproj"

func TestTsprojFileForm(t *testing.T) {
	info, ds, err := ReadTsproj(demoTsproj)
	if err != nil {
		t.Fatal(err)
	}
	if info.PlcName != "Demo" || info.AmsPort != 851 {
		t.Errorf("name/port = %q/%d", info.PlcName, info.AmsPort)
	}
	want := abs(t, filepath.FromSlash("testdata/sln/Demo/Demo/Demo.plcproj"))
	if info.PlcprojPath != want {
		t.Errorf("plcproj = %q, want %q", info.PlcprojPath, want)
	}
	wantTasks := []Task{{Name: "PlcTask", CycleTime: time.Millisecond, CycleNs: 1000000, Priority: 20}}
	if !reflect.DeepEqual(info.Tasks, wantTasks) {
		t.Errorf("tasks = %+v", info.Tasks)
	}
	d := findCode(ds, CodeSkipped)
	if d == nil || d.Severity != diag.Info || !strings.Contains(d.Message, "Safe.xti") {
		t.Errorf("want VEND022 info for Safety project, got %v", ds)
	}
}

func TestTsprojInlineForm(t *testing.T) {
	info, ds, err := ReadTsproj("testdata/inline/Inline.tsproj")
	if err != nil {
		t.Fatal(err)
	}
	if info.PlcName != "Inline" || info.AmsPort != 851 {
		t.Errorf("name/port = %q/%d", info.PlcName, info.AmsPort)
	}
	if want := abs(t, filepath.FromSlash("testdata/inline/Inline/Inline.plcproj")); info.PlcprojPath != want {
		t.Errorf("plcproj = %q, want %q", info.PlcprojPath, want)
	}
	if len(info.Tasks) != 1 || info.Tasks[0].CycleTime != 20*time.Millisecond || info.Tasks[0].CycleNs != 20000000 {
		t.Errorf("tasks = %+v", info.Tasks)
	}
	if len(ds) != 0 {
		t.Errorf("unexpected diags %v", ds)
	}
}

func TestTsprojCRLFBOM(t *testing.T) {
	dir := t.TempDir()
	copyTreeCRLF(t, "testdata/sln", dir)
	a, _, err := ReadTsproj(demoTsproj)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := ReadTsproj(filepath.Join(dir, "Demo", "Demo solution.tsproj"))
	if err != nil {
		t.Fatal(err)
	}
	if a.PlcName != b.PlcName || a.AmsPort != b.AmsPort || !reflect.DeepEqual(a.Tasks, b.Tasks) ||
		filepath.Base(a.PlcprojPath) != filepath.Base(b.PlcprojPath) {
		t.Errorf("CRLF+BOM differs: %+v vs %+v", a, b)
	}
}

func TestTsprojErrors(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := ReadTsproj(filepath.Join(dir, "nope.tsproj")); err == nil || !strings.Contains(err.Error(), "nope.tsproj") {
		t.Errorf("missing tsproj: %v", err)
	}

	bad := filepath.Join(dir, "bad.tsproj")
	writeFile(t, bad, "<TcSmProject><Project><System>")
	_, _, err := ReadTsproj(bad)
	var xe *BadXMLError
	if !errors.As(err, &xe) || !strings.Contains(err.Error(), CodeBadXML) || xe.Path != bad {
		t.Errorf("truncated tsproj: %v", err)
	}

	noPlc := filepath.Join(dir, "noplc.tsproj")
	writeFile(t, noPlc, `<TcSmProject><Project><System><Tasks/></System></Project></TcSmProject>`)
	if _, _, err := ReadTsproj(noPlc); err == nil || !strings.Contains(err.Error(), "no PLC project") {
		t.Errorf("no plc: %v", err)
	}

	missingXti := filepath.Join(dir, "mx", "a.tsproj")
	writeFile(t, missingXti, `<TcSmProject><Project><Plc><Project File="Gone.xti"/></Plc></Project></TcSmProject>`)
	if _, _, err := ReadTsproj(missingXti); err == nil || !strings.Contains(err.Error(), "Gone.xti") {
		t.Errorf("missing xti: %v", err)
	}

	badXti := filepath.Join(dir, "bx", "a.tsproj")
	writeFile(t, badXti, `<TcSmProject><Project><Plc><Project File="B.xti"/></Plc></Project></TcSmProject>`)
	writeFile(t, filepath.Join(dir, "bx", "_Config", "PLC", "B.xti"), "<TcSmItem><Project")
	if _, _, err := ReadTsproj(badXti); !errors.As(err, &xe) {
		t.Errorf("bad xti: %v", err)
	}

	emptyXti := filepath.Join(dir, "ex", "a.tsproj")
	writeFile(t, emptyXti, `<TcSmProject><Project><Plc><Project File="E.xti"/></Plc></Project></TcSmProject>`)
	writeFile(t, filepath.Join(dir, "ex", "_Config", "PLC", "E.xti"), `<TcSmItem><Project Name="E"/></TcSmItem>`)
	if _, _, err := ReadTsproj(emptyXti); err == nil || !strings.Contains(err.Error(), "PrjFilePath") {
		t.Errorf("xti without PrjFilePath: %v", err)
	}

	missingPlcproj := filepath.Join(dir, "mp", "a.tsproj")
	writeFile(t, missingPlcproj, `<TcSmProject><Project><Plc><Project Name="P" PrjFilePath="P\P.plcproj" AmsPort="851"/></Plc></Project></TcSmProject>`)
	if _, _, err := ReadTsproj(missingPlcproj); err == nil || !strings.Contains(err.Error(), "P.plcproj") {
		t.Errorf("missing plcproj: %v", err)
	}
}

func TestTsprojExtraPlcProject(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "two.tsproj")
	writeFile(t, p, `<TcSmProject><Project>
<Plc>
<Project Name="A" PrjFilePath="A\A.plcproj" AmsPort="851"/>
<Project Name="B" PrjFilePath="B\B.plcproj" AmsPort="852"/>
</Plc></Project></TcSmProject>`)
	writeFile(t, filepath.Join(dir, "A", "A.plcproj"), `<Project/>`)
	info, ds, err := ReadTsproj(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.PlcName != "A" || len(info.Tasks) != 0 {
		t.Errorf("info = %+v", info)
	}
	if d := findCode(ds, CodeSkipped); d == nil || !strings.Contains(d.Message, "B") {
		t.Errorf("want VEND022 for second PLC project: %v", ds)
	}
}

func TestNormPath(t *testing.T) {
	got := normPath(`..\..\Demo\Demo.plcproj`)
	if want := filepath.FromSlash("../../Demo/Demo.plcproj"); got != want {
		t.Errorf("normPath = %q, want %q", got, want)
	}
	_ = os.PathSeparator
}
