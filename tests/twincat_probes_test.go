package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/format"
	"github.com/centroid-is/stc/pkg/parser"
)

// ownedTokens are the source fragments whose lines Phase 19 owns. A parse
// diagnostic on a line containing any of them means a Phase 19 construct
// (attributes, GVLs, ACTIONs, AT addresses, empty call arguments) still fails.
var ownedTokens = []string{
	"{attribute", "{warning",
	"VAR_GLOBAL",
	"ACTION", "END_ACTION",
	" AT %",
	":= ,", ":=,", ":= )", ":=)",
	"=> ,", "=>,", "=> )", "=>)",
}

// ownedLine reports whether a source line contains a Phase 19 owned token.
func ownedLine(text string) bool {
	for _, tok := range ownedTokens {
		if strings.Contains(text, tok) {
			return true
		}
	}
	return false
}

// diagLines parses src and returns the sorted, de-duplicated 1-based lines
// that carry at least one parse diagnostic, plus the total diagnostic count.
func diagLines(filename, src string) ([]int, int) {
	res := parser.Parse(filename, src)
	seen := map[int]bool{}
	for _, d := range res.Diags {
		seen[d.Pos.Line] = true
	}
	lines := make([]int, 0, len(seen))
	for l := range seen {
		lines = append(lines, l)
	}
	sort.Ints(lines)
	return lines, len(res.Diags)
}

// lineText returns the 1-based line n of src, or "" when out of range.
func lineText(src string, n int) string {
	lines := strings.Split(src, "\n")
	if n < 1 || n > len(lines) {
		return ""
	}
	return strings.TrimRight(lines[n-1], "\r")
}

func probesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file location")
	}
	return filepath.Join(filepath.Dir(thisFile), "twincat_probes")
}

func TestOwnedLine(t *testing.T) {
	cases := map[string]bool{
		"{attribute 'qualified_only'}":    true,
		"{warning 'x'}":                   true,
		"VAR_GLOBAL":                      true,
		"ACTION A100_input:":              true,
		"END_ACTION":                      true,
		"    x AT %I* : BOOL;":            true,
		"f(a := , b := 1);":               true,
		"f(a :=, b := 1);":                true,
		"f(a := 1, b := );":               true,
		"f(a := 1, b :=);":                true,
		"f(q => , b := 1);":               true,
		"f(q =>, b := 1);":                true,
		"f(a := 1, q => );":               true,
		"f(a := 1, q =>);":                true,
		"b := w.3;":                       false,
		"x := F_X(a := 1, b := 2);":       false,
		"(* plain comment *) y := y + 1;": false,
	}
	for in, want := range cases {
		if got := ownedLine(in); got != want {
			t.Errorf("ownedLine(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestTwinCATProbeFixtures is the CI-safe half of the Phase 19 and Phase 20
// acceptance gates. Every committed probe must parse with zero diagnostics
// and format idempotently. There are no per-line allowances.
func TestTwinCATProbeFixtures(t *testing.T) {
	dir := probesDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".st") {
			continue
		}
		count++
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			src := string(data)
			lines, total := diagLines(name, src)
			for _, l := range lines {
				text := lineText(src, l)
				if ownedLine(text) {
					t.Errorf("%s:%d: diagnostic on Phase 19 owned line: %s", name, l, text)
				} else {
					t.Errorf("%s:%d: unexpected diagnostic: %s", name, l, text)
				}
			}
			if total != 0 {
				return
			}

			// fmt round trip: formatted output re-parses clean and is a fixed point.
			opts := format.DefaultFormatOptions()
			once := format.Format(parser.Parse(name, src).File, opts)
			res2 := parser.Parse(name, once)
			if len(res2.Diags) != 0 {
				t.Fatalf("formatted output does not re-parse clean: %v\n--- formatted ---\n%s", res2.Diags, once)
			}
			twice := format.Format(res2.File, opts)
			if once != twice {
				t.Errorf("fmt is not idempotent\n--- first ---\n%s\n--- second ---\n%s", once, twice)
			}
		})
	}
	if count < 15 {
		t.Errorf("expected at least 15 probe fixtures, found %d", count)
	}
}

// parseCode is the diagnostic code the parser reports (pkg/parser/error.go).
const parseCode = "P001"

// elementaryTypes are type names, keywords and attribute names kept verbatim
// when templating diagnostic messages; every
// other identifier-like word that is not plain lower-case prose is replaced.
var elementaryTypes = map[string]bool{
	"BOOL": true, "BYTE": true, "WORD": true, "DWORD": true, "LWORD": true,
	"SINT": true, "INT": true, "DINT": true, "LINT": true,
	"USINT": true, "UINT": true, "UDINT": true, "ULINT": true,
	"REAL": true, "LREAL": true, "TIME": true, "LTIME": true, "DATE": true,
	"TOD": true, "DT": true, "STRING": true, "WSTRING": true, "VOID": true,
	"FUNCTION_BLOCK": true, "FUNCTION": true, "PROGRAM": true, "METHOD": true,
	"REFERENCE": true, "POINTER": true, "TO": true, "ARRAY": true, "OF": true,
	"THIS": true, "SUPER": true, "REF": true, "ANY": true, "GVL": true,
	"AND": true, "OR": true, "XOR": true, "NOT": true, "MOD": true,
	"qualified_only": true, "strict": true, "to_string": true,
}

var (
	quotedRe = regexp.MustCompile(`"[^"]*"|'[^']*'`)
	wordRe   = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_.]*`)
	numberRe = regexp.MustCompile(`\b[0-9]+\b`)
)

// templateMessage replaces identifiers and numbers in a diagnostic message
// with placeholders so oracle buckets can be logged without leaking customer
// names (T-19-18, T-20-22).
func templateMessage(msg string) string {
	msg = quotedRe.ReplaceAllString(msg, "<id>")
	msg = wordRe.ReplaceAllStringFunc(msg, func(w string) string {
		if elementaryTypes[w] || w == "id" {
			return w
		}
		if strings.ToLower(w) == w && !strings.ContainsAny(w, "_.0123456789") {
			return w // ordinary prose
		}
		return "<id>"
	})
	return numberRe.ReplaceAllString(msg, "<n>")
}

// TestTemplateMessage pins the anonymisation used by the oracle log.
func TestTemplateMessage(t *testing.T) {
	cases := map[string]string{
		`undeclared identifier "fbMotor_1"`:            `undeclared identifier <id>`,
		`cannot assign DINT to INT`:                    `cannot assign DINT to INT`,
		`cannot assign ST_Recipe to E_Mode`:            `cannot assign <id> to <id>`,
		`F_Calc expects 3 argument(s), got 2`:          `<id> expects <n> argument(s), got <n>`,
		`type 'Tc2_System.T_MaxString' is not defined`: `type <id> is not defined`,
		`GVL 'G' is qualified_only; use G.x`:           `GVL <id> is qualified_only; use <id>`,
		`boolean operator AND requires BOOL operands`:  `boolean operator AND requires BOOL operands`,
	}
	for in, want := range cases {
		if got := templateMessage(in); got != want {
			t.Errorf("templateMessage(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestTwinCATProbeOracle runs the Phase 19 classification gate and the Phase
// 20 zero-parse-error gate over the large flattened customer sources. They
// are never committed, so the test only runs when STC_PROBES_DIR points at a
// local directory containing them; it logs counts, line numbers and
// templated messages and never copies the sources.
func TestTwinCATProbeOracle(t *testing.T) {
	dir := os.Getenv("STC_PROBES_DIR")
	if dir == "" {
		t.Skip("STC_PROBES_DIR not set; oracle sources are local-only")
	}
	var parsed []*ast.SourceFile
	for _, name := range []string{"svncorecomponents.st", "st301.st"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Skipf("oracle file unavailable: %v", err)
			}
			src := string(data)
			lines, total := diagLines(name, src)
			t.Logf("%s: %d parse diagnostics", name, total)
			for _, l := range lines {
				if text := lineText(src, l); ownedLine(text) {
					t.Errorf("%s:%d: diagnostic on Phase 19 owned line", name, l)
				} else {
					t.Errorf("%s:%d: parse diagnostic", name, l)
				}
			}
			if total != 0 {
				t.Errorf("%s: want 0 parse diagnostics, got %d", name, total)
			}

			// The stc check path: analyzer.Analyze must not report parse errors either.
			file := parser.Parse(name, src).File
			parsed = append(parsed, file)
			res := analyzer.Analyze([]*ast.SourceFile{file}, nil)
			p001, errs := 0, 0
			for _, d := range res.Diags {
				if d.Code == parseCode {
					p001++
				}
				if d.Severity == diag.Error {
					errs++
				}
			}
			t.Logf("%s: %d %s diagnostics and %d errors from analyzer.Analyze", name, p001, parseCode, errs)
			if p001 != 0 {
				t.Errorf("%s: want 0 %s diagnostics from analyzer.Analyze, got %d", name, parseCode, p001)
			}
		})
	}
	if len(parsed) < 2 {
		return
	}

	// Combined run, logged only: the remaining stc check buckets for Phases 21 and 22.
	res := analyzer.Analyze(parsed, nil)
	buckets := map[string]int{}
	errs := 0
	for _, d := range res.Diags {
		if d.Severity != diag.Error {
			continue
		}
		errs++
		buckets[d.Code+" "+templateMessage(d.Message)]++
	}
	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if buckets[keys[i]] != buckets[keys[j]] {
			return buckets[keys[i]] > buckets[keys[j]]
		}
		return keys[i] < keys[j]
	})
	t.Logf("combined analyzer.Analyze: %d errors in %d buckets", errs, len(keys))
	for i, k := range keys {
		if i == 15 {
			break
		}
		t.Logf("  %5d  %s", buckets[k], k)
	}
}
