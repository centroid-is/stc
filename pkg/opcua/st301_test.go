package opcua

import (
	"reflect"
	"strings"
	"testing"

	"github.com/awcullen/opcua/ua"
)

// TestST301 publishes the ST301-shaped fixture and checks the node ids and
// attribute combinations the sildarvinnsla HMI relies on.
func TestST301(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil)
	r, src := st301Fixture()
	sp, ds := Build(r, src)
	if err := s.Publish(sp, src); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	c := dialAnon(t, s)
	id := func(p string) ua.NodeID { return ua.NewNodeIDString(4, p) }
	hmi := "GVL_BatchLines.Drives_Line1[1].HMI"
	unknown := func(t *testing.T, p string) {
		t.Helper()
		if st := readValue(t, c, id(p)).StatusCode; st != ua.BadNodeIDUnknown {
			t.Errorf("%s read = %v, want BadNodeIdUnknown", p, st)
		}
	}
	// fields decodes the HMI ExtensionObject by the served field names.
	fields := func(t *testing.T) map[string]any {
		t.Helper()
		dv := readValue(t, c, id(hmi))
		if dv.StatusCode != ua.Good {
			t.Fatalf("HMI read %v", dv.StatusCode)
		}
		dt := readAttr(t, c, id(hmi), ua.AttributeIDDataType).Value.(ua.NodeID)
		def := structDefOf(t, c, dt)
		v := reflect.ValueOf(dv.Value)
		if v.Kind() == reflect.Pointer {
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct || v.NumField() != len(def.Fields) {
			t.Fatalf("HMI value %T does not match its definition", dv.Value)
		}
		out := map[string]any{}
		for i, f := range def.Fields {
			out[f.Name] = v.Field(i).Interface()
		}
		return out
	}

	t.Run("a HMI struct reads as one ExtensionObject with enum names", func(t *testing.T) {
		got := fields(t)
		want := map[string]any{
			"p_cmd_JogFwd": false, "p_stat_Error": false, "p_stat_State": int32(2), "p_stat_402_State": int32(4),
			"p_stat_Frequency": float32(49.5), "p_stat_SlaveId": uint16(1001), "p_cfg_AutoFreq": float32(0),
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("HMI fields\n got %#v\nwant %#v", got, want)
		}
		if dt := readAttr(t, c, id(hmi), ua.AttributeIDDataType).Value; dt != id("DT.ST_Drive_HMI") {
			t.Errorf("HMI DataType %v", dt)
		}
		def, ok := readDefinition(t, c, id("DT.E_DriveState")).(ua.EnumDefinition)
		if !ok || len(def.Fields) != 4 || def.Fields[2].Value != 2 || def.Fields[2].Name != "rdy" {
			t.Errorf("E_DriveState definition %+v", def)
		}
		es, ok := readValue(t, c, id(hmi+".p_stat_State.EnumStrings")).Value.([]ua.LocalizedText)
		if !ok || len(es) != 4 || es[2].Text != "rdy" {
			t.Errorf("EnumStrings %#v", es)
		}
		if dt := readAttr(t, c, id(hmi+".p_stat_State"), ua.AttributeIDDataType).Value; dt != id("DT.E_DriveState") {
			t.Errorf("p_stat_State DataType %v", dt)
		}
	})

	t.Run("b p_stat member reads individually and is read-only", func(t *testing.T) {
		p := id(hmi + ".p_stat_Error")
		if dv := readValue(t, c, p); dv.StatusCode != ua.Good || dv.Value != false {
			t.Errorf("p_stat_Error = %v %v", dv.Value, dv.StatusCode)
		}
		if got := writeAttr(t, c, p, true, ""); got != ua.BadNotWritable {
			t.Errorf("p_stat_Error write = %v", got)
		}
		if al := readAttr(t, c, p, ua.AttributeIDAccessLevel).Value; al != uint8(1) {
			t.Errorf("p_stat_Error AccessLevel %v", al)
		}
		if d := readAttr(t, c, p, ua.AttributeIDDescription).Value.(ua.LocalizedText); d.Text != "Drive fault" {
			t.Errorf("p_stat_Error Description %q", d.Text)
		}
	})

	t.Run("c p_cmd write is visible in the struct read", func(t *testing.T) {
		if got := writeAttr(t, c, id(hmi+".p_cmd_JogFwd"), true, ""); got != ua.Good {
			t.Fatalf("p_cmd_JogFwd write = %v", got)
		}
		if got := fields(t)["p_cmd_JogFwd"]; got != true {
			t.Errorf("struct p_cmd_JogFwd = %v", got)
		}
		if al := readAttr(t, c, id(hmi+".p_cfg_AutoFreq"), ua.AttributeIDAccessLevel).Value; al != uint8(3) {
			t.Errorf("p_cfg_AutoFreq AccessLevel %v", al)
		}
	})

	t.Run("d exposure decided inside the FB type", func(t *testing.T) {
		for _, p := range []string{"sensors.EPW01_WA01_IS11.HMI.p_stat_xRaw", "sensors.EPW01_WA01_IS12.HMI"} {
			if st := readValue(t, c, id(p)).StatusCode; st != ua.Good {
				t.Errorf("%s read = %v", p, st)
			}
		}
		unknown(t, "sensors.EPW01_WA01_IS11.xRawIn")
	})

	t.Run("e unmarked MAIN publishes nothing", func(t *testing.T) {
		unknown(t, "MAIN")
		unknown(t, "MAIN.nCycle")
		names := strings.Join(browseNames(browseForward(t, c, id("PLC1"))), ",")
		if strings.Contains(names, "MAIN") || !strings.Contains(names, "GVL_BatchLines") {
			t.Errorf("PLC1 children %s", names)
		}
	})

	t.Run("f prune with 0 and flatten with 2", func(t *testing.T) {
		unknown(t, "GVL_BatchLines.Conveyor.tmrDelay")
		unknown(t, "GVL_BatchLines.Conveyor.tmrDelay.Q")
		bus := id("GVL_BatchLines.Internal_Bus_8")
		if st := readValue(t, c, bus).StatusCode; st != ua.Good {
			t.Errorf("Internal_Bus_8 read = %v", st)
		}
		if refs := browseRefs(t, c, bus, ua.ReferenceTypeIDHasComponent, ua.BrowseDirectionForward); len(refs) != 0 {
			t.Errorf("Internal_Bus_8 has children %v", browseNames(refs))
		}
		if st := readValue(t, c, id("GVL_BatchLines.Conveyor.xRun")).StatusCode; st != ua.Good {
			t.Errorf("Conveyor.xRun read = %v", st)
		}
	})

	t.Run("g Description is unescaped", func(t *testing.T) {
		d := readAttr(t, c, id("GVL_BatchLines.nBatchCount"), ua.AttributeIDDescription).Value.(ua.LocalizedText)
		if d.Text != st301Description {
			t.Errorf("Description %q, want %q", d.Text, st301Description)
		}
	})

	t.Run("h array has ArrayDimensions", func(t *testing.T) {
		p := id("GVL_Roe.aRoe")
		if dims := readAttr(t, c, p, ua.AttributeIDArrayDimensions).Value; !reflect.DeepEqual(dims, []uint32{4}) {
			t.Errorf("ArrayDimensions %v", dims)
		}
		if v := readValue(t, c, p).Value; !reflect.DeepEqual(v, []int16{1, 2, 3, 4}) {
			t.Errorf("aRoe = %#v", v)
		}
	})

	t.Run("i timers and pointers are not published", func(t *testing.T) {
		unknown(t, "GVL_BatchLines.Drives_Line1[1].tmrStart")
		unknown(t, "GVL_BatchLines.Drives_Line1[1].pAxis")
		unknown(t, "GVL_BatchLines.Conveyor.pDrive")
		if len(ds) != 1 || ds[0].Code != CodePointerSkipped || !strings.Contains(ds[0].Message, "Conveyor.pDrive") {
			t.Errorf("diagnostics %v", ds)
		}
	})
}
