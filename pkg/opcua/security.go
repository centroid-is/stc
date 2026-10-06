package opcua

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// ApplySecurity sets c's endpoint security from a mode name, as the
// --security flag of stc serve and stc-mcp takes it: "none" (SecurityPolicy
// None with Anonymous) or "basic256sha256" (secure only). With
// basic256sha256 Anonymous is turned off unless anonymousSet records that
// the caller chose AllowAnonymous explicitly. "none" needs AllowAnonymous,
// because SecurityPolicy None only supports anonymous clients.
func (c *Config) ApplySecurity(mode string, anonymousSet bool) error {
	switch strings.ToLower(mode) {
	case "none":
		if !c.AllowAnonymous {
			return errors.New("--allow-anonymous=false needs --security basic256sha256: SecurityPolicy None only supports anonymous clients")
		}
	case "basic256sha256":
		c.AllowNone = false
		c.EnableBasic256Sha256 = true
		if !anonymousSet {
			c.AllowAnonymous = false
		}
	default:
		return fmt.Errorf("invalid --security %q: want none or basic256sha256", mode)
	}
	return nil
}

// Exposed reports whether a listen address is reachable from other hosts:
// all interfaces, or a host that is not loopback.
func Exposed(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil || host == "" {
		return true
	}
	if host == "localhost" {
		return false
	}
	ip := net.ParseIP(host)
	return ip == nil || !ip.IsLoopback()
}

// AnonymousWriteWarning returns the warning for a server whose anonymous
// clients may write while it listens on an exposed address, or nil.
func AnonymousWriteWarning(c Config, listen string) error {
	if !c.AllowAnonymous || !c.AllowAnonymousWrite || !Exposed(listen) {
		return nil
	}
	return fmt.Errorf("anonymous OPC UA clients can write on %s; bind a loopback host or pass --allow-anonymous-write=false", listen)
}
