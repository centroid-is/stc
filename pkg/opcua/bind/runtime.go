package bind

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/centroid-is/stc/pkg/interp"
	"github.com/centroid-is/stc/pkg/opcua"
	"github.com/centroid-is/stc/pkg/types"
)

// MaxPending bounds the write queue between two scans: the number of
// queued write requests, where a struct write counts once however many
// leaves it has. Requests beyond it fail, so a flooding client cannot grow
// memory without limit.
const MaxPending = 4096

// errQueueFull is returned by Write when MaxPending requests are queued.
var errQueueFull = errors.New("opcua: write queue full")

// pendingBatch is one validated write request waiting for the cycle
// boundary: a single variable, or every leaf of a struct write. It is
// applied as one unit through Runtime.SetMany.
type pendingBatch struct {
	paths []string
	vals  []any // already converted to forms Runtime.Set accepts
}

// RuntimeSource serves an interp.Runtime as an opcua.NodeSource. Reads take
// deep copies under the runtime lock (Snapshot in one critical section);
// writes are validated with CheckSet, queued, and applied by ApplyPending,
// which the scan loop calls between Ticks.
type RuntimeSource struct {
	rt      *interp.Runtime
	mu      sync.Mutex
	pending []pendingBatch
	stopped error // set by Stop: later writes fail
}

var (
	_ opcua.NodeSource  = (*RuntimeSource)(nil)
	_ opcua.BatchWriter = (*RuntimeSource)(nil)
)

// NewRuntimeSource wraps rt.
func NewRuntimeSource(rt *interp.Runtime) *RuntimeSource {
	return &RuntimeSource{rt: rt}
}

// Read returns the value at path in its canonical NodeSource form.
func (s *RuntimeSource) Read(path string) (any, error) {
	m, err := s.Snapshot([]string{path})
	if err != nil {
		return nil, err
	}
	return m[path], nil
}

// Snapshot reads all paths from one scan image.
func (s *RuntimeSource) Snapshot(paths []string) (map[string]any, error) {
	vals, err := s.rt.GetMany(paths)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", opcua.ErrUnknownSymbol, err)
	}
	out := make(map[string]any, len(paths))
	for i, p := range paths {
		v, err := canonical(vals[i])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		out[p] = v
	}
	return out, nil
}

// Write validates v against the variable at path and queues it for the
// next cycle boundary.
func (s *RuntimeSource) Write(path string, v any) error {
	return s.WriteBatch([]opcua.PathWrite{{Path: path, Value: v}})
}

// WriteBatch validates every write against its variable and, only if all
// pass, queues them as one request. ApplyPending applies the request
// within one Runtime critical section before a Tick, so a scan never sees
// part of it, and a rejected request queues nothing.
func (s *RuntimeSource) WriteBatch(ws []opcua.PathWrite) error {
	if err := s.stoppedErr(); err != nil {
		return err
	}
	if len(ws) == 0 {
		return nil
	}
	b := pendingBatch{paths: make([]string, len(ws)), vals: make([]any, len(ws))}
	for i, w := range ws {
		b.paths[i] = w.Path
	}
	cur, err := s.rt.GetMany(b.paths)
	if err != nil {
		return fmt.Errorf("%w: %v", opcua.ErrUnknownSymbol, err)
	}
	for i, w := range ws {
		b.vals[i] = toSet(cur[i], w.Value)
		if err := s.rt.CheckSet(w.Path, b.vals[i]); err != nil {
			return classify(err)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped != nil {
		return s.notWritable()
	}
	if len(s.pending) >= MaxPending {
		return errQueueFull
	}
	s.pending = append(s.pending, b)
	return nil
}

// Stop marks the scan that drains the queue as stopped, for the reason
// given: queued writes are dropped and every later write fails with
// ErrNotWritable (BadNotWritable), so clients are not told Good for a
// write that will never be applied. Reads keep serving the last image.
func (s *RuntimeSource) Stop(reason error) {
	if reason == nil {
		reason = errors.New("stopped")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = reason
	s.pending = nil
}

// stoppedErr is the error a write gets after Stop, else nil.
func (s *RuntimeSource) stoppedErr() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped == nil {
		return nil
	}
	return s.notWritable()
}

// notWritable wraps the stop reason; the caller holds s.mu.
func (s *RuntimeSource) notWritable() error {
	return fmt.Errorf("%w: the scan has stopped: %v", opcua.ErrNotWritable, s.stopped)
}

// Pending returns the number of queued write requests.
func (s *RuntimeSource) Pending() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.pending)
}

// ApplyPending drains the queue in submission order, each request through
// Runtime.SetMany. Call it between Ticks. A request that became invalid
// since it was queued is skipped whole and its error returned (joined)
// without stopping the others.
func (s *RuntimeSource) ApplyPending() error {
	s.mu.Lock()
	q := s.pending
	s.pending = nil
	s.mu.Unlock()
	var errs []error
	for _, b := range q {
		if err := s.rt.SetMany(b.paths, b.vals); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// classify maps a Runtime.Set/CheckSet error onto the opcua sentinels.
func classify(err error) error {
	msg := err.Error()
	sentinel := opcua.ErrTypeMismatch
	switch {
	case strings.Contains(msg, "is a constant"), strings.Contains(msg, "not writable"),
		strings.Contains(msg, "read-only"):
		sentinel = opcua.ErrNotWritable
	case strings.Contains(msg, "out of range"), strings.Contains(msg, "elements to an array of"):
		sentinel = opcua.ErrOutOfRange
	}
	return fmt.Errorf("%w: %v", sentinel, err)
}

// epoch is the zero of DATE and DT values; todBase that of TOD values.
var (
	epoch   = time.Unix(0, 0).UTC()
	todBase = time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC)
)

// unsigned reports whether k is an unsigned integer or bit-string kind.
func unsigned(k types.TypeKind) bool {
	switch k {
	case types.KindUSINT, types.KindUINT, types.KindUDINT, types.KindULINT,
		types.KindBYTE, types.KindWORD, types.KindDWORD, types.KindLWORD:
		return true
	}
	return false
}

// canonical converts an interpreter value to the NodeSource form: BOOL
// bool, signed integers and enum ordinals int64, unsigned integers uint64,
// REAL/LREAL float64, STRING string, TIME/TOD time.Duration, DATE/DT
// time.Time, arrays []any and structs map[string]any.
func canonical(v interp.Value) (any, error) {
	switch v.Kind {
	case interp.ValBool:
		return v.Bool, nil
	case interp.ValInt:
		if v.Enum == "" && unsigned(v.IECType) {
			return uint64(v.Int), nil
		}
		return v.Int, nil
	case interp.ValReal:
		return v.Real, nil
	case interp.ValString:
		return v.Str, nil
	case interp.ValTime, interp.ValTod:
		return v.Time, nil
	case interp.ValDate, interp.ValDateTime:
		return epoch.Add(v.Time), nil
	case interp.ValArray:
		low := min(v.ArrayLow, len(v.Array))
		out := make([]any, 0, len(v.Array)-low)
		for _, e := range v.Array[low:] {
			c, err := canonical(e)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, nil
	case interp.ValStruct:
		out := make(map[string]any, len(v.Struct))
		for k, f := range v.Struct {
			c, err := canonical(f)
			if err != nil {
				return nil, err
			}
			out[k] = c
		}
		return out, nil
	}
	return nil, fmt.Errorf("%w: %s values are not served", opcua.ErrTypeMismatch, v.Kind)
}

// toSet converts a canonical NodeSource value into the Go form Runtime.Set
// coerces for the witness w (the variable's current value). Values that
// need no conversion pass through; CheckSet rejects the rest.
func toSet(w interp.Value, v any) any {
	switch w.Kind {
	case interp.ValString:
		if s, ok := v.(string); ok {
			// Quote so Set never unquotes text that happens to look quoted.
			return "'" + strings.NewReplacer("$", "$$", "'", "$'").Replace(s) + "'"
		}
	case interp.ValTime:
		if d, ok := v.(time.Duration); ok {
			return float64(d) / float64(time.Millisecond)
		}
	case interp.ValTod:
		if d, ok := v.(time.Duration); ok {
			return "TOD#" + todBase.Add(d).Format("15:04:05.999999999")
		}
	case interp.ValDate:
		if t, ok := v.(time.Time); ok {
			return "D#" + t.UTC().Format("2006-01-02")
		}
	case interp.ValDateTime:
		if t, ok := v.(time.Time); ok {
			return "DT#" + t.UTC().Format("2006-01-02-15:04:05.999999999")
		}
	case interp.ValArray:
		rv := reflect.ValueOf(v)
		if rv.Kind() != reflect.Slice {
			return v
		}
		low := min(w.ArrayLow, len(w.Array))
		out := make([]any, rv.Len())
		for i := range out {
			e := rv.Index(i).Interface()
			if low+i < len(w.Array) {
				e = toSet(w.Array[low+i], e)
			}
			out[i] = e
		}
		return out
	}
	return v
}
