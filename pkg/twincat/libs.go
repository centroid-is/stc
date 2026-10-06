package twincat

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/source"
	"github.com/centroid-is/stc/stdlib/beckhoff"
)

// stubDisplayDir is the stable, OS-independent display path prefix for
// embedded Beckhoff stub files.
const stubDisplayDir = "stdlib/vendor/beckhoff"

// maxSiblingLevels is how many ancestor directories the sibling search climbs.
const maxSiblingLevels = 3

// resolution is the cached outcome for one library name.
type resolution struct {
	from string
	path string
}

// resolveCtx carries state across one (recursive) library resolution.
// Output is collected in ordered slices; maps are only used for lookup.
type resolveCtx struct {
	visited      map[string]*resolution // lower-case library name
	libraryPaths map[string]string
	cfgDir       string
	diags        []diag.Diagnostic

	siblings []Source
	libPaths map[string][]Source // keyed by the library_paths key
	stubs    []Source
	stubSeen map[string]bool
}

func newResolveCtx(libraryPaths map[string]string, cfgDir string) *resolveCtx {
	return &resolveCtx{
		visited:      map[string]*resolution{},
		libraryPaths: libraryPaths,
		cfgDir:       cfgDir,
		libPaths:     map[string][]Source{},
		stubSeen:     map[string]bool{},
	}
}

// resolveLibraries resolves the references of info in the locked order:
// own project, sibling plcproj, [build.library_paths], built-in, embedded
// stub, unresolved (VEND020). Sibling libraries are resolved recursively.
// The returned sources are siblings first (in resolution order), then
// library_paths sources sorted by key, then stub files in first-seen
// closure order, so a user override wins under first-library-wins.
func resolveLibraries(info *PlcprojInfo, ownName string, ctx *resolveCtx) ([]LibraryRef, []Source) {
	ctx.visited[strings.ToLower(ownName)] = &resolution{from: FromProject}
	refs := ctx.resolveRefs(info)

	out := append([]Source(nil), ctx.siblings...)
	keys := make([]string, 0, len(ctx.libPaths))
	for k := range ctx.libPaths {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, ctx.libPaths[k]...)
	}
	out = append(out, ctx.stubs...)
	return refs, out
}

func (ctx *resolveCtx) resolveRefs(info *PlcprojInfo) []LibraryRef {
	refs := make([]LibraryRef, 0, len(info.Refs))
	for _, r := range info.Refs {
		res := ctx.resolveOne(info.Path, r)
		refs = append(refs, LibraryRef{
			Name: r.Name, DefaultResolution: r.DefaultResolution, Namespace: r.Namespace,
			ResolvedFrom: res.from, Path: res.path, Pos: r.Pos,
		})
	}
	return refs
}

func (ctx *resolveCtx) resolveOne(plcprojPath string, r PlaceholderRef) *resolution {
	key := strings.ToLower(r.Name)
	if res, ok := ctx.visited[key]; ok {
		return res
	}
	res := &resolution{}
	ctx.visited[key] = res // set before recursing so cycles terminate

	if p := ctx.findSibling(plcprojPath, r); p != "" {
		if ctx.loadSibling(p, r.Name) {
			res.from, res.path = FromSibling, p
			return res
		}
	}
	if lpKey, dir, ok := ctx.libraryPath(r); ok {
		res.from, res.path = FromLibraryPath, dir
		ctx.loadLibraryPath(lpKey, dir, r.Name)
		return res
	}
	if beckhoff.IsBuiltin(r.Name) {
		res.from = FromBuiltin
		return res
	}
	if files, ok := beckhoff.Closure(r.Name); ok {
		res.from = FromStub
		ctx.addStubs(files, r.Name)
		return res
	}
	res.from = FromUnresolved
	ctx.diags = append(ctx.diags, warn(r.Pos, CodeUnresolvedLibrary, "unresolved library reference '%s'", r.Name))
	return res
}

// findSibling looks for <name>.plcproj near plcprojPath: for each ancestor D
// (starting at the plcproj directory's parent, up to maxSiblingLevels) it
// tries D/X/X.plcproj, D/X/*/X.plcproj and D/*/X.plcproj, skipping
// dot-directories. The first level with a hit wins; several hits at that
// level produce VEND026 and the sorted-first path is used.
func (ctx *resolveCtx) findSibling(plcprojPath string, r PlaceholderRef) string {
	self := filepath.Clean(plcprojPath)
	file := r.Name + ".plcproj"
	d := filepath.Dir(filepath.Dir(self))
	for level := 0; level < maxSiblingLevels; level++ {
		seen := map[string]bool{}
		var hits []string
		add := func(p string) {
			if p != "" && p != self && !seen[p] {
				seen[p] = true
				hits = append(hits, p)
			}
		}
		for _, x := range subdirs(d, r.Name) {
			add(fileIn(x, file))
			for _, y := range subdirs(x, "") {
				add(fileIn(y, file))
			}
		}
		for _, y := range subdirs(d, "") {
			add(fileIn(y, file))
		}
		if len(hits) > 0 {
			sort.Strings(hits)
			if len(hits) > 1 {
				ctx.diags = append(ctx.diags, warn(r.Pos, CodeAmbiguousSibling,
					"library '%s' matches several sibling projects: %s; using %s",
					r.Name, strings.Join(hits, ", "), hits[0]))
			}
			return hits[0]
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return ""
}

// subdirs returns the non-dot subdirectories of dir in name order; when
// name is non-empty only those whose name equals it case-insensitively.
func subdirs(dir, name string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if !e.IsDir() || strings.HasPrefix(n, ".") {
			continue
		}
		if name == "" || strings.EqualFold(n, name) {
			out = append(out, filepath.Join(dir, n))
		}
	}
	return out
}

// fileIn returns the path of the regular file in dir named name
// (case-insensitive), or "".
func fileIn(dir, name string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(e.Name(), name) {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

// loadSibling converts the sibling project's objects in plcproj order and
// resolves its own references recursively. It reports false (with a
// diagnostic) when the plcproj cannot be read.
func (ctx *resolveCtx) loadSibling(p, lib string) bool {
	info, ds, err := ReadPlcproj(p)
	ctx.diags = append(ctx.diags, ds...)
	if err != nil {
		ctx.diags = append(ctx.diags, readFailure(p, err))
		return false
	}
	ctx.siblings = append(ctx.siblings, convertItems(info, lib, &ctx.diags, nil)...)
	ctx.resolveRefs(info)
	return true
}

// convertItems converts the POU/GVL/DUT/ITF items of info in plcproj order
// with ModeLayout. TcTTO items are passed to onTask when it is non-nil.
func convertItems(info *PlcprojInfo, lib string, ds *[]diag.Diagnostic, onTask func(Item)) []Source {
	var out []Source
	for _, it := range info.Items {
		switch it.Kind {
		case KindPOU, KindGVL, KindDUT, KindITF:
		case KindTask:
			if onTask != nil {
				onTask(it)
			}
			continue
		default:
			continue
		}
		c, cds, err := ConvertFile(it.AbsPath, it.RelPath, ModeLayout)
		*ds = append(*ds, cds...)
		if err != nil {
			*ds = append(*ds, readFailure(it.AbsPath, err))
			continue
		}
		out = append(out, Source{Path: it.AbsPath, RelPath: it.RelPath, Kind: c.Kind, Name: c.Name, Library: lib, Text: c.Text})
	}
	return out
}

// readFailure turns a read or XML error into a VEND027 error diagnostic.
func readFailure(p string, err error) diag.Diagnostic {
	var bad *BadXMLError
	msg := err.Error()
	if !errors.As(err, &bad) {
		msg = "cannot read " + p + ": " + msg
	}
	return diag.Diagnostic{Severity: diag.Error, Pos: source.Pos{File: p}, Code: CodeBadXML, Message: msg}
}

// libraryPath finds a [build.library_paths] key matching r case-insensitively.
// Keys are visited in sorted order. A configured directory that does not
// exist produces a VEND020 warning and no match.
func (ctx *resolveCtx) libraryPath(r PlaceholderRef) (string, string, bool) {
	keys := make([]string, 0, len(ctx.libraryPaths))
	for k := range ctx.libraryPaths {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !strings.EqualFold(k, r.Name) {
			continue
		}
		dir := ctx.libraryPaths[k]
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(ctx.cfgDir, filepath.FromSlash(dir))
		}
		dir = filepath.Clean(dir)
		if st, err := os.Stat(dir); err != nil || !st.IsDir() {
			ctx.diags = append(ctx.diags, warn(r.Pos, CodeUnresolvedLibrary,
				"library path %s for '%s' is not a directory", dir, r.Name))
			return "", "", false
		}
		return k, dir, true
	}
	return "", "", false
}

func (ctx *resolveCtx) loadLibraryPath(key, dir, lib string) {
	entries, _ := os.ReadDir(dir) // sorted by name; dir was checked by libraryPath
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".st" {
			continue
		}
		m := filepath.Join(dir, e.Name())
		raw, err := os.ReadFile(m)
		if err != nil {
			ctx.diags = append(ctx.diags, readFailure(m, err))
			continue
		}
		ctx.libPaths[key] = append(ctx.libPaths[key], Source{
			Path: m, RelPath: filepath.Base(m), Kind: KindPOU, Name: strings.TrimSuffix(filepath.Base(m), ".st"),
			Library: lib, Text: string(raw),
		})
	}
}

func (ctx *resolveCtx) addStubs(files []string, lib string) {
	for _, f := range files {
		if ctx.stubSeen[f] {
			continue
		}
		ctx.stubSeen[f] = true
		raw, err := fs.ReadFile(beckhoff.FS, f)
		if err != nil {
			// unreachable: Closure only names embedded files
			panic(fmt.Sprintf("embedded stub %s: %v", f, err))
		}
		display := path.Join(stubDisplayDir, f)
		ctx.stubs = append(ctx.stubs, Source{
			Path: display, RelPath: display, Kind: KindPOU, Name: strings.TrimSuffix(f, ".st"),
			Library: lib, Text: string(raw),
		})
	}
}
