package opcua

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
	"github.com/centroid-is/stc/pkg/types"
)

// countingSource counts Snapshot and Read calls of a MapSource.
type countingSource struct {
	*MapSource
	snaps, reads atomic.Int32
}

func (c *countingSource) Snapshot(paths []string) (map[string]any, error) {
	c.snaps.Add(1)
	return c.MapSource.Snapshot(paths)
}

func (c *countingSource) Read(path string) (any, error) {
	c.reads.Add(1)
	return c.MapSource.Read(path)
}

func writeAttr(t *testing.T, c *client.Client, id ua.NodeID, v any, indexRange string) ua.StatusCode {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
	defer cancel()
	res, err := c.Write(ctx, &ua.WriteRequest{NodesToWrite: []ua.WriteValue{{
		NodeID: id, AttributeID: ua.AttributeIDValue, IndexRange: indexRange,
		Value: ua.NewDataValue(v, ua.Good, time.Time{}, 0, time.Time{}, 0),
	}}})
	if err != nil || len(res.Results) != 1 {
		t.Fatalf("Write %v: %v %+v", id, err, res)
	}
	return res.Results[0]
}

var testDT = time.Date(2026, 10, 6, 8, 30, 0, 0, time.UTC)

// scalarRow is one OPCUA-06 row published as GVL_Test.<name>.
type scalarRow struct {
	name string
	typ  types.Type
	src  any
	dt   ua.NodeID
	want any
}

var scalarRows = []scalarRow{
	{"b", types.TypeBOOL, true, ua.DataTypeIDBoolean, true},
	{"si", types.TypeSINT, int64(-5), ua.DataTypeIDSByte, int8(-5)},
	{"us", types.TypeUSINT, uint64(200), ua.DataTypeIDByte, uint8(200)},
	{"by", types.TypeBYTE, uint64(255), ua.DataTypeIDByte, uint8(255)},
	{"ch", types.TypeCHAR, "A", ua.DataTypeIDByte, uint8(65)},
	{"i", types.TypeINT, int64(300), ua.DataTypeIDInt16, int16(300)},
	{"ui", types.TypeUINT, uint64(60000), ua.DataTypeIDUInt16, uint16(60000)},
	{"w", types.TypeWORD, uint64(7), ua.DataTypeIDUInt16, uint16(7)},
	{"wc", types.TypeWCHAR, uint64(0x263A), ua.DataTypeIDUInt16, uint16(0x263A)},
	{"di", types.TypeDINT, int64(-70000), ua.DataTypeIDInt32, int32(-70000)},
	{"ud", types.TypeUDINT, uint64(70000), ua.DataTypeIDUInt32, uint32(70000)},
	{"dw", types.TypeDWORD, uint64(1), ua.DataTypeIDUInt32, uint32(1)},
	{"li", types.TypeLINT, int64(1 << 40), ua.DataTypeIDInt64, int64(1 << 40)},
	{"ul", types.TypeULINT, uint64(1 << 63), ua.DataTypeIDUInt64, uint64(1 << 63)},
	{"lw", types.TypeLWORD, uint64(3), ua.DataTypeIDUInt64, uint64(3)},
	{"r", types.TypeREAL, 1.5, ua.DataTypeIDFloat, float32(1.5)},
	{"lr", types.TypeLREAL, 2.25, ua.DataTypeIDDouble, float64(2.25)},
	{"s", types.TypeSTRING, "hi", ua.DataTypeIDString, "hi"},
	{"ws", types.TypeWSTRING, "hæ", ua.DataTypeIDString, "hæ"},
	{"t", types.TypeTIME, 1500 * time.Millisecond, ua.DataTypeIDInt64, int64(1500)},
	{"tod", types.TypeTOD, time.Hour, ua.DataTypeIDUInt32, uint32(3600000)},
	{"dt", types.TypeDT, testDT, ua.DataTypeIDDateTime, testDT},
	{"date", types.TypeDATE, testDT, ua.DataTypeIDDateTime, testDT},
}

// testSpace mirrors the spike's demo model: GVL_Test with fbMotor.HMI as a
// structured Variable whose members are child Variables.
func testSpace() (*Space, map[string]any) {
	hmi := "GVL_Test.fbMotor.HMI"
	sp := &Space{Nodes: []NodeSpec{
		{Path: "GVL_Test", Name: "GVL_Test", Class: NodeObject},
		{Path: "GVL_Test.fbMotor", Name: "fbMotor", Parent: "GVL_Test", Class: NodeObject},
		{Path: hmi, Name: "HMI", Parent: "GVL_Test.fbMotor", Class: NodeVariable, Type: stDriveHMI(), Structured: true},
		{Path: hmi + ".p_cmd_JogFwd", Name: "p_cmd_JogFwd", Parent: hmi, Class: NodeVariable, Type: types.TypeBOOL},
		{Path: hmi + ".p_stat_State", Name: "p_stat_State", Parent: hmi, Class: NodeVariable, Type: testEnum(), Access: AccessRead},
		{Path: hmi + ".p_stat_Frequency", Name: "p_stat_Frequency", Parent: hmi, Class: NodeVariable, Type: types.TypeREAL,
			Access: AccessRead, Description: "Output frequency [Hz]"},
		{Path: "GVL_Test.arr", Name: "arr", Parent: "GVL_Test", Class: NodeVariable, Type: arr(types.TypeINT, [2]int{0, 4})},
		{Path: "GVL_Test.grid", Name: "grid", Parent: "GVL_Test", Class: NodeVariable, Type: arr(types.TypeBOOL, [2]int{1, 2}, [2]int{1, 2})},
		{Path: "GVL_Test.states", Name: "states", Parent: "GVL_Test", Class: NodeVariable, Type: arr(testEnum(), [2]int{1, 2})},
		{Path: "GVL_Test.state", Name: "state", Parent: "GVL_Test", Class: NodeVariable, Type: testEnum()},
		{Path: "GVL_Test.ro", Name: "ro", Parent: "GVL_Test", Class: NodeVariable, Type: types.TypeINT, Access: AccessRead},
		{Path: "GVL_Test.wo", Name: "wo", Parent: "GVL_Test", Class: NodeVariable, Type: types.TypeINT, Access: AccessWrite},
		{Path: "GVL_Test.rw", Name: "rw", Parent: "GVL_Test", Class: NodeVariable, Type: types.TypeINT, Access: AccessReadWrite},
		{Path: "GVL_Test.big", Name: "big", Parent: "GVL_Test", Class: NodeVariable, Type: types.TypeINT},
		{Path: "GVL_Test.missing", Name: "missing", Parent: "GVL_Test", Class: NodeVariable, Type: types.TypeINT},
		{Path: "GVL_Test.badtype", Name: "badtype", Parent: "GVL_Test", Class: NodeVariable, Type: types.TypeINT},
		{Path: "GVL_Test.outer", Name: "outer", Parent: "GVL_Test", Class: NodeVariable, Type: stOuter(), Structured: true},
		{Path: "GVL_Test.outer.inner", Name: "inner", Parent: "GVL_Test.outer", Class: NodeVariable, Type: stInner(), Structured: true, Access: AccessRead},
		{Path: "GVL_Test.broken", Name: "broken", Parent: "GVL_Test", Class: NodeVariable, Type: stInner(), Structured: true},
		{Path: "GVL_Test.badleaf", Name: "badleaf", Parent: "GVL_Test", Class: NodeVariable, Type: stInner(), Structured: true},
		{Path: "Top", Name: "Top", Class: NodeVariable, Type: types.TypeDINT, Description: "top-level"},
	}}
	vals := map[string]any{
		hmi + ".p_cmd_JogFwd":       false,
		hmi + ".p_stat_State":       "rdy",
		hmi + ".p_stat_Frequency":   float32(49.5),
		"GVL_Test.arr":              []any{int64(1), int64(2), int64(3), int64(4), int64(5)},
		"GVL_Test.grid":             []any{true, false, false, true},
		"GVL_Test.states":           []any{"Idle", int64(3)},
		"GVL_Test.state":            int64(2),
		"GVL_Test.ro":               int64(1),
		"GVL_Test.wo":               int64(2),
		"GVL_Test.rw":               int64(3),
		"GVL_Test.big":              int64(70000),
		"GVL_Test.badtype":          "x",
		"GVL_Test.outer.inner.a":    int64(-7),
		"GVL_Test.outer.inner.b":    true,
		"GVL_Test.outer.vals":       []any{1.5, 2.5, 3.5},
		"GVL_Test.outer.s":          "héllo",
		"GVL_Test.outer.state":      "Run",
		"GVL_Test.outer.items[0].a": int64(1),
		"GVL_Test.outer.items[0].b": false,
		"GVL_Test.outer.items[1].a": int64(2),
		"GVL_Test.outer.items[1].b": true,
		"GVL_Test.broken.a":         int64(1), // .b missing: Snapshot fails
		"GVL_Test.badleaf.a":        "nope",
		"GVL_Test.badleaf.b":        true,
		"Top":                       int64(42),
	}
	for _, r := range scalarRows {
		p := "GVL_Test." + r.name
		sp.Nodes = append(sp.Nodes, NodeSpec{Path: p, Name: r.name, Parent: "GVL_Test", Class: NodeVariable, Type: r.typ})
		vals[p] = r.src
	}
	return sp, vals
}

func TestPublish(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil)
	sp, vals := testSpace()
	src := &countingSource{MapSource: NewMapSource(vals)}
	if err := s.Publish(sp, src); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	c := dialAnon(t, s)
	id := func(p string) ua.NodeID { return ua.NewNodeIDString(4, p) }
	hmi := "GVL_Test.fbMotor.HMI"

	t.Run("browse", func(t *testing.T) {
		names := browseNames(browseForward(t, c, ua.NewNodeIDString(4, "PLC1")))
		joined := strings.Join(names, ",")
		if !strings.Contains(joined, "GVL_Test") || !strings.Contains(joined, "Top") {
			t.Errorf("PLC1 children = %v", names)
		}
		got := browseNames(browseForward(t, c, id(hmi)))
		want := []string{"p_cmd_JogFwd", "p_stat_State", "p_stat_Frequency"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("HMI children = %v, want %v", got, want)
		}
		if bn := readAttr(t, c, id(hmi), ua.AttributeIDBrowseName).Value.(ua.QualifiedName); bn != ua.NewQualifiedName(4, "HMI") {
			t.Errorf("BrowseName = %v", bn)
		}
	})

	t.Run("scalars", func(t *testing.T) {
		for _, r := range scalarRows {
			p := id("GVL_Test." + r.name)
			if dt := readAttr(t, c, p, ua.AttributeIDDataType).Value; dt != r.dt {
				t.Errorf("%s DataType = %v, want %v", r.name, dt, r.dt)
			}
			dv := readValue(t, c, p)
			if dv.StatusCode != ua.Good {
				t.Errorf("%s status %v", r.name, dv.StatusCode)
				continue
			}
			got := dv.Value
			if tm, ok := got.(time.Time); ok {
				got = tm.UTC()
			}
			if !reflect.DeepEqual(got, r.want) {
				t.Errorf("%s = %#v (%T), want %#v (%T)", r.name, got, got, r.want, r.want)
			}
		}
	})

	t.Run("access and description", func(t *testing.T) {
		for p, want := range map[string]byte{"GVL_Test.ro": 1, "GVL_Test.wo": 2, "GVL_Test.rw": 3, "GVL_Test.i": 3} {
			for _, attr := range []uint32{ua.AttributeIDAccessLevel, ua.AttributeIDUserAccessLevel} {
				if got := readAttr(t, c, id(p), attr).Value; got != want {
					t.Errorf("%s attr %d = %v, want %d", p, attr, got, want)
				}
			}
		}
		if got := writeAttr(t, c, id("GVL_Test.ro"), int16(5), ""); got != ua.BadNotWritable {
			t.Errorf("write read-only = %v", got)
		}
		if got := writeAttr(t, c, id(hmi+".p_stat_Frequency"), float32(1), ""); got != ua.BadNotWritable {
			t.Errorf("write p_stat_Frequency = %v", got)
		}
		d := readAttr(t, c, id(hmi+".p_stat_Frequency"), ua.AttributeIDDescription).Value.(ua.LocalizedText)
		if d.Text != "Output frequency [Hz]" {
			t.Errorf("Description = %q", d.Text)
		}
		if d := readAttr(t, c, id("Top"), ua.AttributeIDDescription).Value.(ua.LocalizedText); d.Text != "top-level" {
			t.Errorf("Top Description = %q", d.Text)
		}
	})

	t.Run("writes", func(t *testing.T) {
		if got := writeAttr(t, c, id("GVL_Test.t"), int64(2500), ""); got != ua.Good {
			t.Fatalf("write TIME = %v", got)
		}
		if v, _ := src.Get("GVL_Test.t"); v != 2500*time.Millisecond {
			t.Errorf("TIME stored %#v", v)
		}
		if got := writeAttr(t, c, id("GVL_Test.wo"), int16(-4), ""); got != ua.Good {
			t.Errorf("write write-only = %v", got)
		}
		if v, _ := src.Get("GVL_Test.wo"); v != int64(-4) {
			t.Errorf("INT stored %#v", v)
		}
		if got := writeAttr(t, c, id("GVL_Test.tod"), uint32(1000), ""); got != ua.Good {
			t.Errorf("write TOD = %v", got)
		}
		if v, _ := src.Get("GVL_Test.tod"); v != time.Second {
			t.Errorf("TOD stored %#v", v)
		}
		if got := writeAttr(t, c, id("GVL_Test.i"), "nope", ""); got != ua.BadTypeMismatch {
			t.Errorf("wrong builtin = %v", got)
		}
		if got := writeAttr(t, c, id("GVL_Test.state"), int32(3), ""); got != ua.Good {
			t.Errorf("enum write = %v", got)
		}
		if v, _ := src.Get("GVL_Test.state"); v != int64(3) {
			t.Errorf("enum stored %#v", v)
		}
		if got := writeAttr(t, c, id("GVL_Test.state"), int32(1), ""); got != ua.BadOutOfRange {
			t.Errorf("enum unknown ordinal = %v", got)
		}
		if got := writeAttr(t, c, id("GVL_Test.arr"), []int16{1}, "1"); got != ua.BadWriteNotSupported {
			t.Errorf("IndexRange write = %v", got)
		}
		if got := writeAttr(t, c, id("GVL_Test.arr"), []int16{9, 8, 7, 6, 5}, ""); got != ua.Good {
			t.Errorf("array write = %v", got)
		}
		if v, _ := src.Get("GVL_Test.arr"); !reflect.DeepEqual(v, []any{int64(9), int64(8), int64(7), int64(6), int64(5)}) {
			t.Errorf("array stored %#v", v)
		}
		if got := writeAttr(t, c, id("GVL_Test.arr"), []int16{1, 2}, ""); got != ua.BadTypeMismatch {
			t.Errorf("short array write = %v", got)
		}
		for err, want := range map[error]ua.StatusCode{
			ErrNotWritable:                ua.BadNotWritable,
			ErrOutOfRange:                 ua.BadOutOfRange,
			ErrUnknownSymbol:              ua.BadNodeIDUnknown,
			errors.New("runtime stopped"): ua.BadInternalError,
		} {
			src.FailWrite("GVL_Test.rw", err)
			if got := writeAttr(t, c, id("GVL_Test.rw"), int16(1), ""); got != want {
				t.Errorf("FailWrite(%v) = %v, want %v", err, got, want)
			}
		}
		src.FailWrite("GVL_Test.rw", nil)
	})

	t.Run("read errors", func(t *testing.T) {
		for p, want := range map[string]ua.StatusCode{
			"GVL_Test.big":     ua.BadOutOfRange,
			"GVL_Test.missing": ua.BadNoData,
			"GVL_Test.badtype": ua.BadNoData,
			"GVL_Test.broken":  ua.BadNoData,
			"GVL_Test.badleaf": ua.BadNoData,
		} {
			if got := readValue(t, c, id(p)).StatusCode; got != want {
				t.Errorf("%s read = %v, want %v", p, got, want)
			}
		}
	})

	t.Run("enum and arrays", func(t *testing.T) {
		if dt := readAttr(t, c, id("GVL_Test.state"), ua.AttributeIDDataType).Value; dt != id("DT.E_State") {
			t.Errorf("enum DataType = %v", dt)
		}
		props := browseRefs(t, c, id("GVL_Test.state"), ua.ReferenceTypeIDHasProperty, ua.BrowseDirectionForward)
		if len(props) != 1 || props[0].BrowseName.Name != "EnumValues" || expandedID(props[0].NodeID) != id("GVL_Test.state.EnumValues") {
			t.Errorf("enum property = %+v", props)
		}
		if v := readValue(t, c, id(hmi+".p_stat_State")).Value; v != int32(2) {
			t.Errorf("p_stat_State = %#v", v)
		}
		if r := readAttr(t, c, id("GVL_Test.arr"), ua.AttributeIDValueRank).Value; r != int32(1) {
			t.Errorf("ValueRank = %v", r)
		}
		if d := readAttr(t, c, id("GVL_Test.arr"), ua.AttributeIDArrayDimensions).Value; !reflect.DeepEqual(d, []uint32{5}) {
			t.Errorf("ArrayDimensions = %#v", d)
		}
		if d := readAttr(t, c, id("GVL_Test.grid"), ua.AttributeIDArrayDimensions).Value; !reflect.DeepEqual(d, []uint32{4}) {
			t.Errorf("grid ArrayDimensions = %#v", d)
		}
		if v := readValue(t, c, id("GVL_Test.grid")).Value; !reflect.DeepEqual(v, []bool{true, false, false, true}) {
			t.Errorf("grid = %#v", v)
		}
		if dt := readAttr(t, c, id("GVL_Test.states"), ua.AttributeIDDataType).Value; dt != id("DT.E_State") {
			t.Errorf("enum array DataType = %v", dt)
		}
		if v := readValue(t, c, id("GVL_Test.states")).Value; !reflect.DeepEqual(v, []int32{0, 3}) {
			t.Errorf("states = %#v", v)
		}
	})

	t.Run("structured", func(t *testing.T) {
		d, _ := s.ensureStruct(stDriveHMI())
		if dt := readAttr(t, c, id(hmi), ua.AttributeIDDataType).Value; dt != d.DTID {
			t.Errorf("HMI DataType = %v", dt)
		}
		src.snaps.Store(0)
		src.reads.Store(0)
		dv := readValue(t, c, id(hmi))
		if dv.StatusCode != ua.Good {
			t.Fatalf("HMI read %v", dv.StatusCode)
		}
		if n, r := src.snaps.Load(), src.reads.Load(); n != 1 || r != 0 {
			t.Errorf("HMI read used %d Snapshot and %d Read calls, want 1 and 0", n, r)
		}
		v := reflect.ValueOf(dv.Value)
		if v.Type() != d.GoType {
			t.Fatalf("HMI value type %T", dv.Value)
		}
		if v.Field(0).Bool() || v.Field(1).Int() != 2 || v.Field(2).Float() != 49.5 {
			t.Errorf("HMI = %+v", dv.Value)
		}

		// A member write is visible in the next struct read.
		if got := writeAttr(t, c, id(hmi+".p_cmd_JogFwd"), true, ""); got != ua.Good {
			t.Fatalf("member write = %v", got)
		}
		if v := reflect.ValueOf(readValue(t, c, id(hmi)).Value); !v.Field(0).Bool() {
			t.Error("member write not visible in struct read")
		}

		// A whole-struct write splits into writable members only.
		before := len(src.Writes())
		nv := reflect.New(d.GoType).Elem()
		nv.Field(0).SetBool(false)
		nv.Field(1).SetInt(3)
		nv.Field(2).SetFloat(99)
		if got := writeAttr(t, c, id(hmi), nv.Interface(), ""); got != ua.Good {
			t.Fatalf("struct write = %v", got)
		}
		ws := src.Writes()[before:]
		if len(ws) != 1 || ws[0].Path != hmi+".p_cmd_JogFwd" || ws[0].Value != false {
			t.Errorf("struct write produced %+v", ws)
		}
		if v, _ := src.Get(hmi + ".p_stat_State"); v != "rdy" {
			t.Errorf("read-only member changed to %#v", v)
		}
		if got := writeAttr(t, c, id(hmi), int32(1), ""); got != ua.BadTypeMismatch {
			t.Errorf("struct write wrong type = %v", got)
		}
		if got := writeAttr(t, c, id(hmi), nv.Interface(), "0"); got != ua.BadWriteNotSupported {
			t.Errorf("struct IndexRange write = %v", got)
		}
		src.FailWrite(hmi+".p_cmd_JogFwd", ErrNotWritable)
		if got := writeAttr(t, c, id(hmi), nv.Interface(), ""); got != ua.BadNotWritable {
			t.Errorf("struct write member failure = %v", got)
		}
		src.FailWrite(hmi+".p_cmd_JogFwd", nil)
	})

	t.Run("nested structured", func(t *testing.T) {
		o, _ := s.ensureStruct(stOuter())
		dv := readValue(t, c, id("GVL_Test.outer"))
		if dv.StatusCode != ua.Good {
			t.Fatalf("outer read %v", dv.StatusCode)
		}
		v := reflect.ValueOf(dv.Value)
		if v.Type() != o.GoType || v.Field(0).Field(0).Int() != -7 || v.Field(2).String() != "héllo" ||
			v.Field(3).Int() != 3 || v.Field(4).Index(1).Field(0).Int() != 2 || v.Field(1).Index(2).Float() != 3.5 {
			t.Fatalf("outer = %+v", dv.Value)
		}
		// Unpublished members inherit the outer node's access; the
		// read-only inner member is skipped.
		nv := reflect.New(o.GoType).Elem()
		nv.Set(v)
		nv.Field(0).Field(0).SetInt(100)
		nv.Field(2).SetString("new")
		nv.Field(4).Index(0).Field(0).SetInt(11)
		if got := writeAttr(t, c, id("GVL_Test.outer"), nv.Interface(), ""); got != ua.Good {
			t.Fatalf("outer write = %v", got)
		}
		if v, _ := src.Get("GVL_Test.outer.inner.a"); v != int64(-7) {
			t.Errorf("read-only inner.a changed to %#v", v)
		}
		if v, _ := src.Get("GVL_Test.outer.s"); v != "new" {
			t.Errorf("outer.s = %#v", v)
		}
		if v, _ := src.Get("GVL_Test.outer.items[0].a"); v != int64(11) {
			t.Errorf("items[0].a = %#v", v)
		}
		if v, _ := src.Get("GVL_Test.outer.vals"); !reflect.DeepEqual(v, []any{float32(1.5), float32(2.5), float32(3.5)}) {
			t.Errorf("outer.vals = %#v", v)
		}
	})
}

func TestPublishValidation(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	src := NewMapSource(nil)
	obj := func(p, parent string) NodeSpec {
		return NodeSpec{Path: p, Name: p, Parent: parent, Class: NodeObject}
	}
	bad := map[string]*Space{
		"child before parent":   {Nodes: []NodeSpec{obj("A.b", "A"), obj("A", "")}},
		"duplicate":             {Nodes: []NodeSpec{obj("A", ""), obj("A", "")}},
		"empty path":            {Nodes: []NodeSpec{{Name: "x"}}},
		"empty name":            {Nodes: []NodeSpec{{Path: "x"}}},
		"bad class":             {Nodes: []NodeSpec{{Path: "x", Name: "x", Class: 7}}},
		"untyped variable":      {Nodes: []NodeSpec{{Path: "x", Name: "x", Class: NodeVariable}}},
		"structured scalar":     {Nodes: []NodeSpec{{Path: "x", Name: "x", Class: NodeVariable, Type: types.TypeINT, Structured: true}}},
		"struct not structured": {Nodes: []NodeSpec{{Path: "x", Name: "x", Class: NodeVariable, Type: stInner()}}},
		"pointer":               {Nodes: []NodeSpec{{Path: "x", Name: "x", Class: NodeVariable, Type: &types.PointerType{BaseType: types.TypeINT}}}},
		"bad struct":            {Nodes: []NodeSpec{{Path: "x", Name: "x", Class: NodeVariable, Structured: true, Type: &types.StructType{Name: "ST_P", Members: []types.StructMember{{Name: "p", Type: &types.PointerType{BaseType: types.TypeINT}}}}}}},
		"bad enum":              {Nodes: []NodeSpec{{Path: "x", Name: "x", Class: NodeVariable, Type: &types.EnumType{Name: "E_Q", Values: []string{"a"}}}}},
		"bad enum array":        {Nodes: []NodeSpec{{Path: "x", Name: "x", Class: NodeVariable, Type: arr(&types.EnumType{Name: "E_R", Values: []string{"a"}}, [2]int{0, 1})}}},
		"struct array":          {Nodes: []NodeSpec{{Path: "x", Name: "x", Class: NodeVariable, Type: arr(stInner(), [2]int{0, 1})}}},
		"dynamic array":         {Nodes: []NodeSpec{{Path: "x", Name: "x", Class: NodeVariable, Type: &types.ArrayType{ElementType: types.TypeINT, Dimensions: []types.ArrayDimension{{Text: "1..N"}}}}}},
	}
	for name, sp := range bad {
		if err := s.Publish(sp, src); err == nil {
			t.Errorf("%s: Publish succeeded, want error", name)
		}
	}
	if err := s.Publish(nil, src); err == nil {
		t.Error("nil Space accepted")
	}
	if err := s.Publish(&Space{}, nil); err == nil {
		t.Error("nil NodeSource accepted")
	}
	// The same path twice across Publish calls fails in AddNodes.
	if err := s.Publish(&Space{Nodes: []NodeSpec{obj("Dup", "")}}, src); err != nil {
		t.Fatal(err)
	}
	if err := s.Publish(&Space{Nodes: []NodeSpec{obj("Dup", "")}}, src); err == nil {
		t.Error("re-publishing a node id succeeded")
	}
	if sp := (&Space{Nodes: []NodeSpec{obj("A", "")}}); func() bool { _, ok := sp.Find("B"); return ok }() {
		t.Error("Find of a missing path succeeded")
	}
}

func TestStatusFor(t *testing.T) {
	if statusFor(nil) != ua.Good || statusFor(ErrTypeMismatch) != ua.BadTypeMismatch {
		t.Error("statusFor")
	}
	if got := elementPaths("a", arr(types.TypeINT, [2]int{1, 2}, [2]int{0, 1})); !reflect.DeepEqual(got, []string{"a[1,0]", "a[1,1]", "a[2,0]", "a[2,1]"}) {
		t.Errorf("elementPaths = %v", got)
	}
}
