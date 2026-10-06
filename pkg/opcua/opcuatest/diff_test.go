package opcuatest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func i32(v int32) *int32 { return &v }
func u8(v uint8) *uint8  { return &v }

func base() *Snapshot {
	return &Snapshot{
		Root: "ns=4;s=PLC1",
		Nodes: []Node{
			{NodeID: "ns=4;s=G", NodeClass: "Object", BrowseName: "4:G"},
			{NodeID: "ns=4;s=G.a", NodeClass: "Variable", DataType: "i=6", ValueRank: i32(-1), AccessLevel: u8(3), Description: "x"},
			{NodeID: "ns=4;s=G.arr", NodeClass: "Variable", DataType: "i=4", ValueRank: i32(1), ArrayDimensions: []uint32{4}, AccessLevel: u8(1)},
			{NodeID: "ns=4;s=G.s", NodeClass: "Variable", DataType: "ns=4;s=DT.ST_X", ValueRank: i32(-1), AccessLevel: u8(3)},
		},
		DataTypes: []DataType{
			{NodeID: "ns=4;s=DT.ST_X", Kind: "structure", Fields: []Field{
				{Name: "a", DataType: "i=1", ValueRank: -1},
				{Name: "e", DataType: "ns=4;s=DT.E_Y", ValueRank: -1},
			}},
			{NodeID: "ns=4;s=DT.E_Y", Kind: "enumeration", Values: []EnumValue{{0, "off"}, {2, "rdy"}}},
		},
	}
}

func TestDiffIdentical(t *testing.T) {
	if d := Diff(base(), base(), Full); len(d) != 0 {
		t.Fatalf("want no differences, got %v", d)
	}
	// Description, Root, BrowseName and DataType id spelling are ignored.
	r := base()
	r.Root = "ns=4;s=Other"
	r.Nodes[1].Description = "y"
	r.Nodes[3].DataType = "ns=4;s=<StructuredType>ST_X"
	r.DataTypes[0].NodeID = "ns=4;s=<StructuredType>ST_X"
	r.DataTypes[0].Fields[1].DataType = "ns=4;s=Lib.E_Y"
	r.DataTypes[1].NodeID = "ns=4;s=Lib.E_Y"
	if d := Diff(base(), r, Full); len(d) != 0 {
		t.Fatalf("normalized snapshots should match, got %v", d)
	}
}

func TestDiffEachAttribute(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Snapshot)
		id     string
		attr   string
		e, r   string
	}{
		{"access", func(s *Snapshot) { s.Nodes[1].AccessLevel = u8(1) }, "ns=4;s=G.a", "accessLevel", "3", "1"},
		{"access unset", func(s *Snapshot) { s.Nodes[1].AccessLevel = nil }, "ns=4;s=G.a", "accessLevel", "3", "unset"},
		{"datatype", func(s *Snapshot) { s.Nodes[1].DataType = "i=11" }, "ns=4;s=G.a", "dataType", "i=6", "i=11"},
		{"rank", func(s *Snapshot) { s.Nodes[1].ValueRank = i32(1) }, "ns=4;s=G.a", "valueRank", "-1", "1"},
		{"rank unset", func(s *Snapshot) { s.Nodes[1].ValueRank = nil }, "ns=4;s=G.a", "valueRank", "-1", "unset"},
		{"dims", func(s *Snapshot) { s.Nodes[2].ArrayDimensions = []uint32{5} }, "ns=4;s=G.arr", "arrayDimensions", "[4]", "[5]"},
		{"class", func(s *Snapshot) { s.Nodes[0].NodeClass = "Variable" }, "ns=4;s=G", "nodeClass", "Object", "Variable"},
		{"field name", func(s *Snapshot) { s.DataTypes[0].Fields[0].Name = "b" }, "DataType:ST_X", "field[0]", "a:i=1 rank -1 dims []", "b:i=1 rank -1 dims []"},
		{"field type", func(s *Snapshot) { s.DataTypes[0].Fields[0].DataType = "i=2" }, "DataType:ST_X", "field[0]", "a:i=1 rank -1 dims []", "a:i=2 rank -1 dims []"},
		{"field missing", func(s *Snapshot) { s.DataTypes[0].Fields = s.DataTypes[0].Fields[:1] }, "DataType:ST_X", "field[1]", "e:E_Y rank -1 dims []", "missing"},
		{"enum value", func(s *Snapshot) { s.DataTypes[1].Values[1].Value = 3 }, "DataType:E_Y", "value[1]", "rdy(2)", "rdy(3)"},
		{"kind", func(s *Snapshot) { s.DataTypes[1].Kind = "structure" }, "DataType:E_Y", "kind", "enumeration", "structure"},
		{"dt missing", func(s *Snapshot) { s.DataTypes = s.DataTypes[:1] }, "DataType:E_Y", "definition", "present", "missing"},
		{"node missing", func(s *Snapshot) { s.Nodes = s.Nodes[:3] }, "ns=4;s=G.s", "node", "present", "missing"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := base()
			c.mutate(r)
			d := Diff(base(), r, Subset)
			if len(d) != 1 {
				t.Fatalf("want 1 difference, got %v", d)
			}
			want := Difference{NodeID: c.id, Attribute: c.attr, Emulated: c.e, Real: c.r}
			if d[0] != want {
				t.Fatalf("got %+v, want %+v", d[0], want)
			}
		})
	}
}

func TestDiffModes(t *testing.T) {
	r := base()
	r.Nodes = append(r.Nodes, Node{NodeID: "ns=4;s=G.extra", NodeClass: "Variable"})
	r.DataTypes = append(r.DataTypes, DataType{NodeID: "ns=4;s=DT.ST_Z", Kind: "structure"})
	if d := Diff(base(), r, Subset); len(d) != 0 {
		t.Fatalf("subset ignores real-only nodes, got %v", d)
	}
	d := Diff(base(), r, Full)
	want := []Difference{
		{"DataType:ST_Z", "definition", "missing", "present"},
		{"ns=4;s=G.extra", "node", "missing", "present"},
	}
	if len(d) != 2 || d[0] != want[0] || d[1] != want[1] {
		t.Fatalf("full: got %v, want %v", d, want)
	}
}

func TestDiffSortedDeterministic(t *testing.T) {
	r := base()
	r.Nodes[1].AccessLevel = u8(1)
	r.Nodes[1].ValueRank = i32(2)
	r.Nodes[0].NodeClass = "Variable"
	first := Diff(base(), r, Full)
	for i := 0; i < 20; i++ {
		again := Diff(base(), r, Full)
		if FormatDiff(again, 0) != FormatDiff(first, 0) {
			t.Fatal("diff is not deterministic")
		}
	}
	want := "ns=4;s=G nodeClass: emulated Object, real Variable\n" +
		"ns=4;s=G.a accessLevel: emulated 3, real 1\n" +
		"ns=4;s=G.a valueRank: emulated -1, real 2\n"
	if got := FormatDiff(first, 0); got != want {
		t.Fatalf("got\n%s", got)
	}
}

func TestFormatDiffTruncates(t *testing.T) {
	var ds []Difference
	for i := 0; i < 53; i++ {
		ds = append(ds, Difference{NodeID: "n", Attribute: "a"})
	}
	out := FormatDiff(ds, 50)
	if lines := strings.Count(out, "\n"); lines != 51 {
		t.Fatalf("want 51 lines, got %d", lines)
	}
	if !strings.HasSuffix(out, "... and 3 more\n") {
		t.Fatalf("missing tail: %q", out)
	}
}

func TestDataTypeName(t *testing.T) {
	for in, want := range map[string]string{
		"":                            "",
		"i=6":                         "i=6",
		"ns=0;i=6":                    "ns=0;i=6",
		"ns=4;s=DT.ST_X":              "ST_X",
		"ns=4;s=<StructuredType>ST_X": "ST_X",
		"ns=4;i=5001":                 "i=5001",
		"ns=4":                        "ns=4",
		"ns=4;s=X.":                   "X.",
	} {
		if got := DataTypeName(in); got != want {
			t.Errorf("DataTypeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLoad(t *testing.T) {
	s, err := Load(filepath.Join("..", "..", "..", "tests", "opcua_golden", "st301_shape.json"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Root != "ns=4;s=PLC1" || len(s.Nodes) == 0 || len(s.DataTypes) == 0 {
		t.Fatalf("unexpected golden: root %q, %d nodes", s.Root, len(s.Nodes))
	}
	if d := Diff(s, s, Full); len(d) != 0 {
		t.Fatalf("golden differs from itself: %v", d)
	}
	if _, err := Load(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatal("want error for a missing file")
	}
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(bad); err == nil || !strings.Contains(err.Error(), "bad.json") {
		t.Fatalf("want parse error naming the file, got %v", err)
	}
}
