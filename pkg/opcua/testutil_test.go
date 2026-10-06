package opcua

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
)

// netTimeout bounds every network wait in tests.
const netTimeout = 15 * time.Second

// freeAddr reserves a free 127.0.0.1 port and releases it. awcullen cannot
// listen on port 0 because its advertised URL would keep ":0".
func freeAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("release port: %v", err)
	}
	return fmt.Sprintf("127.0.0.1:%d", port)
}

// testConfig returns DefaultConfig on a free loopback port with a per-test PKI dir.
func testConfig(t *testing.T) Config {
	t.Helper()
	cfg := DefaultConfig()
	cfg.Endpoint = freeAddr(t)
	cfg.PKIDir = t.TempDir()
	return cfg
}

// startServer builds a server from DefaultConfig (after mutate) and starts it,
// retrying on a fresh port when the reserved one was taken in between.
func startServer(t *testing.T, mutate func(*Config)) *Server {
	t.Helper()
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		cfg := testConfig(t)
		if mutate != nil {
			mutate(&cfg)
		}
		s, err := New(cfg)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		if err := s.Start(); err != nil {
			lastErr = err
			_ = s.Stop()
			continue
		}
		t.Cleanup(func() {
			if err := s.Stop(); err != nil {
				t.Errorf("Stop: %v", err)
			}
		})
		return s
	}
	t.Fatalf("Start failed after retries: %v", lastErr)
	return nil
}

// dial connects to s with extra client options and closes the client on cleanup.
func dial(t *testing.T, s *Server, opts ...client.Option) (*client.Client, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
	defer cancel()
	opts = append([]client.Option{client.WithInsecureSkipVerify()}, opts...)
	c, err := client.Dial(ctx, s.Endpoint(), opts...)
	if err != nil {
		return nil, err
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
		defer cancel()
		if err := c.Close(ctx); err != nil {
			_ = c.Abort(ctx)
		}
	})
	return c, nil
}

// dialAnon connects with SecurityPolicy None and an anonymous identity.
func dialAnon(t *testing.T, s *Server) *client.Client {
	t.Helper()
	c, err := dial(t, s)
	if err != nil {
		t.Fatalf("dial %s: %v", s.Endpoint(), err)
	}
	return c
}

// readValue reads the Value attribute of id.
func readValue(t *testing.T, c *client.Client, id ua.NodeID) ua.DataValue {
	t.Helper()
	return readAttr(t, c, id, ua.AttributeIDValue)
}

// readAttr reads one attribute of id.
func readAttr(t *testing.T, c *client.Client, id ua.NodeID, attr uint32) ua.DataValue {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
	defer cancel()
	res, err := c.Read(ctx, &ua.ReadRequest{
		NodesToRead: []ua.ReadValueID{{NodeID: id, AttributeID: attr}},
	})
	if err != nil {
		t.Fatalf("Read %v: %v", id, err)
	}
	if len(res.Results) != 1 {
		t.Fatalf("Read %v: %d results", id, len(res.Results))
	}
	return res.Results[0]
}

// browseForward returns all forward hierarchical references of id, following
// continuation points until exhausted.
func browseForward(t *testing.T, c *client.Client, id ua.NodeID) []ua.ReferenceDescription {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), netTimeout)
	defer cancel()
	res, err := c.Browse(ctx, &ua.BrowseRequest{
		NodesToBrowse: []ua.BrowseDescription{{
			NodeID:          id,
			BrowseDirection: ua.BrowseDirectionForward,
			ReferenceTypeID: ua.ReferenceTypeIDHierarchicalReferences,
			IncludeSubtypes: true,
			ResultMask:      uint32(ua.BrowseResultMaskAll),
		}},
	})
	if err != nil {
		t.Fatalf("Browse %v: %v", id, err)
	}
	if len(res.Results) != 1 {
		t.Fatalf("Browse %v: %d results", id, len(res.Results))
	}
	r := res.Results[0]
	if r.StatusCode.IsBad() {
		t.Fatalf("Browse %v: %v", id, r.StatusCode)
	}
	refs := append([]ua.ReferenceDescription(nil), r.References...)
	cp := r.ContinuationPoint
	for len(cp) > 0 {
		next, err := c.BrowseNext(ctx, &ua.BrowseNextRequest{ContinuationPoints: []ua.ByteString{cp}})
		if err != nil {
			t.Fatalf("BrowseNext %v: %v", id, err)
		}
		if len(next.Results) != 1 || next.Results[0].StatusCode.IsBad() {
			t.Fatalf("BrowseNext %v: %+v", id, next.Results)
		}
		refs = append(refs, next.Results[0].References...)
		cp = next.Results[0].ContinuationPoint
	}
	return refs
}

// browseNames returns the BrowseName texts of refs.
func browseNames(refs []ua.ReferenceDescription) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = r.BrowseName.Name
	}
	return out
}

// errContains reports whether err is non-nil and mentions sub.
func errContains(err error, sub string) bool {
	return err != nil && strings.Contains(err.Error(), sub)
}

// isTimeout reports whether err is a deadline error.
func isTimeout(err error) bool { return errors.Is(err, context.DeadlineExceeded) }

func itoa(n int) string { return fmt.Sprint(n) }
