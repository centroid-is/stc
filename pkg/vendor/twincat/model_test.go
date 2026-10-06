package twincat

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/source"
)

func TestModelJSON(t *testing.T) {
	m := Model{
		PlcName: "Demo", AmsPort: 851, ProjectPath: "/x/Demo.plcproj",
		Tasks:   []Task{{Name: "PlcTask", CycleTime: time.Millisecond, CycleNs: 1000000, Priority: 20, Programs: []string{"MAIN"}}},
		Sources: []Source{{Path: "/x/MAIN.TcPOU", RelPath: "POUs/MAIN.TcPOU", Kind: KindPOU, Name: "MAIN", Text: "SECRET_TEXT"}},
		Libraries: []LibraryRef{{Name: "Tc2_System", ResolvedFrom: FromStub,
			Pos: source.Pos{File: "/x/Demo.plcproj", Line: 3}}},
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back["plc_name"] != "Demo" || back["ams_port"].(float64) != 851 {
		t.Errorf("bad header: %s", b)
	}
	task := back["tasks"].([]any)[0].(map[string]any)
	if task["cycle_time_ns"].(float64) != 1e6 {
		t.Errorf("cycle_time_ns: %s", b)
	}
	lib := back["libraries"].([]any)[0].(map[string]any)
	if lib["resolved_from"] != "stub" {
		t.Errorf("resolved_from: %s", b)
	}
	if strings.Contains(string(b), "SECRET_TEXT") || strings.Contains(string(b), "LibrarySources") {
		t.Errorf("text leaked into JSON: %s", b)
	}
}
