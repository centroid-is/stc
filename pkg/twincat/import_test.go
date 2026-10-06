package twincat

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
)

func gvlName(f *ast.SourceFile) string {
	for _, d := range f.Declarations {
		if g, ok := d.(*ast.GVLDecl); ok && g.Name != nil {
			return g.Name.Name
		}
	}
	return ""
}

func TestImportDemoTsproj(t *testing.T) {
	m, ds, err := Import(demoTsproj, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if m.PlcName != "Demo" || m.AmsPort != 851 {
		t.Errorf("plc %s port %d", m.PlcName, m.AmsPort)
	}
	abs, _ := filepath.Abs(demoTsproj)
	if m.SolutionPath != abs || !filepath.IsAbs(m.ProjectPath) || filepath.Base(m.ProjectPath) != "Demo.plcproj" {
		t.Errorf("paths %s %s", m.SolutionPath, m.ProjectPath)
	}
	if len(m.Tasks) != 1 {
		t.Fatalf("tasks %+v", m.Tasks)
	}
	tk := m.Tasks[0]
	if tk.Name != "PlcTask" || tk.CycleTime != time.Millisecond || tk.Priority != 20 || !reflect.DeepEqual(tk.Programs, []string{"MAIN"}) {
		t.Errorf("task %+v", tk)
	}
	var names []string
	for _, s := range m.Sources {
		names = append(names, s.Name)
		if s.Library != "" || !filepath.IsAbs(s.Path) || s.Text == "" {
			t.Errorf("source %+v", s.Path)
		}
	}
	want := []string{"GVL_Main", "GVL_Quoted", "ST_Point", "E_Mode", "MAIN", "FB_Motor", "I_Motor", "F_Add"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("sources %v", names)
	}
	if m.Sources[6].Kind != KindITF || m.Sources[0].Kind != KindGVL || m.Sources[2].Kind != KindDUT {
		t.Errorf("kinds %s %s %s", m.Sources[6].Kind, m.Sources[0].Kind, m.Sources[2].Kind)
	}
	if len(m.Libraries) != 7 || m.Libraries[0].ResolvedFrom != FromSibling || m.Libraries[6].ResolvedFrom != FromUnresolved {
		t.Errorf("libraries %+v", m.Libraries)
	}
	if len(m.LibrarySources) == 0 {
		t.Error("no library sources")
	}
	if countCode(ds, CodeUnknownItem) != 1 || countCode(ds, CodeUnresolvedLibrary) != 1 || countCode(ds, CodeDefaultCycle) != 0 {
		t.Errorf("diags %v", ds)
	}
	for i := 1; i < len(ds); i++ {
		a, b := ds[i-1], ds[i]
		if a.Pos.File > b.Pos.File || (a.Pos.File == b.Pos.File && a.Pos.Line > b.Pos.Line) {
			t.Errorf("diagnostics not sorted: %v before %v", a, b)
		}
	}
	m2, ds2, _ := Import(demoTsproj, Options{})
	if !reflect.DeepEqual(m, m2) || !reflect.DeepEqual(ds, ds2) {
		t.Error("two imports differ")
	}
}

func TestImportPlcprojDirect(t *testing.T) {
	m, ds, err := Import(demoPlcproj, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if m.PlcName != "Demo" || m.SolutionPath != "" || m.Tasks[0].CycleTime != time.Millisecond || countCode(ds, CodeDefaultCycle) != 0 {
		t.Errorf("model %s %s %+v %v", m.PlcName, m.SolutionPath, m.Tasks, ds)
	}

	m, ds, err = Import("testdata/inline/Inline/Inline.plcproj", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if m.PlcName != "Inline" || m.Tasks[0].CycleTime != 10*time.Millisecond || countCode(ds, CodeDefaultCycle) != 1 {
		t.Errorf("inline %s %+v %v", m.PlcName, m.Tasks, ds)
	}
}

func TestImportErrors(t *testing.T) {
	if _, _, err := Import("testdata/sln/Demo/Demo/POUs/MAIN.TcPOU", Options{}); err == nil {
		t.Error("want error for unsupported extension")
	}
	if _, _, err := Import("testdata/nope/Nope.plcproj", Options{}); err == nil {
		t.Error("want error for missing plcproj")
	}
	if _, _, err := Import("testdata/nope/Nope.tsproj", Options{}); err == nil {
		t.Error("want error for missing tsproj")
	}
}

func TestImportBadItemsAndLibraryPathsConfig(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "App", "App", "App.plcproj")
	writeFile(t, p, plcproj([]string{`POUs\Bad.TcPOU`, `POUs\Gone.TcPOU`, `Task.TcTTO`}, "Tc2_Missing"))
	writeFile(t, filepath.Join(root, "App", "App", "POUs", "Bad.TcPOU"), "<TcPlcObject><POU")
	writeFile(t, filepath.Join(root, "App", "App", "Task.TcTTO"), "<TcPlcObject")
	writeFile(t, filepath.Join(root, "stc.toml"), "[build.library_paths]\nTc2_Missing = \"libs/missing\"\n")
	writeFile(t, filepath.Join(root, "libs", "missing", "m.st"), "FUNCTION_BLOCK FB_M\nEND_FUNCTION_BLOCK\n")

	m, ds, err := Import(p, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Sources) != 0 || countCode(ds, CodeBadXML) != 3 {
		t.Errorf("sources %d diags %v", len(m.Sources), ds)
	}
	if m.Libraries[0].ResolvedFrom != FromLibraryPath || len(m.LibrarySources) != 1 {
		t.Errorf("libraries %+v", m.Libraries)
	}

	writeFile(t, filepath.Join(root, "stc.toml"), "[build\n")
	if _, _, err := Import(p, Options{}); err == nil {
		t.Error("want error for a broken stc.toml")
	}
}

func TestParseModel(t *testing.T) {
	m, _, err := Import(demoTsproj, Options{})
	if err != nil {
		t.Fatal(err)
	}
	user, libs, ds := ParseModel(m, nil)
	for _, d := range ds {
		if d.Severity == diag.Error {
			t.Errorf("parse error %v", d)
		}
	}
	if len(user) != len(m.Sources) || len(libs) != len(m.LibrarySources) {
		t.Fatalf("user %d libs %d", len(user), len(libs))
	}
	if g := gvlName(user[1]); g != "GVL_Quoted" {
		t.Errorf("GVL name %q", g)
	}
	if g := gvlName(user[0]); g != "GVL_Main" {
		t.Errorf("GVL name %q", g)
	}
	if libs[len(libs)-1].Span().Start.File != "stdlib/vendor/beckhoff/tc2_serialcom.st" {
		t.Errorf("library display path %q", libs[len(libs)-1].Span().Start.File)
	}
}
