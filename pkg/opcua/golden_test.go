package opcua

import (
	"bytes"
	"context"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/awcullen/opcua/ua"
	"github.com/centroid-is/stc/pkg/opcua/opcuatest"
)

var update = flag.Bool("update", false, "regenerate golden files")

// goldenST301 is shared with plan 28-04, which proves the real symtree
// adapter yields the same shape.
var goldenST301 = filepath.Join("..", "..", "tests", "opcua_golden", "st301_shape.json")

func TestGoldenST301Shape(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil)
	r, src := st301Fixture()
	sp, _ := Build(r, src)
	if err := s.Publish(sp, src); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	c := dialAnon(t, s)
	ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
	defer cancel()
	got, err := opcuatest.BrowseSnapshot(ctx, c, ua.NewNodeIDString(4, "PLC1"))
	if err != nil {
		t.Fatalf("BrowseSnapshot: %v", err)
	}
	if *update {
		if err := os.MkdirAll(filepath.Dir(goldenST301), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenST301, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenST301)
	if err != nil {
		t.Fatalf("read golden (run with -update to create it): %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("browse snapshot differs from %s (run go test ./pkg/opcua -run TestGoldenST301Shape -update)\n%s", goldenST301, got)
	}
}
