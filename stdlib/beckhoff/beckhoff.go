// Package beckhoff exposes the embedded Beckhoff TwinCAT library stubs and
// resolves a library name to the ordered list of stub files it needs.
package beckhoff

import (
	"io/fs"
	"sort"
	"strings"

	"github.com/centroid-is/stc/stdlib"
)

// FS contains the stub .st files at its root (for example "tc2_system.st").
var FS fs.FS = mustSub(stdlib.VendorFS, "vendor/beckhoff")

func mustSub(fsys fs.FS, dir string) fs.FS {
	sub, err := fs.Sub(fsys, dir)
	if err != nil {
		panic(err) // unreachable: dir is a constant valid path
	}
	return sub
}

// stub describes one library: the files it adds and the libraries it needs.
type stub struct {
	files []string
	deps  []string
}

// stubs maps lower-case library names to their stub files and dependencies.
// Tc2_Standard is not listed: it is built in to the checker.
var stubs = map[string]stub{
	"tc2_system":      {files: []string{"common_types.st", "tc2_system.st"}},
	"tc2_utilities":   {files: []string{"tc2_utilities.st"}, deps: []string{"tc2_system"}},
	"tc2_ethercat":    {files: []string{"tc2_ethercat.st"}, deps: []string{"tc2_system", "tc2_utilities"}},
	"tc2_modbussrv":   {files: []string{"tc2_modbussrv.st"}, deps: []string{"tc2_system"}},
	"tc2_serialcom":   {files: []string{"tc2_serialcom.st"}, deps: []string{"tc2_system"}},
	"tc2_mc2":         {files: []string{"tc2_mc2.st"}, deps: []string{"tc2_system"}},
	"tc3_module":      {files: []string{"tc3_module.st"}, deps: []string{"tc2_system"}},
	"tc3_ipcdiag":     {files: []string{"tc3_ipcdiag.st"}, deps: []string{"tc2_system"}},
	"tc3_eventlogger": {files: []string{"tc3_eventlogger.st"}, deps: []string{"tc2_system"}},
}

// builtins are libraries whose symbols the checker already provides.
var builtins = map[string]bool{"tc2_standard": true}

// Closure returns the stub files needed for lib, dependencies first,
// without duplicates and in a deterministic order. Lookup is
// case-insensitive. ok is false for unknown and built-in libraries.
func Closure(lib string) (files []string, ok bool) {
	key := strings.ToLower(lib)
	if _, found := stubs[key]; !found {
		return nil, false
	}
	seenLib := map[string]bool{}
	seenFile := map[string]bool{}
	var visit func(name string)
	visit = func(name string) {
		if seenLib[name] {
			return
		}
		seenLib[name] = true
		s := stubs[name]
		for _, d := range s.deps {
			visit(d)
		}
		for _, f := range s.files {
			if !seenFile[f] {
				seenFile[f] = true
				files = append(files, f)
			}
		}
	}
	visit(key)
	return files, true
}

// IsBuiltin reports whether lib is provided by the checker itself
// (case-insensitive), so a reference to it needs no stub files.
func IsBuiltin(lib string) bool {
	return builtins[strings.ToLower(lib)]
}

// Libraries returns the lower-case names of all stub libraries, sorted.
func Libraries() []string {
	out := make([]string, 0, len(stubs))
	for name := range stubs {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
