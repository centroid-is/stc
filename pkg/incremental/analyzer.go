package incremental

import (
	"os"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
	"github.com/centroid-is/stc/pkg/parser"
)

// IncrStats holds statistics about an incremental analysis run.
type IncrStats struct {
	TotalFiles   int
	StaleFiles   int
	SkippedFiles int
}

// IncrResult holds the output of incremental parsing: the collected ASTs,
// any parse diagnostics, and incremental stats.
type IncrResult struct {
	Files []*ast.SourceFile
	Diags []diag.Diagnostic
	Stats IncrStats
}

// IncrementalAnalyzer orchestrates incremental compilation by tracking
// file content hashes, computing dirty sets via the dependency graph,
// and skipping parsing for unchanged files.
type IncrementalAnalyzer struct {
	cache    *FileCache
	graph    *DepGraph
	cacheDir string
	stats    IncrStats
}

// NewIncrementalAnalyzer creates an incremental analyzer that persists
// its cache index in the given directory under .stc-cache/.
// It restores previous hashes from disk if available.
func NewIncrementalAnalyzer(cacheDir string) *IncrementalAnalyzer {
	cache := NewFileCache()
	_ = cache.LoadIndex(cacheDir)
	return &IncrementalAnalyzer{
		cache:    cache,
		graph:    NewDepGraph(),
		cacheDir: cacheDir,
	}
}

// Stats returns the statistics from the most recent Parse call.
func (ia *IncrementalAnalyzer) Stats() IncrStats {
	return ia.stats
}

// Parse performs incremental parsing on the given files.
// It skips parsing for files whose content hash has not changed since
// the previous invocation. Returns the collected ASTs and parse diagnostics.
// The caller is responsible for running semantic analysis on the returned files.
func (ia *IncrementalAnalyzer) Parse(filenames []string) IncrResult {
	// Build set of current filenames for removal detection
	currentFiles := make(map[string]bool, len(filenames))
	for _, f := range filenames {
		currentFiles[f] = true
	}

	// Detect removed files: in cache but not in current filenames
	for _, cached := range ia.cache.Files() {
		if !currentFiles[cached] {
			ia.graph.RemoveFile(cached)
			ia.cache.Remove(cached)
		}
	}

	// Phase 1: Read files, compute hashes, identify stale files
	type fileEntry struct {
		filename string
		content  []byte
		hash     string
		stale    bool
	}
	entries := make([]fileEntry, 0, len(filenames))
	var staleFiles []string

	for _, filename := range filenames {
		content, err := os.ReadFile(filename)
		if err != nil {
			// Skip files that can't be read
			continue
		}
		hash := ContentHash(content)
		stale := ia.cache.IsStale(filename, hash)
		entries = append(entries, fileEntry{
			filename: filename,
			content:  content,
			hash:     hash,
			stale:    stale,
		})
		if stale {
			staleFiles = append(staleFiles, filename)
		}
	}

	// Phase 2: Parse stale files and non-stale files that need parsing
	for i := range entries {
		e := &entries[i]
		if e.stale {
			result := parser.Parse(e.filename, string(e.content))
			ia.cache.Store(e.filename, e.hash, &result)
		} else if ia.cache.NeedsParse(e.filename) {
			result := parser.Parse(e.filename, string(e.content))
			ia.cache.Store(e.filename, e.hash, &result)
		}
	}

	// Phase 3: Rebuild dependency graph from all cached parse results
	for _, e := range entries {
		if pr, ok := ia.cache.Load(e.filename); ok && pr != nil && pr.File != nil {
			ia.graph.ScanFile(pr.File, e.filename)
		}
	}

	// Phase 4: Collect all ASTs and parse diagnostics
	var allFiles []*ast.SourceFile
	var allDiags []diag.Diagnostic
	for _, e := range entries {
		if pr, ok := ia.cache.Load(e.filename); ok && pr != nil && pr.File != nil {
			allFiles = append(allFiles, pr.File)
			allDiags = append(allDiags, pr.Diags...)
		}
	}

	// Phase 5: Record stats and persist cache
	ia.stats = IncrStats{
		TotalFiles:   len(entries),
		StaleFiles:   len(staleFiles),
		SkippedFiles: len(entries) - len(staleFiles),
	}

	_ = ia.cache.SaveIndex(ia.cacheDir)

	return IncrResult{
		Files: allFiles,
		Diags: allDiags,
		Stats: ia.stats,
	}
}
