package vendor

import (
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/twincat"
)

const demoPlcproj = "../twincat/testdata/sln/Demo/Demo/Demo.plcproj"

func TestExtractProjectDemo(t *testing.T) {
	stubs, ds, err := ExtractProject(demoPlcproj)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, s := range stubs {
		names = append(names, s.Name)
	}
	want := []string{"GVL_Main", "GVL_Quoted", "ST_Point", "E_Mode", "MAIN", "FB_Motor", "I_Motor", "F_Add"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("stubs %v", names)
	}
	unknown := 0
	for _, d := range ds {
		if d.Code == twincat.CodeUnknownItem {
			unknown++
			if !strings.Contains(d.Message, "Screen.TcVIS") {
				t.Errorf("VEND021 %v", d)
			}
		}
	}
	if unknown != 1 {
		t.Errorf("want one VEND021, got %v", ds)
	}

	fb := stubs[5]
	if fb.Kind != twincat.KindPOU || fb.RelPath != "POUs/FB_Motor.TcPOU" {
		t.Errorf("FB_Motor %+v", fb.RelPath)
	}
	for _, s := range []string{"END_FUNCTION_BLOCK", "PROPERTY PUBLIC Speed : INT", "GET", "SET", "END_PROPERTY"} {
		if !strings.Contains(fb.Text, s) {
			t.Errorf("FB_Motor stub lacks %q:\n%s", s, fb.Text)
		}
	}
	methods := regexp.MustCompile(`(?m)^METHOD\b`).FindAllStringIndex(fb.Text, -1)
	ends := regexp.MustCompile(`(?m)^END_METHOD\b`).FindAllStringIndex(fb.Text, -1)
	if len(methods) != 2 || len(ends) != 2 || methods[0][0] > ends[0][0] || methods[1][0] > ends[1][0] {
		t.Errorf("method headers %v ends %v:\n%s", methods, ends, fb.Text)
	}
	for _, body := range []string{"startCount := startCount + 1", "running := enable", "speedSet := Speed", "wasRunning := running"} {
		if strings.Contains(fb.Text, body) {
			t.Errorf("stub keeps body statement %q", body)
		}
	}

	if stubs[0].Kind != twincat.KindGVL || !strings.Contains(stubs[0].Text, "VAR_GLOBAL") {
		t.Errorf("GVL stub %+v", stubs[0])
	}
	if stubs[2].Kind != twincat.KindDUT || !strings.Contains(stubs[2].Text, "TYPE ST_Point") {
		t.Errorf("DUT stub %+v", stubs[2])
	}

	for _, s := range stubs {
		r := pipeline.Parse(s.Name+".st", s.Text, nil)
		for _, d := range r.Diags {
			if d.Severity == diag.Error {
				t.Errorf("%s: %v", s.Name, d)
			}
		}
	}

	again, ds2, _ := ExtractProject(demoPlcproj)
	if !reflect.DeepEqual(stubs, again) || !reflect.DeepEqual(ds, ds2) {
		t.Error("two extractions differ")
	}
}

func TestExtractProjectErrors(t *testing.T) {
	if _, _, err := ExtractProject("../twincat/testdata/nope.plcproj"); err == nil {
		t.Error("want error for missing plcproj")
	}
	dir := t.TempDir()
	writeFileT(t, dir+"/P.plcproj", `<Project><ItemGroup><Compile Include="Bad.TcPOU" /><Compile Include="Gone.TcPOU" /></ItemGroup></Project>`)
	writeFileT(t, dir+"/Bad.TcPOU", "<TcPlcObject><POU")
	stubs, ds, err := ExtractProject(dir + "/P.plcproj")
	if err != nil || len(stubs) != 0 || len(ds) != 2 || ds[0].Code != twincat.CodeBadXML || !strings.Contains(ds[1].Message, "cannot read") {
		t.Errorf("stubs %v diags %v err %v", stubs, ds, err)
	}
}

func writeFileT(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
