// Package scenario implements deterministic plant scenarios: a TOML file
// format with scan-clock triggered steps, a parser that reports SCN
// diagnostics, and an executor that drives any Target tick by tick.
package scenario

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
)

// Limits from D-11.
const (
	MaxFileSize = 1 << 20
	MaxSteps    = 10000
	MaxWithin   = 1000000
	MaxOver     = 24 * time.Hour
	MaxChannel  = 64
	DefaultTol  = 1e-6
)

// ActionKind names a scenario action.
type ActionKind string

// Action kinds (D-03).
const (
	ActSet        ActionKind = "set"
	ActLink       ActionKind = "link"
	ActAnalog     ActionKind = "analog"
	ActTrip       ActionKind = "trip"
	ActSlaveState ActionKind = "slave_state"
	ActDriveFault ActionKind = "drive_fault"
	ActRamp       ActionKind = "ramp"
	ActSerialPeer ActionKind = "serial_peer"
)

// actionOrder is the fixed lookup order of action keys in a step.
var actionOrder = []ActionKind{ActSet, ActLink, ActAnalog, ActTrip, ActSlaveState, ActDriveFault, ActRamp, ActSerialPeer}

// SlaveStateNames are the accepted slave_state names (D-14).
var SlaveStateNames = []string{"init", "preop", "safeop", "op", "not_present", "link_error", "ok"}

// SerialScripts are the accepted serial_peer scripts (D-15).
var SerialScripts = []string{"baader", "loopback", "none"}

// Action is one stimulus. Analog stores its number in Value as float64 and
// Unit as "mA", "V" or "raw". A slave ramp has Kind ActRamp, Slave, Channel
// and Unit set and Path empty.
type Action struct {
	Kind         ActionKind
	Path         string
	Value        any
	Slave        string
	Channel      int
	State        string
	StateCode    int64
	HasStateCode bool
	LFT          int
	Script       string
	Unit         string
	From, To     float64
	Over         time.Duration
}

// Expect is an assertion evaluated after Ticks k0..k0+Within.
type Expect struct {
	Path   string
	Value  any
	Within int
	Tol    float64
}

// Step is one [[step]] entry. Index is 1-based.
type Step struct {
	Index  int
	Line   int
	AtSet  bool
	At     time.Duration
	Cycle  int
	Action *Action
	Expect *Expect
}

// Scenario is a parsed scenario file.
type Scenario struct {
	Name, Description string
	Cycles            int
	Steps             []Step
}

type rawHeader struct {
	Name        string `toml:"name"`
	Description string `toml:"description"`
	Cycles      int64  `toml:"cycles"`
}

type rawFile struct {
	Scenario rawHeader        `toml:"scenario"`
	Step     []map[string]any `toml:"step"`
}

var stepHeader = regexp.MustCompile(`^\s*\[\[\s*step\s*\]\]`)

// Load reads and parses a scenario file, rejecting files over 1 MiB.
func Load(path string) (*Scenario, []diag.Diagnostic) {
	f, err := os.Open(path)
	if err != nil {
		return nil, []diag.Diagnostic{mkDiag(path, 0, "SCN001", "cannot read scenario: %v", err)}
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil {
		return nil, []diag.Diagnostic{mkDiag(path, 0, "SCN001", "cannot read scenario: %v", err)}
	}
	return Parse(data, path)
}

// Parse parses scenario TOML. file is used for diagnostic positions.
func Parse(data []byte, file string) (*Scenario, []diag.Diagnostic) {
	if len(data) > MaxFileSize {
		return nil, []diag.Diagnostic{mkDiag(file, 0, "SCN005", "scenario file exceeds %d bytes", MaxFileSize)}
	}
	lines := stepLines(data)
	if len(lines) > MaxSteps {
		return nil, []diag.Diagnostic{mkDiag(file, lines[MaxSteps], "SCN005", "scenario has more than %d steps", MaxSteps)}
	}
	var raw rawFile
	md, err := toml.Decode(string(data), &raw)
	if err != nil {
		line := 0
		var pe toml.ParseError
		if errors.As(err, &pe) {
			line = pe.Position.Line
			err = errors.New(pe.Message)
		}
		return nil, []diag.Diagnostic{mkDiag(file, line, "SCN001", "TOML syntax error: %v", err)}
	}
	p := &parser{file: file}
	undecoded := md.Undecoded()
	keys := make([]string, 0, len(undecoded))
	for _, k := range undecoded {
		if len(k) > 0 && k[0] == "step" {
			continue // step entries are validated key by key below
		}
		keys = append(keys, k.String())
	}
	sort.Strings(keys)
	for _, k := range keys {
		p.errf(0, "SCN002", "unknown key %q", k)
	}
	if len(raw.Step) > MaxSteps {
		p.errf(0, "SCN005", "scenario has more than %d steps", MaxSteps)
		return nil, p.diags
	}
	if raw.Scenario.Cycles < 0 || raw.Scenario.Cycles > math.MaxInt32 {
		p.errf(0, "SCN005", "scenario.cycles %d out of range", raw.Scenario.Cycles)
	}
	sc := &Scenario{Name: raw.Scenario.Name, Description: raw.Scenario.Description, Cycles: int(raw.Scenario.Cycles)}
	for i, m := range raw.Step {
		line := 0
		if i < len(lines) {
			line = lines[i]
		}
		if st, ok := p.step(i+1, line, m); ok {
			sc.Steps = append(sc.Steps, st)
		}
	}
	if len(p.diags) > 0 {
		return sc, p.diags
	}
	return sc, nil
}

// stepLines returns the 1-based line of every [[step]] header in order.
func stepLines(data []byte) []int {
	var out []int
	for i, l := range bytes.Split(data, []byte("\n")) {
		if stepHeader.Match(l) {
			out = append(out, i+1)
		}
	}
	return out
}

func mkDiag(file string, line int, code, format string, args ...any) diag.Diagnostic {
	return diag.Diagnostic{
		Severity: diag.Error,
		Pos:      source.Pos{File: file, Line: line, Col: 1},
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
	}
}

type parser struct {
	file  string
	diags []diag.Diagnostic
	bad   bool
}

func (p *parser) errf(line int, code, format string, args ...any) {
	p.diags = append(p.diags, mkDiag(p.file, line, code, format, args...))
	p.bad = true
}

func (p *parser) step(idx, line int, m map[string]any) (Step, bool) {
	p.bad = false
	st := Step{Index: idx, Line: line}
	pre := fmt.Sprintf("step %d", idx)

	for _, k := range sortedKeys(m) {
		if k == "at" || k == "cycle" || k == "expect" || isActionKey(k) {
			continue
		}
		p.errf(line, "SCN002", "%s: unknown key %q", pre, k)
	}

	at, hasAt := m["at"]
	cyc, hasCycle := m["cycle"]
	switch {
	case hasAt && hasCycle:
		p.errf(line, "SCN003", "%s: has both 'at' and 'cycle'; use exactly one trigger", pre)
	case !hasAt && !hasCycle:
		p.errf(line, "SCN003", "%s: needs a trigger, 'at' or 'cycle'", pre)
	case hasAt:
		s, ok := at.(string)
		d, err := time.ParseDuration(s)
		if !ok || err != nil || d < 0 {
			p.errf(line, "SCN005", "%s: at = %v is not a non-negative duration such as \"100ms\"", pre, fmtVal(at))
		} else {
			st.AtSet, st.At = true, d
		}
	default:
		n, ok := cyc.(int64)
		if !ok || n < 0 || n > math.MaxInt32 {
			p.errf(line, "SCN005", "%s: cycle = %v must be a non-negative integer", pre, fmtVal(cyc))
		} else {
			st.Cycle = int(n)
		}
	}

	var kinds []ActionKind
	for _, k := range actionOrder {
		if _, ok := m[string(k)]; ok {
			kinds = append(kinds, k)
		}
	}
	if len(kinds) > 1 {
		p.errf(line, "SCN004", "%s: has %d actions; at most one action per step", pre, len(kinds))
	} else if len(kinds) == 1 {
		st.Action = p.action(pre, line, kinds[0], m[string(kinds[0])])
	}
	if e, ok := m["expect"]; ok {
		st.Expect = p.expect(pre, line, e)
	}
	if _, ok := m["expect"]; !ok && len(kinds) == 0 {
		p.errf(line, "SCN004", "%s: has no action and no expect", pre)
	}
	return st, !p.bad
}

func isActionKey(k string) bool {
	for _, a := range actionOrder {
		if string(a) == k {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fmtVal(v any) string {
	if s, ok := v.(string); ok {
		return fmt.Sprintf("%q", s)
	}
	return fmt.Sprintf("%v", v)
}

// table checks that v is an inline table whose keys are all in allowed.
func (p *parser) table(pre string, line int, name string, v any, allowed ...string) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	if !ok {
		p.errf(line, "SCN004", "%s: %s must be an inline table", pre, name)
		return nil, false
	}
	for _, k := range sortedKeys(m) {
		found := false
		for _, a := range allowed {
			if a == k {
				found = true
				break
			}
		}
		if !found {
			p.errf(line, "SCN002", "%s: unknown key %q in %s", pre, k, name)
		}
	}
	return m, true
}

func (p *parser) str(pre string, line int, name, key string, m map[string]any) (string, bool) {
	v, ok := m[key]
	if !ok {
		p.errf(line, "SCN004", "%s: %s needs %q", pre, name, key)
		return "", false
	}
	s, ok := v.(string)
	if !ok || strings.TrimSpace(s) == "" {
		p.errf(line, "SCN004", "%s: %s.%s must be a non-empty string", pre, name, key)
		return "", false
	}
	return s, true
}

func (p *parser) num(pre string, line int, name, key string, m map[string]any) (float64, bool) {
	v, ok := m[key]
	if !ok {
		p.errf(line, "SCN004", "%s: %s needs %q", pre, name, key)
		return 0, false
	}
	switch n := v.(type) {
	case int64:
		return float64(n), true
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			p.errf(line, "SCN005", "%s: %s.%s must be finite", pre, name, key)
			return 0, false
		}
		return n, true
	}
	p.errf(line, "SCN005", "%s: %s.%s must be a number", pre, name, key)
	return 0, false
}

func (p *parser) integer(pre string, line int, name, key string, m map[string]any, lo, hi int64) (int64, bool) {
	v, ok := m[key]
	if !ok {
		p.errf(line, "SCN004", "%s: %s needs %q", pre, name, key)
		return 0, false
	}
	n, ok := v.(int64)
	if !ok || n < lo || n > hi {
		p.errf(line, "SCN005", "%s: %s.%s = %v must be an integer in %d..%d", pre, name, key, fmtVal(v), lo, hi)
		return 0, false
	}
	return n, true
}

func (p *parser) value(pre string, line int, name string, m map[string]any) (any, bool) {
	v, ok := m["value"]
	if !ok {
		p.errf(line, "SCN004", "%s: %s needs \"value\"", pre, name)
		return nil, false
	}
	switch v.(type) {
	case bool, int64, float64, string:
		return v, true
	}
	p.errf(line, "SCN005", "%s: %s.value must be a bool, integer, float or string", pre, name)
	return nil, false
}

func (p *parser) channel(pre string, line int, name string, m map[string]any) (int, bool) {
	n, ok := p.integer(pre, line, name, "channel", m, 1, MaxChannel)
	return int(n), ok
}

func (p *parser) over(pre string, line int, name string, m map[string]any) (time.Duration, bool) {
	v, ok := m["over"]
	if !ok {
		p.errf(line, "SCN004", "%s: %s needs \"over\"", pre, name)
		return 0, false
	}
	s, _ := v.(string)
	d, err := time.ParseDuration(s)
	if err != nil || d <= 0 || d > MaxOver {
		p.errf(line, "SCN005", "%s: %s.over = %v must be a duration in (0, 24h]", pre, name, fmtVal(v))
		return 0, false
	}
	return d, true
}

func (p *parser) action(pre string, line int, kind ActionKind, v any) *Action {
	name := string(kind)
	a := &Action{Kind: kind}
	var m map[string]any
	var ok bool
	switch kind {
	case ActSet, ActLink:
		if m, ok = p.table(pre, line, name, v, "path", "value"); !ok {
			return nil
		}
		a.Path, _ = p.str(pre, line, name, "path", m)
		a.Value, _ = p.value(pre, line, name, m)
		if kind == ActLink && a.Path != "" && !strings.Contains(a.Path, "^") {
			p.errf(line, "SCN004", "%s: link.path %q is not a link path (TIID^...)", pre, a.Path)
		}
	case ActAnalog:
		if m, ok = p.table(pre, line, name, v, "slave", "channel", "ma", "volts", "raw"); !ok {
			return nil
		}
		a.Slave, _ = p.str(pre, line, name, "slave", m)
		a.Channel, _ = p.channel(pre, line, name, m)
		var units []string
		for _, u := range []string{"ma", "volts", "raw"} {
			if _, has := m[u]; has {
				units = append(units, u)
			}
		}
		if len(units) != 1 {
			p.errf(line, "SCN004", "%s: analog needs exactly one of ma, volts or raw", pre)
			break
		}
		switch units[0] {
		case "ma":
			a.Unit = "mA"
			a.Value, _ = p.num(pre, line, name, "ma", m)
		case "volts":
			a.Unit = "V"
			a.Value, _ = p.num(pre, line, name, "volts", m)
		default:
			a.Unit = "raw"
			n, _ := p.integer(pre, line, name, "raw", m, math.MinInt16, math.MaxInt16)
			a.Value = float64(n)
		}
	case ActTrip:
		if m, ok = p.table(pre, line, name, v, "slave", "channel"); !ok {
			return nil
		}
		a.Slave, _ = p.str(pre, line, name, "slave", m)
		a.Channel, _ = p.channel(pre, line, name, m)
	case ActSlaveState:
		if m, ok = p.table(pre, line, name, v, "slave", "state"); !ok {
			return nil
		}
		a.Slave, _ = p.str(pre, line, name, "slave", m)
		switch s := m["state"].(type) {
		case string:
			ls := strings.ToLower(strings.TrimSpace(s))
			if !contains(SlaveStateNames, ls) {
				p.errf(line, "SCN004", "%s: slave_state.state %q is not one of %s or an integer", pre, s, strings.Join(SlaveStateNames, ", "))
			}
			a.State = ls
		case int64:
			if s < 0 || s > 0xFFFF {
				p.errf(line, "SCN005", "%s: slave_state.state %d out of range 0..65535", pre, s)
			}
			a.StateCode, a.HasStateCode = s, true
		case nil:
			p.errf(line, "SCN004", "%s: slave_state needs \"state\"", pre)
		default:
			p.errf(line, "SCN004", "%s: slave_state.state must be a name or an integer", pre)
		}
	case ActDriveFault:
		if m, ok = p.table(pre, line, name, v, "slave", "lft"); !ok {
			return nil
		}
		a.Slave, _ = p.str(pre, line, name, "slave", m)
		n, _ := p.integer(pre, line, name, "lft", m, 0, 0xFFFF)
		a.LFT = int(n)
	case ActRamp:
		if m, ok = p.table(pre, line, name, v, "path", "slave", "channel", "unit", "from", "to", "over"); !ok {
			return nil
		}
		_, hasPath := m["path"]
		_, hasSlave := m["slave"]
		switch {
		case hasPath == hasSlave:
			p.errf(line, "SCN004", "%s: ramp needs exactly one of path or slave", pre)
		case hasPath:
			a.Path, _ = p.str(pre, line, name, "path", m)
			for _, k := range []string{"channel", "unit"} {
				if _, has := m[k]; has {
					p.errf(line, "SCN004", "%s: ramp.%s is only valid with slave", pre, k)
				}
			}
		default:
			a.Slave, _ = p.str(pre, line, name, "slave", m)
			a.Channel, _ = p.channel(pre, line, name, m)
			u, _ := p.str(pre, line, name, "unit", m)
			switch strings.ToLower(u) {
			case "ma":
				a.Unit = "mA"
			case "v":
				a.Unit = "V"
			case "":
			default:
				p.errf(line, "SCN004", "%s: ramp.unit %q must be \"mA\" or \"V\"", pre, u)
			}
		}
		a.From, _ = p.num(pre, line, name, "from", m)
		a.To, _ = p.num(pre, line, name, "to", m)
		a.Over, _ = p.over(pre, line, name, m)
	case ActSerialPeer:
		if m, ok = p.table(pre, line, name, v, "slave", "script"); !ok {
			return nil
		}
		a.Slave, _ = p.str(pre, line, name, "slave", m)
		s, has := p.str(pre, line, name, "script", m)
		if has {
			ls := strings.ToLower(s)
			if !contains(SerialScripts, ls) {
				p.errf(line, "SCN004", "%s: serial_peer.script %q is not one of %s", pre, s, strings.Join(SerialScripts, ", "))
			}
			a.Script = ls
		}
	}
	return a
}

func (p *parser) expect(pre string, line int, v any) *Expect {
	m, ok := p.table(pre, line, "expect", v, "path", "value", "within", "tol")
	if !ok {
		return nil
	}
	e := &Expect{Tol: DefaultTol}
	e.Path, _ = p.str(pre, line, "expect", "path", m)
	e.Value, _ = p.value(pre, line, "expect", m)
	if _, has := m["within"]; has {
		n, _ := p.integer(pre, line, "expect", "within", m, 0, MaxWithin)
		e.Within = int(n)
	}
	if _, has := m["tol"]; has {
		t, ok := p.num(pre, line, "expect", "tol", m)
		if ok && t < 0 {
			p.errf(line, "SCN005", "%s: expect.tol must not be negative", pre)
		}
		e.Tol = t
	}
	return e
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
