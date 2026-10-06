package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/parser"
)

// phase22LiteralTyping lists the only error messages the TwinCAT dialect gate
// tolerates. Untyped integer literals are typed DINT until Phase 22 literal
// typing (RUNT-05), so `n := n + 1;` on an INT still reports this message.
// Remove entries here as Phase 22 lands; do not add unrelated ones.
var phase22LiteralTyping = []string{
	"cannot assign DINT to INT",
}

func allowedPhase22(msg string) bool {
	for _, m := range phase22LiteralTyping {
		if msg == m {
			return true
		}
	}
	return false
}

// checkSource runs the stc check pipeline (parse, then analyzer.Analyze) on a
// single file and returns parse and analysis diagnostics together.
func checkSource(t *testing.T, name, src string) []diag.Diagnostic {
	t.Helper()
	res := parser.Parse(name, src)
	ar := analyzer.Analyze([]*ast.SourceFile{res.File}, nil)
	return append(append([]diag.Diagnostic{}, res.Diags...), ar.Diags...)
}

func errorsOf(ds []diag.Diagnostic) []diag.Diagnostic {
	var out []diag.Diagnostic
	for _, d := range ds {
		if d.Severity == diag.Error {
			out = append(out, d)
		}
	}
	return out
}

// TestTwinCATDialectCheck is the analyzer half of the Phase 20 gate: every
// TwinCAT dialect ST suite and the Phase 19 hand-off fixture check clean
// except for the Phase 22 literal-typing message.
func TestTwinCATDialectCheck(t *testing.T) {
	root := filepath.Dir(probesDir(t))
	files, err := filepath.Glob(filepath.Join(root, "twincat_dialect", "*.st"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) < 7 {
		t.Fatalf("expected at least 7 dialect suites, found %d", len(files))
	}
	files = append(files, filepath.Join(root, "twincat_probes", "action_inside.st"))

	for _, path := range files {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			for _, d := range errorsOf(checkSource(t, name, string(data))) {
				if !allowedPhase22(d.Message) {
					t.Errorf("%s", d.String())
				}
			}
		})
	}

	t.Run("undeclared FB type reports SEMA037 once", func(t *testing.T) {
		ds := errorsOf(checkSource(t, "missing.st", `PROGRAM MAIN
VAR
	x : FB_DoesNotExist;
END_VAR
END_PROGRAM
`))
		if len(ds) != 1 || ds[0].Code != "SEMA037" {
			t.Fatalf("want exactly one SEMA037, got %v", ds)
		}
	})

	t.Run("ten standard FBs check clean without stubs", func(t *testing.T) {
		ds := checkSource(t, "stdfb.st", `PROGRAM MAIN
VAR
	b : BOOL;
	pv : INT := 3;
	cv : INT;
	q : BOOL;
	et : TIME;
	ton1 : TON;
	tof1 : TOF;
	tp1 : TP;
	ctu1 : CTU;
	ctu2 : CTU;
	ctd1 : CTD;
	ctd2 : CTD;
	ctud1 : CTUD;
	ctud2 : CTUD;
	rt : R_TRIG;
	ft : F_TRIG;
	sr1 : SR;
	sr2 : SR;
	rs1 : RS;
	rs2 : RS;
END_VAR
ton1(IN := b, PT := T#1S, Q => q, ET => et);
tof1(IN := b, PT := T#1S);
tp1(IN := b, PT := T#1S);
q := ton1.Q OR tof1.Q OR tp1.Q;
et := tof1.ET;
ctu1(CU := b, R := FALSE, PV := pv);
ctu2(CU := b, RESET := FALSE, PV := pv);
ctd1(CD := b, LD := FALSE, PV := pv);
ctd2(CD := b, LOAD := FALSE, PV := pv);
ctud1(CU := b, CD := FALSE, R := FALSE, LD := FALSE, PV := pv);
ctud2(CU := b, CD := FALSE, RESET := FALSE, LOAD := FALSE, PV := pv);
cv := ctu1.CV;
q := ctu2.Q OR ctd1.Q OR ctd2.Q OR ctud1.QU OR ctud2.QD;
rt(CLK := b);
ft(CLK := b);
q := rt.Q OR ft.Q;
sr1(S1 := b, R := FALSE);
sr2(SET1 := b, RESET := FALSE);
rs1(S := b, R1 := FALSE);
rs2(SET := b, RESET1 := FALSE);
q := sr1.Q1 OR sr2.Q1 OR rs1.Q1 OR rs2.Q1;
END_PROGRAM
`)
		for _, d := range errorsOf(ds) {
			t.Errorf("%s", d.String())
		}
		for _, d := range ds {
			if strings.HasPrefix(d.Code, "SEMA037") {
				t.Errorf("standard FB reported as undeclared: %s", d.String())
			}
		}
	})
}
