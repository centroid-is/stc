package interp

import (
	"time"

	"github.com/centroid-is/stc/pkg/ecat"
)

// SetIOBinder attaches b to the whole Runtime: every Tick copies its input
// slots into the env (stepping its network) before the first PROGRAM and
// its outputs into the image after the last one. Binding roots resolve
// against every GVL and every PROGRAM. A nil binder detaches.
func (r *Runtime) SetIOBinder(b *IOBinder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if b != nil {
		b.interp = r.interp
		b.progEnv = func(name string) *Env {
			if pr := r.program(name); pr != nil {
				pr.engine.Initialize()
				return pr.engine.env
			}
			return nil
		}
		b.resolved = false
	}
	r.binder = b
}

// IOBinder returns the binder attached with SetIOBinder, or nil.
func (r *Runtime) IOBinder() *IOBinder {
	return r.binder
}

// Network returns the network passed in RuntimeOpts.Network, or nil.
func (r *Runtime) Network() *ecat.Network {
	if r.ecat == nil {
		return nil
	}
	return r.ecat.net
}

// preScan runs the scan-boundary input work once per Tick: the binder
// copies inputs (stepping its network), then the mocks count the scan and
// step the network when no binder on the same network did.
func (r *Runtime) preScan(dt time.Duration) {
	if r.binder != nil {
		r.binder.preScan(dt)
	}
	if r.ecat != nil {
		r.ecat.preScan(dt, r.binder)
	}
}
