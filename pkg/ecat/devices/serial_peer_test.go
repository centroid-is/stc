package devices

import (
	"strings"
	"testing"
)

func TestScriptedPeerMd(t *testing.T) {
	p := BaaderPeer()
	p.Write([]byte("md\r"))
	if got := string(p.Read()); got != "{ 1 0 0 369 0 }\r\n+" {
		t.Fatalf("md reply %q", got)
	}
	if got := p.Read(); len(got) != 0 {
		t.Fatalf("second Read not drained: %q", got)
	}
	if r := p.Requests(); len(r) != 1 || r[0] != "md" {
		t.Fatalf("requests %q", r)
	}
}

func TestScriptedPeerSplitWrite(t *testing.T) {
	p := BaaderPeer()
	p.Write([]byte("m"))
	if got := p.Read(); len(got) != 0 {
		t.Fatalf("reply before CR: %q", got)
	}
	p.Write([]byte("d\r"))
	if got := string(p.Read()); got != BaaderMdReply {
		t.Fatalf("split md reply %q", got)
	}
}

func TestScriptedPeerMt1LongerThanChunk(t *testing.T) {
	p := BaaderPeer()
	p.Write([]byte("\rmt1\r")) // CLI-style leading CR flush is ignored
	got := string(p.Read())
	if got != BaaderMt1Reply || len(got) != 60 || !strings.HasSuffix(got, "\r\n+") {
		t.Fatalf("mt1 reply %q (%d bytes)", got, len(got))
	}
}

func TestScriptedPeerDelay(t *testing.T) {
	p := &ScriptedPeer{Rules: []ScriptRule{{Request: "md", Reply: "{ 1 }", Delay: 3}, {Request: "x", Reply: "now"}}}
	p.Write([]byte("md\r"))
	for i := 0; i < 2; i++ {
		p.Tick()
		if got := p.Read(); len(got) != 0 {
			t.Fatalf("reply after %d ticks: %q", i+1, got)
		}
	}
	// An undelayed reply queued behind a delayed one keeps its order.
	p.Write([]byte("x\r"))
	if got := p.Read(); len(got) != 0 {
		t.Fatalf("reply overtook delayed one: %q", got)
	}
	p.Tick()
	if got := string(p.Read()); got != "{ 1 }\r\n+now\r\n+" {
		t.Fatalf("delayed reply %q", got)
	}
	p.Tick() // ticking an empty queue is harmless
}

func TestScriptedPeerDefaultAndUnknown(t *testing.T) {
	p := &ScriptedPeer{Rules: []ScriptRule{{Request: "md", Reply: "ok\r\n+"}}}
	p.Write([]byte("zz\r\r"))
	if got := p.Read(); len(got) != 0 {
		t.Fatalf("unknown request without Default answered %q", got)
	}
	p.Write([]byte("md\r"))
	if got := string(p.Read()); got != "ok\r\n+" {
		t.Fatalf("terminator appended twice: %q", got)
	}
	p.Default = "?"
	p.Write([]byte("zz\r"))
	if got := string(p.Read()); got != "?\r\n+" {
		t.Fatalf("default reply %q", got)
	}
	p.Write([]byte("\r"))
	if got := p.Read(); len(got) != 0 {
		t.Fatalf("lone CR answered %q", got)
	}
}

func TestScriptedPeerCustomFraming(t *testing.T) {
	p := &ScriptedPeer{Rules: []ScriptRule{{Request: "a", Reply: "b"}}, Terminator: ";", RequestEnd: '\n'}
	p.Write([]byte(" a \n"))
	if got := string(p.Read()); got != "b;" {
		t.Fatalf("custom framing %q", got)
	}
}

func TestScriptedPeerRequestBufferCap(t *testing.T) {
	p := BaaderPeer()
	p.Write([]byte(strings.Repeat("x", maxPendingRequest+10)))
	if n := len(p.pending); n != maxPendingRequest {
		t.Fatalf("pending %d bytes, want cap %d", n, maxPendingRequest)
	}
	// The oldest bytes were dropped, so the request is just x's: no rule.
	p.Write([]byte("\r"))
	if got := p.Read(); len(got) != 0 {
		t.Fatalf("overlong request answered %q", got)
	}
}

func TestLoopbackPeer(t *testing.T) {
	p := &LoopbackPeer{}
	p.Write([]byte("ab"))
	p.Write([]byte("c"))
	if got := string(p.Read()); got != "abc" {
		t.Fatalf("loopback %q", got)
	}
	if got := p.Read(); len(got) != 0 {
		t.Fatalf("loopback not drained: %q", got)
	}
	var _ SerialPeer = p
	var _ SerialPeer = &ScriptedPeer{}
	var _ Ticker = &ScriptedPeer{}
}
