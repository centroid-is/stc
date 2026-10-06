package scenario

import (
	"fmt"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/interp"
)

// MaxRunCycles bounds one RUN_CYCLES or ADVANCE_TIME call (T-27-07).
const MaxRunCycles = 10_000_000

// Session is the ST built-in library bound to one Plant: the variable and
// EtherCAT stimuli of a scenario plus RUN_CYCLES, callable from ST code
// running on the Plant's interpreter. Ramps started with SIM_RAMP advance
// before every Tick exactly like scenario ramps. A Session is not safe for
// concurrent use, and ST code calling it must not run under the Runtime
// mutex (Runtime.Tick, Project.Tick), because the built-ins Get, Set and
// Tick through it.
type Session struct {
	p     *Plant
	in    *interp.Interpreter
	ramps rampSet
}

// builtin is one registered function and its signature for errors.
type builtin struct {
	name string
	sig  string
	min  int
	max  int
	fn   func(s *Session, args []interp.Value) (interp.Value, error)
}

var builtins = []builtin{
	{"SET", "SET(path : STRING, value : ANY)", 2, 2, (*Session).set},
	{"GET", "GET(path : STRING) : ANY", 1, 1, (*Session).get},
	{"SIM_SET_LINK", "SIM_SET_LINK(link : STRING, value : ANY)", 2, 2, (*Session).setLink},
	{"SIM_TRIP", "SIM_TRIP(slave : STRING, channel : INT)", 2, 2, (*Session).trip},
	{"SIM_SLAVE_STATE", "SIM_SLAVE_STATE(slave : STRING, state : STRING | INT)", 2, 2, (*Session).slaveState},
	{"SIM_ANALOG", "SIM_ANALOG(slave : STRING, channel : INT, value : REAL [, unit : 'mA' | 'V' | 'raw' (default)])", 3, 4, (*Session).analog},
	{"SIM_DRIVE_FAULT", "SIM_DRIVE_FAULT(slave : STRING, lft : INT)", 2, 2, (*Session).driveFault},
	{"SIM_SERIAL_PEER", "SIM_SERIAL_PEER(slave : STRING, script : STRING)", 2, 2, (*Session).serialPeer},
	{"SIM_RAMP", "SIM_RAMP(path : STRING, from : REAL, to : REAL, over : TIME)", 4, 4, (*Session).ramp},
	{"RUN_CYCLES", "RUN_CYCLES(n : DINT)", 1, 1, (*Session).runCycles},
	{"ADVANCE_TIME", "ADVANCE_TIME(d : TIME)", 1, 1, (*Session).advanceTime},
}

// RegisterBuiltins registers SET, GET, SIM_SET_LINK, SIM_TRIP,
// SIM_SLAVE_STATE, SIM_ANALOG, SIM_DRIVE_FAULT, SIM_SERIAL_PEER, SIM_RAMP
// and RUN_CYCLES on in, and replaces ADVANCE_TIME with whole Plant Ticks
// (d must be a multiple of BaseTick). in is normally the Plant's own
// interpreter (p.Runtime().Interpreter()). The built-ins are test
// functions: they resolve only in the environment passed to
// in.SetTestEnv (the TEST_CASE body), never in project POUs, and fail
// with a runtime error if called while a scan holds the Runtime mutex.
func RegisterBuiltins(in *interp.Interpreter, p *Plant) *Session {
	s := &Session{p: p, in: in}
	for _, b := range builtins {
		b := b
		in.RegisterTestFunction(b.name, func(args []interp.Value, pos ast.Pos) (interp.Value, error) {
			return s.call(b, args, pos)
		})
	}
	return s
}

// call runs built-in b, checking the argument count and refusing to run
// while a scan holds the Runtime mutex (Get, Set and Tick would deadlock).
func (s *Session) call(b builtin, args []interp.Value, pos ast.Pos) (interp.Value, error) {
	if s.p.Runtime().InTick() {
		return interp.Value{}, s.errf(pos, b, "cannot run inside a scan; call it from the TEST_CASE body")
	}
	if len(args) < b.min || len(args) > b.max {
		return interp.Value{}, s.errf(pos, b, "got %d argument(s)", len(args))
	}
	v, err := b.fn(s, args)
	if err != nil {
		return interp.Value{}, s.errf(pos, b, "%v", err)
	}
	return v, nil
}

func (s *Session) errf(pos ast.Pos, b builtin, format string, args ...any) error {
	return &interp.RuntimeError{Msg: fmt.Sprintf("%s: %s (signature %s)", b.name, fmt.Sprintf(format, args...), b.sig), Pos: pos}
}

// argError reports an argument of the wrong kind.
func argError(i int, want string, v interp.Value) error {
	return fmt.Errorf("argument %d must be %s, got %s", i+1, want, v.Kind)
}

func strArg(args []interp.Value, i int) (string, error) {
	if args[i].Kind != interp.ValString {
		return "", argError(i, "a STRING", args[i])
	}
	return args[i].Str, nil
}

func intArg(args []interp.Value, i int) (int64, error) {
	if args[i].Kind != interp.ValInt {
		return 0, argError(i, "an integer", args[i])
	}
	return args[i].Int, nil
}

func numArg(args []interp.Value, i int) (float64, error) {
	switch args[i].Kind {
	case interp.ValInt:
		return float64(args[i].Int), nil
	case interp.ValReal:
		return args[i].Real, nil
	}
	return 0, argError(i, "a number", args[i])
}

func timeArg(args []interp.Value, i int) (time.Duration, error) {
	if args[i].Kind != interp.ValTime {
		return 0, argError(i, "a TIME", args[i])
	}
	return args[i].Time, nil
}

// apply runs a on the Plant; set, link and analog actions cancel the ramp
// on the same target, like scenario steps.
func (s *Session) apply(a Action) (interp.Value, error) {
	s.ramps.override(&a)
	if err := s.p.Apply(a); err != nil {
		return interp.Value{}, err
	}
	return interp.BoolValue(true), nil
}

// pathValue is a path argument and a value argument as the Plant takes
// them (ToJSON form).
func (s *Session) pathValue(args []interp.Value) (string, any, error) {
	path, err := strArg(args, 0)
	if err != nil {
		return "", nil, err
	}
	return path, s.p.Runtime().ToJSON(args[1]), nil
}

func (s *Session) set(args []interp.Value) (interp.Value, error) {
	path, v, err := s.pathValue(args)
	if err != nil {
		return interp.Value{}, err
	}
	return s.apply(Action{Kind: ActSet, Path: path, Value: v})
}

func (s *Session) setLink(args []interp.Value) (interp.Value, error) {
	path, v, err := s.pathValue(args)
	if err != nil {
		return interp.Value{}, err
	}
	return s.apply(Action{Kind: ActLink, Path: path, Value: v})
}

// get returns a variable's typed value (enums as their STRING name) or a
// link slot's value.
func (s *Session) get(args []interp.Value) (interp.Value, error) {
	path, err := strArg(args, 0)
	if err != nil {
		return interp.Value{}, err
	}
	if isLink(path) {
		v, err := s.p.Read(path)
		if err != nil {
			return interp.Value{}, err
		}
		return fromScalar(v), nil
	}
	rt := s.p.Runtime()
	v, err := rt.Get(path)
	if err != nil {
		return interp.Value{}, fmt.Errorf("%w: %s: %v", ErrUnknownPath, path, err)
	}
	if v.Enum != "" {
		return interp.StringValue(fmt.Sprint(rt.ToJSON(v))), nil
	}
	return v, nil
}

// fromScalar converts a link slot value (bool, int64 or float64) to a Value.
func fromScalar(v any) interp.Value {
	switch x := v.(type) {
	case bool:
		return interp.BoolValue(x)
	case float64:
		return interp.RealValue(x)
	}
	return interp.IntValue(v.(int64))
}

// slaveChannel is the (slave, channel) argument pair of the SIM_* built-ins.
func slaveChannel(args []interp.Value) (string, int, error) {
	slave, err := strArg(args, 0)
	if err != nil {
		return "", 0, err
	}
	ch, err := intArg(args, 1)
	if err != nil {
		return "", 0, err
	}
	if ch < 1 || ch > MaxChannel {
		return "", 0, fmt.Errorf("channel %d out of range 1..%d", ch, MaxChannel)
	}
	return slave, int(ch), nil
}

func (s *Session) trip(args []interp.Value) (interp.Value, error) {
	slave, ch, err := slaveChannel(args)
	if err != nil {
		return interp.Value{}, err
	}
	return s.apply(Action{Kind: ActTrip, Slave: slave, Channel: ch})
}

func (s *Session) slaveState(args []interp.Value) (interp.Value, error) {
	slave, err := strArg(args, 0)
	if err != nil {
		return interp.Value{}, err
	}
	a := Action{Kind: ActSlaveState, Slave: slave}
	switch args[1].Kind {
	case interp.ValString:
		a.State = strings.ToLower(args[1].Str)
	case interp.ValInt:
		a.StateCode, a.HasStateCode = args[1].Int, true
	default:
		return interp.Value{}, argError(1, "a STRING preset or an integer state", args[1])
	}
	return s.apply(a)
}

func (s *Session) analog(args []interp.Value) (interp.Value, error) {
	slave, ch, err := slaveChannel(args)
	if err != nil {
		return interp.Value{}, err
	}
	v, err := numArg(args, 2)
	if err != nil {
		return interp.Value{}, err
	}
	// Without a unit the value is the raw process-data count.
	a := Action{Kind: ActAnalog, Slave: slave, Channel: ch, Value: v, Unit: "raw"}
	if len(args) == 4 {
		if a.Unit, err = strArg(args, 3); err != nil {
			return interp.Value{}, err
		}
	}
	return s.apply(a)
}

func (s *Session) driveFault(args []interp.Value) (interp.Value, error) {
	slave, err := strArg(args, 0)
	if err != nil {
		return interp.Value{}, err
	}
	lft, err := intArg(args, 1)
	if err != nil {
		return interp.Value{}, err
	}
	return s.apply(Action{Kind: ActDriveFault, Slave: slave, LFT: int(lft)})
}

func (s *Session) serialPeer(args []interp.Value) (interp.Value, error) {
	slave, err := strArg(args, 0)
	if err != nil {
		return interp.Value{}, err
	}
	script, err := strArg(args, 1)
	if err != nil {
		return interp.Value{}, err
	}
	return s.apply(Action{Kind: ActSerialPeer, Slave: slave, Script: script})
}

// ramp starts a linear ramp of path from from to to over over, starting
// at the current clock; it is written before each following Tick.
func (s *Session) ramp(args []interp.Value) (interp.Value, error) {
	path, err := strArg(args, 0)
	if err != nil {
		return interp.Value{}, err
	}
	from, err := numArg(args, 1)
	if err != nil {
		return interp.Value{}, err
	}
	to, err := numArg(args, 2)
	if err != nil {
		return interp.Value{}, err
	}
	over, err := timeArg(args, 3)
	if err != nil {
		return interp.Value{}, err
	}
	if over < 0 || over > MaxOver {
		return interp.Value{}, fmt.Errorf("over %v out of range 0..%v", over, MaxOver)
	}
	a := Action{Kind: ActRamp, Path: path, From: from, To: to, Over: over}
	if err := s.p.Check(a); err != nil {
		return interp.Value{}, err
	}
	s.ramps.start(nil, a, s.p.Clock())
	return interp.BoolValue(true), nil
}

func (s *Session) runCycles(args []interp.Value) (interp.Value, error) {
	n, err := intArg(args, 0)
	if err != nil {
		return interp.Value{}, err
	}
	return interp.BoolValue(true), s.RunCycles(n)
}

func (s *Session) advanceTime(args []interp.Value) (interp.Value, error) {
	d, err := timeArg(args, 0)
	if err != nil {
		return interp.Value{}, err
	}
	base := s.p.BaseTick()
	if d < 0 || d%base != 0 {
		return interp.Value{}, fmt.Errorf("%v is not a non-negative multiple of the base tick %v", d, base)
	}
	return interp.BoolValue(true), s.RunCycles(int64(d / base))
}

// RunCycles runs n Plant Ticks, writing the active ramps before each. The
// interpreter's dt and call depth are restored afterwards, so the calling
// ST body continues unchanged. n must be in 0..MaxRunCycles.
func (s *Session) RunCycles(n int64) error {
	if n < 0 || n > MaxRunCycles {
		return fmt.Errorf("cycle count %d out of range 0..%d", n, MaxRunCycles)
	}
	restore := s.in.SaveCallState()
	defer restore()
	for k := int64(0); k < n; k++ {
		var rampErr error
		s.ramps.advance(s.p, func(r *ramp, err error) {
			if rampErr == nil {
				rampErr = fmt.Errorf("ramp of %s: %w", r.a.Path, err)
			}
		})
		if rampErr != nil {
			return rampErr
		}
		if err := s.p.Tick(); err != nil {
			return fmt.Errorf("cycle %d: %w", k+1, err)
		}
	}
	return nil
}
