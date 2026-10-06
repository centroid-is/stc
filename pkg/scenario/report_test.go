package scenario

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func sampleReport(t *testing.T) *Report {
	t.Helper()
	sc := mustParse(t, `
[scenario]
name = "r"
[[step]]
cycle = 0
set = {path="a", value=1}
expect = {path="a", value=1}
[[step]]
cycle = 0
expect = {path="a", value=2}
[[step]]
cycle = 9
trip = {slave="S", channel=1}
`)
	f := newFake()
	rep, err := NewExecutor(sc, f).Run(2)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func keysOf(t *testing.T, b []byte) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	var k []string
	for x := range m {
		k = append(k, x)
	}
	sort.Strings(k)
	return k
}

func TestReportJSON(t *testing.T) {
	rep := sampleReport(t)
	b1, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	b2, _ := json.Marshal(rep)
	if !bytes.Equal(b1, b2) {
		t.Fatal("not byte-identical")
	}
	if got := keysOf(t, b1); !reflect.DeepEqual(got, []string{"assertions", "cycles", "diagnostics", "name", "passed", "sim_time_ns", "steps"}) {
		t.Fatalf("report keys %v", got)
	}
	var raw struct {
		Steps       []json.RawMessage `json:"steps"`
		Assertions  []json.RawMessage `json:"assertions"`
		Diagnostics []json.RawMessage `json:"diagnostics"`
	}
	_ = json.Unmarshal(b1, &raw)
	if got := keysOf(t, raw.Steps[0]); !reflect.DeepEqual(got, []string{"action", "at_ns", "cycle", "fired", "index", "line"}) {
		t.Fatalf("step keys %v", got)
	}
	if got := keysOf(t, raw.Assertions[1]); !reflect.DeepEqual(got, []string{"actual", "cycle", "expected", "message", "pass", "path", "step"}) {
		t.Fatalf("assertion keys %v", got)
	}
	if got := keysOf(t, raw.Diagnostics[0]); !reflect.DeepEqual(got, []string{"code", "end_pos", "message", "pos", "severity"}) {
		t.Fatalf("diag keys %v", got)
	}
	if !strings.Contains(string(raw.Diagnostics[0]), `"severity":"error"`) {
		t.Fatalf("severity: %s", raw.Diagnostics[0])
	}
}

func TestReportText(t *testing.T) {
	rep := sampleReport(t)
	rep.Steps[0].Error = "oops"
	var b strings.Builder
	rep.Text(&b)
	out := b.String()
	for _, w := range []string{
		"scenario r: 2 cycles",
		"step 1 (line 4) cycle 0: set a = 1 [fired] error: oops",
		"step 3 (line 11) cycle 9: trip S ch 1 [not fired]",
		"PASS step 1: a = 1 at cycle 0",
		"FAIL step 2: a at cycle 0: expected 2, got 1",
		"warning SCN010",
		"FAIL: 1 assertion(s) failed",
	} {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in\n%s", w, out)
		}
	}
	b.Reset()
	(&Report{Name: "ok", Passed: true}).Text(&b)
	if !strings.HasSuffix(b.String(), "PASS\n") {
		t.Errorf("pass: %q", b.String())
	}
}
