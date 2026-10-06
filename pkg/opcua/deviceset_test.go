package opcua

import (
	"reflect"
	"testing"

	"github.com/awcullen/opcua/ua"
	"github.com/centroid-is/stc/pkg/types"
)

func TestDeviceSet(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil)
	sp, ds := Build(root(gvl("GVL_Test", scalar("x", types.TypeINT, da("1")))), nil)
	if len(ds) != 0 {
		t.Fatalf("Build diagnostics %v", ds)
	}
	src := NewMapSource(map[string]any{"GVL_Test.x": int64(5)})
	if err := s.Publish(sp, src); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	c := dialAnon(t, s)
	plc1 := ua.NewNodeIDString(4, "PLC1")

	hasChild := func(parent, child ua.NodeID) bool {
		for _, r := range browseForward(t, c, parent) {
			if ua.ToNodeID(r.NodeID, nil) == child {
				return true
			}
		}
		return false
	}
	if !hasChild(ua.ObjectIDObjectsFolder, ua.NewNodeIDNumeric(2, 5001)) {
		t.Fatal("Objects lacks DeviceSet ns=2;i=5001")
	}
	if !hasChild(ua.NewNodeIDNumeric(2, 5001), plc1) {
		t.Fatal("DeviceSet lacks PLC1")
	}
	if !hasChild(plc1, ua.NewNodeIDString(4, "GVL_Test")) {
		t.Fatal("PLC1 lacks GVL_Test")
	}
	if hasChild(ua.ObjectIDObjectsFolder, ua.NewNodeIDString(4, "GVL_Test")) {
		t.Error("GVL_Test must not hang directly under Objects")
	}
	if bn := readAttr(t, c, ua.NewNodeIDNumeric(2, 5001), ua.AttributeIDBrowseName).Value; bn != ua.NewQualifiedName(2, "DeviceSet") {
		t.Errorf("DeviceSet BrowseName %v", bn)
	}

	want := map[string]any{
		"Model": "TwinCAT 3 PLC emulated by stc", "DeviceManual": "", "DeviceRevision": "3.1",
		"SerialNumber": "stc-emulator", "DeviceState": int32(0),
	}
	for _, p := range plc1Properties {
		id := ua.NewNodeIDString(4, "PLC1."+p.name)
		dv := readValue(t, c, id)
		if dv.StatusCode != ua.Good || !reflect.DeepEqual(dv.Value, want[p.name]) {
			t.Errorf("%s = %#v (%v), want %#v", p.name, dv.Value, dv.StatusCode, want[p.name])
		}
		if got := writeAttr(t, c, id, want[p.name], ""); got != ua.BadNotWritable {
			t.Errorf("%s write = %v, want BadNotWritable", p.name, got)
		}
	}

	// A second Publish reuses PLC1; a node named PLC1 collides with it.
	sp2 := &Space{Nodes: []NodeSpec{{Path: "GVL_Two", Name: "GVL_Two", Class: NodeObject}}}
	if err := s.Publish(sp2, src); err != nil {
		t.Fatalf("second Publish: %v", err)
	}
	if !hasChild(plc1, ua.NewNodeIDString(4, "GVL_Two")) {
		t.Error("second Publish not under PLC1")
	}
	bad := &Space{Nodes: []NodeSpec{{Path: "PLC1", Name: "PLC1", Class: NodeObject}}}
	if err := s.Publish(bad, src); !errContains(err, "already published") {
		t.Errorf("Publish PLC1 = %v", err)
	}
}
