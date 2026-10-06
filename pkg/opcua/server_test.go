package opcua

import (
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
)

func loadCert(t *testing.T, path string) *x509.Certificate {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	blk, _ := pem.Decode(b)
	if blk == nil {
		t.Fatalf("%s: no PEM block", path)
	}
	c, err := x509.ParseCertificate(blk.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestEnsureCert(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "pki")
	certPath, keyPath, err := EnsureCert(dir, "urn:stc:opcua")
	if err != nil {
		t.Fatal(err)
	}
	if certPath != filepath.Join(dir, "server.crt") || keyPath != filepath.Join(dir, "server.key") {
		t.Fatalf("paths = %s, %s", certPath, keyPath)
	}
	if runtime.GOOS != "windows" {
		for p, want := range map[string]os.FileMode{keyPath: 0o600, dir: 0o700} {
			fi, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if got := fi.Mode().Perm(); got != want {
				t.Errorf("%s mode = %o, want %o", p, got, want)
			}
		}
	}
	c := loadCert(t, certPath)
	if len(c.URIs) != 1 || c.URIs[0].String() != "urn:stc:opcua" {
		t.Errorf("URI SANs = %v", c.URIs)
	}
	hasDNS := false
	for _, d := range c.DNSNames {
		hasDNS = hasDNS || d == "localhost"
	}
	if !hasDNS {
		t.Errorf("DNS SANs = %v, want localhost", c.DNSNames)
	}
	hasIP := false
	for _, ip := range c.IPAddresses {
		hasIP = hasIP || ip.Equal(net.IPv4(127, 0, 0, 1))
	}
	if !hasIP {
		t.Errorf("IP SANs = %v, want 127.0.0.1", c.IPAddresses)
	}
	if c.NotAfter.Before(time.Now().AddDate(9, 0, 0)) {
		t.Errorf("NotAfter = %v, want ~10 years", c.NotAfter)
	}
	if c.PublicKey == nil || c.PublicKeyAlgorithm != x509.RSA {
		t.Errorf("key algorithm = %v", c.PublicKeyAlgorithm)
	}

	before, _ := os.ReadFile(certPath)
	c2, k2, err := EnsureCert(dir, "urn:stc:opcua")
	if err != nil || c2 != certPath || k2 != keyPath {
		t.Fatalf("second EnsureCert = %s, %s, %v", c2, k2, err)
	}
	after, _ := os.ReadFile(certPath)
	if string(before) != string(after) {
		t.Fatal("second EnsureCert regenerated the certificate")
	}
}

func TestEnsureCertErrors(t *testing.T) {
	if _, _, err := EnsureCert(t.TempDir(), "://bad uri"); err == nil {
		t.Error("bad appURI accepted")
	}
	// A regular file where the directory should be.
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := EnsureCert(filepath.Join(f, "pki"), "urn:x"); err == nil {
		t.Error("unwritable dir accepted")
	}
	// Cert present but key missing: regenerate both.
	dir := t.TempDir()
	cp, kp, err := EnsureCert(dir, "urn:x")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(kp); err != nil {
		t.Fatal(err)
	}
	if _, _, err := EnsureCert(dir, "urn:x"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(kp); err != nil {
		t.Fatalf("key not regenerated: %v", err)
	}
	_ = cp
}

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.Endpoint != ":4840" || c.ApplicationURI != "urn:stc:opcua" ||
		c.PLCNamespace != "urn:BeckhoffAutomation:Ua:PLC1" || !c.AllowNone || !c.AllowAnonymous ||
		c.EnableBasic256Sha256 || c.ProductName != "stc TF6100 emulator" {
		t.Fatalf("DefaultConfig = %+v", c)
	}
}

func TestNewNamespaceIndex(t *testing.T) {
	cfg := testConfig(t)
	cfg.MaxWorkerThreads = 2
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	if s.NamespaceIndex() != 4 {
		t.Fatalf("NamespaceIndex = %d, want 4", s.NamespaceIndex())
	}
	if want := "opc.tcp://" + cfg.Endpoint; s.Endpoint() != want {
		t.Fatalf("Endpoint = %q, want %q", s.Endpoint(), want)
	}
	if s.uaServer() == nil || s.namespaceManager() == nil {
		t.Fatal("nil internal accessors")
	}
	if _, err := os.Stat(filepath.Join(cfg.PKIDir, "server.crt")); err != nil {
		t.Fatalf("cert not generated: %v", err)
	}
}

func TestNewEmptyHostUsesHostname(t *testing.T) {
	cfg := testConfig(t)
	_, port, _ := net.SplitHostPort(cfg.Endpoint)
	cfg.Endpoint = ":" + port
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	host, _ := os.Hostname()
	if host == "" {
		host = "localhost"
	}
	if want := "opc.tcp://" + net.JoinHostPort(host, port); s.Endpoint() != want {
		t.Fatalf("Endpoint = %q, want %q", s.Endpoint(), want)
	}
}

func TestNewExplicitCert(t *testing.T) {
	dir := t.TempDir()
	cp, kp, err := EnsureCert(dir, "urn:stc:opcua")
	if err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t)
	cfg.PKIDir = filepath.Join(t.TempDir(), "unused")
	cfg.CertFile, cfg.KeyFile = cp, kp
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	if _, err := os.Stat(cfg.PKIDir); !os.IsNotExist(err) {
		t.Fatalf("PKIDir created although cert was given: %v", err)
	}
}

func TestNewDefaultPKIDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir()) // Linux
	t.Setenv("HOME", t.TempDir())           // macOS UserCacheDir
	t.Setenv("LocalAppData", t.TempDir())   // Windows
	cfg := testConfig(t)
	cfg.PKIDir = ""
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Stop() })
	if s.cfg.PKIDir == "" {
		t.Fatal("PKIDir not resolved")
	}
	if _, err := os.Stat(filepath.Join(s.cfg.PKIDir, "server.crt")); err != nil {
		t.Fatalf("cert not generated in default PKI dir: %v", err)
	}
}

func TestNewRejects(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"no policy", func(c *Config) { c.AllowNone = false; c.EnableBasic256Sha256 = false }, "no security policy enabled"},
		{"no anonymous with None", func(c *Config) { c.AllowAnonymous = false }, "anonymous can only be disabled"},
		{"no anonymous, None and Basic256Sha256", func(c *Config) { c.AllowAnonymous = false; c.EnableBasic256Sha256 = true }, "anonymous can only be disabled"},
		{"no port", func(c *Config) { c.Endpoint = "127.0.0.1" }, "endpoint"},
		{"port zero", func(c *Config) { c.Endpoint = "127.0.0.1:0" }, "endpoint"},
		{"bad port", func(c *Config) { c.Endpoint = "127.0.0.1:http" }, "endpoint"},
		{"port too big", func(c *Config) { c.Endpoint = "127.0.0.1:70000" }, "endpoint"},
		{"cert without key", func(c *Config) { c.CertFile = "x.crt" }, "CertFile and KeyFile"},
		{"key without cert", func(c *Config) { c.KeyFile = "x.key" }, "CertFile and KeyFile"},
		{"missing cert files", func(c *Config) { c.CertFile, c.KeyFile = "/nonexistent/x.crt", "/nonexistent/x.key" }, "opcua"},
		{"empty app uri", func(c *Config) { c.ApplicationURI = "" }, "ApplicationURI"},
		{"empty namespace", func(c *Config) { c.PLCNamespace = "" }, "PLCNamespace"},
		{"filler namespace", func(c *Config) { c.PLCNamespace = "urn:stc:filler:2" }, "index"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig(t)
			tc.mutate(&cfg)
			s, err := New(cfg)
			if err == nil {
				_ = s.Stop()
				t.Fatal("New succeeded")
			}
			if !errContains(err, tc.want) {
				t.Fatalf("err = %v, want mention of %q", err, tc.want)
			}
		})
	}
}

func TestStartStop(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil)
	host, port, _ := net.SplitHostPort(s.cfg.Endpoint)
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), netTimeout)
	if err != nil {
		t.Fatalf("port not accepting after Start: %v", err)
	}
	_ = conn.Close()
	if err := s.Start(); err == nil {
		t.Fatal("second Start succeeded")
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatalf("second Stop: %v", err)
	}
	// The port is free again: a new server binds it.
	cfg := testConfig(t)
	cfg.Endpoint = s.cfg.Endpoint
	s2, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := s2.Start(); err != nil {
		t.Fatalf("rebind %s: %v", cfg.Endpoint, err)
	}
	if err := s2.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestStartPortInUse(t *testing.T) {
	t.Parallel()
	// Hold the exact address the server binds.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	cfg := testConfig(t)
	cfg.Endpoint = net.JoinHostPort("127.0.0.1", itoa(ln.Addr().(*net.TCPAddr).Port))
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Stop() }()
	if err := s.Start(); err == nil {
		t.Fatal("Start on a busy port succeeded")
	}
}

func TestStopWithoutStart(t *testing.T) {
	s, err := New(testConfig(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := s.Start(); err == nil {
		t.Fatal("Start after Stop succeeded")
	}
}

const beckhoffNS = "urn:BeckhoffAutomation:Ua:PLC1"

func TestConnectNoneAnonymous(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil)
	c := dialAnon(t, s)
	dv := readValue(t, c, ua.VariableIDServerNamespaceArray)
	if !dv.StatusCode.IsGood() {
		t.Fatalf("NamespaceArray status %v", dv.StatusCode)
	}
	arr, ok := dv.Value.([]string)
	if !ok {
		t.Fatalf("NamespaceArray is %T", dv.Value)
	}
	if len(arr) < 5 || arr[4] != beckhoffNS || arr[1] != "urn:stc:opcua" ||
		arr[2] != fillerNamespace2 || arr[3] != fillerNamespace3 {
		t.Fatalf("NamespaceArray = %q", arr)
	}
}

func TestServerStatusRunning(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil)
	c := dialAnon(t, s)
	dv := readValue(t, c, ua.VariableIDServerServerStatusState)
	if dv.StatusCode != ua.Good {
		t.Fatalf("i=2259 status %v", dv.StatusCode)
	}
	if v, ok := dv.Value.(int32); !ok || v != 0 {
		t.Fatalf("i=2259 = %v (%T), want int32 0 (Running)", dv.Value, dv.Value)
	}
	dv = readValue(t, c, ua.VariableIDServerServerStatusBuildInfoProductName)
	if dv.Value != "stc TF6100 emulator" {
		t.Fatalf("ProductName = %v", dv.Value)
	}
}

func TestConnectBasic256Sha256(t *testing.T) {
	t.Parallel()
	s := startServer(t, func(c *Config) { c.AllowNone = false; c.EnableBasic256Sha256 = true })
	cp, kp, err := EnsureCert(t.TempDir(), "urn:stc:test-client")
	if err != nil {
		t.Fatal(err)
	}
	c, err := dial(t, s,
		client.WithSecurityPolicyURI(ua.SecurityPolicyURIBasic256Sha256, ua.MessageSecurityModeSignAndEncrypt),
		client.WithClientCertificatePaths(cp, kp))
	if err != nil {
		t.Fatalf("secure dial: %v", err)
	}
	dv := readValue(t, c, ua.VariableIDServerServerStatusState)
	if v, ok := dv.Value.(int32); !ok || v != 0 || dv.StatusCode != ua.Good {
		t.Fatalf("i=2259 over Basic256Sha256 = %v (%v)", dv.Value, dv.StatusCode)
	}
}

func TestSecureOnlyRejectsNone(t *testing.T) {
	t.Parallel()
	s := startServer(t, func(c *Config) { c.AllowNone = false; c.EnableBasic256Sha256 = true })
	if _, err := dial(t, s); err == nil {
		t.Fatal("SecurityPolicy None dial succeeded against a secure-only server")
	}
}

func TestBrowseObjects(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil)
	c := dialAnon(t, s)
	refs := browseForward(t, c, ua.ObjectIDObjectsFolder)
	found := false
	for _, r := range refs {
		found = found || ua.ToNodeID(r.NodeID, nil) == ua.ObjectIDServer
	}
	if !found {
		t.Fatalf("Objects children %v lack the Server object", browseNames(refs))
	}
}
