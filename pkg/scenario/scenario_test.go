package scenario

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/diag"
)

func codes(ds []diag.Diagnostic) string {
	var out []string
	for _, d := range ds {
		out = append(out, d.Code)
	}
	return strings.Join(out, ",")
}

func hasDiag(ds []diag.Diagnostic, code, substr string) bool {
	for _, d := range ds {
		if d.Code == code && strings.Contains(d.Message, substr) {
			return true
		}
	}
	return false
}

func TestLoadValid(t *testing.T) {
	sc, ds := Load(filepath.Join("testdata", "valid.toml"))
	if len(ds) != 0 {
		t.Fatalf("unexpected diagnostics: %v", ds)
	}
	if sc.Name != "jam" || sc.Description != "every action kind" || sc.Cycles != 300 {
		t.Fatalf("header: %+v", sc)
	}
	if len(sc.Steps) != 11 {
		t.Fatalf("steps = %d", len(sc.Steps))
	}
	s := sc.Steps
	if s[0].Index != 1 || s[0].Line != 6 || s[0].AtSet || s[0].Cycle != 0 {
		t.Errorf("step1: %+v", s[0])
	}
	if a := s[0].Action; a.Kind != ActSet || a.Path != "ECT.A1_01.I1" || a.Value != true {
		t.Errorf("set: %+v", a)
	}
	if !s[1].AtSet || s[1].At != 100*time.Millisecond {
		t.Errorf("at: %+v", s[1])
	}
	if a := s[1].Action; a.Kind != ActTrip || a.Slave != "DEMO.A1.03 (EL9222-5500)" || a.Channel != 1 {
		t.Errorf("trip: %+v", a)
	}
	if e := s[1].Expect; e.Path != "ECT.A1_03.p_stat_Enabled" || e.Value != false || e.Within != 2 || e.Tol != DefaultTol {
		t.Errorf("expect: %+v", e)
	}
	if a := s[2].Action; a.Kind != ActSlaveState || a.State != "not_present" || a.HasStateCode {
		t.Errorf("slave_state: %+v", a)
	}
	if a := s[3].Action; a.Kind != ActLink || a.Value != "16#FF" || !strings.Contains(a.Path, "^") {
		t.Errorf("link: %+v", a)
	}
	if a := s[4].Action; a.Kind != ActAnalog || a.Unit != "mA" || a.Value != 12.0 || a.Channel != 2 {
		t.Errorf("analog: %+v", a)
	}
	if a := s[5].Action; a.Kind != ActDriveFault || a.LFT != 23 {
		t.Errorf("drive_fault: %+v", a)
	}
	if a := s[6].Action; a.Kind != ActRamp || a.Path != "GVL.rSpeed" || a.From != 0 || a.To != 100.5 || a.Over != 50*time.Millisecond {
		t.Errorf("path ramp: %+v", a)
	}
	if a := s[7].Action; a.Kind != ActRamp || a.Path != "" || a.Slave == "" || a.Channel != 1 || a.Unit != "mA" || a.From != 4 || a.To != 20 || a.Over != time.Second {
		t.Errorf("slave ramp: %+v", a)
	}
	if a := s[8].Action; !a.HasStateCode || a.StateCode != 17 {
		t.Errorf("slave_state int: %+v", a)
	}
	if a := s[9].Action; a.Kind != ActSerialPeer || a.Script != "baader" {
		t.Errorf("serial_peer: %+v", a)
	}
	if s[10].Action != nil || s[10].Expect.Tol != 0.01 || s[10].Expect.Value != 100.5 {
		t.Errorf("expect-only: %+v", s[10])
	}
}

func TestLoadBadTrigger(t *testing.T) {
	_, ds := Load(filepath.Join("testdata", "bad_trigger.toml"))
	if len(ds) != 2 {
		t.Fatalf("diags: %v", ds)
	}
	if ds[0].Code != "SCN003" || ds[0].Pos.Line != 5 || !strings.Contains(ds[0].Message, "step 2") || ds[0].Pos.File != filepath.Join("testdata", "bad_trigger.toml") {
		t.Errorf("both: %+v", ds[0])
	}
	if ds[1].Code != "SCN003" || ds[1].Pos.Line != 10 || !strings.Contains(ds[1].Message, "step 3") {
		t.Errorf("neither: %+v", ds[1])
	}
}

func TestLoadErrors(t *testing.T) {
	dir := t.TempDir()
	if _, ds := Load(filepath.Join(dir, "missing.toml")); codes(ds) != "SCN001" {
		t.Errorf("missing: %v", ds)
	}
	big := filepath.Join(dir, "big.toml")
	if err := os.WriteFile(big, make([]byte, MaxFileSize+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ds := Load(big); codes(ds) != "SCN005" {
		t.Errorf("big: %v", ds)
	}
	if _, ds := Load(dir); codes(ds) != "SCN001" {
		t.Errorf("dir: %v", ds)
	}
}

func TestParseTooManySteps(t *testing.T) {
	var b strings.Builder
	for i := 0; i <= MaxSteps; i++ {
		b.WriteString("[[step]]\ncycle=1\n")
	}
	if _, ds := Parse([]byte(b.String()), "x"); codes(ds) != "SCN005" || !hasDiag(ds, "SCN005", "more than") {
		t.Errorf("diags: %v", ds)
	}
}

func TestParseCases(t *testing.T) {
	cases := []struct {
		name, src, code, msg string
	}{
		{"syntax", "[[step]]\ncycle = 1\nset = {path = \n", "SCN001", "TOML"},
		{"unknown step key", "[[step]]\ncycle = 1\ncolour = 1\nset={path=\"a\",value=1}", "SCN002", `step 1: unknown key "colour"`},
		{"unknown inline key", "[[step]]\ncycle = 1\nset={path=\"x\",value=1,extra=2}", "SCN002", `"extra" in set`},
		{"unknown top key", "foo = 1\n[[step]]\ncycle=1\nset={path=\"a\",value=1}", "SCN002", `"foo"`},
		{"unknown header key", "[scenario]\nbar = 1\n", "SCN002", `"scenario.bar"`},
		{"two actions", "[[step]]\ncycle=1\nset={path=\"a\",value=1}\ntrip={slave=\"s\",channel=1}", "SCN004", "2 actions"},
		{"no action", "[[step]]\ncycle=1", "SCN004", "no action"},
		{"analog two units", "[[step]]\ncycle=1\nanalog={slave=\"s\",channel=1,ma=4,volts=1}", "SCN004", "exactly one of ma"},
		{"analog no unit", "[[step]]\ncycle=1\nanalog={slave=\"s\",channel=1}", "SCN004", "exactly one of ma"},
		{"analog volts", "[[step]]\ncycle=1\nanalog={slave=\"s\",channel=1,volts=1.5}", "", ""},
		{"analog raw", "[[step]]\ncycle=1\nanalog={slave=\"s\",channel=1,raw=-5}", "", ""},
		{"analog raw range", "[[step]]\ncycle=1\nanalog={slave=\"s\",channel=1,raw=40000}", "SCN005", "raw"},
		{"analog raw float", "[[step]]\ncycle=1\nanalog={slave=\"s\",channel=1,raw=1.5}", "SCN005", "raw"},
		{"ma string", "[[step]]\ncycle=1\nanalog={slave=\"s\",channel=1,ma=\"x\"}", "SCN005", "number"},
		{"ma nan", "[[step]]\ncycle=1\nanalog={slave=\"s\",channel=1,ma=nan}", "SCN005", "finite"},
		{"ramp neither", "[[step]]\ncycle=1\nramp={from=0,to=1,over=\"1s\"}", "SCN004", "exactly one of path or slave"},
		{"ramp both", "[[step]]\ncycle=1\nramp={path=\"p\",slave=\"s\",from=0,to=1,over=\"1s\"}", "SCN004", "exactly one of path or slave"},
		{"ramp path with unit", "[[step]]\ncycle=1\nramp={path=\"p\",unit=\"mA\",from=0,to=1,over=\"1s\"}", "SCN004", "ramp.unit is only valid"},
		{"ramp bad unit", "[[step]]\ncycle=1\nramp={slave=\"s\",channel=1,unit=\"A\",from=0,to=1,over=\"1s\"}", "SCN004", "ramp.unit"},
		{"ramp volts", "[[step]]\ncycle=1\nramp={slave=\"s\",channel=1,unit=\"v\",from=0,to=1,over=\"1s\"}", "", ""},
		{"over zero", "[[step]]\ncycle=1\nramp={path=\"p\",from=0,to=1,over=\"0s\"}", "SCN005", "over"},
		{"over huge", "[[step]]\ncycle=1\nramp={path=\"p\",from=0,to=1,over=\"25h\"}", "SCN005", "over"},
		{"over missing", "[[step]]\ncycle=1\nramp={path=\"p\",from=0,to=1}", "SCN004", "needs \"over\""},
		{"from missing", "[[step]]\ncycle=1\nramp={path=\"p\",to=1,over=\"1s\"}", "SCN004", "needs \"from\""},
		{"at fast", "[[step]]\nat=\"fast\"\nset={path=\"a\",value=1}", "SCN005", "at = \"fast\""},
		{"at negative", "[[step]]\nat=\"-1s\"\nset={path=\"a\",value=1}", "SCN005", "at"},
		{"at int", "[[step]]\nat=5\nset={path=\"a\",value=1}", "SCN005", "at"},
		{"cycle negative", "[[step]]\ncycle=-1\nset={path=\"a\",value=1}", "SCN005", "cycle"},
		{"cycle string", "[[step]]\ncycle=\"1\"\nset={path=\"a\",value=1}", "SCN005", "cycle"},
		{"within negative", "[[step]]\ncycle=1\nexpect={path=\"a\",value=1,within=-1}", "SCN005", "within"},
		{"within huge", "[[step]]\ncycle=1\nexpect={path=\"a\",value=1,within=1000001}", "SCN005", "within"},
		{"tol negative", "[[step]]\ncycle=1\nexpect={path=\"a\",value=1.0,tol=-1}", "SCN005", "tol"},
		{"expect not table", "[[step]]\ncycle=1\nexpect=1", "SCN004", "expect must be an inline table"},
		{"expect no value", "[[step]]\ncycle=1\nexpect={path=\"a\"}", "SCN004", "needs \"value\""},
		{"value array", "[[step]]\ncycle=1\nset={path=\"a\",value=[1]}", "SCN005", "value must be"},
		{"channel zero", "[[step]]\ncycle=1\ntrip={slave=\"s\",channel=0}", "SCN005", "channel"},
		{"channel 65", "[[step]]\ncycle=1\ntrip={slave=\"s\",channel=65}", "SCN005", "channel"},
		{"channel missing", "[[step]]\ncycle=1\ntrip={slave=\"s\"}", "SCN004", "needs \"channel\""},
		{"slave empty", "[[step]]\ncycle=1\ntrip={slave=\" \",channel=1}", "SCN004", "non-empty string"},
		{"slave missing", "[[step]]\ncycle=1\ntrip={channel=1}", "SCN004", "needs \"slave\""},
		{"set not table", "[[step]]\ncycle=1\nset=1", "SCN004", "set must be an inline table"},
		{"link no caret", "[[step]]\ncycle=1\nlink={path=\"GVL.x\",value=1}", "SCN004", "not a link path"},
		{"state upper", "[[step]]\ncycle=1\nslave_state={slave=\"s\",state=\"SafeOp\"}", "", ""},
		{"state unknown", "[[step]]\ncycle=1\nslave_state={slave=\"s\",state=\"sleepy\"}", "SCN004", "sleepy"},
		{"state range", "[[step]]\ncycle=1\nslave_state={slave=\"s\",state=70000}", "SCN005", "out of range"},
		{"state missing", "[[step]]\ncycle=1\nslave_state={slave=\"s\"}", "SCN004", "needs \"state\""},
		{"state bool", "[[step]]\ncycle=1\nslave_state={slave=\"s\",state=true}", "SCN004", "name or an integer"},
		{"lft range", "[[step]]\ncycle=1\ndrive_fault={slave=\"s\",lft=-1}", "SCN005", "lft"},
		{"script loopback", "[[step]]\ncycle=1\nserial_peer={slave=\"s\",script=\"loopback\"}", "", ""},
		{"script none", "[[step]]\ncycle=1\nserial_peer={slave=\"s\",script=\"none\"}", "", ""},
		{"script bad", "[[step]]\ncycle=1\nserial_peer={slave=\"s\",script=\"modbus\"}", "SCN004", "modbus"},
		{"header cycles negative", "[scenario]\ncycles=-1\n", "SCN005", "scenario.cycles"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ds := Parse([]byte(c.src), "f.toml")
			if c.code == "" {
				if len(ds) != 0 {
					t.Fatalf("unexpected: %v", ds)
				}
				return
			}
			if !hasDiag(ds, c.code, c.msg) {
				t.Fatalf("want %s %q, got %v", c.code, c.msg, ds)
			}
			for _, d := range ds {
				if d.Pos.File != "f.toml" || d.Severity != diag.Error {
					t.Errorf("bad position/severity: %+v", d)
				}
			}
		})
	}
}

func TestParseSyntaxLine(t *testing.T) {
	_, ds := Parse([]byte("[[step]]\ncycle = 1\n\nset = {path = \n"), "f.toml")
	if len(ds) != 1 || ds[0].Code != "SCN001" || ds[0].Pos.Line != 4 {
		t.Fatalf("diags: %+v", ds)
	}
}
