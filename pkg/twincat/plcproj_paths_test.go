package twincat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
)

// TestPlcprojItemPaths is the LO-03 regression: MSBuild escapes such as
// %28 are decoded, and an Include whose case differs from the file on
// disk resolves to the on-disk spelling with a warning, as on Windows.
func TestPlcprojItemPaths(t *testing.T) {
	dir := t.TempDir()
	for _, f := range []string{"POUs/MAIN.TcPOU", "POUs/FB_Foo(Bar).TcPOU", "GVLs/GVL_A.TcGVL"} {
		p := filepath.Join(dir, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("<TcPlcObject/>"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	proj := filepath.Join(dir, "P.plcproj")
	xml := `<Project><ItemGroup>
<Compile Include="pous\main.TcPOU" />
<Compile Include="POUs\FB_Foo%28Bar%29.TcPOU" />
<Compile Include="GVLs\GVL_A.TcGVL" />
<Compile Include="POUs\Missing.TcPOU" />
<Compile Include=".\GVLs\GVL_A.TcGVL" />
<Compile Include="GVLs\GVL_A.TcGVL\Inner.TcPOU" />
</ItemGroup></Project>`
	if err := os.WriteFile(proj, []byte(xml), 0o644); err != nil {
		t.Fatal(err)
	}
	info, ds, err := ReadPlcproj(proj)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, it := range info.Items {
		got = append(got, it.RelPath)
		if filepath.Clean(it.AbsPath) != filepath.Join(filepath.Dir(info.Path), filepath.FromSlash(it.RelPath)) {
			t.Errorf("abs path %q for %q", it.AbsPath, it.RelPath)
		}
	}
	want := "POUs/MAIN.TcPOU POUs/FB_Foo(Bar).TcPOU GVLs/GVL_A.TcGVL POUs/Missing.TcPOU ./GVLs/GVL_A.TcGVL GVLs/GVL_A.TcGVL/Inner.TcPOU"
	if strings.Join(got, " ") != want {
		t.Errorf("items %v, want %s", got, want)
	}
	var cases []string
	for _, d := range ds {
		if d.Code == CodeCaseMismatch {
			if d.Severity != diag.Warning {
				t.Errorf("%s severity %v", d.Code, d.Severity)
			}
			cases = append(cases, d.Message)
		}
	}
	if len(cases) != 1 || !strings.Contains(cases[0], `pous\main.TcPOU`) || !strings.Contains(cases[0], "POUs/MAIN.TcPOU") {
		t.Errorf("case mismatch diagnostics %v", cases)
	}
	if got := msbuildUnescape("a%3Bb%25%zz%4"); got != "a;b%%zz%4" {
		t.Errorf("msbuildUnescape = %q", got)
	}
}
