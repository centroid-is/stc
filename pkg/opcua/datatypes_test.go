package opcua

import (
	"context"
	"errors"
	"math"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
	"github.com/centroid-is/stc/pkg/types"
)

// newServer builds a Server that is never started (no listener, no 3 s
// Close grace), for registry-only tests.
func newServer(t *testing.T) *Server {
	t.Helper()
	s, err := New(testConfig(t))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	return s
}

// browseRefs returns the references of id of refType (with subtypes) in dir.
func browseRefs(t *testing.T, c *client.Client, id, refType ua.NodeID, dir ua.BrowseDirection) []ua.ReferenceDescription {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
	defer cancel()
	res, err := c.Browse(ctx, &ua.BrowseRequest{
		NodesToBrowse: []ua.BrowseDescription{{
			NodeID: id, BrowseDirection: dir, ReferenceTypeID: refType,
			IncludeSubtypes: true, ResultMask: uint32(ua.BrowseResultMaskAll),
		}},
	})
	if err != nil || len(res.Results) != 1 || res.Results[0].StatusCode.IsBad() {
		t.Fatalf("Browse %v: %v %+v", id, err, res)
	}
	return res.Results[0].References
}

// readDefinition reads the DataTypeDefinition attribute (23) of dt.
func readDefinition(t *testing.T, c *client.Client, dt ua.NodeID) any {
	t.Helper()
	dv := readAttr(t, c, dt, ua.AttributeIDDataTypeDefinition)
	if dv.StatusCode.IsBad() {
		t.Fatalf("DataTypeDefinition of %v: %v", dt, dv.StatusCode)
	}
	switch v := dv.Value.(type) {
	case *ua.StructureDefinition:
		return *v
	case *ua.EnumDefinition:
		return *v
	}
	return dv.Value
}

func structDefOf(t *testing.T, c *client.Client, dt ua.NodeID) ua.StructureDefinition {
	t.Helper()
	def, ok := readDefinition(t, c, dt).(ua.StructureDefinition)
	if !ok {
		t.Fatalf("DataTypeDefinition of %v is not a StructureDefinition", dt)
	}
	return def
}

func expandedID(e ua.ExpandedNodeID) ua.NodeID { return ua.ToNodeID(e, nil) }

func stInner() *types.StructType {
	return &types.StructType{Name: "ST_Inner", Members: []types.StructMember{
		{Name: "a", Type: types.TypeDINT}, {Name: "b", Type: types.TypeBOOL},
	}}
}

func stOuter() *types.StructType {
	return &types.StructType{Name: "ST_Outer", Members: []types.StructMember{
		{Name: "inner", Type: stInner()},
		{Name: "vals", Type: arr(types.TypeREAL, [2]int{1, 3})},
		{Name: "s", Type: types.TypeSTRING},
		{Name: "state", Type: testEnum()},
		{Name: "items", Type: arr(stInner(), [2]int{0, 1})},
	}}
}

func stDriveHMI() *types.StructType {
	return &types.StructType{Name: "ST_Drive_HMI", Members: []types.StructMember{
		{Name: "p_cmd_JogFwd", Type: types.TypeBOOL},
		{Name: "p_stat_State", Type: testEnum()},
		{Name: "p_stat_Frequency", Type: types.TypeREAL},
	}}
}

func TestEnsureEnum(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil)
	c := dialAnon(t, s)

	id, err := s.ensureEnum(testEnum())
	if err != nil {
		t.Fatal(err)
	}
	if want := ua.NewNodeIDString(4, "DT.E_State"); id != want {
		t.Fatalf("id = %v, want %v", id, want)
	}
	again, err := s.ensureEnum(&types.EnumType{Name: "e_state"})
	if err != nil || again != id {
		t.Fatalf("second ensureEnum = %v, %v", again, err)
	}

	def, ok := readDefinition(t, c, id).(ua.EnumDefinition)
	if !ok || len(def.Fields) != 3 || def.Fields[1].Name != "rdy" || def.Fields[1].Value != 2 {
		t.Fatalf("EnumDefinition = %+v", readDefinition(t, c, id))
	}
	sup := browseRefs(t, c, id, ua.ReferenceTypeIDHasSubtype, ua.BrowseDirectionInverse)
	if len(sup) != 1 || expandedID(sup[0].NodeID) != ua.DataTypeIDEnumeration {
		t.Fatalf("supertype = %+v", sup)
	}
	props := browseRefs(t, c, id, ua.ReferenceTypeIDHasProperty, ua.BrowseDirectionForward)
	if len(props) != 1 || props[0].BrowseName.Name != "EnumValues" {
		t.Fatalf("non-contiguous enum properties = %v", browseNames(props))
	}
	dv := readValue(t, c, expandedID(props[0].NodeID))
	vals, ok := dv.Value.([]ua.ExtensionObject)
	if !ok || len(vals) != 3 {
		t.Fatalf("EnumValues = %#v", dv.Value)
	}
	if ev, ok := vals[2].(ua.EnumValueType); !ok || ev.Value != 3 || ev.DisplayName.Text != "Run" {
		t.Fatalf("EnumValues[2] = %#v", vals[2])
	}

	cont := &types.EnumType{Name: "E_Mode", Values: []string{"Off", "On"}, Ordinals: []int64{0, 1}}
	mid, err := s.ensureEnum(cont)
	if err != nil {
		t.Fatal(err)
	}
	props = browseRefs(t, c, mid, ua.ReferenceTypeIDHasProperty, ua.BrowseDirectionForward)
	if len(props) != 1 || props[0].BrowseName.Name != "EnumStrings" {
		t.Fatalf("contiguous enum properties = %v", browseNames(props))
	}
	strs, ok := readValue(t, c, expandedID(props[0].NodeID)).Value.([]ua.LocalizedText)
	if !ok || len(strs) != 2 || strs[1].Text != "On" {
		t.Fatalf("EnumStrings = %#v", strs)
	}
}

func TestEnsureEnumErrors(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	for _, et := range []*types.EnumType{
		{Name: "E_Big", Values: []string{"x"}, Ordinals: []int64{math.MaxInt32 + 1}},
		{Name: "E_Bad", Values: []string{"x", "y"}, Ordinals: []int64{0}},
		{Name: ""},
	} {
		if _, err := s.ensureEnum(et); err == nil {
			t.Errorf("ensureEnum(%s): want error", et.Name)
		}
	}
	_, err := s.ensureEnum(&types.EnumType{Name: "E_Big", Values: []string{"x"}, Ordinals: []int64{math.MinInt32 - 1}})
	if !errors.Is(err, ErrOutOfRange) {
		t.Errorf("E_Big: %v, want ErrOutOfRange", err)
	}
}

func TestEnsureStruct(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil)
	c := dialAnon(t, s)

	d, err := s.ensureStruct(stDriveHMI())
	if err != nil {
		t.Fatal(err)
	}
	if want := ua.NewNodeIDString(4, "DT.ST_Drive_HMI"); d.DTID != want {
		t.Fatalf("DTID = %v", d.DTID)
	}
	// The encoding id is process-global: when another test (the ST301
	// fixture) registered a different ST_Drive_HMI layout first, this one
	// gets the layout-versioned form.
	encText, ok := d.EncID.(ua.NodeIDString)
	if !ok || encText.NamespaceIndex != 4 || !regexp.MustCompile(`^TE\.ST_Drive_HMI(\.v[0-9a-f]{8})?\.DefaultBinary$`).MatchString(encText.ID) {
		t.Fatalf("EncID = %v", d.EncID)
	}
	def := structDefOf(t, c, d.DTID)
	if def.DefaultEncodingID != d.EncID {
		t.Errorf("DefaultEncodingID = %v", def.DefaultEncodingID)
	}
	wantFields := []struct {
		name string
		dt   ua.NodeID
	}{
		{"p_cmd_JogFwd", ua.DataTypeIDBoolean},
		{"p_stat_State", ua.NewNodeIDString(4, "DT.E_State")},
		{"p_stat_Frequency", ua.DataTypeIDFloat},
	}
	if len(def.Fields) != len(wantFields) {
		t.Fatalf("fields = %+v", def.Fields)
	}
	for i, w := range wantFields {
		f := def.Fields[i]
		if f.Name != w.name || f.DataType != w.dt || f.ValueRank != ua.ValueRankScalar {
			t.Errorf("field %d = %+v, want %s %v", i, f, w.name, w.dt)
		}
	}
	enc := browseRefs(t, c, d.DTID, ua.ReferenceTypeIDHasEncoding, ua.BrowseDirectionForward)
	if len(enc) != 1 || expandedID(enc[0].NodeID) != d.EncID || enc[0].BrowseName.Name != "Default Binary" {
		t.Fatalf("HasEncoding = %+v", enc)
	}
	sup := browseRefs(t, c, d.DTID, ua.ReferenceTypeIDHasSubtype, ua.BrowseDirectionInverse)
	if len(sup) != 1 || expandedID(sup[0].NodeID) != ua.DataTypeIDStructure {
		t.Fatalf("supertype = %+v", sup)
	}

	// Nested: ST_Inner registered first and referenced by DataType.
	o, err := s.ensureStruct(stOuter())
	if err != nil {
		t.Fatal(err)
	}
	odef := structDefOf(t, c, o.DTID)
	if got := odef.Fields[0].DataType; got != ua.NewNodeIDString(4, "DT.ST_Inner") {
		t.Errorf("inner DataType = %v", got)
	}
	if f := odef.Fields[1]; f.ValueRank != ua.ValueRankOneDimension || !reflect.DeepEqual(f.ArrayDimensions, []uint32{3}) || f.DataType != ua.DataTypeIDFloat {
		t.Errorf("vals field = %+v", f)
	}
	if f := odef.Fields[4]; f.ValueRank != ua.ValueRankOneDimension || f.DataType != ua.NewNodeIDString(4, "DT.ST_Inner") {
		t.Errorf("items field = %+v", f)
	}
	idef := structDefOf(t, c, ua.NewNodeIDString(4, "DT.ST_Inner"))
	if len(idef.Fields) != 2 || idef.Fields[0].Name != "a" {
		t.Errorf("ST_Inner = %+v", idef.Fields)
	}

	// Identical layouts under different names get distinct Go types.
	twin := stInner()
	twin.Name = "ST_InnerTwin"
	tw, err := s.ensureStruct(twin)
	if err != nil {
		t.Fatal(err)
	}
	if tw.GoType == o.Fields[0].Nested.GoType {
		t.Error("ST_InnerTwin shares ST_Inner's Go type")
	}
	if same, _ := s.ensureStruct(&types.StructType{Name: "st_inner"}); same != o.Fields[0].Nested {
		t.Error("case-insensitive lookup did not reuse ST_Inner")
	}
}

func TestEnsureStructAcrossServers(t *testing.T) {
	t.Parallel()
	a, b := newServer(t), newServer(t)
	st := &types.StructType{Name: "ST_Shared", Members: []types.StructMember{{Name: "x", Type: types.TypeINT}}}
	da, err := a.ensureStruct(st)
	if err != nil {
		t.Fatal(err)
	}
	db, err := b.ensureStruct(st) // second Server, same name+layout: no registry panic
	if err != nil {
		t.Fatal(err)
	}
	if da.GoType != db.GoType || da.EncID != db.EncID {
		t.Errorf("second server: %v %v vs %v %v", da.GoType, da.EncID, db.GoType, db.EncID)
	}

	// Same name, new layout (a reload): a layout-versioned encoding id.
	c := newServer(t)
	v2 := &types.StructType{Name: "ST_Shared", Members: []types.StructMember{
		{Name: "x", Type: types.TypeINT}, {Name: "y", Type: types.TypeBOOL},
	}}
	dc, err := c.ensureStruct(v2)
	if err != nil {
		t.Fatal(err)
	}
	enc := dc.EncID.(ua.NodeIDString).ID
	if !strings.HasPrefix(enc, "TE.ST_Shared.v") || !strings.HasSuffix(enc, ".DefaultBinary") {
		t.Errorf("versioned EncID = %q", enc)
	}
	if id, ok := ua.FindBinaryEncodingIDForType(dc.GoType); !ok || id.NodeID.(ua.NodeIDString).ID != enc {
		t.Errorf("registered id = %v, %v", id, ok)
	}
}

func TestEnsureStructErrors(t *testing.T) {
	t.Parallel()
	s := newServer(t)
	self := &types.StructType{Name: "ST_Self"}
	self.Members = []types.StructMember{{Name: "me", Type: self}}
	bad := []*types.StructType{
		{Name: "ST_Ptr", Members: []types.StructMember{{Name: "p", Type: &types.PointerType{BaseType: types.TypeINT}}}},
		{Name: "ST_Ref", Members: []types.StructMember{{Name: "r", Type: &types.ReferenceType{BaseType: types.TypeINT}}}},
		{Name: "ST_FB", Members: []types.StructMember{{Name: "t", Type: &types.FunctionBlockType{Name: "TON"}}}},
		{Name: "ST_DynArr", Members: []types.StructMember{{Name: "a", Type: &types.ArrayType{ElementType: types.TypeINT, Dimensions: []types.ArrayDimension{{Text: "1..N"}}}}}},
		{Name: "ST_BadNested", Members: []types.StructMember{{Name: "n", Type: &types.StructType{Name: "ST_PtrIn", Members: []types.StructMember{{Name: "p", Type: &types.PointerType{BaseType: types.TypeINT}}}}}}},
		{Name: "ST_BadEnum", Members: []types.StructMember{{Name: "e", Type: &types.EnumType{Name: "E_X", Values: []string{"a"}}}}},
		{Name: ""},
		self,
	}
	for _, st := range bad {
		if _, err := s.ensureStruct(st); err == nil {
			t.Errorf("ensureStruct(%s): want error", st.Name)
		}
	}
	// A failed struct is not cached: a later valid one with another name works.
	if _, err := s.ensureStruct(stInner()); err != nil {
		t.Fatal(err)
	}
}
