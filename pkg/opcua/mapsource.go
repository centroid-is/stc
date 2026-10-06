package opcua

import (
	"fmt"
	"sync"
)

// WriteRecord is one successful MapSource.Write, in call order.
type WriteRecord struct {
	Path  string
	Value any
}

// MapSource is a goroutine-safe in-memory NodeSource. It backs the unit
// tests of this package and of the adapters built on it.
type MapSource struct {
	mu        sync.RWMutex
	vals      map[string]any
	failWrite map[string]error
	writes    []WriteRecord
}

var _ NodeSource = (*MapSource)(nil)

// NewMapSource returns a MapSource holding a copy of vals.
func NewMapSource(vals map[string]any) *MapSource {
	m := &MapSource{vals: make(map[string]any, len(vals)), failWrite: map[string]error{}}
	for k, v := range vals {
		m.vals[k] = v
	}
	return m
}

// Read returns the value at path or a wrapped ErrUnknownSymbol.
func (m *MapSource) Read(path string) (any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.vals[path]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownSymbol, path)
	}
	return v, nil
}

// Write stores v at an existing path and records the write. An error
// injected with FailWrite takes precedence.
func (m *MapSource) Write(path string, v any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.failWrite[path]; err != nil {
		return err
	}
	if _, ok := m.vals[path]; !ok {
		return fmt.Errorf("%w: %s", ErrUnknownSymbol, path)
	}
	m.vals[path] = v
	m.writes = append(m.writes, WriteRecord{Path: path, Value: v})
	return nil
}

// Snapshot reads all paths under one lock; it fails on the first unknown path.
func (m *MapSource) Snapshot(paths []string) (map[string]any, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]any, len(paths))
	for _, p := range paths {
		v, ok := m.vals[p]
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrUnknownSymbol, p)
		}
		out[p] = v
	}
	return out, nil
}

// FailWrite makes every later Write to path return err; nil clears it.
func (m *MapSource) FailWrite(path string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err == nil {
		delete(m.failWrite, path)
		return
	}
	m.failWrite[path] = err
}

// Set stores v at path (creating it) without recording a write.
func (m *MapSource) Set(path string, v any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.vals[path] = v
}

// Get returns the current value at path.
func (m *MapSource) Get(path string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	v, ok := m.vals[path]
	return v, ok
}

// Writes returns a copy of the write log in call order.
func (m *MapSource) Writes() []WriteRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]WriteRecord(nil), m.writes...)
}
