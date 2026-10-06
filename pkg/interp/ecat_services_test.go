package interp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ecatStubFile is the Tc2_EtherCAT stub library on this branch.
func ecatStubFile() string {
	return filepath.Join("..", "..", "stdlib", "vendor", "beckhoff", "tc2_ethercat.st")
}

// ecatEngine builds an engine from the Tc2_EtherCAT stubs plus src (which
// holds exactly one PROGRAM) and attaches net when it is not nil.
func ecatEngine(t *testing.T, src string, net *ecat.Network) *ScanCycleEngine {
	t.Helper()
	stub, err := os.ReadFile(ecatStubFile())
	require.NoError(t, err)
	var prog *ast.ProgramDecl
	var gvls []*ast.GVLDecl
	typeDecls := map[string]ast.TypeSpec{}
	fbDecls := map[string]*ast.FunctionBlockDecl{}
	for name, text := range map[string]string{"tc2_ethercat.st": string(stub), "main.st": src} {
		res := pipeline.Parse(name, text, nil)
		for _, d := range res.Diags {
			require.NotEqual(t, diag.Error, d.Severity, "%s: %s", name, d.Message)
		}
		for _, d := range res.File.Declarations {
			switch d := d.(type) {
			case *ast.ProgramDecl:
				prog = d
			case *ast.GVLDecl:
				gvls = append(gvls, d)
			case *ast.TypeDecl:
				typeDecls[strings.ToUpper(d.Name.Name)] = d.Type
			case *ast.FunctionBlockDecl:
				fbDecls[strings.ToUpper(d.Name.Name)] = d
			}
		}
	}
	require.NotNil(t, prog)
	eng := NewScanCycleEngine(prog)
	eng.interp.TypeDecls = typeDecls
	eng.interp.FBDecls = fbDecls
	eng.SetGlobals(gvls)
	if net != nil {
		eng.SetNetwork(net)
	}
	return eng
}

func envVal(t *testing.T, e *ScanCycleEngine, name string) Value {
	t.Helper()
	v, ok := e.env.Get(strings.ToUpper(name))
	require.True(t, ok, name)
	return v
}

func tick(t *testing.T, e *ScanCycleEngine, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		require.NoError(t, e.Tick(10*time.Millisecond))
	}
}

// twoMasterNet has two empty masters with distinct AmsNetIds.
func twoMasterNet(t *testing.T) *ecat.Network {
	topo := &ecat.Topology{Masters: []*ecat.Master{
		{Name: "A", InBytes: 16, OutBytes: 16},
		{Name: "B", InBytes: 16, OutBytes: 16},
	}}
	n := ecat.NewNetwork(topo, nil)
	require.NoError(t, n.SetMasterNetID("A", [6]byte{10, 0, 0, 1, 1, 1}))
	require.NoError(t, n.SetMasterNetID("B", [6]byte{10, 0, 0, 2, 1, 1}))
	return n
}

func TestEcatAmsNetId(t *testing.T) {
	e := ecatEngine(t, `
PROGRAM P
VAR
	ids : ARRAY[0..5] OF BYTE;
	a, b : STRING;
END_VAR
ids[0] := 192; ids[1] := 168; ids[2] := 1; ids[3] := 10; ids[4] := 1; ids[5] := 1;
a := F_CreateAmsNetId(ids);
b := F_CreateAmsNetId(nIds := ids);
END_PROGRAM
`, ioNet())
	tick(t, e, 1)
	assert.Equal(t, "192.168.1.10.1.1", envVal(t, e, "a").Str)
	assert.Equal(t, "192.168.1.10.1.1", envVal(t, e, "b").Str)

	id, ok := parseNetID("192.168.1.10.1.1")
	assert.True(t, ok)
	assert.Equal(t, [6]byte{192, 168, 1, 10, 1, 1}, id)
	for _, bad := range []string{"", "1.2.3", "1.2.3.4.5.256", "a.b.c.d.e.f"} {
		_, ok := parseNetID(bad)
		assert.False(t, ok, bad)
	}

	_, err := createAmsNetID([]Value{IntValue(1)})
	assert.Error(t, err)
	v, err := createAmsNetID([]Value{{Kind: ValArray, Array: []Value{IntValue(1), IntValue(2)}}})
	require.NoError(t, err)
	assert.Equal(t, "1.2.0.0.0.0", v.Str)
}

func TestEcatLocalArgsErrors(t *testing.T) {
	for name, call := range map[string]string{
		"unknown param":  "F_CreateAmsNetId(nFoo := ids)",
		"too many pos":   "F_CreateAmsNetId(ids, ids)",
		"too many named": "F_CreateAmsNetId(nIds := ids, ids)",
		"duplicate":      "F_CreateAmsNetId(ids, nIds := ids)",
		"bad pos expr":   "F_CreateAmsNetId(nope, nIds := ids)",
		"bad named expr": "F_CreateAmsNetId(nIds := nope)",
		"missing":        "F_CreateAmsNetId(, nIds := ids)",
	} {
		t.Run(name, func(t *testing.T) {
			res := pipeline.Parse("x.st", "PROGRAM P\nVAR ids : ARRAY[0..5] OF BYTE; s : STRING; END_VAR\ns := "+call+";\nEND_PROGRAM\n", nil)
			if res.File == nil || len(res.File.Declarations) == 0 {
				t.Skip("does not parse")
			}
			var prog *ast.ProgramDecl
			for _, d := range res.File.Declarations {
				if p, ok := d.(*ast.ProgramDecl); ok {
					prog = p
				}
			}
			if prog == nil {
				t.Skip("no program")
			}
			e := NewScanCycleEngine(prog)
			e.SetNetwork(ioNet())
			assert.Error(t, e.Tick(time.Millisecond))
		})
	}
	// Named arguments to a function without localParams are rejected.
	e := ecatEngine(t, "PROGRAM P\nVAR x : BOOL; END_VAR\nx := MYFN(a := 1);\nEND_PROGRAM\n", nil)
	e.interp.RegisterFunction("MYFN", func([]Value, ast.Pos) (Value, error) { return BoolValue(true), nil })
	assert.Error(t, e.Tick(time.Millisecond))
}

func TestEcatResolveMaster(t *testing.T) {
	one := &ecatServices{net: ioNet()}
	for _, id := range []string{"", "junk", "1.2.3.4.5.6"} {
		m, code := one.resolveMaster(id)
		assert.Equal(t, ioMaster, m, id)
		assert.Zero(t, code)
	}
	two := &ecatServices{net: twoMasterNet(t)}
	m, code := two.resolveMaster("10.0.0.2.1.1")
	assert.Equal(t, "B", m)
	assert.Zero(t, code)
	_, code = two.resolveMaster("")
	assert.Equal(t, adsErrMachineNotFound, code)
	_, code = two.resolveMaster("9.9.9.9.9.9")
	assert.Equal(t, adsErrMachineNotFound, code)
	_, _, _, code = two.resolveSlave("", 1001)
	assert.Equal(t, adsErrMachineNotFound, code)
	_, _, _, code = one.resolveSlave("", 1001)
	assert.Equal(t, adsErrPortNotFound, code)
}

const ptrSrc = `
TYPE ST_Mixed :
STRUCT
	z  : BOOL;
	a  : SINT;
	r  : REAL;
	lr : LREAL;
	t  : TIME;
	l  : LINT;
	e  : E_EcAdressingType;
END_STRUCT
END_TYPE
PROGRAM P
VAR
	u   : UDINT;
	arr : ARRAY[0..3] OF UDINT;
	sts : ARRAY[0..2] OF ST_EcSlaveState;
	mix : ST_Mixed;
	txt : STRING;
	p1, p2, p3, p4, p5 : POINTER TO BYTE;
	np  : POINTER TO BYTE;
END_VAR
p1 := ADR(u);
p2 := ADR(arr);
p3 := ADR(sts);
p4 := ADR(mix);
p5 := ADR(txt);
END_PROGRAM
`

func TestEcatPtrBytes(t *testing.T) {
	e := ecatEngine(t, ptrSrc, ioNet())
	tick(t, e, 1)
	s := e.ecat

	// UDINT: extra data past the variable end is ignored.
	require.NoError(t, s.writePtrBytes(envVal(t, e, "p1"), []byte{0x78, 0x56, 0x34, 0x12, 0xFF, 0xFF}))
	assert.Equal(t, int64(0x12345678), envVal(t, e, "u").Int)
	b, err := s.readPtrBytes(envVal(t, e, "p1"), 100)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x78, 0x56, 0x34, 0x12}, b)
	b, err = s.readPtrBytes(envVal(t, e, "p1"), -1)
	require.NoError(t, err)
	assert.Empty(t, b)

	// Array: short data updates only the leading elements.
	require.NoError(t, s.writePtrBytes(envVal(t, e, "p2"), []byte{1, 0, 0, 0, 2, 0}))
	arr := envVal(t, e, "arr").Array
	assert.Equal(t, []int64{1, 2, 0, 0}, []int64{arr[0].Int, arr[1].Int, arr[2].Int, arr[3].Int})
	b, err = s.readPtrBytes(envVal(t, e, "p2"), 8)
	require.NoError(t, err)
	assert.Equal(t, []byte{1, 0, 0, 0, 2, 0, 0, 0}, b)

	// Array of ST_EcSlaveState in declaration order deviceState, linkState.
	require.NoError(t, s.writePtrBytes(envVal(t, e, "p3"), []byte{8, 0, 2, 1, 4, 0}))
	sts := envVal(t, e, "sts").Array
	assert.Equal(t, int64(8), sts[0].Struct["DEVICESTATE"].Int)
	assert.Equal(t, int64(2), sts[1].Struct["DEVICESTATE"].Int)
	assert.Equal(t, int64(1), sts[1].Struct["LINKSTATE"].Int)
	assert.Equal(t, int64(4), sts[2].Struct["DEVICESTATE"].Int)

	// Mixed struct round trip, declaration order from TypeDecls.
	mix := envVal(t, e, "mix").Clone()
	mix.Struct["Z"] = BoolValue(true)
	mix.Struct["A"] = Value{Kind: ValInt, Int: -3, IECType: types.KindSINT}
	mix.Struct["R"] = Value{Kind: ValReal, Real: 1.5, IECType: types.KindREAL}
	mix.Struct["LR"] = Value{Kind: ValReal, Real: -2.25, IECType: types.KindLREAL}
	mix.Struct["T"] = TimeValue(1500 * time.Millisecond)
	mix.Struct["L"] = Value{Kind: ValInt, Int: -1 << 40, IECType: types.KindLINT}
	e.env.Set("MIX", mix)
	img, err := s.readPtrBytes(envVal(t, e, "p4"), 1000)
	require.NoError(t, err)
	assert.Len(t, img, 1+1+4+8+4+8+2)
	assert.Equal(t, byte(1), img[0])
	assert.Equal(t, byte(0xFD), img[1])
	e.env.Set("MIX", zeroStruct(e.interp.TypeDecls["ST_MIXED"].(*ast.StructType)))
	require.NoError(t, s.writePtrBytes(envVal(t, e, "p4"), img))
	got := envVal(t, e, "mix").Struct
	assert.True(t, got["Z"].Bool)
	assert.Equal(t, int64(-3), got["A"].Int)
	assert.Equal(t, 1.5, got["R"].Real)
	assert.Equal(t, -2.25, got["LR"].Real)
	assert.Equal(t, 1500*time.Millisecond, got["T"].Time)
	assert.Equal(t, int64(-1<<40), got["L"].Int)

	// Errors never panic.
	assert.Error(t, s.writePtrBytes(envVal(t, e, "u"), []byte{1}))
	assert.Error(t, s.writePtrBytes(envVal(t, e, "np"), []byte{1}))
	_, err = s.readPtrBytes(envVal(t, e, "np"), 1)
	assert.Error(t, err)
	_, err = s.readPtrBytes(Value{Kind: ValPointer, PtrEnv: NewEnv(nil), PtrVar: "GONE"}, 1)
	assert.Error(t, err)
	assert.Error(t, s.writePtrBytes(envVal(t, e, "p5"), []byte{1}))
	_, err = s.readPtrBytes(envVal(t, e, "p5"), 1)
	assert.Error(t, err)
	arrStr := Value{Kind: ValArray, Array: []Value{StringValue("x")}}
	_, err = s.encodeValue(nil, arrStr)
	assert.Error(t, err)
	_, err = s.encodeValue(nil, Value{Kind: ValStruct, Struct: map[string]Value{"S": StringValue("x")}})
	assert.Error(t, err)
}

func TestEcatMemberOrderFallback(t *testing.T) {
	s := &ecatServices{interp: New()}
	s.interp.TypeDecls = map[string]ast.TypeSpec{
		"ALIAS":  &ast.NamedType{Name: &ast.Ident{Name: "INT"}},
		"OTHER":  &ast.StructType{Members: []*ast.StructMember{{Name: &ast.Ident{Name: "x"}}, {Name: &ast.Ident{Name: "q"}}}},
		"NONAME": &ast.StructType{Members: []*ast.StructMember{{}, {}}},
	}
	v := Value{Kind: ValStruct, Struct: map[string]Value{"B": BoolValue(false), "A": BoolValue(false)}}
	assert.Equal(t, []string{"A", "B"}, s.memberOrder(v))
	assert.Equal(t, []string{"A", "B"}, (&ecatServices{}).memberOrder(v))
	// Unknown kinds keep a 16-bit width.
	n, signed := scalarBytes(Value{Kind: ValInt})
	assert.Equal(t, 2, n)
	assert.True(t, signed)
}

// testMock is a minimal StandardFB used to observe override precedence.
type testMock struct{ s *ecatServices }

func (m *testMock) Execute(time.Duration)  {}
func (m *testMock) SetInput(string, Value) {}
func (m *testMock) GetOutput(string) Value { return Value{} }
func (m *testMock) GetInput(string) Value  { return Value{} }

func TestEcatOverridesAndScan(t *testing.T) {
	ecatMockCtors["FB_ECGETMASTERSTATE"+"_T"] = func(s *ecatServices) StandardFB { return &testMock{s} }
	defer delete(ecatMockCtors, "FB_ECGETMASTERSTATE_T")
	src := `
FUNCTION_BLOCK FB_EcGetMasterState_T
VAR_OUTPUT q : BOOL; END_VAR
END_FUNCTION_BLOCK
FUNCTION_BLOCK FB_Holder
VAR inner : FB_EcGetMasterState_T; END_VAR
END_FUNCTION_BLOCK
PROGRAM P
VAR
	fb : FB_EcGetMasterState_T;
	h  : FB_Holder;
END_VAR
fb();
END_PROGRAM
`
	// Without a network the stub declaration is a user FB.
	plain := ecatEngine(t, src, nil)
	tick(t, plain, 1)
	assert.Nil(t, envVal(t, plain, "fb").FBRef.FB)
	assert.Nil(t, plain.ecat)

	net := ioNet()
	e := ecatEngine(t, src, net)
	tick(t, e, 3)
	mock, ok := envVal(t, e, "fb").FBRef.FB.(*testMock)
	require.True(t, ok)
	assert.Same(t, e.ecat, mock.s)
	inner, ok := envVal(t, e, "h").FBRef.Env.Get("INNER")
	require.True(t, ok)
	assert.IsType(t, &testMock{}, inner.FBRef.FB)
	assert.Equal(t, uint64(3), e.ecat.scan)

	// An IOBinder on the same network steps it; the mocks still count scans.
	e.SetIOBinder(NewIOBinder(nil, net))
	tick(t, e, 2)
	assert.Equal(t, uint64(5), e.ecat.scan)

	// Detach.
	e.SetNetwork(nil)
	assert.Nil(t, e.ecat)
	assert.Nil(t, e.interp.fbOverrides)
	_, has := e.interp.LocalFunctions["F_CREATEAMSNETID"]
	assert.False(t, has)
	tick(t, e, 1)

	// Nil interpreter has no overrides.
	var nilInterp *Interpreter
	_, ok = nilInterp.stdFBFactory("FB_ECGETMASTERSTATE_T")
	assert.False(t, ok)
	_, ok = nilInterp.stdFBFactory("TON")
	assert.True(t, ok)
}

func TestEcatAsyncReq(t *testing.T) {
	s := &ecatServices{}
	var a asyncReq
	begins, completes := 0, 0
	begin := func() uint32 { begins++; return 0 }
	complete := func() (uint32, bool) { completes++; return 0, true }
	step := func(exec bool) {
		s.scan++
		a.run(s, exec, 10*time.Millisecond, 0, begin, complete)
	}
	step(false)
	assert.False(t, a.busy)
	step(true) // edge
	assert.True(t, a.busy)
	step(false) // edge falls while busy: still busy
	assert.True(t, a.busy)
	step(true) // new edge while busy: ignored, completes
	assert.False(t, a.busy)
	assert.Equal(t, 1, begins)
	assert.Equal(t, 1, completes)
	step(true) // no edge: outputs hold
	assert.False(t, a.busy)

	// Pending error from begin ends after the latency.
	var b asyncReq
	b.run(s, true, 0, 0, func() uint32 { return adsErrPortNotFound }, complete)
	s.scan++
	b.run(s, true, 0, 0, nil, nil)
	assert.True(t, b.busy)
	s.scan++
	b.run(s, true, 0, 0, nil, nil)
	assert.False(t, b.busy)
	assert.True(t, b.err)
	assert.Equal(t, adsErrPortNotFound, b.errID)

	// Timeout when never done.
	var c asyncReq
	never := func() (uint32, bool) { return 0, false }
	c.run(s, true, time.Second, 1500*time.Millisecond, begin, never)
	for i := 0; i < 3 && c.busy; i++ {
		s.scan++
		c.run(s, true, time.Second, 1500*time.Millisecond, begin, never)
	}
	assert.False(t, c.busy)
	assert.Equal(t, adsErrTimeout, c.errID)
}
