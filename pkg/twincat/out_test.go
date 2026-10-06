package twincat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/pipeline"
)

func TestWriteOutDemo(t *testing.T) {
	m, _, err := Import(demoTsproj, Options{})
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := WriteOut(m, out); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{
		"GVLs/GVL_Main.st", "GVLs/GVL_Quoted.st", "DUTs/ST_Point.st", "DUTs/E_Mode.st",
		"POUs/MAIN.st", "POUs/FB_Motor.st", "POUs/I_Motor.st", "POUs/F_Add.st",
		"libs/DemoLib/FB_LibThing.st", "libs/DemoLib/E_LibState.st", "stc.toml",
	} {
		raw, err := os.ReadFile(filepath.Join(out, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("%s: %v", rel, err)
		}
		if strings.HasSuffix(rel, ".st") {
			r := pipeline.Parse(rel, string(raw), nil)
			for _, d := range r.Diags {
				if d.Severity == diag.Error {
					t.Errorf("%s: %v", rel, d)
				}
			}
		}
	}
	raw, _ := os.ReadFile(filepath.Join(out, "POUs", "FB_Motor.st"))
	if !strings.HasPrefix(string(raw), "// source: POUs/FB_Motor.TcPOU") {
		t.Errorf("FB_Motor.st is not compact:\n%s", raw)
	}
	stubs, err := os.ReadDir(filepath.Join(out, "libs", "stubs"))
	if err != nil || len(stubs) == 0 {
		t.Fatalf("no stub files: %v", err)
	}
	var cfg struct {
		Build struct {
			VendorTarget string            `toml:"vendor_target"`
			LibraryPaths map[string]string `toml:"library_paths"`
		} `toml:"build"`
	}
	if _, err := toml.DecodeFile(filepath.Join(out, "stc.toml"), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Build.LibraryPaths["DemoLib"] != "libs/DemoLib" || cfg.Build.LibraryPaths["stubs"] != "libs/stubs" || cfg.Build.VendorTarget != "beckhoff" {
		t.Errorf("stc.toml %+v", cfg)
	}
	// Writing twice gives identical output.
	out2 := t.TempDir()
	if err := WriteOut(m, out2); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(filepath.Join(out, "stc.toml"))
	b, _ := os.ReadFile(filepath.Join(out2, "stc.toml"))
	if string(a) != string(b) {
		t.Error("stc.toml differs between runs")
	}
}

func TestWriteOutLibraryPathSources(t *testing.T) {
	m := &Model{
		PlcName: "X",
		LibrarySources: []Source{
			{Path: "/somewhere/m.st", RelPath: "m.st", Kind: KindPOU, Name: "m", Library: "Tc2_Missing", Text: "FUNCTION_BLOCK FB_M\nEND_FUNCTION_BLOCK\n"},
		},
	}
	out := t.TempDir()
	if err := WriteOut(m, out); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(out, "libs", "Tc2_Missing", "m.st"))
	if err != nil || !strings.Contains(string(raw), "FB_M") {
		t.Errorf("library_path source: %v %s", err, raw)
	}
}

func TestWriteOutRejectsTraversal(t *testing.T) {
	motor, _ := filepath.Abs("testdata/sln/Demo/Demo/POUs/FB_Motor.TcPOU")
	root := t.TempDir()
	out := filepath.Join(root, "a", "b")
	cases := []*Model{
		{Sources: []Source{{Path: motor, RelPath: `..\..\evil.TcPOU`, Kind: KindPOU, Name: "FB_Motor"}}},
		{Sources: []Source{{Path: motor, RelPath: "/abs/evil.TcPOU", Kind: KindPOU, Name: "FB_Motor"}}},
		{Sources: []Source{{Path: motor, RelPath: `C:\abs\evil.TcPOU`, Kind: KindPOU, Name: "FB_Motor"}}},
		{LibrarySources: []Source{{Path: motor, RelPath: "FB_Motor.TcPOU", Kind: KindPOU, Name: "FB_Motor", Library: "../../evil"}}},
	}
	for i, m := range cases {
		// The first source is valid so a guard that writes before validating
		// would leave files behind.
		m.Sources = append([]Source{{Path: motor, RelPath: "POUs/FB_Motor.TcPOU", Kind: KindPOU, Name: "FB_Motor"}}, m.Sources...)
		if err := WriteOut(m, out); err == nil {
			t.Errorf("case %d: want error", i)
		}
	}
	var found []string
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			found = append(found, p)
		}
		return nil
	})
	if len(found) != 0 {
		t.Errorf("files written: %v", found)
	}

	// A GVL name is sanitised to an identifier, so it cannot escape.
	gvl := filepath.Join(root, "gvl")
	m := &Model{Sources: []Source{{Path: motor, RelPath: "GVLs/x.TcGVL", Kind: KindGVL, Name: "../../evil"}}}
	if err := WriteOut(m, gvl); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(gvl, "GVLs", "______evil.st")); err != nil {
		t.Errorf("sanitised GVL file: %v", err)
	}
}

func TestWriteOutErrors(t *testing.T) {
	motor, _ := filepath.Abs("testdata/sln/Demo/Demo/POUs/FB_Motor.TcPOU")
	dup := &Model{Sources: []Source{
		{Path: motor, RelPath: "POUs/FB_Motor.TcPOU", Kind: KindPOU, Name: "FB_Motor"},
		{Path: motor, RelPath: "POUs/fb_motor.TcPOU", Kind: KindPOU, Name: "FB_Motor"},
	}}
	if err := WriteOut(dup, t.TempDir()); err == nil || !strings.Contains(err.Error(), "same output") {
		t.Errorf("duplicate: %v", err)
	}
	gone := &Model{Sources: []Source{{Path: filepath.Join(t.TempDir(), "Gone.TcPOU"), RelPath: "Gone.TcPOU", Kind: KindPOU}}}
	if err := WriteOut(gone, t.TempDir()); err == nil {
		t.Error("want error for a missing source file")
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	ok := &Model{Sources: []Source{{Path: motor, RelPath: "FB_Motor.TcPOU", Kind: KindPOU}}}
	if err := WriteOut(ok, file); err == nil {
		t.Error("want error when outDir is a file")
	}
}
