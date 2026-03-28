// Package incremental provides data structures for incremental compilation
// of IEC 61131-3 Structured Text projects. It tracks file-level dependencies,
// caches parsed ASTs, and supports per-file invalidation.
package incremental

import (
	"sort"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
)

// FileInfo stores the declarations and references for a single source file.
type FileInfo struct {
	Filename   string
	Declares   []string // POU names declared in this file (uppercased)
	References []string // POU names referenced in this file (uppercased)
}

// DepGraph tracks file-level dependencies based on POU declarations and references.
// It supports computing the transitive closure of dirty files when source files change.
type DepGraph struct {
	files      map[string]*FileInfo // filename -> file info
	declaredIn map[string]string    // POU name (uppercased) -> filename
}

// NewDepGraph creates an empty dependency graph.
func NewDepGraph() *DepGraph {
	return &DepGraph{
		files:      make(map[string]*FileInfo),
		declaredIn: make(map[string]string),
	}
}

// AddFile records the declarations and references for a source file.
// If the file was previously added, it replaces the previous entry.
// All POU names are uppercased for case-insensitive matching.
func (g *DepGraph) AddFile(filename string, declares, references []string) {
	// Remove old entry if exists
	if old, ok := g.files[filename]; ok {
		for _, d := range old.Declares {
			if g.declaredIn[d] == filename {
				delete(g.declaredIn, d)
			}
		}
	}

	// Uppercase all names
	ucDeclares := make([]string, len(declares))
	for i, d := range declares {
		ucDeclares[i] = strings.ToUpper(d)
	}
	ucRefs := make([]string, len(references))
	for i, r := range references {
		ucRefs[i] = strings.ToUpper(r)
	}

	info := &FileInfo{
		Filename:   filename,
		Declares:   ucDeclares,
		References: ucRefs,
	}
	g.files[filename] = info

	// Update declaredIn index
	for _, d := range ucDeclares {
		g.declaredIn[d] = filename
	}
}

// RemoveFile removes a file and its edges from the dependency graph.
func (g *DepGraph) RemoveFile(filename string) {
	if old, ok := g.files[filename]; ok {
		for _, d := range old.Declares {
			if g.declaredIn[d] == filename {
				delete(g.declaredIn, d)
			}
		}
		delete(g.files, filename)
	}
}

// Dependents returns the filenames of files that reference any POU declared
// in the given filename. Does NOT include the filename itself.
func (g *DepGraph) Dependents(filename string) []string {
	info, ok := g.files[filename]
	if !ok {
		return nil
	}

	// Build set of POU names declared in this file
	declaredSet := make(map[string]bool, len(info.Declares))
	for _, d := range info.Declares {
		declaredSet[d] = true
	}

	// Find files that reference any of these POUs
	seen := make(map[string]bool)
	for _, fi := range g.files {
		if fi.Filename == filename {
			continue
		}
		for _, ref := range fi.References {
			if declaredSet[ref] {
				seen[fi.Filename] = true
				break
			}
		}
	}

	result := make([]string, 0, len(seen))
	for f := range seen {
		result = append(result, f)
	}
	sort.Strings(result)
	return result
}

// AllDirty computes the transitive closure of files affected by changes to
// the given set of files. Returns all files in the dirty set (including
// the changed files themselves), sorted for determinism.
func (g *DepGraph) AllDirty(changed []string) []string {
	dirty := make(map[string]bool)
	queue := make([]string, 0, len(changed))

	for _, f := range changed {
		if !dirty[f] {
			dirty[f] = true
			queue = append(queue, f)
		}
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, dep := range g.Dependents(current) {
			if !dirty[dep] {
				dirty[dep] = true
				queue = append(queue, dep)
			}
		}
	}

	result := make([]string, 0, len(dirty))
	for f := range dirty {
		result = append(result, f)
	}
	sort.Strings(result)
	return result
}

// ScanFile walks an ast.SourceFile to extract declared POU names and referenced
// type names, then calls AddFile with the results. This bridges the AST to the
// dependency graph.
func (g *DepGraph) ScanFile(file *ast.SourceFile, filename string) {
	var declares []string
	var references []string

	for _, decl := range file.Declarations {
		switch d := decl.(type) {
		case *ast.ProgramDecl:
			if d.Name != nil {
				declares = append(declares, d.Name.Name)
			}
			references = append(references, extractVarReferences(d.VarBlocks)...)
		case *ast.FunctionBlockDecl:
			if d.Name != nil {
				declares = append(declares, d.Name.Name)
			}
			references = append(references, extractVarReferences(d.VarBlocks)...)
		case *ast.FunctionDecl:
			if d.Name != nil {
				declares = append(declares, d.Name.Name)
			}
			references = append(references, extractVarReferences(d.VarBlocks)...)
		case *ast.TypeDecl:
			if d.Name != nil {
				declares = append(declares, d.Name.Name)
			}
		case *ast.InterfaceDecl:
			if d.Name != nil {
				declares = append(declares, d.Name.Name)
			}
		}
	}

	g.AddFile(filename, declares, references)
}

// extractVarReferences scans var blocks for NamedType references that are not
// elementary types, returning referenced POU/type names.
func extractVarReferences(blocks []*ast.VarBlock) []string {
	var refs []string
	for _, vb := range blocks {
		for _, vd := range vb.Declarations {
			if nt, ok := vd.Type.(*ast.NamedType); ok && nt.Name != nil {
				name := nt.Name.Name
				// Skip elementary types
				if _, isElem := types.LookupElementaryType(name); !isElem {
					refs = append(refs, name)
				}
			}
		}
	}
	return refs
}
