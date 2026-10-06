package devices

import "strings"

// SerialPeer is the device on the far side of a serial terminal's line.
// Write receives the bytes the terminal put on the wire; Read returns (and
// drains) the bytes the peer has sent back since the last Read.
type SerialPeer interface {
	Write(tx []byte)
	Read() []byte
}

// Ticker is optionally implemented by a SerialPeer that models time. The
// EL6001 calls Tick once per Step, after Write and before Read.
type Ticker interface {
	Tick()
}

// ScriptRule answers Request with Reply after Delay Ticks.
type ScriptRule struct {
	Request string // matched against the trimmed request text
	Reply   string // Terminator is appended unless Reply already ends with it
	Delay   int    // Ticks before the reply becomes readable
}

// DefaultTerminator ends every ScriptedPeer reply unless overridden.
const DefaultTerminator = "\r\n+"

// maxPendingRequest caps the bytes buffered while waiting for RequestEnd;
// the oldest bytes are dropped beyond it.
const maxPendingRequest = 4096

// ScriptedPeer answers line-oriented requests from a fixed script. Written
// bytes are buffered and split on RequestEnd; each request is trimmed of
// surrounding whitespace and matched exactly against Rules in order. Empty
// requests (a lone RequestEnd) get no reply; unmatched ones get Default, or
// nothing when Default is empty. Replies are released in request order, so a
// delayed reply holds back the ones queued after it. No goroutines or wall
// clock: delay is counted in Tick calls.
type ScriptedPeer struct {
	Rules      []ScriptRule
	Terminator string // "" means DefaultTerminator
	RequestEnd byte   // 0 means '\r'
	Default    string

	pending  []byte
	queue    []queuedReply
	out      []byte
	requests []string
}

type queuedReply struct {
	data  string
	delay int
}

// Write buffers tx and queues a reply for every completed request.
func (p *ScriptedPeer) Write(tx []byte) {
	end := p.RequestEnd
	if end == 0 {
		end = '\r'
	}
	for _, b := range tx {
		if b != end {
			p.pending = append(p.pending, b)
			if len(p.pending) > maxPendingRequest {
				p.pending = append(p.pending[:0], p.pending[len(p.pending)-maxPendingRequest:]...)
			}
			continue
		}
		req := strings.TrimSpace(string(p.pending))
		p.pending = p.pending[:0]
		if req == "" {
			continue
		}
		p.requests = append(p.requests, req)
		if reply, delay, ok := p.match(req); ok {
			p.queue = append(p.queue, queuedReply{p.terminate(reply), delay})
		}
	}
	p.release()
}

func (p *ScriptedPeer) match(req string) (string, int, bool) {
	for _, r := range p.Rules {
		if r.Request == req {
			return r.Reply, r.Delay, true
		}
	}
	if p.Default != "" {
		return p.Default, 0, true
	}
	return "", 0, false
}

func (p *ScriptedPeer) terminate(reply string) string {
	term := p.Terminator
	if term == "" {
		term = DefaultTerminator
	}
	if strings.HasSuffix(reply, term) {
		return reply
	}
	return reply + term
}

// Tick advances every queued reply's delay by one step.
func (p *ScriptedPeer) Tick() {
	for i := range p.queue {
		if p.queue[i].delay > 0 {
			p.queue[i].delay--
		}
	}
	p.release()
}

// release moves due replies at the head of the queue to the output.
func (p *ScriptedPeer) release() {
	n := 0
	for n < len(p.queue) && p.queue[n].delay <= 0 {
		p.out = append(p.out, p.queue[n].data...)
		n++
	}
	p.queue = p.queue[n:]
}

// Read returns and drains the released reply bytes.
func (p *ScriptedPeer) Read() []byte {
	out := p.out
	p.out = nil
	return out
}

// Requests returns every non-empty request received so far, trimmed.
func (p *ScriptedPeer) Requests() []string {
	return append([]string(nil), p.requests...)
}

// Baader reply fixtures. md is the documented realtime counter reply; mt1 is
// synthetic, padded to 60 bytes so it spans three 22-byte PDO chunks.
const (
	BaaderMdReply  = "{ 1 0 0 369 0 }\r\n+"
	BaaderMt1Reply = "{ 10485760 1048575 524288 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 }\r\n+"
	BaaderMaReply  = "Machine (221 on herring) # 1 is working.\r\n+"
)

// BaaderPeer returns a ScriptedPeer answering the Baader md, mt1 and ma polls.
func BaaderPeer() *ScriptedPeer {
	return &ScriptedPeer{Rules: []ScriptRule{
		{Request: "md", Reply: BaaderMdReply},
		{Request: "mt1", Reply: BaaderMt1Reply},
		{Request: "ma", Reply: BaaderMaReply},
	}}
}

// LoopbackPeer echoes every written byte back (a TX-RX jumper).
type LoopbackPeer struct {
	buf []byte
}

// Write queues tx for Read.
func (p *LoopbackPeer) Write(tx []byte) { p.buf = append(p.buf, tx...) }

// Read returns and drains the echoed bytes.
func (p *LoopbackPeer) Read() []byte {
	out := p.buf
	p.buf = nil
	return out
}
