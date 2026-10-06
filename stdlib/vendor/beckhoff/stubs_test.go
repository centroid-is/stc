package beckhoff_test

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/stdlib/vendor/beckhoff"
)

// TestBeckhoffStubsParse verifies that all .st stub files in this directory
// parse without errors through the stc parser pipeline.
func TestBeckhoffStubsParse(t *testing.T) {
	dir := "."
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading directory: %v", err)
	}

	stFiles := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".st") {
			continue
		}
		stFiles++
		t.Run(e.Name(), func(t *testing.T) {
			path := filepath.Join(dir, e.Name())
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}

			result := pipeline.Parse(path, string(content), nil)

			// Check for parse errors (filter out warnings)
			var errors []string
			for _, d := range result.Diags {
				if d.Severity == diag.Error {
					errors = append(errors, d.Message)
				}
			}

			if len(errors) > 0 {
				for _, e := range errors {
					t.Errorf("parse error: %s", e)
				}
			}

			if result.File == nil {
				t.Fatalf("parsed file is nil")
			}

			if len(result.File.Declarations) == 0 {
				t.Errorf("no declarations found in %s", e.Name())
			}
		})
	}

	if stFiles == 0 {
		t.Fatal("no .st files found in directory")
	}
}

// TestBeckhoffStubDeclarationCounts verifies the expected number of
// declarations in each stub file.
func TestBeckhoffStubDeclarationCounts(t *testing.T) {
	tests := []struct {
		file     string
		minDecls int
		desc     string
	}{
		{"tc2_mc2.st", 10, "10 motion control FBs"},
		{"tc2_system.st", 9, "6 FBs + 3 functions (MEMCPY, MEMSET, MEMMOVE)"},
		{"tc2_utilities.st", 3, "1 FB + 2 functions (CRC16, CRC32)"},
		{"tc3_eventlogger.st", 2, "2 event logger FBs"},
		{"common_types.st", 4, "AXIS_REF, MC_Direction, T_AmsNetId/Port, E_OpenPath types"},
	}

	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			content, err := os.ReadFile(tt.file)
			if err != nil {
				t.Fatalf("reading %s: %v", tt.file, err)
			}

			result := pipeline.Parse(tt.file, string(content), nil)
			if result.File == nil {
				t.Fatalf("parsed file is nil")
			}

			got := len(result.File.Declarations)
			if got < tt.minDecls {
				t.Errorf("expected at least %d declarations (%s), got %d",
					tt.minDecls, tt.desc, got)
			}
		})
	}
}

// TestStubClosure verifies library-name lookup and dependency ordering.
func TestStubClosure(t *testing.T) {
	cases := []struct {
		lib  string
		want []string
	}{
		{"Tc2_EtherCAT", []string{"common_types.st", "tc2_system.st", "tc2_utilities.st", "tc2_ethercat.st"}},
		{"TC2_ETHERCAT", []string{"common_types.st", "tc2_system.st", "tc2_utilities.st", "tc2_ethercat.st"}},
		{"Tc2_System", []string{"common_types.st", "tc2_system.st"}},
		{"Tc2_Utilities", []string{"common_types.st", "tc2_system.st", "tc2_utilities.st"}},
		{"Tc3_Module", []string{"common_types.st", "tc2_system.st", "tc3_module.st"}},
		{"Tc2_ModbusSrv", []string{"common_types.st", "tc2_system.st", "tc2_modbussrv.st"}},
	}
	for _, c := range cases {
		got, ok := beckhoff.Closure(c.lib)
		if !ok {
			t.Errorf("Closure(%q): not found", c.lib)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("Closure(%q) = %v, want %v", c.lib, got, c.want)
		}
	}

	if _, ok := beckhoff.Closure("Tc2_Standard"); ok {
		t.Error("Closure(Tc2_Standard) should report not found (builtin)")
	}
	if !beckhoff.IsBuiltin("Tc2_Standard") || !beckhoff.IsBuiltin("tc2_standard") {
		t.Error("IsBuiltin(Tc2_Standard) should be true")
	}
	if _, ok := beckhoff.Closure("Nope"); ok {
		t.Error("Closure(Nope) should report not found")
	}
	if beckhoff.IsBuiltin("Nope") || beckhoff.IsBuiltin("Tc2_System") {
		t.Error("IsBuiltin should be false for non-builtin names")
	}

	wantLibs := []string{"tc2_ethercat", "tc2_mc2", "tc2_modbussrv", "tc2_serialcom",
		"tc2_system", "tc2_utilities", "tc3_eventlogger", "tc3_ipcdiag", "tc3_module"}
	libs := beckhoff.Libraries()
	if !reflect.DeepEqual(libs, wantLibs) {
		t.Errorf("Libraries() = %v, want %v", libs, wantLibs)
	}
	if !sort.StringsAreSorted(libs) {
		t.Error("Libraries() is not sorted")
	}
}

// TestStubFilesAllReachable ensures every embedded .st file belongs to the
// closure of some library, so a new stub cannot ship unreachable.
func TestStubFilesAllReachable(t *testing.T) {
	reachable := map[string]bool{}
	for _, lib := range beckhoff.Libraries() {
		files, ok := beckhoff.Closure(lib)
		if !ok {
			t.Fatalf("Closure(%q) not found", lib)
		}
		for _, f := range files {
			reachable[f] = true
		}
	}
	entries, err := beckhoff.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("reading embedded FS: %v", err)
	}
	n := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".st") {
			continue
		}
		n++
		if !reachable[e.Name()] {
			t.Errorf("embedded stub %s is not in any library closure", e.Name())
		}
	}
	if n == 0 {
		t.Fatal("no embedded .st files")
	}
}

// parseEmbedded parses the named embedded stub files in order.
func parseEmbedded(t *testing.T, names []string) []*ast.SourceFile {
	t.Helper()
	var files []*ast.SourceFile
	for _, name := range names {
		content, err := beckhoff.FS.ReadFile(name)
		if err != nil {
			t.Fatalf("reading embedded %s: %v", name, err)
		}
		r := pipeline.Parse(name, string(content), nil)
		for _, d := range r.Diags {
			if d.Severity == diag.Error {
				t.Errorf("%s: parse error: %s", name, d)
			}
		}
		if r.File == nil {
			t.Fatalf("%s: parsed file is nil", name)
		}
		files = append(files, r.File)
	}
	return files
}

func requireNoErrors(t *testing.T, diags []diag.Diagnostic) {
	t.Helper()
	for _, d := range diags {
		if d.Severity == diag.Error {
			t.Errorf("%s [%s]", d, d.Code)
		}
	}
}

// TestStubClosuresCheckClean analyses every library closure as user code,
// so initialisers, undeclared types and duplicate declarations in stubs are
// caught (library files are otherwise only registered, never checked).
func TestStubClosuresCheckClean(t *testing.T) {
	for _, lib := range beckhoff.Libraries() {
		t.Run(lib, func(t *testing.T) {
			names, ok := beckhoff.Closure(lib)
			if !ok {
				t.Fatalf("Closure(%q) not found", lib)
			}
			files := parseEmbedded(t, names)
			res := analyzer.Analyze(files, nil)
			requireNoErrors(t, res.Diags)
		})
	}
}

// allClosureFiles returns the de-duplicated union of every library closure,
// in Libraries() order.
func allClosureFiles(t *testing.T) []string {
	t.Helper()
	seen := map[string]bool{}
	var out []string
	for _, lib := range beckhoff.Libraries() {
		names, _ := beckhoff.Closure(lib)
		for _, n := range names {
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out
}

// TestStubUsage checks a program that calls every shipped EtherCAT, Modbus,
// serial and time FB with Beckhoff parameter names against all stubs.
func TestStubUsage(t *testing.T) {
	path := filepath.Join("testdata", "usage.st")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	r := pipeline.Parse(path, string(content), nil)
	for _, d := range r.Diags {
		if d.Severity == diag.Error {
			t.Errorf("parse error: %s", d)
		}
	}
	if r.File == nil {
		t.Fatal("usage fixture parsed to nil")
	}
	libs := parseEmbedded(t, allClosureFiles(t))
	res := analyzer.Analyze([]*ast.SourceFile{r.File}, nil, analyzer.AnalyzeOpts{LibraryFiles: libs})
	requireNoErrors(t, res.Diags)
}

// TestNoSVNCoreDeclarationsInStubs guards against shipping SVNCoreComponents
// symbols that would compete with the real library.
func TestNoSVNCoreDeclarationsInStubs(t *testing.T) {
	libs := parseEmbedded(t, allClosureFiles(t))
	res := analyzer.Analyze(libs, nil)
	for _, name := range []string{"E_EcSlaveState", "EcDiagParam", "FB_EcDeviceDiag"} {
		if sym := res.Symbols.LookupGlobal(name); sym != nil {
			t.Errorf("%s must not be declared by a Beckhoff stub", name)
		}
	}
}
