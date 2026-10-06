package opcuatest_test

import (
	"bytes"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/opcua"
	"github.com/centroid-is/stc/pkg/opcua/bind"
	"github.com/centroid-is/stc/pkg/opcua/opcuatest"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/symtree"
)

const netTimeout = 20 * time.Second

var golden = filepath.Join("..", "..", "..", "tests", "opcua_golden")

// serveFixture serves the st301_shape ST fixture on a free loopback port
// and returns an anonymous client.
func serveFixture(t *testing.T) *client.Client {
	t.Helper()
	dir := filepath.Join(golden, "st301_shape")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.SourceFile
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, pipeline.Parse(e.Name(), string(b), map[string]bool{"STC_SIM": true}).File)
	}
	res := analyzer.Analyze(files, nil, analyzer.AnalyzeOpts{})
	rt, err := interp.NewRuntime(res.Files, interp.RuntimeOpts{LibraryFiles: res.LibraryFiles})
	if err != nil {
		t.Fatal(err)
	}
	tree, err := symtree.Build(res)
	if err != nil {
		t.Fatal(err)
	}
	src := bind.NewRuntimeSource(rt)
	space, _ := opcua.Build(bind.Root(tree), src)
	var srv *opcua.Server
	for attempt := 0; ; attempt++ {
		cfg := opcua.DefaultConfig()
		cfg.Endpoint = freeAddr(t)
		cfg.PKIDir = t.TempDir()
		srv, err = opcua.New(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err = srv.Publish(space, src); err != nil {
			t.Fatal(err)
		}
		if err = srv.Start(); err == nil {
			break
		}
		_ = srv.Stop()
		if attempt == 2 {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = srv.Stop() })
	ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
	defer cancel()
	c, err := client.Dial(ctx, srv.Endpoint(), client.WithInsecureSkipVerify())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
		defer cancel()
		if err := c.Close(ctx); err != nil {
			_ = c.Abort(ctx)
		}
	})
	return c
}

func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// TestTakeMatchesGolden takes the served fixture and requires the golden
// bytes, then exercises the error paths of Take.
func TestTakeMatchesGolden(t *testing.T) {
	c := serveFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
	defer cancel()
	got, err := opcuatest.BrowseSnapshot(ctx, c, ua.NewNodeIDString(4, "PLC1"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(golden, "st301_shape.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("snapshot of the served fixture differs from the golden")
	}

	// A struct DataType node itself has a definition, and a sub-tree root works.
	snap, err := opcuatest.Take(ctx, c, ua.NewNodeIDString(4, "sensors"))
	if err != nil {
		t.Fatal(err)
	}
	if snap.Root != "ns=4;s=sensors" || len(snap.Nodes) == 0 {
		t.Fatalf("sub-tree snapshot: %+v", snap)
	}

	if _, err := opcuatest.BrowseSnapshot(ctx, c, ua.NewNodeIDString(4, "NoSuchNode")); err == nil ||
		!strings.Contains(err.Error(), "NoSuchNode") {
		t.Fatalf("want an error naming the bad root, got %v", err)
	}

	cctx, ccancel := context.WithCancel(context.Background())
	ccancel()
	if _, err := opcuatest.Take(cctx, c, ua.NewNodeIDString(4, "PLC1")); err == nil {
		t.Fatal("want an error from a cancelled context")
	}
}
