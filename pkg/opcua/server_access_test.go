package opcua

import (
	"net"
	"strings"
	"testing"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
)

// externalIPv4 returns a non-loopback IPv4 address of this host, or "".
func externalIPv4() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && !n.IP.IsLoopback() && n.IP.To4() != nil && !n.IP.IsLinkLocalUnicast() {
			return n.IP.String()
		}
	}
	return ""
}

// TestListenHost is the HI-04 binding regression: a host in the endpoint
// restricts the listener to it, and an empty or wildcard host binds all
// interfaces.
func TestListenHost(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil) // 127.0.0.1:<port>
	_, port, _ := net.SplitHostPort(s.cfg.Endpoint)
	if s.ListenAddr() != "127.0.0.1:"+port || s.AllInterfaces() {
		t.Fatalf("ListenAddr = %q, AllInterfaces = %v", s.ListenAddr(), s.AllInterfaces())
	}
	ext := externalIPv4()
	if ext != "" {
		if c, err := net.DialTimeout("tcp", net.JoinHostPort(ext, port), netTimeout); err == nil {
			_ = c.Close()
			t.Errorf("loopback-only server accepted a connection on %s", ext)
		}
	}

	for _, host := range []string{"", "0.0.0.0"} {
		all := startServer(t, func(c *Config) {
			_, p, _ := net.SplitHostPort(c.Endpoint)
			c.Endpoint = net.JoinHostPort(host, p)
		})
		_, p, _ := net.SplitHostPort(all.cfg.Endpoint)
		if all.ListenAddr() != ":"+p || !all.AllInterfaces() {
			t.Errorf("host %q: ListenAddr = %q", host, all.ListenAddr())
		}
		if ext == "" {
			continue
		}
		c, err := net.DialTimeout("tcp", net.JoinHostPort(ext, p), netTimeout)
		if err != nil {
			t.Errorf("host %q: all-interfaces server refused %s: %v", host, ext, err)
			continue
		}
		_ = c.Close()
	}
}

// TestAnonymousWriteFlag: with AllowAnonymousWrite false, anonymous clients
// still read but every write is denied.
func TestAnonymousWriteFlag(t *testing.T) {
	t.Parallel()
	for _, allow := range []bool{true, false} {
		s := startServer(t, func(c *Config) { c.AllowAnonymousWrite = allow })
		sp, vals := testSpace()
		src := NewMapSource(vals)
		if err := s.Publish(sp, src); err != nil {
			t.Fatal(err)
		}
		c := dialAnon(t, s)
		id := ua.NewNodeIDString(4, "GVL_Test.rw")
		if dv := readValue(t, c, id); dv.StatusCode != ua.Good {
			t.Fatalf("allow=%v: read = %v", allow, dv.StatusCode)
		}
		got := writeAttr(t, c, id, int16(5), "")
		switch {
		case allow && got != ua.Good:
			t.Errorf("anonymous write allowed: %v", got)
		case !allow && got != ua.BadUserAccessDenied:
			t.Errorf("anonymous write denied: got %v, want BadUserAccessDenied", got)
		}
		if n := len(src.Writes()); allow != (n == 1) {
			t.Errorf("allow=%v: %d writes reached the source", allow, n)
		}
	}
}

// TestSecureOnlyWithoutAnonymous: a secure-only server with AllowAnonymous
// false rejects anonymous sessions and accepts a certificate identity with
// write access.
func TestSecureOnlyWithoutAnonymous(t *testing.T) {
	t.Parallel()
	s := startServer(t, func(c *Config) {
		c.AllowNone = false
		c.EnableBasic256Sha256 = true
		c.AllowAnonymous = false
		c.AllowAnonymousWrite = false
	})
	sp, vals := testSpace()
	src := NewMapSource(vals)
	if err := s.Publish(sp, src); err != nil {
		t.Fatal(err)
	}
	cp, kp, err := EnsureCert(t.TempDir(), "urn:stc:test-client")
	if err != nil {
		t.Fatal(err)
	}
	secure := []client.Option{
		client.WithSecurityPolicyURI(ua.SecurityPolicyURIBasic256Sha256, ua.MessageSecurityModeSignAndEncrypt),
		client.WithClientCertificatePaths(cp, kp),
	}
	if _, err := dial(t, s, secure...); err == nil {
		t.Fatal("anonymous session accepted on a server without Anonymous")
	}
	c, err := dial(t, s, append(secure, client.WithX509IdentityPaths(cp, kp))...)
	if err != nil {
		t.Fatalf("certificate identity: %v", err)
	}
	id := ua.NewNodeIDString(4, "GVL_Test.rw")
	if got := writeAttr(t, c, id, int16(5), ""); got != ua.Good {
		t.Errorf("authenticated write = %v", got)
	}
	if !strings.HasPrefix(s.Endpoint(), "opc.tcp://") {
		t.Errorf("Endpoint = %q", s.Endpoint())
	}
}
