package opcua

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/awcullen/opcua/server"
	"github.com/awcullen/opcua/ua"
)

// Filler namespaces occupy indices 2 and 3 so the PLC namespace lands at 4,
// the index TF6100 uses for urn:BeckhoffAutomation:Ua:PLC1.
const (
	fillerNamespace2 = "urn:stc:filler:2"
	fillerNamespace3 = "urn:stc:filler:3"
	plcNamespaceIdx  = 4
)

const (
	startTimeout = 10 * time.Second
	stopTimeout  = 5 * time.Second
	pollInterval = 50 * time.Millisecond
)

// Config configures a Server. Start from DefaultConfig: several defaults are
// true, which a zero Config cannot express.
//
// Security note: the server is a development emulator. It trusts any
// client certificate (WithInsecureSkipVerify), both for the secure channel
// and as an X509 user identity. Anonymous clients may write (gated per node
// by AccessLevel) unless AllowAnonymousWrite is false.
type Config struct {
	// Endpoint is "host:port". The listener binds that host only; an empty
	// host, "0.0.0.0" or "::" binds all interfaces. The host also shapes
	// the advertised URL (empty means os.Hostname()). Port 0 is rejected
	// because the advertised URL would keep ":0".
	Endpoint       string
	ApplicationURI string
	PLCNamespace   string // forced to namespace index 4
	AllowNone      bool   // advertise SecurityPolicy None
	// AllowAnonymous accepts the Anonymous identity. It may only be false
	// on a secure-only server (EnableBasic256Sha256, !AllowNone), where
	// clients then authenticate with an X509 certificate identity.
	AllowAnonymous bool
	// AllowAnonymousWrite grants the Anonymous role Write permission. When
	// false, anonymous clients can browse, read and subscribe only.
	AllowAnonymousWrite bool
	// EnableBasic256Sha256 records that a secured endpoint is wanted.
	// awcullen v1.4.0 always advertises its secured policies (Basic256Sha256
	// among them) once a certificate is loaded, so this flag does not toggle
	// them; with AllowNone=false it yields a secure-only server.
	EnableBasic256Sha256 bool
	// CertFile and KeyFile name the server key pair; both or neither. When
	// empty, a self-signed pair is generated (once) in PKIDir.
	CertFile, KeyFile string
	// PKIDir holds generated certificates. Empty means
	// <UserCacheDir>/stc/opcua/pki, or a fresh temp dir if there is none.
	PKIDir           string
	ProductName      string
	SoftwareVersion  string
	MaxWorkerThreads int // >0 overrides awcullen's worker pool size
}

// DefaultConfig returns the TF6100-like defaults: port 4840, SecurityPolicy
// None with Anonymous, and the Beckhoff PLC namespace.
func DefaultConfig() Config {
	return Config{
		Endpoint:       ":4840",
		ApplicationURI: "urn:stc:opcua",
		PLCNamespace:   "urn:BeckhoffAutomation:Ua:PLC1",
		AllowNone:      true,
		AllowAnonymous: true,
		// The tfc-hmi app connects anonymously and writes commands.
		AllowAnonymousWrite: true,
		ProductName:         "stc TF6100 emulator",
		SoftwareVersion:     "dev",
	}
}

// Server is an OPC UA server with the PLC namespace at index 4.
type Server struct {
	cfg    Config
	url    string // advertised endpoint URL
	port   string
	host   string // host used for the readiness probe
	listen string // address the listener binds; ":port" is all interfaces
	srv    *server.Server
	nm     *server.NamespaceManager
	ns     uint16
	reg    typeRegistry // custom enum and struct DataTypes (datatypes.go)

	dsOnce sync.Once // Objects/DeviceSet/PLC1 (deviceset.go)
	dsErr  error

	mu      sync.Mutex
	started bool
	stopped bool
	done    chan struct{} // closed when ListenAndServe returns
	stopOne sync.Once
	stopErr error
}

// New validates cfg, loads or generates the key pair and builds the server
// with the PLC namespace at index 4. It does not listen; call Start.
func New(cfg Config) (*Server, error) {
	if !cfg.AllowNone && !cfg.EnableBasic256Sha256 {
		return nil, errors.New("opcua: no security policy enabled (set AllowNone or EnableBasic256Sha256)")
	}
	if !cfg.AllowAnonymous && (cfg.AllowNone || !cfg.EnableBasic256Sha256) {
		return nil, errors.New("opcua: anonymous can only be disabled on a secure-only (Basic256Sha256) server")
	}
	if cfg.ApplicationURI == "" {
		return nil, errors.New("opcua: ApplicationURI is empty")
	}
	if cfg.PLCNamespace == "" {
		return nil, errors.New("opcua: PLCNamespace is empty")
	}
	host, port, err := splitEndpoint(cfg.Endpoint)
	if err != nil {
		return nil, err
	}
	if (cfg.CertFile == "") != (cfg.KeyFile == "") {
		return nil, errors.New("opcua: CertFile and KeyFile must be set together")
	}
	certPath, keyPath := cfg.CertFile, cfg.KeyFile
	if certPath == "" {
		if cfg.PKIDir == "" {
			if cfg.PKIDir, err = defaultPKIDir(); err != nil {
				return nil, err
			}
		}
		if certPath, keyPath, err = EnsureCert(cfg.PKIDir, cfg.ApplicationURI); err != nil {
			return nil, err
		}
	}

	advHost, probeHost := host, host
	if host == "" {
		advHost, _ = os.Hostname()
		if advHost == "" {
			advHost = "localhost"
		}
		probeHost = "127.0.0.1"
	}
	endpointURL := "opc.tcp://" + net.JoinHostPort(advHost, port)
	listen := net.JoinHostPort(host, port)
	if allInterfaces(host) {
		listen = ":" + port
	}

	// awcullen's default role permissions give Anonymous only Browse|Read,
	// which makes every anonymous write BadUserAccessDenied. Per-node
	// AccessLevel still decides what is writable.
	const readPerms = ua.PermissionTypeBrowse | ua.PermissionTypeRead | ua.PermissionTypeReceiveEvents
	const perms = readPerms | ua.PermissionTypeWrite
	anonPerms := ua.PermissionType(readPerms)
	if cfg.AllowAnonymousWrite {
		anonPerms = perms
	}
	opts := []server.Option{
		server.WithListenAddress(listen),
		server.WithBuildInfo(ua.BuildInfo{
			ProductURI:       "urn:stc",
			ManufacturerName: "stc",
			ProductName:      cfg.ProductName,
			SoftwareVersion:  cfg.SoftwareVersion,
			BuildNumber:      cfg.SoftwareVersion,
		}),
		server.WithAnonymousIdentity(cfg.AllowAnonymous),
		server.WithSecurityPolicyNone(cfg.AllowNone),
		server.WithInsecureSkipVerify(),
		server.WithServerDiagnostics(false),
		server.WithRolePermissions([]ua.RolePermissionType{
			{RoleID: ua.ObjectIDWellKnownRoleAnonymous, Permissions: anonPerms},
			{RoleID: ua.ObjectIDWellKnownRoleAuthenticatedUser, Permissions: perms},
		}),
	}
	if !cfg.AllowAnonymous {
		// Without Anonymous a client needs another identity: accept its
		// certificate (trusted like the channel's, see the security note).
		opts = append(opts, server.WithAuthenticateX509IdentityFunc(
			func(ua.X509Identity, string, string) error { return nil }))
	}
	if cfg.MaxWorkerThreads > 0 {
		opts = append(opts, server.WithMaxWorkerThreads(cfg.MaxWorkerThreads))
	}
	srv, err := server.New(ua.ApplicationDescription{
		ApplicationURI:  cfg.ApplicationURI,
		ProductURI:      "urn:stc",
		ApplicationName: ua.NewLocalizedText(cfg.ProductName, "en"),
		ApplicationType: ua.ApplicationTypeServer,
		DiscoveryURLs:   []string{endpointURL},
	}, certPath, keyPath, endpointURL, opts...)
	if err != nil {
		return nil, fmt.Errorf("opcua: create server: %w", err)
	}

	// ns0 is the OPC UA namespace and ns1 the ApplicationURI (both fixed).
	nm := srv.NamespaceManager()
	nm.Add(fillerNamespace2)
	nm.Add(fillerNamespace3)
	ns := nm.Add(cfg.PLCNamespace)
	if ns != plcNamespaceIdx {
		_ = srv.Close()
		return nil, fmt.Errorf("opcua: PLC namespace %q landed at index %d, want %d", cfg.PLCNamespace, ns, plcNamespaceIdx)
	}
	return &Server{cfg: cfg, url: endpointURL, port: port, host: probeHost, listen: listen, srv: srv, nm: nm, ns: ns}, nil
}

// allInterfaces reports whether an endpoint host means every interface.
func allInterfaces(host string) bool {
	return host == "" || host == "0.0.0.0" || host == "::"
}

// ListenAddr is the address the listener binds: "host:port", or ":port"
// for all interfaces.
func (s *Server) ListenAddr() string { return s.listen }

// AllInterfaces reports whether the listener binds every interface.
func (s *Server) AllInterfaces() bool { return strings.HasPrefix(s.listen, ":") }

// splitEndpoint validates "host:port" with a port in 1..65535.
func splitEndpoint(ep string) (host, port string, err error) {
	host, port, err = net.SplitHostPort(ep)
	if err != nil {
		return "", "", fmt.Errorf("opcua: endpoint %q: %w", ep, err)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", "", fmt.Errorf("opcua: endpoint %q: port must be 1..65535", ep)
	}
	return host, port, nil
}

func defaultPKIDir() (string, error) {
	if dir, err := os.UserCacheDir(); err == nil && dir != "" {
		return filepath.Join(dir, "stc", "opcua", "pki"), nil
	}
	dir, err := os.MkdirTemp("", "stc-opcua-pki-")
	if err != nil {
		return "", fmt.Errorf("opcua: PKI dir: %w", err)
	}
	return dir, nil
}

// Start listens and serves in the background. It returns once the port
// accepts TCP connections, or with the listener error.
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.started || s.stopped {
		return errors.New("opcua: server already started or stopped")
	}
	// ListenAndServe binds s.listen; check that first so a port held by
	// another process fails here instead of fooling the readiness probe.
	ln, err := net.Listen("tcp", s.listen)
	if err != nil {
		return fmt.Errorf("opcua: listen on %s: %w", s.listen, err)
	}
	_ = ln.Close()

	s.started = true
	s.done = make(chan struct{})
	errc := make(chan error, 1)
	go func() {
		defer close(s.done)
		errc <- s.srv.ListenAndServe()
	}()

	addr := net.JoinHostPort(s.host, s.port)
	deadline := time.Now().Add(startTimeout)
	for {
		select {
		case err := <-errc:
			return fmt.Errorf("opcua: listen on port %s: %w", s.port, err)
		default:
		}
		conn, err := net.DialTimeout("tcp", addr, pollInterval)
		if err == nil {
			_ = conn.Close()
			select {
			case err := <-errc:
				return fmt.Errorf("opcua: listen on port %s: %w", s.port, err)
			default:
				return nil
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("opcua: %s not accepting connections after %s", addr, startTimeout)
		}
		time.Sleep(pollInterval)
	}
}

// Stop closes the server and waits for the listener to exit. awcullen gives
// connected clients a 3 s shutdown grace period. Stop is idempotent.
func (s *Server) Stop() error {
	s.stopOne.Do(func() {
		s.mu.Lock()
		s.stopped = true
		done := s.done
		s.mu.Unlock()
		if err := s.srv.Close(); err != nil {
			s.stopErr = fmt.Errorf("opcua: close: %w", err)
			return
		}
		if done == nil {
			return
		}
		select {
		case <-done:
		case <-time.After(stopTimeout):
			s.stopErr = fmt.Errorf("opcua: server did not stop within %s", stopTimeout)
		}
	})
	return s.stopErr
}

// Endpoint returns the advertised endpoint URL, e.g. "opc.tcp://127.0.0.1:4840".
func (s *Server) Endpoint() string { return s.url }

// NamespaceIndex returns the index of the PLC namespace (always 4).
func (s *Server) NamespaceIndex() uint16 { return s.ns }

// uaServer exposes the awcullen server to the address-space builder.
func (s *Server) uaServer() *server.Server { return s.srv }

// namespaceManager exposes the awcullen namespace manager to the builder.
func (s *Server) namespaceManager() *server.NamespaceManager { return s.nm }
