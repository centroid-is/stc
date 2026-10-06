package opcua

import (
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
)

// fakeNode is a hand-built SymbolNode for tests. Paths are assigned by
// root(), which mirrors the SymbolNode contract: members join with ".",
// array elements append their "[i]" suffix to the array path.
type fakeNode struct {
	path, name, typeName string
	kind                 Kind
	typ                  types.Type
	attrs                []ast.Attribute
	enums                map[int64]string
	kids                 []*fakeNode
}

var _ SymbolNode = (*fakeNode)(nil)

func (f *fakeNode) Path() string                  { return f.path }
func (f *fakeNode) Name() string                  { return f.name }
func (f *fakeNode) Kind() Kind                    { return f.kind }
func (f *fakeNode) Type() types.Type              { return f.typ }
func (f *fakeNode) TypeName() string              { return f.typeName }
func (f *fakeNode) Attributes() []ast.Attribute   { return f.attrs }
func (f *fakeNode) EnumStrings() map[int64]string { return f.enums }
func (f *fakeNode) Children() []SymbolNode {
	out := make([]SymbolNode, len(f.kids))
	for i, k := range f.kids {
		if k != nil { // keep a nil child an untyped nil interface
			out[i] = k
		}
	}
	return out
}

// at builds an attribute pragma value as the parser delivers it.
func at(name, value string) ast.Attribute {
	return ast.Attribute{Name: name, Value: value, HasValue: true}
}

func da(v string) ast.Attribute                { return at("OPC.UA.DA", v) }
func acc(v string) ast.Attribute               { return at("OPC.UA.DA.Access", v) }
func descr(v string) ast.Attribute             { return at("OPC.UA.DA.Description", v) }
func structured() ast.Attribute                { return at("OPC.UA.DA.StructuredType", "1") }
func attrs(a ...ast.Attribute) []ast.Attribute { return a }

func typeNameOf(t types.Type) string {
	if t == nil {
		return ""
	}
	return t.String()
}

func node(kind Kind, name string, typ types.Type, as []ast.Attribute, kids ...*fakeNode) *fakeNode {
	return &fakeNode{name: name, kind: kind, typ: typ, typeName: typeNameOf(typ), attrs: as, kids: kids}
}

func gvl(name string, kids ...*fakeNode) *fakeNode { return node(KindGVL, name, nil, nil, kids...) }
func prog(name string, kids ...*fakeNode) *fakeNode {
	return node(KindProgram, name, nil, nil, kids...)
}

func scalar(name string, typ types.Type, as ...ast.Attribute) *fakeNode {
	return node(KindScalar, name, typ, as)
}

func enumVar(name string, et *types.EnumType, as ...ast.Attribute) *fakeNode {
	n := node(KindEnum, name, et, as)
	n.enums = map[int64]string{}
	for i, v := range et.Values {
		if i < len(et.Ordinals) {
			n.enums[et.Ordinals[i]] = v
		}
	}
	return n
}

func fbInst(name, fbType string, as []ast.Attribute, kids ...*fakeNode) *fakeNode {
	return node(KindFBInstance, name, &types.FunctionBlockType{Name: fbType}, as, kids...)
}

func structInst(name string, st *types.StructType, as []ast.Attribute, kids ...*fakeNode) *fakeNode {
	return node(KindStruct, name, st, as, kids...)
}

func arrayOf(name string, at *types.ArrayType, as []ast.Attribute, elems ...*fakeNode) *fakeNode {
	return node(KindArray, name, at, as, elems...)
}

func ptr(name string, as ...ast.Attribute) *fakeNode {
	return node(KindPointer, name, &types.PointerType{BaseType: types.TypeINT}, as)
}

func refVar(name string, as ...ast.Attribute) *fakeNode {
	return node(KindReference, name, &types.ReferenceType{BaseType: types.TypeINT}, as)
}

// elems builds array elements "<arr>[lo]".."<arr>[hi]" with mk.
func elems(arrName string, lo, hi int, mk func(name string) *fakeNode) []*fakeNode {
	var out []*fakeNode
	for i := lo; i <= hi; i++ {
		out = append(out, mk(arrName+"["+itoa(i)+"]"))
	}
	return out
}

// root wraps top-level GVLs and PROGRAMs and assigns every path.
func root(kids ...*fakeNode) *fakeNode {
	r := &fakeNode{kind: KindRoot, kids: kids}
	for _, k := range kids {
		if k != nil {
			setPaths(k, r)
		}
	}
	return r
}

func setPaths(n, parent *fakeNode) {
	switch {
	case parent.kind == KindRoot:
		n.path = n.name
	case parent.kind == KindArray:
		n.path = parent.path + strings.TrimPrefix(n.name, parent.name)
	default:
		n.path = parent.path + "." + n.name
	}
	for _, k := range n.kids {
		setPaths(k, n)
	}
}

// tonInst is a standard TON instance with its interface members.
func tonInst(name string, as ...ast.Attribute) *fakeNode {
	return fbInst(name, "TON", as,
		scalar("IN", types.TypeBOOL), scalar("PT", types.TypeTIME),
		scalar("Q", types.TypeBOOL), scalar("ET", types.TypeTIME))
}

// zeroValue returns the canonical IEC zero value MapSource holds for t.
func zeroValue(t types.Type) any {
	switch tt := t.(type) {
	case *types.EnumType:
		return int64(0)
	case *types.ArrayType:
		n, _ := arrayLen(tt)
		out := make([]any, n)
		for i := range out {
			out[i] = zeroValue(tt.ElementType)
		}
		return out
	case *types.PrimitiveType:
		switch tt.Kind_ {
		case types.KindBOOL:
			return false
		case types.KindREAL, types.KindLREAL:
			return float32(0)
		case types.KindSTRING, types.KindWSTRING:
			return ""
		case types.KindTIME, types.KindTOD:
			return time.Duration(0)
		case types.KindDATE, types.KindDT:
			return time.Time{}
		}
	}
	return int64(0)
}

// seedLeaves stores a zero value for every scalar, enum and array-of-
// elementary leaf below n.
func seedLeaves(n *fakeNode, vals map[string]any) {
	switch n.kind {
	case KindScalar, KindEnum:
		vals[n.path] = zeroValue(n.typ)
		return
	case KindArray:
		if at, ok := n.typ.(*types.ArrayType); ok {
			if _, leaf := uaGoType(at); leaf {
				vals[n.path] = zeroValue(at)
				return
			}
		}
	}
	for _, k := range n.kids {
		seedLeaves(k, vals)
	}
}

// --- ST301-shaped fixture ---

func e(name string, vals ...string) *types.EnumType {
	et := &types.EnumType{Name: name, BaseType: types.KindINT}
	for i, v := range vals {
		et.Values = append(et.Values, v)
		et.Ordinals = append(et.Ordinals, int64(i))
	}
	return et
}

var (
	eDriveState = e("E_DriveState", "init", "cnf", "rdy", "run")
	e402State   = e("E_402State", "not_ready", "switch_on_disabled", "ready_to_switch_on", "fault", "switched_on", "operation_enabled")

	st301DriveHMI = &types.StructType{Name: "ST_Drive_HMI", Members: []types.StructMember{
		{Name: "p_cmd_JogFwd", Type: types.TypeBOOL},
		{Name: "p_stat_Error", Type: types.TypeBOOL},
		{Name: "p_stat_State", Type: eDriveState},
		{Name: "p_stat_402_State", Type: e402State},
		{Name: "p_stat_Frequency", Type: types.TypeREAL},
		{Name: "p_stat_SlaveId", Type: types.TypeUINT},
		{Name: "p_cfg_AutoFreq", Type: types.TypeREAL},
	}}
	stSensorHMI = &types.StructType{Name: "ST_Sensor_HMI", Members: []types.StructMember{
		{Name: "p_stat_xRaw", Type: types.TypeBOOL},
		{Name: "p_cfg_Invert", Type: types.TypeBOOL},
	}}
	stBus = &types.StructType{Name: "ST_Bus", Members: []types.StructMember{
		{Name: "nNodes", Type: types.TypeINT},
		{Name: "xOk", Type: types.TypeBOOL},
	}}
	stConveyorCfg = &types.StructType{Name: "ST_ConveyorCfg", Members: []types.StructMember{
		{Name: "rMax", Type: types.TypeREAL},
		{Name: "eMode", Type: eDriveState},
	}}
)

// fbDrive instantiates FB_Drive: its member HMI carries OPC.UA.DA '1' in
// the FB body (type level), ST_Drive_HMI carries StructuredType on its
// TYPE header; the timer and the pointer are internal.
func fbDrive(name string, as ...ast.Attribute) *fakeNode {
	return fbInst(name, "FB_Drive", as,
		structInst("HMI", st301DriveHMI, attrs(structured(), da("1")),
			scalar("p_cmd_JogFwd", types.TypeBOOL),
			scalar("p_stat_Error", types.TypeBOOL, acc("1"), descr("Drive fault")),
			enumVar("p_stat_State", eDriveState, acc("1")),
			enumVar("p_stat_402_State", e402State, acc("1")),
			scalar("p_stat_Frequency", types.TypeREAL, acc("1")),
			scalar("p_stat_SlaveId", types.TypeUINT, acc("1")),
			scalar("p_cfg_AutoFreq", types.TypeREAL),
		),
		tonInst("tmrStart"),
		ptr("pAxis"),
	)
}

func fbSensor(name string) *fakeNode {
	return fbInst(name, "FB_Sensor", nil,
		structInst("HMI", stSensorHMI, attrs(structured(), da("1")),
			scalar("p_stat_xRaw", types.TypeBOOL, acc("1")),
			scalar("p_cfg_Invert", types.TypeBOOL),
		),
		scalar("xRawIn", types.TypeBOOL),
	)
}

// st301Description is the escaped Description of GVL_BatchLines.nBatchCount.
const (
	st301DescriptionRaw = "Batches on line $'1$' ($$ counted)"
	st301Description    = "Batches on line '1' ($ counted)"
)

// st301Fixture mirrors the shapes the sildarvinnsla HMI reads from ST301.
func st301Fixture() (SymbolNode, *MapSource) {
	r := root(
		gvl("GVL_BatchLines",
			arrayOf("Drives_Line1", arr(&types.FunctionBlockType{Name: "FB_Drive"}, [2]int{1, 2}), nil,
				elems("Drives_Line1", 1, 2, func(n string) *fakeNode { return fbDrive(n) })...),
			structInst("Internal_Bus_8", stBus, attrs(da("2")),
				scalar("nNodes", types.TypeINT), scalar("xOk", types.TypeBOOL)),
			scalar("nBatchCount", types.TypeDINT, da("1"), descr(st301DescriptionRaw)),
			fbInst("Conveyor", "FB_Conveyor", attrs(da("1")),
				scalar("xRun", types.TypeBOOL),
				tonInst("tmrDelay", da("0")),
				ptr("pDrive"),
				scalar("rSpeed", types.TypeREAL, acc("1")),
				structInst("Cfg", stConveyorCfg, nil,
					scalar("rMax", types.TypeREAL), enumVar("eMode", eDriveState)),
			),
		),
		gvl("sensors",
			fbSensor("EPW01_WA01_IS11"),
			fbSensor("EPW01_WA01_IS12"),
		),
		prog("MAIN",
			scalar("nCycle", types.TypeDINT),
			scalar("xInit", types.TypeBOOL),
			tonInst("tmrCycle"),
		),
		gvl("GVL_Roe",
			arrayOf("aRoe", arr(types.TypeINT, [2]int{0, 3}), attrs(da("1"))),
		),
	)
	vals := map[string]any{}
	seedLeaves(r, vals)
	hmi := "GVL_BatchLines.Drives_Line1[1].HMI"
	vals[hmi+".p_stat_State"] = int64(2)
	vals[hmi+".p_stat_402_State"] = int64(4)
	vals[hmi+".p_stat_Frequency"] = float32(49.5)
	vals[hmi+".p_stat_SlaveId"] = uint64(1001)
	vals["GVL_BatchLines.nBatchCount"] = int64(17)
	vals["GVL_Roe.aRoe"] = []any{int64(1), int64(2), int64(3), int64(4)}
	return r, NewMapSource(vals)
}
