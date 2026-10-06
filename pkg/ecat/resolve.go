package ecat

import (
	"fmt"
	"sort"
	"strings"

	"github.com/centroid-is/stc/pkg/diag"
)

// maxSuggestions caps the child segments listed in an ECAT001 message.
const maxSuggestions = 3

// Binding pairs a linked variable with its process image slot.
type Binding struct {
	Var  LinkedVar
	Slot Slot
}

func bindingLess(a, b Binding) bool {
	if a.Var.Path != b.Var.Path {
		return a.Var.Path < b.Var.Path
	}
	return a.Var.Link < b.Var.Link
}

type slotKey struct {
	master string
	dir    Dir
	byte_  int
	bit    int
}

// Resolve maps every linked variable to its topology slot. Unresolved
// targets (ECAT001), width mismatches (ECAT003) and direction mismatches
// (ECAT004) are errors and the variable is not bound; a slot bound twice
// warns (ECAT005) and keeps both bindings. Bindings are sorted by path then
// link and diagnostics by position.
func Resolve(topo *Topology, vars []LinkedVar) ([]Binding, []diag.Diagnostic) {
	var bindings []Binding
	var diags []diag.Diagnostic
	add := func(sev diag.Severity, v LinkedVar, code, format string, args ...any) {
		diags = append(diags, diag.Diagnostic{Severity: sev, Pos: v.Pos, EndPos: v.EndPos, Code: code, Message: fmt.Sprintf(format, args...)})
	}
	var tree [][]string // split topology paths, built on first miss
	seen := map[slotKey]string{}
	for _, v := range vars {
		slot, ok := topo.Slot(v.Link)
		if !ok {
			if tree == nil {
				for _, p := range topo.Paths() {
					tree = append(tree, strings.Split(p, "^"))
				}
			}
			add(diag.Error, v, CodeUnresolved, "link target of %s not found: %q; %s", v.Path, v.Link, nearest(tree, v.Link))
			continue
		}
		if !widthOK(v, slot) {
			add(diag.Error, v, CodeSizeMismatch, "size mismatch: %s is %s (%d-bit) but %q is %d-bit", v.Path, v.TypeName, v.BitWidth, v.Link, slot.BitLen)
			continue
		}
		if v.HasAT && v.Dir != slot.Dir {
			add(diag.Error, v, CodeDirMismatch, "direction mismatch: %s is declared %s but %q is an %s entry", v.Path, atName(v.Dir), v.Link, dirWord(slot.Dir))
			continue
		}
		key := slotKey{slot.Master, slot.Dir, slot.Byte, slot.Bit}
		if first, dup := seen[key]; dup {
			add(diag.Warning, v, CodeDuplicate, "%s binds the same slot as %s (%q)", v.Path, first, v.Link)
		} else {
			seen[key] = v.Path
		}
		bindings = append(bindings, Binding{Var: v, Slot: slot})
	}
	sort.SliceStable(bindings, func(i, j int) bool { return bindingLess(bindings[i], bindings[j]) })
	SortDiagnostics(diags)
	return bindings, diags
}

// SortDiagnostics orders diagnostics by file, line and column, keeping the
// input order for equal positions.
func SortDiagnostics(diags []diag.Diagnostic) {
	sort.SliceStable(diags, func(i, j int) bool {
		a, b := diags[i].Pos, diags[j].Pos
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Col < b.Col
	})
}

// widthOK reports whether v's declared width fits the slot. Unknown widths
// (library types that are not loaded) are not checked; a STRING may link to
// a 48-bit AmsNetId entry via TwinCAT's T_AmsNetId conversion.
func widthOK(v LinkedVar, slot Slot) bool {
	if strings.EqualFold(v.TypeName, "STRING") {
		return slot.BitLen == 48
	}
	return v.BitWidth == 0 || v.BitWidth == slot.BitLen
}

func atName(d Dir) string {
	if d == DirOut {
		return "AT %Q*"
	}
	return "AT %I*"
}

func dirWord(d Dir) string {
	if d == DirOut {
		return "output"
	}
	return "input"
}

// nearest describes the longest existing '^'-prefix of target and up to
// maxSuggestions child segments under it.
func nearest(tree [][]string, target string) string {
	segs := strings.Split(target, "^")
	best := 0
	for _, p := range tree {
		n := 0
		for n < len(p) && n < len(segs) && p[n] == segs[n] {
			n++
		}
		if n > best {
			best = n
		}
	}
	children := map[string]bool{}
	for _, p := range tree {
		if len(p) > best && strings.Join(p[:best], "^") == strings.Join(segs[:best], "^") {
			children[p[best]] = true
		}
	}
	names := make([]string, 0, len(children))
	for c := range children {
		names = append(names, c)
	}
	sort.Strings(names)
	if len(names) > maxSuggestions {
		names = names[:maxSuggestions]
	}
	quoted := make([]string, len(names))
	for i, n := range names {
		quoted[i] = fmt.Sprintf("%q", n)
	}
	return fmt.Sprintf("nearest existing prefix %q, did you mean %s", strings.Join(segs[:best], "^"), strings.Join(quoted, ", "))
}
