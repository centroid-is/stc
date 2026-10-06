package tests

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

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

// TestTwinCATProbeFixtures is the CI-safe half of the Phase 19 acceptance
// gate. Every committed probe must parse clean except for the lines that
// Phase 20 owns, and every clean probe must format idempotently.
func TestTwinCATProbeFixtures(t *testing.T) {
	dir := probesDir(t)
	// Lines deliberately left for Phase 20 (see tests/twincat_probes/README.md).
	allowed := map[string]map[int]bool{
		"prog.st": {9: true},           // b := w.3; bit access
		"link.st": {9: true, 10: true}, // named function arguments to F_X
	}

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
				}
				if !allowed[name][l] {
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
	if count < 11 {
		t.Errorf("expected at least 11 probe fixtures, found %d", count)
	}
}

// TestTwinCATProbeOracle runs the Phase 19 classification gate over the large
// flattened customer sources. They are never committed, so the test only runs
// when STC_PROBES_DIR points at a local directory containing them; it logs
// counts and line numbers and never copies the sources.
func TestTwinCATProbeOracle(t *testing.T) {
	dir := os.Getenv("STC_PROBES_DIR")
	if dir == "" {
		t.Skip("STC_PROBES_DIR not set; oracle sources are local-only")
	}
	for _, name := range []string{"st301.st", "svncorecomponents.st"} {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Skipf("oracle file unavailable: %v", err)
			}
			src := string(data)
			lines, total := diagLines(name, src)
			t.Logf("%s: %d parse diagnostics on %d lines", name, total, len(lines))
			for _, l := range lines {
				if text := lineText(src, l); ownedLine(text) {
					t.Errorf("%s:%d: diagnostic on Phase 19 owned line: %s", name, l, strings.TrimSpace(text))
				}
			}
		})
	}
}
