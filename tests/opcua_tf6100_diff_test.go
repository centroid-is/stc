package tests

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
	"github.com/centroid-is/stc/pkg/opcua/opcuatest"
)

// TF6100 fidelity diff (OPCUA-09, D-10). The real capture holds plant
// identifiers, so it is optional: tests/opcua_golden/st301_real.json or
// the file named by STC_TF6100_SNAPSHOT. Only node ids and attribute
// differences are reported; snapshots carry no values.

const tf6100CaptureHint = "capture with: stc opcua snapshot opc.tcp://<plc>:4840 --out tests/opcua_golden/st301_real.json " +
	"(or set STC_TF6100_SNAPSHOT)"

// realSnapshotPath gives the real TF6100 capture path, or "" when absent.
func realSnapshotPath() string {
	if p := os.Getenv("STC_TF6100_SNAPSHOT"); p != "" {
		return p
	}
	p := filepath.Join(opcuaGolden, "st301_real.json")
	if _, err := os.Stat(p); err != nil {
		return ""
	}
	return p
}

// fidelityFailure diffs emulated against real and returns the failure
// text, or "" when they match.
func fidelityFailure(emulated, real *opcuatest.Snapshot, mode opcuatest.DiffMode) string {
	ds := opcuatest.Diff(emulated, real, mode)
	if len(ds) == 0 {
		return ""
	}
	kind := "subset"
	if mode == opcuatest.Full {
		kind = "full"
	}
	return fmt.Sprintf("%d differences (%s diff) between the stc address space and the real TF6100 capture:\n%s",
		len(ds), kind, opcuatest.FormatDiff(ds, 50))
}

func takePLC1(t *testing.T, c *client.Client) *opcuatest.Snapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), uaTimeout)
	defer cancel()
	snap, err := opcuatest.Take(ctx, c, ua.NewNodeIDString(4, "PLC1"))
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

// TestTF6100Diff runs the subset diff of the st301_shape fixture against
// the real capture and, with STC_SILD_DIR, the full diff of the real ST301
// project. It skips when no capture exists.
func TestTF6100Diff(t *testing.T) {
	path := realSnapshotPath()
	if path == "" {
		t.Skip("no real TF6100 snapshot; " + tf6100CaptureHint)
	}
	real, err := opcuatest.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("real capture: %d nodes, %d data types", len(real.Nodes), len(real.DataTypes))

	t.Run("subset st301_shape", func(t *testing.T) {
		emu := takePLC1(t, serveInProcess(t, analyzeSTDir(t, st301Fixture)))
		if msg := fidelityFailure(emu, real, opcuatest.Subset); msg != "" {
			t.Fatal(msg)
		}
	})

	t.Run("full ST301", func(t *testing.T) {
		dir := os.Getenv("STC_SILD_DIR")
		if dir == "" {
			t.Skip("STC_SILD_DIR not set; the full diff needs the ST301 sources")
		}
		proj := filepath.Join(dir, "ST301", "ST301 solution.tsproj")
		if _, err := os.Stat(proj); err != nil {
			t.Skipf("ST301 project unavailable: %v", err)
		}
		emu := takePLC1(t, serveInProcess(t, analyzeTwinCAT(t, proj)))
		if msg := fidelityFailure(emu, real, opcuatest.Full); msg != "" {
			t.Fatal(msg)
		}
	})
}

// TestTF6100DiffDetects drives the failure path without a real capture: a
// synthetic "real" copy of the golden with one access level changed.
func TestTF6100DiffDetects(t *testing.T) {
	emu := takePLC1(t, serveInProcess(t, analyzeSTDir(t, st301Fixture)))
	real, err := opcuatest.Load(filepath.Join(opcuaGolden, "st301_shape.json"))
	if err != nil {
		t.Fatal(err)
	}
	if msg := fidelityFailure(emu, real, opcuatest.Subset); msg != "" {
		t.Fatalf("served fixture should equal its golden:\n%s", msg)
	}
	if msg := fidelityFailure(emu, real, opcuatest.Full); msg != "" {
		t.Fatalf("full diff of fixture against golden:\n%s", msg)
	}

	const target = "ns=4;s=GVL_BatchLines.Drives_Line1[1].HMI.p_stat_Error"
	mutated := false
	for i, n := range real.Nodes {
		if n.NodeID == target {
			rw := uint8(3)
			real.Nodes[i].AccessLevel = &rw
			mutated = true
		}
	}
	if !mutated {
		t.Fatalf("%s not in the golden", target)
	}
	msg := fidelityFailure(emu, real, opcuatest.Subset)
	want := "1 differences (subset diff)"
	if !strings.Contains(msg, want) || !strings.Contains(msg, target+" accessLevel: emulated 1, real 3") {
		t.Fatalf("unexpected failure text:\n%s", msg)
	}

	// A real capture lacking an emulated node fails the subset diff; an
	// extra real node fails only the full diff.
	real.Nodes[0].NodeID = "ns=4;s=OnlyOnThePLC"
	msg = fidelityFailure(emu, real, opcuatest.Full)
	if !strings.Contains(msg, "(full diff)") || !strings.Contains(msg, "ns=4;s=OnlyOnThePLC node: emulated missing, real present") {
		t.Fatalf("unexpected full failure text:\n%s", msg)
	}
}

// TestTF6100DiffSkipHint pins the skip message and the env override.
func TestTF6100DiffSkipHint(t *testing.T) {
	if !strings.Contains(tf6100CaptureHint, "stc opcua snapshot") {
		t.Fatal("skip hint must name the capture command")
	}
	t.Setenv("STC_TF6100_SNAPSHOT", "/some/where.json")
	if got := realSnapshotPath(); got != "/some/where.json" {
		t.Fatalf("env override: %q", got)
	}
}
