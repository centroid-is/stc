package twincat

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
)


func readInfo(t *testing.T, path string) *PlcprojInfo {
	t.Helper()
	info, _, err := ReadPlcproj(path)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

func refByName(refs []LibraryRef, name string) *LibraryRef {
	for i := range refs {
		if refs[i].Name == name {
			return &refs[i]
		}
	}
	return nil
}

func countCode(ds []diag.Diagnostic, code string) int {
	n := 0
	for _, d := range ds {
		if d.Code == code {
			n++
		}
	}
	return n
}

func sourceKeys(srcs []Source) []string {
	out := make([]string, len(srcs))
	for i, s := range srcs {
		out[i] = s.Library + ":" + s.RelPath
	}
	return out
}

func TestResolveDemo(t *testing.T) {
	info := readInfo(t, demoPlcproj)
	ctx := newResolveCtx(nil, "")
	refs, srcs := resolveLibraries(info, "Demo", ctx)

	want := map[string]string{
		"DemoLib": FromSibling, "Tc2_EtherCAT": FromStub, "Tc2_Standard": FromBuiltin,
		"Tc2_System": FromStub, "Tc3_Module": FromStub, "Tc2_SerialCom": FromStub, "Tc2_Missing": FromUnresolved,
	}
	if len(refs) != len(want) {
		t.Fatalf("got %d refs, want %d", len(refs), len(want))
	}
	order := []string{"DemoLib", "Tc2_EtherCAT", "Tc2_Standard", "Tc2_System", "Tc3_Module", "Tc2_SerialCom", "Tc2_Missing"}
	for i, name := range order {
		if refs[i].Name != name || refs[i].ResolvedFrom != want[name] {
			t.Errorf("ref %d = %s/%s, want %s/%s", i, refs[i].Name, refs[i].ResolvedFrom, name, want[name])
		}
	}
	lib := refByName(refs, "DemoLib")
	if !strings.HasSuffix(lib.Path, filepath.Join("sln", "DemoLib", "DemoLib", "DemoLib.plcproj")) {
		t.Errorf("DemoLib path %q", lib.Path)
	}
	if countCode(ctx.diags, CodeAmbiguousSibling) != 0 {
		t.Errorf("unexpected VEND026: %v", ctx.diags)
	}
	if countCode(ctx.diags, CodeUnresolvedLibrary) != 1 {
		t.Fatalf("want one VEND020, got %v", ctx.diags)
	}
	d := findCode(ctx.diags, CodeUnresolvedLibrary)
	if d.Message != "unresolved library reference 'Tc2_Missing'" || d.Pos.File != info.Path || d.Pos.Line != 64 || d.Severity != diag.Warning {
		t.Errorf("VEND020 = %+v", d)
	}

	gotKeys := sourceKeys(srcs)
	wantKeys := []string{
		"DemoLib:POUs/FB_LibThing.TcPOU",
		"DemoLib:DUTs/E_LibState.TcDUT",
		"Tc2_System:stdlib/vendor/beckhoff/common_types.st",
		"Tc2_System:stdlib/vendor/beckhoff/tc2_system.st",
		"Tc2_EtherCAT:stdlib/vendor/beckhoff/tc2_utilities.st",
		"Tc2_EtherCAT:stdlib/vendor/beckhoff/tc2_ethercat.st",
		"Tc3_Module:stdlib/vendor/beckhoff/tc3_module.st",
		"Tc2_SerialCom:stdlib/vendor/beckhoff/tc2_serialcom.st",
	}
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Errorf("sources:\n got %v\nwant %v", gotKeys, wantKeys)
	}
	for _, s := range srcs[2:] {
		if s.Path != s.RelPath || s.Kind != KindPOU || s.Text == "" {
			t.Errorf("stub source %+v", s.Path)
		}
	}
	if srcs[0].Kind != KindPOU || srcs[0].Name != "FB_LibThing" || srcs[1].Kind != KindDUT || !filepath.IsAbs(srcs[0].Path) {
		t.Errorf("sibling sources %+v %+v", srcs[0].Path, srcs[1].Kind)
	}
}

func TestResolveDeterministic(t *testing.T) {
	info := readInfo(t, demoPlcproj)
	c1, c2 := newResolveCtx(nil, ""), newResolveCtx(nil, "")
	r1, s1 := resolveLibraries(info, "Demo", c1)
	r2, s2 := resolveLibraries(info, "Demo", c2)
	if !reflect.DeepEqual(r1, r2) || !reflect.DeepEqual(s1, s2) || !reflect.DeepEqual(c1.diags, c2.diags) {
		t.Fatal("two runs differ")
	}
}

const plcprojTmpl = `<?xml version="1.0" encoding="utf-8"?>
<Project xmlns="http://schemas.microsoft.com/developer/msbuild/2003">
  <ItemGroup>
%s  </ItemGroup>
  <ItemGroup>
%s  </ItemGroup>
</Project>
`

func plcproj(items []string, refs ...string) string {
	var ib, rb strings.Builder
	for _, it := range items {
		ib.WriteString("    <Compile Include=\"" + it + "\" />\n")
	}
	for _, r := range refs {
		rb.WriteString("    <PlaceholderReference Include=\"" + r + "\"><Namespace>" + r + "</Namespace></PlaceholderReference>\n")
	}
	return strings.Replace(strings.Replace(plcprojTmpl, "%s", ib.String(), 1), "%s", rb.String(), 1)
}

const libPOU = `<?xml version="1.0" encoding="utf-8"?>
<TcPlcObject Version="1.1.0.1">
  <POU Name="%N" Id="{00000000-0000-0000-0000-000000000001}" SpecialFunc="None">
    <Declaration><![CDATA[FUNCTION_BLOCK %N
VAR_INPUT
	x : BOOL;
END_VAR
]]></Declaration>
    <Implementation>
      <ST><![CDATA[]]></ST>
    </Implementation>
  </POU>
</TcPlcObject>
`

func writePOU(t *testing.T, path, name string) {
	writeFile(t, path, strings.ReplaceAll(libPOU, "%N", name))
}

func TestResolveAmbiguousSiblingAndDotDirs(t *testing.T) {
	root := t.TempDir()
	app := filepath.Join(root, "App", "App", "App.plcproj")
	writeFile(t, app, plcproj(nil, "Lib"))
	writeFile(t, filepath.Join(root, ".x", "Lib.plcproj"), plcproj([]string{`POUs\FB_Hidden.TcPOU`}))
	writeFile(t, filepath.Join(root, "B", "Lib.plcproj"), plcproj([]string{`POUs\FB_B.TcPOU`}))
	writeFile(t, filepath.Join(root, "A", "Lib.plcproj"), plcproj([]string{`POUs\FB_A.TcPOU`}))
	writePOU(t, filepath.Join(root, "A", "POUs", "FB_A.TcPOU"), "FB_A")
	writePOU(t, filepath.Join(root, "B", "POUs", "FB_B.TcPOU"), "FB_B")

	ctx := newResolveCtx(nil, "")
	refs, srcs := resolveLibraries(readInfo(t, app), "App", ctx)
	if refs[0].ResolvedFrom != FromSibling || refs[0].Path != filepath.Join(root, "A", "Lib.plcproj") {
		t.Fatalf("ref = %+v", refs[0])
	}
	d := findCode(ctx.diags, CodeAmbiguousSibling)
	if d == nil || !strings.Contains(d.Message, filepath.Join(root, "A", "Lib.plcproj")) || !strings.Contains(d.Message, filepath.Join(root, "B", "Lib.plcproj")) {
		t.Fatalf("VEND026 = %+v", ctx.diags)
	}
	if strings.Contains(d.Message, ".x") {
		t.Errorf("dot directory considered: %s", d.Message)
	}
	if len(srcs) != 1 || srcs[0].Name != "FB_A" {
		t.Errorf("sources %v", sourceKeys(srcs))
	}
}

func TestResolveOwnNameAndMutualRecursion(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "A", "A", "A.plcproj")
	writeFile(t, a, plcproj(nil, "A", "B"))
	writeFile(t, filepath.Join(root, "B", "B", "B.plcproj"), plcproj([]string{`FB_B.TcPOU`}, "A", "B"))
	writePOU(t, filepath.Join(root, "B", "B", "FB_B.TcPOU"), "FB_B")

	ctx := newResolveCtx(nil, "")
	refs, srcs := resolveLibraries(readInfo(t, a), "A", ctx)
	if refs[0].ResolvedFrom != FromProject || refs[0].Path != "" {
		t.Errorf("own ref = %+v", refs[0])
	}
	if refs[1].ResolvedFrom != FromSibling {
		t.Errorf("B ref = %+v", refs[1])
	}
	if len(srcs) != 1 || srcs[0].Library != "B" {
		t.Errorf("sources %v", sourceKeys(srcs))
	}
	if len(ctx.diags) != 0 {
		t.Errorf("diags %v", ctx.diags)
	}
}

func TestResolveLibraryPaths(t *testing.T) {
	cfgDir := t.TempDir()
	writeFile(t, filepath.Join(cfgDir, "libs", "missing", "b.st"), "FUNCTION_BLOCK FB_MB\nEND_FUNCTION_BLOCK\n")
	writeFile(t, filepath.Join(cfgDir, "libs", "missing", "a.st"), "FUNCTION_BLOCK FB_MA\nEND_FUNCTION_BLOCK\n")
	writeFile(t, filepath.Join(cfgDir, "libs", "serial", "s.st"), "FUNCTION_BLOCK FB_S\nEND_FUNCTION_BLOCK\n")
	lp := map[string]string{"tc2_missing": "libs/missing", "Tc2_SerialCom": filepath.Join(cfgDir, "libs", "serial")}

	ctx := newResolveCtx(lp, cfgDir)
	refs, srcs := resolveLibraries(readInfo(t, demoPlcproj), "Demo", ctx)
	if r := refByName(refs, "Tc2_Missing"); r.ResolvedFrom != FromLibraryPath || r.Path != filepath.Join(cfgDir, "libs", "missing") {
		t.Errorf("Tc2_Missing = %+v", r)
	}
	if r := refByName(refs, "Tc2_SerialCom"); r.ResolvedFrom != FromLibraryPath {
		t.Errorf("Tc2_SerialCom = %+v", r)
	}
	if countCode(ctx.diags, CodeUnresolvedLibrary) != 0 {
		t.Errorf("diags %v", ctx.diags)
	}
	keys := sourceKeys(srcs)
	want := []string{
		"DemoLib:POUs/FB_LibThing.TcPOU",
		"DemoLib:DUTs/E_LibState.TcDUT",
		"Tc2_SerialCom:s.st",
		"Tc2_Missing:a.st",
		"Tc2_Missing:b.st",
		"Tc2_System:stdlib/vendor/beckhoff/common_types.st",
		"Tc2_System:stdlib/vendor/beckhoff/tc2_system.st",
		"Tc2_EtherCAT:stdlib/vendor/beckhoff/tc2_utilities.st",
		"Tc2_EtherCAT:stdlib/vendor/beckhoff/tc2_ethercat.st",
		"Tc3_Module:stdlib/vendor/beckhoff/tc3_module.st",
	}
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("sources:\n got %v\nwant %v", keys, want)
	}
	if srcs[3].Path != filepath.Join(cfgDir, "libs", "missing", "a.st") || srcs[3].Text == "" {
		t.Errorf("library path source %+v", srcs[3].Path)
	}
}

func TestResolveLibraryPathMissingDir(t *testing.T) {
	ctx := newResolveCtx(map[string]string{"Tc2_Missing": "nope"}, t.TempDir())
	refs, _ := resolveLibraries(readInfo(t, demoPlcproj), "Demo", ctx)
	if r := refByName(refs, "Tc2_Missing"); r.ResolvedFrom != FromUnresolved {
		t.Errorf("Tc2_Missing = %+v", r)
	}
	if countCode(ctx.diags, CodeUnresolvedLibrary) != 2 {
		t.Errorf("want path warning and VEND020, got %v", ctx.diags)
	}
}
