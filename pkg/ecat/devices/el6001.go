package devices

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/centroid-is/stc/pkg/ecat"
)

// el6001FIFO is the terminal's receive FIFO size in bytes.
const el6001FIFO = 128

// EL6001 models the EL6001 RS232 serial terminal (and EL6002 by name).
//
// The PLC side follows the Beckhoff handshake (EL600x documentation, "Control
// word"/"Status word"): Ctrl bit 0 TransmitRequest and bit 1 ReceiveAccepted
// are toggles, bit 2 InitRequest is a level; Status bit 0 TransmitAccepted and
// bit 1 ReceiveRequest are toggles, bit 2 InitAccepted, bit 3 BufferFull. In
// the 22-byte image Ctrl/Status are 16-bit words with the lengths in bits 8-15
// and Parity/Framing/Overrun errors in Status bits 4-6; in the small/medium
// image they are bytes with the lengths in bits 4-6 and no error bits. The
// "COM Inputs"/"COM Outputs" PDOs carry the same fields as split Status__* and
// Ctrl__* entries. The number of data bytes is the number of "Data Out n" /
// "Data In n" entries. Send continuous (Ctrl bit 3) is not modelled.
//
// Each Step: InitRequest clears both FIFOs, the toggles and the latched
// overrun, discards what the peer sends meanwhile and sets InitAccepted. Otherwise a change of
// TransmitRequest hands OutputLength bytes (clamped to the data bytes) to the
// peer and toggles TransmitAccepted; the peer is ticked and read into the
// 128-byte RX FIFO (excess bytes are dropped and latch Overrun); and once the
// PLC has toggled ReceiveAccepted for the previous chunk, the next chunk of up
// to the data length is presented with InputLength and a ReceiveRequest
// toggle. Without a peer, transmitted bytes are dropped.
type EL6001 struct {
	Base

	ctrl, status       ecat.Field
	hasCtrl, hasStatus bool
	cbits, sbits       map[string]ecat.Field // entry suffix after "Ctrl__"/"Status__"
	dataOut, dataIn    []ecat.Field

	peer SerialPeer

	rx                         []byte
	chunk                      []byte
	lastTR, lastRA             bool
	ta, rr, ia, awaitAck       bool
	overrun                    bool
	parity, framing, overrunIn bool
}

// Bind resolves the control/status entries and the data bytes.
func (d *EL6001) Bind(l *ecat.Layout) {
	d.Base.Bind(l)
	d.cbits, d.sbits = map[string]ecat.Field{}, map[string]ecat.Field{}
	d.hasCtrl, d.hasStatus = false, false
	d.dataOut = bindSerial(l.Fields(ecat.DirOut), "Ctrl", "Data Out ", &d.ctrl, &d.hasCtrl, d.cbits)
	d.dataIn = bindSerial(l.Fields(ecat.DirIn), "Status", "Data In ", &d.status, &d.hasStatus, d.sbits)
}

// bindSerial sorts one direction's fields into the control word (or its
// split bits) and the data bytes ordered by their index.
func bindSerial(fields []ecat.Field, word, data string, wf *ecat.Field, has *bool, bits map[string]ecat.Field) []ecat.Field {
	type indexed struct {
		n int
		f ecat.Field
	}
	var ds []indexed
	for _, f := range fields {
		switch {
		case f.Entry == word:
			*wf, *has = f, true
		case strings.HasPrefix(f.Entry, word+"__"):
			bits[strings.TrimPrefix(f.Entry, word+"__")] = f
		case strings.HasPrefix(f.Entry, data):
			if n, err := strconv.Atoi(strings.TrimPrefix(f.Entry, data)); err == nil && n >= 0 {
				ds = append(ds, indexed{n, f})
			}
		}
	}
	sort.SliceStable(ds, func(i, j int) bool { return ds[i].n < ds[j].n })
	out := make([]ecat.Field, len(ds))
	for i, x := range ds {
		out[i] = x.f
	}
	return out
}

// DataLen returns the number of data bytes per direction in the active PDOs.
func (d *EL6001) DataLen() int { return len(d.dataIn) }

// SetPeer attaches the device on the far end of the serial line (nil drops
// transmitted bytes and receives nothing).
func (d *EL6001) SetPeer(p SerialPeer) { d.peer = p }

// Peer returns the attached serial peer.
func (d *EL6001) Peer() SerialPeer { return d.peer }

// SetErrors sets the Parity, Framing and Overrun error bits (Status bits
// 4-6) reported from the next Step. A FIFO overrun also sets Overrun.
func (d *EL6001) SetErrors(parity, framing, overrun bool) {
	d.parity, d.framing, d.overrunIn = parity, framing, overrun
}

func (d *EL6001) readCtrl(out []byte) (tr, ra, ir bool, ol int) {
	if d.hasCtrl {
		v := ecat.Get(out, d.ctrl)
		tr, ra, ir = v&1 != 0, v&2 != 0, v&4 != 0
		if d.ctrl.BitLen >= 16 {
			ol = int(v >> 8 & 0xff)
		} else {
			ol = int(v >> 4 & 7)
		}
		return tr, ra, ir, ol
	}
	get := func(k string) uint64 {
		if f, ok := d.cbits[k]; ok {
			return ecat.Get(out, f)
		}
		return 0
	}
	return get("Transmit request") != 0, get("Receive accepted") != 0, get("Init request") != 0, int(get("Output length"))
}

// maxChunk is the most bytes one handshake can carry in this layout.
func (d *EL6001) maxChunk() int {
	n := len(d.dataIn)
	if d.hasStatus && d.status.BitLen < 16 && n > 7 {
		n = 7
	}
	return n
}

// Step runs the init, transmit and receive handshakes and writes the status.
func (d *EL6001) Step(_ time.Duration, out, in []byte) {
	tr, ra, ir, ol := d.readCtrl(out)
	if ir {
		d.rx, d.chunk = nil, nil
		d.ta, d.rr, d.awaitAck, d.overrun = false, false, false, false
		d.ia = true
		d.lastTR, d.lastRA = tr, ra
		if d.peer != nil {
			d.peer.Read() // bytes arriving during init are lost
		}
	} else {
		d.ia = false
		if tr != d.lastTR {
			d.lastTR = tr
			d.transmit(out, ol)
			d.ta = !d.ta
		}
		if t, ok := d.peer.(Ticker); ok {
			t.Tick()
		}
		if d.peer != nil {
			d.receive(d.peer.Read())
		}
		if ra != d.lastRA {
			d.lastRA = ra
			d.awaitAck = false
		}
		if !d.awaitAck && len(d.rx) > 0 {
			n := d.maxChunk()
			if n > len(d.rx) {
				n = len(d.rx)
			}
			d.chunk = append(d.chunk[:0], d.rx[:n]...)
			d.rx = d.rx[n:]
			d.rr = !d.rr
			d.awaitAck = true
		}
	}
	d.writeStatus(in)
	for i, f := range d.dataIn {
		var v uint64
		if i < len(d.chunk) {
			v = uint64(d.chunk[i])
		}
		ecat.Put(in, f, v)
	}
	d.stepIO(out, in)
}

func (d *EL6001) transmit(out []byte, ol int) {
	if ol > len(d.dataOut) {
		ol = len(d.dataOut)
	}
	if ol <= 0 || d.peer == nil {
		return
	}
	buf := make([]byte, ol)
	for i := range buf {
		buf[i] = byte(ecat.Get(out, d.dataOut[i]))
	}
	d.peer.Write(buf)
}

func (d *EL6001) receive(b []byte) {
	if space := el6001FIFO - len(d.rx); len(b) > space {
		b = b[:space]
		d.overrun = true
	}
	d.rx = append(d.rx, b...)
}

func (d *EL6001) writeStatus(in []byte) {
	bf := len(d.rx) >= el6001FIFO
	oe := d.overrun || d.overrunIn
	il := uint64(len(d.chunk))
	if d.hasStatus {
		v := b2u(d.ta) | b2u(d.rr)<<1 | b2u(d.ia)<<2 | b2u(bf)<<3
		if d.status.BitLen >= 16 {
			v |= b2u(d.parity)<<4 | b2u(d.framing)<<5 | b2u(oe)<<6 | il<<8
		} else {
			v |= il & 7 << 4
		}
		ecat.Put(in, d.status, v)
		return
	}
	for k, v := range map[string]uint64{
		"Transmit accepted": b2u(d.ta),
		"Receive request":   b2u(d.rr),
		"Init accepted":     b2u(d.ia),
		"Buffer full":       b2u(bf),
		"Parity error":      b2u(d.parity),
		"Framing error":     b2u(d.framing),
		"Overrun error":     b2u(oe),
		"Input length":      il,
	} {
		if f, ok := d.sbits[k]; ok {
			ecat.Put(in, f, v)
		}
	}
}
