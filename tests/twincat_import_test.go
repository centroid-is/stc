package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/twincat"
	"github.com/centroid-is/stc/pkg/vendor"
)

// The sildarvinnsla oracle is customer code. It is read only from
// STC_SILD_DIR at test time, nothing is copied into testdata, and the log
// carries templated messages and file basenames only (T-21-13).

// Bucket names. phase21 must stay empty; every other bucket is an explicit,
// owner-tagged allowlist (T-21-14). Errors that match no rule fail the test.
const (
	bucketPhase21  = "phase21"  // owned by this phase: must be zero
	bucketPhase22  = "phase22"  // literal typing, built-ins, pointer ops
	bucketDeferred = "deferred" // pre-existing checker bugs, see deferred-items.md
	bucketGenuine  = "genuine"  // real drift in sildarvinnsla HEAD, reported to the user
	bucketPhase24  = "phase24"  // TwinCAT IO-generated types (Baader)
)

// sildRule allowlists one error class. project and file narrow the rule when
// set; empty means any.
type sildRule struct {
	bucket  string
	code    string
	msg     *regexp.Regexp
	project string
	file    string
}

const intTypes = `(?:BYTE|WORD|DWORD|LWORD|SINT|INT|DINT|LINT|USINT|UINT|UDINT|ULINT)`

func rule(bucket, code, pattern, project, file string) sildRule {
	return sildRule{bucket, code, regexp.MustCompile(`^` + pattern + `$`), project, file}
}

// sildRules is data-driven so it can shrink as later phases land. Remove
// Phase 22 entries as Phase 22 lands; do not add unrelated ones.
var sildRules = []sildRule{
	// Phase 22: built-ins and conversions (Phase 20 deferred list).
	rule(bucketPhase22, "SEMA010", `undeclared identifier "(?:ADR|SIZEOF|SHL|SHR|[A-Z]+_TO_[A-Z]+)"`, "", ""),
	// Phase 22: untyped literals are DINT/LREAL until literal typing lands,
	// plus WORD/UINT implicit conversion (amsaddr.port is WORD).
	rule(bucketPhase22, "SEMA021", `cannot pass (?:DINT|LREAL|WORD) as input parameter "[^"]+" \(expected (?:`+intTypes+`|REAL)\)`, "", ""),
	rule(bucketPhase22, "SEMA001", `cannot assign (?:DINT|LREAL|WORD) to (?:`+intTypes+`|REAL)`, "", ""),
	// Phase 22: pointer comparison with 0 and pointer/STRING indexing
	// (PVOID is POINTER TO BYTE for now, see deferred-items.md).
	rule(bucketPhase22, "SEMA003", `cannot compare POINTER TO \w+ and DINT`, "", ""),
	rule(bucketPhase22, "SEMA023", `type (?:POINTER TO \w+|STRING) does not support indexing`, "Baader", ""),
	// Phase 22: chained assignment a := b := c (Phase 20 deferred list).
	rule(bucketPhase22, "P001", `expected Semicolon, got Assign`, "ST201", "MAIN.TcPOU"),
	rule(bucketPhase22, "P002", `unexpected Assign in statement context`, "ST201", "MAIN.TcPOU"),

	// Deferred checker bugs that predate Phase 21 (deferred-items.md):
	// a method call used as a statement with arguments reports "no member",
	// external read of an FB's internal VAR, and LEN typed as STRING by
	// generic candidate resolution.
	rule(bucketDeferred, "SEMA024", `type FB_Fifo has no member "(?:Configure|Push|nElemSize)"`, "Baader", ""),
	rule(bucketDeferred, "SEMA024", `type FB_SerialFramer has no member "SendBytes"`, "Baader", ""),
	rule(bucketDeferred, "SEMA001", `cannot assign STRING to INT`, "Baader", "F_ParseBraceNumbers.TcPOU"),
	rule(bucketDeferred, "SEMA001", `array index must be an integer type, got REAL`, "Baader", "FB_BaaderNoCamera.TcPOU"),

	// Genuine drift at sildarvinnsla HEAD: FB_TwoWayConveyor was renamed to
	// FB_BatchConveyor, ST_LineRecipe lost two members, ST_Batch is declared
	// nowhere.
	rule(bucketGenuine, "SEMA037", `undeclared type 'FB_TwoWayConveyor'`, "", ""),
	rule(bucketGenuine, "SEMA037", `undeclared type 'ST_Batch'`, "", ""),
	rule(bucketGenuine, "SEMA024", `type ST_LineRecipe has no member "(?:stopDistanceFromEnd|drivePastForDelivery)"`, "", ""),

	// Phase 24: IO-generated types live in per-box _Config/IO xti files.
	rule(bucketPhase24, "SEMA037", `undeclared type '(?:MDP5001_600_[0-9A-F]+|Status_FBD35181_Plc|Ctrl_903458A3_Plc)'`, "Baader", ""),
	rule(bucketPhase24, "SEMA024", `type Invalid does not support member access`, "Baader", ""),
}

// phase21Codes are always owned by this phase, whatever the message.
var phase21Codes = map[string]bool{"SEMA022": true, "SEMA033": true, "VEND020": true}

// classifySild returns the bucket for one error, or "" when nothing matches.
func classifySild(project string, d diag.Diagnostic) string {
	if strings.HasPrefix(d.Code, "P0") && !(project == "ST201" && filepath.Base(d.Pos.File) == "MAIN.TcPOU") {
		return bucketPhase21
	}
	if phase21Codes[d.Code] {
		return bucketPhase21
	}
	base := filepath.Base(d.Pos.File)
	for _, r := range sildRules {
		if r.code != d.Code || (r.project != "" && r.project != project) || (r.file != "" && r.file != base) {
			continue
		}
		if r.msg.MatchString(d.Message) {
			return r.bucket
		}
	}
	if d.Code == "SEMA037" || d.Code == "SEMA010" || strings.HasPrefix(d.Code, "P0") {
		return bucketPhase21 // unresolved symbol or parse error with no owner
	}
	return ""
}

func TestClassifySild(t *testing.T) {
	cases := []struct {
		project, code, msg, file, want string
	}{
		{"ST301", "SEMA010", `undeclared identifier "ADR"`, "MAIN.TcPOU", bucketPhase22},
		{"ST301", "SEMA010", `undeclared identifier "F_Unknown"`, "MAIN.TcPOU", bucketPhase21},
		{"ST301", "SEMA037", `undeclared type 'FB_TwoWayConveyor'`, "SPB03.TcGVL", bucketGenuine},
		{"ST301", "SEMA037", `undeclared type 'FB_Other'`, "SPB03.TcGVL", bucketPhase21},
		{"ST301", "SEMA033", `GVL 'SPB03' is qualified_only; use SPB03.gate`, "MAIN.TcPOU", bucketPhase21},
		{"ST301", "SEMA022", `"fb" is not callable (type FB_X)`, "MAIN.TcPOU", bucketPhase21},
		{"ST201", "P001", `expected Semicolon, got Assign`, "MAIN.TcPOU", bucketPhase22},
		{"ST301", "P001", `expected Semicolon, got Assign`, "MAIN.TcPOU", bucketPhase21},
		{"ST201", "P001", `expected Semicolon, got Assign`, "FB_X.TcPOU", bucketPhase21},
		{"ST301", "SEMA021", `cannot pass DINT as input parameter "nUnitID" (expected BYTE)`, "MAIN.TcPOU", bucketPhase22},
		{"ST301", "SEMA001", `cannot assign LREAL to REAL`, "MAIN.TcPOU", bucketPhase22},
		{"ST301", "SEMA001", `cannot assign STRING to INT`, "F_ParseBraceNumbers.TcPOU", ""},
		{"Baader", "SEMA001", `cannot assign STRING to INT`, "F_ParseBraceNumbers.TcPOU", bucketDeferred},
		{"Baader", "SEMA037", `undeclared type 'MDP5001_600_08AE5913'`, "FB_Echo.TcPOU", bucketPhase24},
		{"ST301", "SEMA037", `undeclared type 'MDP5001_600_08AE5913'`, "FB_Echo.TcPOU", bucketPhase21},
		{"ST301", "SEMA099", `something new`, "MAIN.TcPOU", ""},
	}
	for _, c := range cases {
		d := diag.Diagnostic{Severity: diag.Error, Code: c.code, Message: c.msg}
		d.Pos.File = filepath.Join("x", c.file)
		if got := classifySild(c.project, d); got != c.want {
			t.Errorf("%s %s %q in %s: got %q, want %q", c.project, c.code, c.msg, c.file, got, c.want)
		}
	}
}

type sildProject struct {
	name string
	path string
}

var sildProjects = []sildProject{
	{"ST301", "ST301/ST301 solution.tsproj"},
	{"ST101", "ST101/ST101 solution.tsproj"},
	{"ST201", "ST201/ST201 solution.tsproj"},
	{"Baader", "Baader/Sildarvinnsla Baader.tsproj"},
	{"SVNCoreComponents", "SVNCoreComponents/SVNCoreComponents/SVNCoreComponents.plcproj"},
}

func sildDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("STC_SILD_DIR")
	if dir == "" {
		t.Skip("STC_SILD_DIR not set; sildarvinnsla sources are local-only")
	}
	return dir
}

// TestSildarvinnslaImport imports each real project, checks it, and requires
// every error to land in an owner bucket with the Phase 21 bucket empty.
func TestSildarvinnslaImport(t *testing.T) {
	dir := sildDir(t)
	for _, p := range sildProjects {
		t.Run(p.name, func(t *testing.T) {
			path := filepath.Join(dir, filepath.FromSlash(p.path))
			if _, err := os.Stat(path); err != nil {
				t.Skipf("project unavailable: %v", err)
			}
			m, importDiags, err := twincat.Import(path, twincat.Options{})
			if err != nil {
				t.Fatalf("import: %v", err)
			}
			res := analyzer.AnalyzeProject(m, nil, nil)
			all := append(append([]diag.Diagnostic{}, importDiags...), res.Diags...)

			buckets := map[string]int{}
			codes := map[string]int{}
			templates := map[string]int{}
			total := 0
			for _, d := range all {
				if d.Severity != diag.Error {
					continue
				}
				total++
				codes[d.Code]++
				b := classifySild(p.name, d)
				base := filepath.Base(d.Pos.File)
				switch b {
				case bucketPhase21:
					t.Errorf("phase 21 owned: %s:%d: %s %s", base, d.Pos.Line, d.Code, templateMessage(d.Message))
				case "":
					t.Errorf("unclassified: %s:%d: %s %s", base, d.Pos.Line, d.Code, templateMessage(d.Message))
					b = "unclassified"
				}
				buckets[b]++
				templates[fmt.Sprintf("%-12s %-8s %s", b, d.Code, templateMessage(d.Message))]++
			}
			t.Logf("%s: plc=%s cycle=%s errors=%d", p.name, m.PlcName, cycleOf(m), total)
			t.Logf("  buckets: %s", sortedCounts(buckets))
			t.Logf("  codes:   %s", sortedCounts(codes))
			for _, l := range m.Libraries {
				t.Logf("  library %-18s %s", l.Name, l.ResolvedFrom)
				if l.ResolvedFrom == twincat.FromUnresolved {
					t.Errorf("library %s unresolved", l.Name)
				}
			}
			keys := make([]string, 0, len(templates))
			for k := range templates {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				t.Logf("  %4d %s", templates[k], k)
			}

			switch p.name {
			case "ST301":
				assertST301Model(t, m)
			case "Baader":
				assertTask(t, m, 20*time.Millisecond)
				assertResolved(t, m, map[string]string{"Tc3_IPCDiag": twincat.FromStub})
			case "ST101", "ST201":
				assertTask(t, m, time.Millisecond)
			}
		})
	}
}

func cycleOf(m *twincat.Model) string {
	if len(m.Tasks) == 0 {
		return "none"
	}
	return m.Tasks[0].CycleTime.String()
}

func sortedCounts(c map[string]int) string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s=%d", k, c[k])
	}
	return strings.Join(parts, " ")
}

func assertTask(t *testing.T, m *twincat.Model, cycle time.Duration) {
	t.Helper()
	if len(m.Tasks) == 0 {
		t.Fatalf("no tasks")
	}
	if m.Tasks[0].CycleTime != cycle {
		t.Errorf("cycle time = %s, want %s", m.Tasks[0].CycleTime, cycle)
	}
	if got := strings.Join(m.Tasks[0].Programs, ","); got != "MAIN" {
		t.Errorf("programs = %q, want MAIN", got)
	}
}

func assertResolved(t *testing.T, m *twincat.Model, want map[string]string) {
	t.Helper()
	got := map[string]string{}
	for _, l := range m.Libraries {
		got[l.Name] = l.ResolvedFrom
	}
	for name, from := range want {
		if got[name] != from {
			t.Errorf("library %s resolved_from = %q, want %q", name, got[name], from)
		}
	}
}

func assertST301Model(t *testing.T, m *twincat.Model) {
	t.Helper()
	if m.PlcName != "ST301" || m.AmsPort != 851 {
		t.Errorf("plc = %s:%d, want ST301:851", m.PlcName, m.AmsPort)
	}
	assertTask(t, m, time.Millisecond)
	assertResolved(t, m, map[string]string{
		"SVNCoreComponents": twincat.FromSibling,
		"Tc2_EtherCAT":      twincat.FromStub,
		"Tc2_ModbusSrv":     twincat.FromStub,
		"Tc2_System":        twincat.FromStub,
		"Tc3_Module":        twincat.FromStub,
		"Tc2_Standard":      twincat.FromBuiltin,
	})
	// Orchestrator ruling: these are SVNCoreComponents types, not Beckhoff.
	for _, name := range []string{"E_EcSlaveState", "EcDiagParam", "FB_EcDeviceDiag"} {
		lib := ""
		for _, s := range m.LibrarySources {
			if s.Name == name {
				lib = s.Library
				break
			}
		}
		if lib != "SVNCoreComponents" {
			t.Errorf("%s library = %q, want SVNCoreComponents", name, lib)
		}
	}
}

// TestSildarvinnslaExtract requires vendor extract to render every object in
// SVNCoreComponents as a stub that parses.
func TestSildarvinnslaExtract(t *testing.T) {
	dir := sildDir(t)
	path := filepath.Join(dir, "SVNCoreComponents", "SVNCoreComponents", "SVNCoreComponents.plcproj")
	if _, err := os.Stat(path); err != nil {
		t.Skipf("project unavailable: %v", err)
	}
	stubs, ds, err := vendor.ExtractProject(path)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	for _, d := range ds {
		if d.Severity == diag.Error {
			t.Errorf("extract diagnostic: %s %s", d.Code, templateMessage(d.Message))
		}
	}
	if len(stubs) != 65 {
		t.Errorf("extracted %d objects, want 65", len(stubs))
	}
	kinds := map[string]int{}
	for _, s := range stubs {
		kinds[string(s.Kind)]++
		res := parser.Parse(s.Name+".st", s.Text)
		for _, d := range res.Diags {
			if d.Severity == diag.Error {
				t.Errorf("%s: %s %s", s.RelPath, d.Code, templateMessage(d.Message))
			}
		}
		if s.Name == "FB_ATV320" && !strings.Contains(s.Text, "END_METHOD") {
			t.Errorf("FB_ATV320 stub has no methods")
		}
	}
	t.Logf("extracted %d objects: %s", len(stubs), sortedCounts(kinds))
}
