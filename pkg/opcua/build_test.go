package opcua

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/types"
)

// shape renders a Space as "Path|Class|S|Access" lines; S marks Structured,
// Access is R, W or RW for Variables and empty for Objects.
func shape(sp *Space) []string {
	out := make([]string, 0, len(sp.Nodes))
	for _, n := range sp.Nodes {
		class, s, a := "Obj", "", ""
		if n.Class == NodeVariable {
			class = "Var"
			a = map[AccessLevel]string{AccessRead: "R", AccessWrite: "W", AccessReadWrite: "RW"}[n.Access]
		}
		if n.Structured {
			s = "S"
		}
		out = append(out, fmt.Sprintf("%s|%s|%s|%s", n.Path, class, s, a))
	}
	return out
}

func codes(ds []diag.Diagnostic) []string {
	var out []string
	for _, d := range ds {
		out = append(out, d.Code)
	}
	return out
}

func badStruct() *types.StructType {
	return &types.StructType{Name: "ST_Bad", Members: []types.StructMember{
		{Name: "fb", Type: &types.FunctionBlockType{Name: "TON"}},
	}}
}

func unknownBounds(elem types.Type) *types.ArrayType {
	return &types.ArrayType{ElementType: elem, Dimensions: []types.ArrayDimension{{Text: "1..GVL.N"}}}
}

func TestBuild(t *testing.T) {
	stAB := &types.StructType{Name: "ST_AB", Members: []types.StructMember{
		{Name: "a", Type: types.TypeINT}, {Name: "b", Type: types.TypeBOOL}}}
	stNest := &types.StructType{Name: "ST_Nest", Members: []types.StructMember{
		{Name: "inner", Type: stAB}, {Name: "items", Type: arr(stAB, [2]int{0, 1})}}}
	abKids := func() []*fakeNode {
		return []*fakeNode{scalar("a", types.TypeINT), scalar("b", types.TypeBOOL)}
	}

	cases := []struct {
		name  string
		root  *fakeNode
		want  []string
		diags []string
	}{
		{
			name: "da 1 on an FB instance exposes every descendant",
			root: root(gvl("G", fbInst("fb", "FB_X", attrs(da("1")),
				scalar("x", types.TypeINT),
				structInst("s", stAB, nil, abKids()...)))),
			want: []string{"G|Obj||", "G.fb|Obj||", "G.fb.x|Var||RW", "G.fb.s|Obj||", "G.fb.s.a|Var||RW", "G.fb.s.b|Var||RW"},
		},
		{
			name: "da 0 prunes a subtree even under da 1",
			root: root(gvl("G", fbInst("fb", "FB_X", attrs(da("1")),
				scalar("x", types.TypeINT),
				tonInst("tmr", da("0")),
				scalar("y", types.TypeINT, da("0"))))),
			want: []string{"G|Obj||", "G.fb|Obj||", "G.fb.x|Var||RW"},
		},
		{
			name: "da 2 publishes a struct as one Variable without member nodes",
			root: root(gvl("G", structInst("s", stAB, attrs(da("2"), acc("1")), abKids()...))),
			want: []string{"G|Obj||", "G.s|Var|S|R"},
		},
		{
			name:  "da 2 on a struct that cannot be a DataType is skipped",
			root:  root(gvl("G", structInst("s", badStruct(), attrs(da("2")), tonInst("fb")))),
			want:  []string{},
			diags: []string{"OPCUA005"},
		},
		{
			name: "da 2 on an FB instance exposes it like da 1",
			root: root(gvl("G", fbInst("fb", "FB_X", attrs(da("2")), scalar("x", types.TypeINT)))),
			want: []string{"G|Obj||", "G.fb|Obj||", "G.fb.x|Var||RW"},
		},
		{
			name: "type-level HMI mark publishes HMI in unmarked instances",
			root: root(gvl("sensors", fbSensor("S1"), fbSensor("S2"))),
			want: []string{
				"sensors|Obj||",
				"sensors.S1|Obj||", "sensors.S1.HMI|Var|S|RW", "sensors.S1.HMI.p_stat_xRaw|Var||R", "sensors.S1.HMI.p_cfg_Invert|Var||RW",
				"sensors.S2|Obj||", "sensors.S2.HMI|Var|S|RW", "sensors.S2.HMI.p_stat_xRaw|Var||R", "sensors.S2.HMI.p_cfg_Invert|Var||RW",
			},
		},
		{
			name: "instance da 0 overrides the type-level da 1 (last wins)",
			root: root(gvl("G", fbInst("fb", "FB_X", nil,
				structInst("HMI", stAB, attrs(structured(), da("1"), da("0")), abKids()...)))),
			want: []string{},
		},
		{
			name: "type-level mark inside an unmarked PROGRAM still publishes",
			root: root(prog("MAIN", scalar("x", types.TypeINT), fbSensor("fbLocal"))),
			want: []string{"MAIN|Obj||", "MAIN.fbLocal|Obj||", "MAIN.fbLocal.HMI|Var|S|RW", "MAIN.fbLocal.HMI.p_stat_xRaw|Var||R", "MAIN.fbLocal.HMI.p_cfg_Invert|Var||RW"},
		},
		{
			name: "PROGRAM MAIN without any mark is silent",
			root: root(prog("MAIN", scalar("x", types.TypeINT), tonInst("t"))),
			want: []string{},
		},
		{
			name: "PROGRAM MAIN with one marked variable",
			root: root(prog("MAIN", scalar("x", types.TypeINT), scalar("y", types.TypeINT, da("1")))),
			want: []string{"MAIN|Obj||", "MAIN.y|Var||RW"},
		},
		{
			name: "Access maps 1 2 3 and absent and does not inherit",
			root: root(gvl("G",
				scalar("r", types.TypeINT, da("1"), acc("1")),
				scalar("w", types.TypeINT, da("1"), acc("2")),
				scalar("rw", types.TypeINT, da("1"), acc("3")),
				scalar("none", types.TypeINT, da("1")),
				scalar("bad", types.TypeINT, da("1"), acc("9")),
				fbInst("fb", "FB_X", attrs(da("1"), acc("1")), scalar("x", types.TypeINT)))),
			want: []string{"G|Obj||", "G.r|Var||R", "G.w|Var||W", "G.rw|Var||RW", "G.none|Var||RW", "G.bad|Var||RW",
				"G.fb|Obj||", "G.fb.x|Var||RW"},
			diags: []string{"OPCUA001"},
		},
		{
			name: "StructuredType on the instance makes a structured Variable with member Variables",
			root: root(gvl("G", structInst("s", stAB, attrs(da("1"), structured()), abKids()...))),
			want: []string{"G|Obj||", "G.s|Var|S|RW", "G.s.a|Var||RW", "G.s.b|Var||RW"},
		},
		{
			name: "StructuredType on the TYPE header applies to every instance",
			root: root(gvl("G",
				structInst("s1", stAB, attrs(structured(), da("1")), abKids()...),
				structInst("s2", stAB, attrs(structured(), da("1")), scalar("a", types.TypeINT, da("0")), scalar("b", types.TypeBOOL)))),
			want: []string{"G|Obj||", "G.s1|Var|S|RW", "G.s1.a|Var||RW", "G.s1.b|Var||RW", "G.s2|Var|S|RW", "G.s2.b|Var||RW"},
		},
		{
			name: "struct without StructuredType is an Object",
			root: root(gvl("G", structInst("s", stAB, attrs(da("1")), abKids()...))),
			want: []string{"G|Obj||", "G.s|Obj||", "G.s.a|Var||RW", "G.s.b|Var||RW"},
		},
		{
			name: "structured parent forces nested structs and array-of-struct elements to be structured",
			root: root(gvl("G", structInst("n", stNest, attrs(structured(), da("1")),
				structInst("inner", stAB, nil, abKids()...),
				arrayOf("items", arr(stAB, [2]int{0, 1}), nil,
					elems("items", 0, 1, func(n string) *fakeNode { return structInst(n, stAB, nil, abKids()...) })...)))),
			want: []string{"G|Obj||", "G.n|Var|S|RW", "G.n.inner|Var|S|RW", "G.n.inner.a|Var||RW", "G.n.inner.b|Var||RW",
				"G.n.items|Obj||", "G.n.items[0]|Var|S|RW", "G.n.items[0].a|Var||RW", "G.n.items[0].b|Var||RW",
				"G.n.items[1]|Var|S|RW", "G.n.items[1].a|Var||RW", "G.n.items[1].b|Var||RW"},
		},
		{
			name:  "StructuredType on an FB instance is ignored",
			root:  root(gvl("G", fbInst("fb", "FB_X", attrs(da("1"), structured()), scalar("x", types.TypeINT)))),
			want:  []string{"G|Obj||", "G.fb|Obj||", "G.fb.x|Var||RW"},
			diags: []string{"OPCUA004"},
		},
		{
			name:  "StructuredType on an ineligible struct falls back to an Object",
			root:  root(gvl("G", structInst("s", badStruct(), attrs(da("1"), structured()), tonInst("fb")))),
			want:  []string{"G|Obj||", "G.s|Obj||", "G.s.fb|Obj||", "G.s.fb.IN|Var||RW", "G.s.fb.PT|Var||RW", "G.s.fb.Q|Var||RW", "G.s.fb.ET|Var||RW"},
			diags: []string{"OPCUA005"},
		},
		{
			name: "arrays of scalars and enums are single Variables",
			root: root(gvl("G",
				arrayOf("ai", arr(types.TypeINT, [2]int{0, 3}), attrs(da("1"))),
				arrayOf("ae", arr(eDriveState, [2]int{1, 2}), attrs(da("1"))),
				arrayOf("quiet", arr(types.TypeINT, [2]int{0, 3}), nil))),
			want: []string{"G|Obj||", "G.ai|Var||RW", "G.ae|Var||RW"},
		},
		{
			name: "array of FBs is an Object with element children",
			root: root(gvl("G", arrayOf("fbs", arr(&types.FunctionBlockType{Name: "FB_X"}, [2]int{1, 2}), attrs(da("1")),
				elems("fbs", 1, 2, func(n string) *fakeNode { return fbInst(n, "FB_X", nil, scalar("x", types.TypeINT)) })...))),
			want: []string{"G|Obj||", "G.fbs|Obj||", "G.fbs[1]|Obj||", "G.fbs[1].x|Var||RW", "G.fbs[2]|Obj||", "G.fbs[2].x|Var||RW"},
		},
		{
			name: "unknown array bounds are skipped",
			root: root(gvl("G",
				arrayOf("a", unknownBounds(types.TypeINT), attrs(da("1"))),
				arrayOf("s", unknownBounds(stAB), attrs(da("1"))),
				arrayOf("quiet", unknownBounds(types.TypeINT), nil),
				node(KindArray, "notarray", types.TypeINT, attrs(da("1"))))),
			want:  []string{"G|Obj||"},
			diags: []string{"OPCUA003", "OPCUA003", "OPCUA003"},
		},
		{
			name: "pointers and references are skipped",
			root: root(gvl("G",
				ptr("p", da("1")), refVar("r", da("1")), ptr("quiet"),
				arrayOf("ap", arr(&types.PointerType{BaseType: types.TypeINT}, [2]int{0, 1}), attrs(da("1"))),
				scalar("ps", &types.PointerType{BaseType: types.TypeINT}, da("1")))),
			want:  []string{"G|Obj||"},
			diags: []string{"OPCUA002", "OPCUA002", "OPCUA002", "OPCUA002"},
		},
		{
			name: "unmappable scalars and enums are skipped",
			root: root(gvl("G",
				scalar("v", types.TypeVOID, da("1")),
				enumVar("e", &types.EnumType{Name: "E_Bad", Values: []string{"a"}}, da("1")))),
			want:  []string{"G|Obj||"},
			diags: []string{"OPCUA003", "OPCUA003"},
		},
		{
			name:  "unknown OPC.UA.DA value is treated as unset",
			root:  root(gvl("G", scalar("x", types.TypeINT, da("7")))),
			want:  []string{},
			diags: []string{"OPCUA001"},
		},
		{
			name:  "duplicate case-insensitive NodeId keeps the first",
			root:  root(gvl("G", scalar("x", types.TypeINT, da("1")), scalar("X", types.TypeINT, da("1")))),
			want:  []string{"G|Obj||", "G.x|Var||RW"},
			diags: []string{"OPCUA007"},
		},
		{
			name:  "a GVL named PLC1 collides with the device object",
			root:  root(gvl("plc1", scalar("x", types.TypeINT, da("1")))),
			want:  []string{},
			diags: []string{"OPCUA007"},
		},
		{
			name: "Description is unescaped and kept on Objects too",
			root: root(gvl("G", fbInst("fb", "FB_X", attrs(da("1"), descr("Motor $'A$'")), scalar("x", types.TypeINT)))),
			want: []string{"G|Obj||", "G.fb|Obj||", "G.fb.x|Var||RW"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sp, ds := Build(c.root, nil)
			if got := shape(sp); !reflect.DeepEqual(got, c.want) {
				t.Errorf("nodes\n got %q\nwant %q", got, c.want)
			}
			if got := codes(ds); !reflect.DeepEqual(got, c.diags) {
				t.Errorf("diagnostics %q, want %q (%v)", got, c.diags, ds)
			}
			for _, d := range ds {
				if d.Message == "" || d.Severity == diag.Error {
					t.Errorf("diagnostic %+v: want a message and a non-error severity", d)
				}
			}
		})
	}
}

func TestBuildDescription(t *testing.T) {
	sp, _ := Build(root(gvl("G", scalar("x", types.TypeINT, da("1"), descr("Motor $'A$' $$5")),
		fbInst("fb", "FB_X", attrs(da("1"), descr("an $'FB$'"))))), nil)
	if n, _ := sp.Find("G.x"); n.Description != "Motor 'A' $5" {
		t.Errorf("Description = %q", n.Description)
	}
	if n, _ := sp.Find("G.fb"); n.Description != "an 'FB'" || n.Class != NodeObject {
		t.Errorf("Object Description = %q", n.Description)
	}
	if n, _ := sp.Find("G.x"); n.Type != types.TypeINT || n.Name != "x" || n.Parent != "G" {
		t.Errorf("spec = %+v", n)
	}
}

func TestBuildMissingLeaf(t *testing.T) {
	r := root(gvl("G", scalar("x", types.TypeINT, da("1")), scalar("y", types.TypeINT, da("1")),
		structInst("s", stBus, attrs(da("2")))))
	src := NewMapSource(map[string]any{"G.x": int64(1)})
	sp, ds := Build(r, src)
	if len(sp.Nodes) != 4 {
		t.Fatalf("nodes %q", shape(sp))
	}
	if got := codes(ds); !reflect.DeepEqual(got, []string{"OPCUA006"}) || !strings.Contains(ds[0].Message, "G.y") || ds[0].Severity != diag.Warning {
		t.Errorf("diagnostics %v", ds)
	}
}

func TestBuildDepthLimit(t *testing.T) {
	leaf := scalar("x", types.TypeINT)
	n := leaf
	for i := 0; i < 70; i++ {
		n = fbInst(fmt.Sprintf("f%d", i), "FB_X", nil, n)
	}
	n.attrs = attrs(da("1"))
	sp, ds := Build(root(gvl("G", n)), nil)
	if got := codes(ds); !reflect.DeepEqual(got, []string{"OPCUA008"}) {
		t.Fatalf("diagnostics %v", ds)
	}
	if len(sp.Nodes) != maxBuildDepth {
		t.Errorf("%d nodes, want %d (GVL plus nested FBs up to the depth limit)", len(sp.Nodes), maxBuildDepth)
	}
}

func TestBuildEmpty(t *testing.T) {
	if sp, ds := Build(nil, nil); sp == nil || len(sp.Nodes) != 0 || len(ds) != 0 {
		t.Errorf("Build(nil) = %+v %v", sp, ds)
	}
	r := root(gvl("G", scalar("x", types.TypeINT, da("1"))))
	r.kids = append(r.kids, nil)
	if sp, _ := Build(r, nil); len(sp.Nodes) != 2 {
		t.Errorf("nil child: %q", shape(sp))
	}
}

func TestBuildST301Space(t *testing.T) {
	r, src := st301Fixture()
	sp, ds := Build(r, src)
	if err := validateSpace(sp); err != nil {
		t.Fatalf("validateSpace: %v", err)
	}
	if got := codes(ds); !reflect.DeepEqual(got, []string{"OPCUA002"}) || !strings.Contains(ds[0].Message, "GVL_BatchLines.Conveyor.pDrive") {
		t.Errorf("diagnostics %v", ds)
	}
	for _, n := range sp.Nodes {
		if strings.HasPrefix(n.Path, "MAIN") || strings.Contains(n.Path, "tmr") || strings.Contains(n.Path, "pAxis") ||
			strings.Contains(n.Path, "xRawIn") || strings.HasPrefix(n.Path, "GVL_BatchLines.Internal_Bus_8.") {
			t.Errorf("unexpected node %s", n.Path)
		}
	}
	for _, p := range []string{"GVL_BatchLines.Drives_Line1[1].HMI.p_stat_Error", "sensors.EPW01_WA01_IS11.HMI.p_stat_xRaw", "GVL_Roe.aRoe"} {
		if _, ok := sp.Find(p); !ok {
			t.Errorf("missing %s", p)
		}
	}
}
