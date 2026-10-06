package interp

import (
	"sort"
	"strings"

	"github.com/centroid-is/stc/pkg/ecat"
)

// ready resolves the bindings once an engine or Runtime is attached.
// It reports false when the binder is detached (nothing to resolve against).
func (b *IOBinder) ready() bool {
	if b.interp == nil || b.progEnv == nil {
		return false
	}
	if !b.resolved {
		b.resolve()
	}
	return true
}

// candidates is the resolved slot list of dir, or every binding of dir
// while the binder is detached.
func (b *IOBinder) candidates(dir ecat.Dir) []ecat.Binding {
	if !b.ready() {
		var out []ecat.Binding
		for _, bd := range b.bindings {
			if bd.Slot.Dir == dir {
				out = append(out, bd)
			}
		}
		return out
	}
	src := b.ins
	if dir == ecat.DirOut {
		src = b.outs
	}
	out := make([]ecat.Binding, len(src))
	for i := range src {
		out[i] = src[i].b
	}
	return out
}

// InputBinding returns the input-direction binding whose variable path
// (e.g. "ECT.A1_01.I1") equals path, case-insensitively. Output leaves,
// unknown paths and bindings dropped at resolution return false.
func (b *IOBinder) InputBinding(path string) (ecat.Binding, bool) {
	want := strings.TrimSpace(path)
	for _, bd := range b.candidates(ecat.DirIn) {
		if strings.EqualFold(bd.Var.Path, want) {
			return bd, true
		}
	}
	return ecat.Binding{}, false
}

// OutputBindings returns the output-direction bindings sorted by variable
// path (case-insensitive, then declared case).
func (b *IOBinder) OutputBindings() []ecat.Binding {
	out := b.candidates(ecat.DirOut)
	sort.SliceStable(out, func(i, j int) bool {
		a, c := strings.ToUpper(out[i].Var.Path), strings.ToUpper(out[j].Var.Path)
		if a != c {
			return a < c
		}
		return out[i].Var.Path < out[j].Var.Path
	})
	return out
}
