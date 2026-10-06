package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
	"github.com/centroid-is/stc/pkg/opcua"
	"github.com/centroid-is/stc/pkg/opcua/opcuatest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const serveTimeout = 20 * time.Second

var serveFixture = filepath.Join("..", "..", "tests", "opcua_golden", "st301_shape")

// freeServeAddr returns a 127.0.0.1 address with a currently free port.
func freeServeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}

// syncBuffer is a goroutine-safe bytes.Buffer.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

// startupInfo decodes the --format json start-up line.
type startupInfo struct {
	Endpoint       string `json:"endpoint"`
	NamespaceIndex uint16 `json:"namespace_index"`
	NodeCount      int    `json:"node_count"`
	Cycle          string `json:"cycle"`
	Diagnostics    []struct {
		Severity string `json:"severity"`
		Code     string `json:"code"`
		Message  string `json:"message"`
	} `json:"diagnostics"`
}

// served is an in-process `stc serve` run.
type served struct {
	info   startupInfo
	stdout *syncBuffer
	stderr *syncBuffer
	cancel context.CancelFunc
	done   chan struct{} // closed when serve returned; err holds its error
	err    error
}

// wait waits for serve to return and gives its error.
func (s *served) wait(t *testing.T) error {
	t.Helper()
	select {
	case <-s.done:
		return s.err
	case <-time.After(serveTimeout):
		t.Fatal("serve did not stop")
		return nil
	}
}

// startServe runs `stc serve args...` in-process and waits for its JSON
// start-up line when json is set, else for the first stdout line.
func startServe(t *testing.T, json bool, args ...string) *served {
	t.Helper()
	pr, pw := io.Pipe()
	s := &served{stdout: &syncBuffer{}, stderr: &syncBuffer{}, done: make(chan struct{})}
	root := newRootCmd()
	all := append([]string{"serve", "--pki-dir", t.TempDir()}, args...)
	if json {
		all = append(all, "--format", "json")
	}
	root.SetArgs(all)
	root.SetOut(io.MultiWriter(pw, s.stdout))
	root.SetErr(s.stderr)
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	go func() {
		s.err = root.ExecuteContext(ctx)
		_ = pw.Close()
		close(s.done)
	}()
	go func() { _, _ = io.Copy(io.Discard, pr) }() // drained after the first line below
	t.Cleanup(func() {
		cancel()
		select {
		case <-s.done:
		case <-time.After(serveTimeout):
			t.Error("serve did not stop")
		}
	})
	deadline := time.Now().Add(serveTimeout)
	for time.Now().Before(deadline) {
		if line, _, ok := strings.Cut(s.stdout.String(), "\n"); ok {
			if json {
				require.NoError(t, jsonUnmarshal(line, &s.info), line)
			}
			return s
		}
		select {
		case <-s.done:
			t.Fatalf("serve exited early: %v\nstderr: %s", s.err, s.stderr.String())
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Fatalf("no start-up line; stderr: %s", s.stderr.String())
	return nil
}

func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }

func dialServe(t *testing.T, endpoint string, opts ...client.Option) *client.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), serveTimeout)
	defer cancel()
	opts = append([]client.Option{client.WithInsecureSkipVerify()}, opts...)
	c, err := client.Dial(ctx, endpoint, opts...)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), serveTimeout)
		defer cancel()
		if err := c.Close(ctx); err != nil {
			_ = c.Abort(ctx)
		}
	})
	return c
}

func readNode(t *testing.T, c *client.Client, id ua.NodeID) ua.DataValue {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), serveTimeout)
	defer cancel()
	res, err := c.Read(ctx, &ua.ReadRequest{NodesToRead: []ua.ReadValueID{{NodeID: id, AttributeID: ua.AttributeIDValue}}})
	require.NoError(t, err)
	require.Len(t, res.Results, 1)
	return res.Results[0]
}

func writeNode(t *testing.T, c *client.Client, id ua.NodeID, v any) ua.StatusCode {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), serveTimeout)
	defer cancel()
	res, err := c.Write(ctx, &ua.WriteRequest{NodesToWrite: []ua.WriteValue{{
		NodeID: id, AttributeID: ua.AttributeIDValue,
		Value: ua.NewDataValue(v, ua.Good, time.Time{}, 0, time.Time{}, 0),
	}}})
	require.NoError(t, err)
	require.Len(t, res.Results, 1)
	return res.Results[0]
}

// eventually polls cond until it holds or the deadline passes.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func requireRunning(t *testing.T, c *client.Client) {
	t.Helper()
	dv := readNode(t, c, ua.VariableIDServerServerStatusState)
	require.Equal(t, ua.Good, dv.StatusCode)
	require.Equal(t, int32(0), dv.Value, "i=2259 Running")
}

func s4(path string) ua.NodeID { return ua.NewNodeIDString(4, path) }

// TestServeST301Parity serves the parsed ST fixture and checks the browse
// snapshot against the 28-03 golden, then reads and writes through the
// live scan.
func TestServeST301Parity(t *testing.T) {
	addr := freeServeAddr(t)
	s := startServe(t, true, "--project", serveFixture, "--opcua", addr, "--cycle", "5ms")
	assert.Equal(t, "opc.tcp://"+addr, s.info.Endpoint)
	assert.Equal(t, uint16(4), s.info.NamespaceIndex)
	assert.Positive(t, s.info.NodeCount)
	assert.Equal(t, "5ms", s.info.Cycle)
	require.NotEmpty(t, s.info.Diagnostics)
	assert.Equal(t, opcua.CodePointerSkipped, s.info.Diagnostics[0].Code)

	c := dialServe(t, s.info.Endpoint)
	requireRunning(t, c)

	ctx, cancel := context.WithTimeout(context.Background(), serveTimeout)
	defer cancel()
	got, err := opcuatest.BrowseSnapshot(ctx, c, s4("PLC1"))
	require.NoError(t, err)
	want, err := os.ReadFile(filepath.Join("..", "..", "tests", "opcua_golden", "st301_shape.json"))
	require.NoError(t, err)
	if !bytes.Equal(want, got) {
		t.Fatalf("parsed ST browse snapshot differs from the golden:\n%s", firstDiff(string(want), string(got)))
	}

	// Initial value from the declaration.
	dv := readNode(t, c, s4("GVL_BatchLines.nBatchCount"))
	assert.Equal(t, int32(17), dv.Value)
	dv = readNode(t, c, s4("GVL_Roe.aRoe"))
	assert.Equal(t, []int16{1, 2, 3, 4}, dv.Value)

	// A write lands at a cycle boundary and the scan reacts to it.
	require.Equal(t, ua.Good, writeNode(t, c, s4("GVL_BatchLines.nBatchCount"), int32(42)))
	require.Equal(t, ua.Good, writeNode(t, c, s4("GVL_BatchLines.Conveyor.Cfg.rMax"), float32(3.5)))
	require.Equal(t, ua.Good, writeNode(t, c, s4("GVL_BatchLines.Conveyor.xRun"), true))
	eventually(t, "nBatchCount=42", func() bool {
		return readNode(t, c, s4("GVL_BatchLines.nBatchCount")).Value == int32(42)
	})
	// rSpeed is computed by the scan after the 500 ms TON elapses.
	eventually(t, "rSpeed follows rMax", func() bool {
		return readNode(t, c, s4("GVL_BatchLines.Conveyor.rSpeed")).Value == float32(3.5)
	})
	// rSpeed is read-only (Access '1').
	assert.Equal(t, ua.BadNotWritable, writeNode(t, c, s4("GVL_BatchLines.Conveyor.rSpeed"), float32(1)))

	s.cancel()
	assert.NoError(t, s.wait(t))
}

func TestServeBasic256Sha256(t *testing.T) {
	addr := freeServeAddr(t)
	s := startServe(t, true, serveFixture, "--opcua", addr, "--security", "basic256sha256")
	cp, kp, err := opcua.EnsureCert(t.TempDir(), "urn:stc:test-client")
	require.NoError(t, err)
	c := dialServe(t, s.info.Endpoint,
		client.WithSecurityPolicyURI(ua.SecurityPolicyURIBasic256Sha256, ua.MessageSecurityModeSignAndEncrypt),
		client.WithClientCertificatePaths(cp, kp))
	requireRunning(t, c)

	// The secure-only server refuses SecurityPolicy None.
	ctx, cancel := context.WithTimeout(context.Background(), serveTimeout)
	defer cancel()
	nc, err := client.Dial(ctx, s.info.Endpoint, client.WithInsecureSkipVerify(),
		client.WithSecurityPolicyURI(ua.SecurityPolicyURINone, ua.MessageSecurityModeNone))
	if err == nil {
		_ = nc.Abort(ctx)
	}
	assert.Error(t, err)
}

func TestServeTextScanErrorAndRealtime(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "GVL.st"), []byte(`VAR_GLOBAL
	{attribute 'OPC.UA.DA' := '1'}
	x : INT := 5;
	zero : INT;
END_VAR
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.st"), []byte(`PROGRAM MAIN
GVL.x := GVL.x / GVL.zero;
END_PROGRAM
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignored"), 0o644))
	addr := freeServeAddr(t)
	s := startServe(t, false, dir, "--opcua", addr, "--realtime", "--cycle", "2ms")
	assert.Contains(t, s.stdout.String(), "listening on opc.tcp://"+addr)
	// The runtime error is reported and the scan stops, but the server
	// keeps serving the initialised image.
	eventually(t, "scan stopped", func() bool { return strings.Contains(s.stderr.String(), "error: scan stopped") })
	assert.Contains(t, s.stderr.String(), "division by zero")
	c := dialServe(t, "opc.tcp://"+addr)
	assert.Equal(t, int16(5), readNode(t, c, s4("GVL.x")).Value)
}

// TestServeAnalysisErrorFailsStart: analysis errors fail serve before the
// server starts (the Phase 23 project loader; drift such as undeclared
// types is tolerated as warnings).
func TestServeAnalysisErrorFailsStart(t *testing.T) {
	proj := filepath.Join("..", "..", "pkg", "twincat", "testdata", "broken", "Broken.plcproj")
	root := newRootCmd()
	root.SetArgs([]string{"serve", proj, "--pki-dir", t.TempDir(), "--opcua", freeServeAddr(t), "--run-for", "200ms"})
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	err := root.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "undeclared_name")
}

func TestServeRealtimeCycles(t *testing.T) {
	addr := freeServeAddr(t)
	s := startServe(t, false, serveFixture, "--opcua", addr, "--realtime", "--cycle", "1ms", "--run-for", "300ms")
	require.NoError(t, s.wait(t), "--run-for stops serve")
	assert.Regexp(t, `scan: \d+ cycles, \d+ overruns`, s.stderr.String())
}

func TestServeErrors(t *testing.T) {
	// Hold a port on all interfaces, as awcullen binds.
	busy, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	defer busy.Close()
	port := strconv.Itoa(busy.Addr().(*net.TCPAddr).Port)
	empty := t.TempDir()
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no project", nil, "no project given"},
		{"missing project", []string{filepath.Join(empty, "nope.st")}, "no such file"},
		{"missing tsproj", []string{filepath.Join(empty, "x.tsproj")}, "importing"},
		{"two projects", []string{"a.tsproj", "b.plcproj"}, "must be the only"},
		{"empty dir", []string{empty}, "no .st files"},
		{"bad security", []string{serveFixture, "--security", "aes"}, "invalid --security"},
		{"bad cycle", []string{serveFixture, "--cycle", "0s"}, "--cycle must be positive"},
		{"port in use", []string{serveFixture, "--opcua", "127.0.0.1:" + port}, "starting opc ua server"},
		{"cert without key", []string{serveFixture, "--opcua", freeServeAddr(t), "--cert", filepath.Join(empty, "c.der")}, "opc ua server"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := newRootCmd()
			// --run-for turns a wrongly accepted case into a failure, not a hang.
			root.SetArgs(append([]string{"serve", "--pki-dir", t.TempDir(), "--run-for", "200ms"}, c.args...))
			var out, errb bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&errb)
			err := root.Execute()
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)
		})
	}
}

func TestServeHelpAndExec(t *testing.T) {
	out, err := exec.Command(stcBinary, "serve", "--help").CombinedOutput()
	require.NoError(t, err, string(out))
	for _, f := range []string{"--project", "--opcua", "--security", "--cert", "--key", "--cycle", "--run-for", "--realtime"} {
		assert.Contains(t, string(out), f)
	}

	addr := freeServeAddr(t)
	cmd := exec.Command(stcBinary, "serve", serveFixture, "--opcua", addr, "--run-for", "1s",
		"--pki-dir", t.TempDir(), "--format", "json")
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	var info startupInfo
	require.NoError(t, json.Unmarshal([]byte(line), &info))
	assert.Equal(t, "opc.tcp://"+addr, info.Endpoint)
	require.NoError(t, cmd.Wait(), "exit 0 after --run-for")

	bad := exec.Command(stcBinary, "serve", "--security", "x", serveFixture)
	out, err = bad.CombinedOutput()
	require.Error(t, err)
	assert.Contains(t, string(out), "invalid --security")
}

// firstDiff shows the golden and actual lines around the first difference.
func firstDiff(want, got string) string {
	wl, gl := strings.Split(want, "\n"), strings.Split(got, "\n")
	i := 0
	for i < len(wl) && i < len(gl) && wl[i] == gl[i] {
		i++
	}
	lo := max(0, i-8)
	hiW, hiG := min(len(wl), i+8), min(len(gl), i+8)
	return "line " + strconv.Itoa(i+1) + "\n--- golden\n" + strings.Join(wl[lo:hiW], "\n") +
		"\n+++ served\n" + strings.Join(gl[lo:hiG], "\n")
}

// TestServeTwinCATProject serves an imported TwinCAT solution: the scan
// cycle defaults to the first task's cycle time.
func TestServeTwinCATProject(t *testing.T) {
	proj := filepath.Join("..", "..", "pkg", "twincat", "testdata", "sln", "Demo", "Demo solution.tsproj")
	s := startServe(t, true, proj, "--opcua", freeServeAddr(t), "--run-for", "300ms")
	assert.Equal(t, "1ms", s.info.Cycle)
	require.NoError(t, s.wait(t))
}

// TestServeJSONScanStopped reports a runtime error as a JSON event and
// keeps serving until --run-for expires.
func TestServeJSONScanStopped(t *testing.T) {
	div := filepath.Join(t.TempDir(), "div.st")
	require.NoError(t, os.WriteFile(div, []byte("PROGRAM MAIN\nVAR\n\tz : INT;\n\tq : INT;\nEND_VAR\nq := 1 / z;\nEND_PROGRAM\n"), 0o644))
	s := startServe(t, true, div, "--opcua", freeServeAddr(t), "--run-for", "500ms")
	require.NoError(t, s.wait(t))
	var ev map[string]string
	line, _, _ := strings.Cut(s.stderr.String(), "\n")
	require.NoError(t, json.Unmarshal([]byte(line), &ev), s.stderr.String())
	assert.Equal(t, "scan_stopped", ev["event"])
	assert.Contains(t, ev["error"], "division by zero")
}
