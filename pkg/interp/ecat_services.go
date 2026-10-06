package interp

import (
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/ecat"
	"github.com/centroid-is/stc/pkg/types"
)

// MockLatency is the number of scans a Tc2_EtherCAT mock request stays busy
// after its bExecute rising edge.
const MockLatency = 2

// ADS error codes the Tc2_EtherCAT mocks report in nErrId.
const (
	adsErrPortNotFound    uint32 = 0x6   // target port (slave address) not found
	adsErrMachineNotFound uint32 = 0x7   // target machine (master) not found
	adsErrTimeout         uint32 = 0x745 // timeout elapsed
	adsErrInvalidParm     uint32 = 0x705 // invalid parameter (bad pointer)
)

// defaultEcTimeout is the tTimeout used when a request passes T#0S.
const defaultEcTimeout = 5 * time.Second

// ecatServices is the simulated EtherCAT master-service backend shared by
// all Tc2_EtherCAT mocks of one engine.
type ecatServices struct {
	net    *ecat.Network
	interp *Interpreter
	scan   uint64 // incremented once per Tick
}

// SetNetwork attaches net as the backend of the Tc2_EtherCAT mocks. Call it
// before Initialize or the first Tick: FB variables are instantiated then,
// and from that point the mocked FB names (FB_EcGetSlaveState, ...) resolve
// to Go StandardFBs ahead of any stub declaration of the same name.
// F_CreateAmsNetId becomes available too. A nil net detaches the mocks.
//
// Each Tick counts one scan for the mocks' latency. The network is stepped
// by the IOBinder when one is attached to the same network, otherwise by the
// mocks themselves at the same point of the scan.
func (e *ScanCycleEngine) SetNetwork(net *ecat.Network) {
	in := e.interp
	if net == nil {
		e.ecat = nil
		in.fbOverrides = nil
		delete(in.LocalFunctions, "F_CREATEAMSNETID")
		delete(in.localParams, "F_CREATEAMSNETID")
		return
	}
	s := &ecatServices{net: net, interp: in}
	e.ecat = s
	in.fbOverrides = ecatFBFactories(s)
	in.RegisterFunction("F_CREATEAMSNETID", func(args []Value, _ ast.Pos) (Value, error) {
		return createAmsNetID(args)
	})
	if in.localParams == nil {
		in.localParams = map[string][]string{}
	}
	in.localParams["F_CREATEAMSNETID"] = []string{"NIDS"}
}

// preScan counts the scan and steps the network unless the IOBinder steps it.
func (s *ecatServices) preScan(dt time.Duration, b *IOBinder) {
	s.scan++
	if b == nil || b.net != s.net {
		s.net.Step(dt)
	}
}

// localArgs evaluates the arguments of a LocalFunctions call. Functions
// registered in localParams also accept named arguments (name := value);
// everything else is positional only.
func (interp *Interpreter) localArgs(env *Env, e *ast.CallExpr, name string) ([]Value, error) {
	params, ok := interp.localParams[name]
	if !ok || len(e.NamedArgs) == 0 {
		return interp.positionalArgs(env, e, name)
	}
	args := make([]Value, len(params))
	set := make([]bool, len(params))
	for i, a := range e.Args {
		if i >= len(params) {
			return nil, &RuntimeError{Msg: fmt.Sprintf("%s: too many arguments", name), Pos: e.Span().Start}
		}
		v, err := interp.evalExpr(env, a)
		if err != nil {
			return nil, err
		}
		args[i], set[i] = v, true
	}
	next := len(e.Args)
	for _, a := range e.NamedArgs {
		idx := next
		if a.Name != nil {
			idx = slicesIndex(params, strings.ToUpper(a.Name.Name))
			if idx < 0 {
				return nil, &RuntimeError{Msg: fmt.Sprintf("%s has no parameter %s", name, a.Name.Name), Pos: e.Span().Start}
			}
		}
		if idx >= len(params) {
			return nil, &RuntimeError{Msg: fmt.Sprintf("%s: too many arguments", name), Pos: e.Span().Start}
		}
		if set[idx] {
			return nil, &RuntimeError{Msg: fmt.Sprintf("%s: argument %s given twice", name, params[idx]), Pos: e.Span().Start}
		}
		v, err := interp.evalExpr(env, a.Value)
		if err != nil {
			return nil, err
		}
		args[idx], set[idx] = v, true
		next = idx + 1
	}
	for i, ok := range set {
		if !ok {
			return nil, &RuntimeError{Msg: fmt.Sprintf("%s: missing argument %s", name, params[i]), Pos: e.Span().Start}
		}
	}
	return args, nil
}

func slicesIndex(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

// createAmsNetID implements F_CreateAmsNetId: six bytes to "a.b.c.d.e.f".
func createAmsNetID(args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValArray {
		return Value{}, &RuntimeError{Msg: "F_CreateAmsNetId requires an ARRAY[0..5] OF BYTE argument"}
	}
	parts := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		var b int64
		if i < len(args[0].Array) {
			b = args[0].Array[i].Int & 0xFF
		}
		parts = append(parts, strconv.FormatInt(b, 10))
	}
	return StringValue(strings.Join(parts, ".")), nil
}

// parseNetID parses a dotted AmsNetId; ok is false unless it has six bytes.
func parseNetID(s string) ([6]byte, bool) {
	var id [6]byte
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) != 6 {
		return id, false
	}
	for i, p := range parts {
		n, err := strconv.ParseUint(p, 10, 8)
		if err != nil {
			return id, false
		}
		id[i] = byte(n)
	}
	return id, true
}

// resolveMaster maps sNetId to a master name. An empty, malformed or
// unknown id falls back to the only master when there is exactly one;
// otherwise the result is ADS error 0x7.
func (s *ecatServices) resolveMaster(netID string) (string, uint32) {
	if id, ok := parseNetID(netID); ok {
		if m, err := s.net.MasterByNetID(id); err == nil {
			return m, 0
		}
	}
	if names := s.net.MasterNames(); len(names) == 1 {
		return names[0], 0
	}
	return "", adsErrMachineNotFound
}

// resolveSlave maps sNetId and an EtherCAT address to a bus index and device.
func (s *ecatServices) resolveSlave(netID string, addr uint16) (string, int, ecat.Device, uint32) {
	m, code := s.resolveMaster(netID)
	if code != 0 {
		return "", -1, nil, code
	}
	i, dev, err := s.net.Slave(m, addr)
	if err != nil {
		return m, -1, nil, adsErrPortNotFound
	}
	return m, i, dev, 0
}

// asyncReq is the bExecute / bBusy / bError / nErrId handshake shared by
// the mocks. A rising edge of bExecute starts a request; it completes no
// earlier than MockLatency scans later. Outputs hold until the next rising
// edge; an edge while busy is ignored.
type asyncReq struct {
	prevExec bool
	busy     bool
	err      bool
	errID    uint32
	start    uint64
	elapsed  time.Duration
	pending  uint32 // error found when the request started
}

// run advances the request. begin runs on the rising edge and returns an
// error code that ends the request after the latency. complete runs once
// the latency has passed and reports whether the request is done and with
// which error code. A positive timeout bounds the request in summed dt.
func (a *asyncReq) run(s *ecatServices, exec bool, dt, timeout time.Duration,
	begin func() uint32, complete func() (uint32, bool)) {
	rising := exec && !a.prevExec
	a.prevExec = exec
	if !a.busy {
		if !rising {
			return
		}
		a.busy, a.err, a.errID = true, false, 0
		a.start, a.elapsed = s.scan, 0
		a.pending = begin()
		return
	}
	a.elapsed += dt
	if s.scan-a.start >= MockLatency {
		if a.pending != 0 {
			a.finish(a.pending)
			return
		}
		if code, done := complete(); done {
			a.finish(code)
			return
		}
	}
	if timeout > 0 && a.elapsed > timeout {
		a.finish(adsErrTimeout)
	}
}

func (a *asyncReq) finish(code uint32) {
	a.busy = false
	a.err = code != 0
	a.errID = code
}

// ----- pointer buffers -----

// ptrTarget reads the variable a pointer refers to.
func ptrTarget(v Value) (Value, error) {
	if v.Kind != ValPointer {
		return Value{}, fmt.Errorf("not a pointer")
	}
	if v.PtrEnv == nil || v.PtrVar == "" {
		return Value{}, fmt.Errorf("null pointer")
	}
	cur, ok := v.PtrEnv.Get(v.PtrVar)
	if !ok {
		return Value{}, fmt.Errorf("dangling pointer to %s", v.PtrVar)
	}
	return cur, nil
}

// readPtrBytes returns up to n bytes of the variable v points to, in
// little-endian packed layout (no alignment padding).
func (s *ecatServices) readPtrBytes(v Value, n int) ([]byte, error) {
	cur, err := ptrTarget(v)
	if err != nil {
		return nil, err
	}
	img, err := s.encodeValue(nil, cur)
	if err != nil {
		return nil, err
	}
	if n < 0 {
		n = 0
	}
	if n < len(img) {
		img = img[:n]
	}
	return img, nil
}

// writePtrBytes overlays data onto the variable v points to: the copy stops
// at the end of data or of the variable, whichever comes first.
func (s *ecatServices) writePtrBytes(v Value, data []byte) error {
	cur, err := ptrTarget(v)
	if err != nil {
		return err
	}
	img, err := s.encodeValue(nil, cur)
	if err != nil {
		return err
	}
	copy(img, data)
	pos := 0
	out := s.decodeValue(cur.Clone(), img, &pos)
	v.PtrEnv.Set(v.PtrVar, out)
	return nil
}

// scalarBytes is the in-memory width of a scalar value.
func scalarBytes(v Value) (int, bool) {
	switch v.Kind {
	case ValBool:
		return 1, false
	case ValReal:
		if v.IECType == types.KindREAL {
			return 4, false
		}
		return 8, false
	case ValTime:
		return 4, false
	}
	w, signed := intWidth(v.IECType)
	if w == 0 {
		w = 16 // enum default base INT
	}
	return w / 8, signed
}

func (s *ecatServices) encodeValue(buf []byte, v Value) ([]byte, error) {
	switch v.Kind {
	case ValBool, ValInt, ValReal, ValTime:
		n, _ := scalarBytes(v)
		var raw uint64
		switch v.Kind {
		case ValBool:
			if v.Bool {
				raw = 1
			}
		case ValInt:
			raw = uint64(v.Int)
		case ValReal:
			if n == 4 {
				raw = uint64(math.Float32bits(float32(v.Real)))
			} else {
				raw = math.Float64bits(v.Real)
			}
		case ValTime:
			raw = uint64(v.Time / time.Millisecond)
		}
		var tmp [8]byte
		binary.LittleEndian.PutUint64(tmp[:], raw)
		return append(buf, tmp[:n]...), nil
	case ValArray:
		var err error
		for _, el := range v.Array {
			if buf, err = s.encodeValue(buf, el); err != nil {
				return nil, err
			}
		}
		return buf, nil
	case ValStruct:
		var err error
		for _, m := range s.memberOrder(v) {
			if buf, err = s.encodeValue(buf, v.Struct[m]); err != nil {
				return nil, err
			}
		}
		return buf, nil
	}
	return nil, fmt.Errorf("cannot address a %s as bytes", v.Kind)
}

// decodeValue rebuilds cur from img starting at *pos. Only kinds accepted
// by encodeValue reach it, so the image always covers the value.
func (s *ecatServices) decodeValue(cur Value, img []byte, pos *int) Value {
	switch cur.Kind {
	case ValArray:
		for i := range cur.Array {
			cur.Array[i] = s.decodeValue(cur.Array[i], img, pos)
		}
		return cur
	case ValStruct:
		for _, m := range s.memberOrder(cur) {
			cur.Struct[m] = s.decodeValue(cur.Struct[m], img, pos)
		}
		return cur
	}
	n, signed := scalarBytes(cur)
	var tmp [8]byte
	copy(tmp[:n], img[*pos:*pos+n])
	*pos += n
	raw := binary.LittleEndian.Uint64(tmp[:])
	switch cur.Kind {
	case ValBool:
		cur.Bool = raw != 0
	case ValReal:
		if n == 4 {
			cur.Real = float64(math.Float32frombits(uint32(raw)))
		} else {
			cur.Real = math.Float64frombits(raw)
		}
	case ValTime:
		cur.Time = time.Duration(uint32(raw)) * time.Millisecond
	default:
		if signed && n < 8 {
			shift := 64 - 8*n
			cur.Int = int64(raw<<shift) >> shift
		} else {
			cur.Int = int64(raw)
		}
	}
	return cur
}

// memberOrder lists a struct value's members in declaration order, taken
// from the first registered STRUCT type (by name) with exactly these
// members, else sorted by name.
func (s *ecatServices) memberOrder(v Value) []string {
	var names []string
	if s.interp != nil {
		for _, tn := range sortedKeys(s.interp.TypeDecls) {
			st, ok := s.interp.TypeDecls[tn].(*ast.StructType)
			if !ok || len(st.Members) != len(v.Struct) {
				continue
			}
			names = names[:0]
			for _, m := range st.Members {
				if m.Name == nil {
					break
				}
				name := strings.ToUpper(m.Name.Name)
				if _, ok := v.Struct[name]; !ok {
					break
				}
				names = append(names, name)
			}
			if len(names) == len(v.Struct) {
				return names
			}
		}
	}
	names = names[:0]
	for k := range v.Struct {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

func sortedKeys(m map[string]ast.TypeSpec) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// ecatMockCtors builds the Tc2_EtherCAT mocks by upper-case FB name.
var ecatMockCtors = map[string]func(*ecatServices) StandardFB{}

// ecatFBFactories binds every mock constructor to s.
func ecatFBFactories(s *ecatServices) map[string]func() StandardFB {
	out := make(map[string]func() StandardFB, len(ecatMockCtors))
	for name, mk := range ecatMockCtors {
		mk := mk
		out[name] = func() StandardFB { return mk(s) }
	}
	return out
}
