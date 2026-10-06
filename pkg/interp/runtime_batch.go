package interp

import (
	"errors"
	"fmt"
)

// CheckSet reports the error Set(path, v) would return without writing
// anything, so callers that queue writes (the OPC UA server applies them
// between scans) can reject bad values when they are submitted.
func (r *Runtime) CheckSet(path string, v any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.checkSet(path, v)
}

// SetMany writes vals[i] to paths[i] as one unit: every write is validated
// first (as CheckSet), and only if all pass are they applied, in order,
// within one critical section. No Tick or reader can observe a partial
// batch, and a rejected batch changes nothing. The error is that of the
// first rejected write.
func (r *Runtime) SetMany(paths []string, vals []any) error {
	if len(paths) != len(vals) {
		return fmt.Errorf("SetMany: %d paths for %d values", len(paths), len(vals))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, p := range paths {
		if err := r.checkSet(p, vals[i]); err != nil {
			return err
		}
	}
	var errs []error
	for i, p := range paths {
		if err := r.set(p, vals[i]); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// checkSet is CheckSet without taking r.mu; the caller holds it.
func (r *Runtime) checkSet(path string, v any) error {
	loc, err := r.resolve(path)
	if err != nil {
		return err
	}
	if r.consts[loc.head] {
		return fmt.Errorf("%s is a constant", path)
	}
	if err := deref(loc, path); err != nil {
		return err
	}
	switch {
	case loc.val.Kind == ValPointer:
		return fmt.Errorf("%s: pointer not writable by path", path)
	case loc.std != nil && loc.stdOut:
		return fmt.Errorf("%s is a read-only output of %s", path, loc.std.TypeName)
	case loc.isBit:
		if _, err := coerceBool(v); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		return nil
	}
	if _, err := r.coerceWith(loc.val, v, false); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// GetMany reads every path in one critical section, so the values come from
// a single scan image. Values are deep copies (Clone), safe to use after the
// lock is released while the scan keeps running. The first failing path
// aborts with its error.
func (r *Runtime) GetMany(paths []string) ([]Value, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Value, len(paths))
	for i, p := range paths {
		loc, err := r.resolve(p)
		if err != nil {
			return nil, err
		}
		if err := deref(loc, p); err != nil {
			return nil, err
		}
		out[i] = loc.val.Clone()
	}
	return out, nil
}
