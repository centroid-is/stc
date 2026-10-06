package main

import (
	"bytes"
	"encoding/json"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/opcua"
	"github.com/centroid-is/stc/pkg/opcua/bind"
	"github.com/centroid-is/stc/pkg/symtree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var st301Golden = filepath.Join("..", "..", "tests", "opcua_golden", "st301_shape.json")

// runSnapshot runs `stc opcua snapshot args...` in-process.
func runSnapshot(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	root := newRootCmd()
	root.SetArgs(append([]string{"opcua", "snapshot"}, args...))
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	err := root.Execute()
	return out.String(), errb.String(), err
}

// serveMapSource serves the st301_shape fixture's address space backed by
// a MapSource, so a test can assert that no write reached the source.
func serveMapSource(t *testing.T) (*opcua.Server, *opcua.MapSource) {
	t.Helper()
	p, err := loadServeProject([]string{serveFixture}, map[string]bool{"STC_SIM": true})
	require.NoError(t, err)
	tree, err := symtree.Build(p.res)
	require.NoError(t, err)
	ms := opcua.NewMapSource(map[string]any{})
	space, _ := opcua.Build(bind.Root(tree), ms)
	var srv *opcua.Server
	for attempt := 0; ; attempt++ {
		cfg := opcua.DefaultConfig()
		cfg.Endpoint = freeServeAddr(t)
		cfg.PKIDir = t.TempDir()
		srv, err = opcua.New(cfg)
		require.NoError(t, err)
		require.NoError(t, srv.Publish(space, ms))
		if err = srv.Start(); err == nil {
			break
		}
		_ = srv.Stop()
		require.Less(t, attempt, 2, "start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop() })
	return srv, ms
}

// TestOpcuaSnapshotGolden captures the served fixture into a file that is
// byte-identical to the golden, and proves the command never writes (T-29-05).
func TestOpcuaSnapshotGolden(t *testing.T) {
	srv, ms := serveMapSource(t)
	want, err := os.ReadFile(st301Golden)
	require.NoError(t, err)

	file := filepath.Join(t.TempDir(), "snap.json")
	out, _, err := runSnapshot(t, srv.Endpoint(), "--out", file)
	require.NoError(t, err)
	assert.Contains(t, out, "wrote "+file)
	got, err := os.ReadFile(file)
	require.NoError(t, err)
	if !bytes.Equal(want, got) {
		t.Fatalf("snapshot differs from the golden:\n%s", firstDiff(string(want), string(got)))
	}
	fi, err := os.Stat(file)
	require.NoError(t, err)
	if filepath.Separator == '/' {
		assert.Equal(t, os.FileMode(0o644), fi.Mode().Perm())
	}

	// Without --out the snapshot goes to stdout, also under --format json.
	out, _, err = runSnapshot(t, srv.Endpoint(), "--format", "json")
	require.NoError(t, err)
	assert.Equal(t, string(want), out)

	// --format json with --out prints a status object.
	out, _, err = runSnapshot(t, srv.Endpoint(), "--out", file, "--format", "json")
	require.NoError(t, err)
	var st snapshotStatus
	require.NoError(t, json.Unmarshal([]byte(out), &st), out)
	assert.Equal(t, file, st.Out)
	assert.Positive(t, st.Nodes)
	assert.Positive(t, st.DataTypes)

	// --root selects a sub-tree.
	out, _, err = runSnapshot(t, srv.Endpoint(), "--root", "ns=4;s=sensors")
	require.NoError(t, err)
	assert.Contains(t, out, `"root": "ns=4;s=sensors"`)
	assert.NotContains(t, out, "GVL_BatchLines")

	assert.Empty(t, ms.Writes(), "snapshot must never write")
}

// TestOpcuaSnapshotSecure captures from a secure-only stc serve with an
// auto-generated and an explicit client certificate.
func TestOpcuaSnapshotSecure(t *testing.T) {
	addr := freeServeAddr(t)
	s := startServe(t, true, serveFixture, "--opcua", addr, "--security", "basic256sha256")
	want, err := os.ReadFile(st301Golden)
	require.NoError(t, err)

	out, _, err := runSnapshot(t, s.info.Endpoint, "--security", "basic256sha256", "--pki-dir", t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, string(want), out)

	cp, kp, err := opcua.EnsureCert(t.TempDir(), "urn:stc:test-client")
	require.NoError(t, err)
	out, _, err = runSnapshot(t, s.info.Endpoint, "--security", "BASIC256SHA256", "--cert", cp, "--key", kp)
	require.NoError(t, err)
	assert.Equal(t, string(want), out)

	// SecurityPolicy None is refused by the secure-only server.
	_, _, err = runSnapshot(t, s.info.Endpoint, "--timeout", "3s")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "connecting to")
}

func TestOpcuaSnapshotErrors(t *testing.T) {
	srv, _ := serveMapSource(t)
	// A closed port: reserve and release.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	dead := "opc.tcp://" + l.Addr().String()
	require.NoError(t, l.Close())
	empty := t.TempDir()
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no endpoint", nil, "accepts 1 arg"},
		{"unreachable", []string{dead, "--timeout", "2s"}, "connecting to"},
		{"bad root syntax", []string{srv.Endpoint(), "--root", "PLC1"}, "invalid --root"},
		{"unknown root", []string{srv.Endpoint(), "--root", "ns=4;s=Nope"}, "browsing"},
		{"bad security", []string{srv.Endpoint(), "--security", "aes"}, "invalid --security"},
		{"cert without key", []string{srv.Endpoint(), "--security", "basic256sha256", "--cert", "c.der"}, "--cert and --key"},
		{"bad cert dir", []string{srv.Endpoint(), "--security", "basic256sha256", "--pki-dir", filepath.Join(st301Golden, "x")}, "client certificate"},
		{"bad timeout", []string{srv.Endpoint(), "--timeout", "0s"}, "--timeout must be positive"},
		{"bad out", []string{srv.Endpoint(), "--out", filepath.Join(empty, "no", "dir", "f.json")}, "no such file"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			start := time.Now()
			_, _, err := runSnapshot(t, c.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)
			assert.Less(t, time.Since(start), 15*time.Second)
		})
	}
}

// TestOpcuaSnapshotDefaultPKI uses the user cache dir for the generated
// client certificate when --pki-dir is not given.
func TestOpcuaSnapshotDefaultPKI(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	t.Setenv("HOME", cache)
	t.Setenv("LocalAppData", cache)
	srv, _ := serveMapSource(t)
	_, _, err := runSnapshot(t, srv.Endpoint(), "--security", "basic256sha256", "--timeout", "5s")
	require.NoError(t, err)
	found := false
	_ = filepath.Walk(cache, func(p string, _ os.FileInfo, _ error) error {
		if strings.Contains(p, "opcua-client-pki") {
			found = true
		}
		return nil
	})
	assert.True(t, found, "client certificate generated under the user cache dir")
}

func TestOpcuaSnapshotHelpExec(t *testing.T) {
	out, err := exec.Command(stcBinary, "opcua", "snapshot", "--help").CombinedOutput()
	require.NoError(t, err, string(out))
	for _, flag := range []string{"--out", "--root", "--security", "--cert", "--key", "--timeout"} {
		assert.Contains(t, string(out), flag)
	}
	out, err = exec.Command(stcBinary, "opcua", "snapshot", "opc.tcp://127.0.0.1:1", "--timeout", "1s").CombinedOutput()
	require.Error(t, err)
	assert.Contains(t, string(out), "connecting to")
}
