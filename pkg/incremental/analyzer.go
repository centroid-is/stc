package incremental

import (
	"os"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/project"
)

// IncrStats holds statistics about an incremental analysis run.
type IncrStats struct {
	TotalFiles   int
	StaleFiles   int
	SkippedFiles int
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

// Stats returns the statistics from the most recent Analyze call.
func (ia *IncrementalAnalyzer) Stats() IncrStats {
	return ia.stats
}

// Analyze performs incremental analysis on the given files.
// It skips parsing for files whose content hash has not changed since
// the previous invocation. The full semantic analysis pass runs on all
// files regardless (v1 simplification).
func (ia *IncrementalAnalyzer) Analyze(filenames []string, cfg *project.Config) analyzer.AnalysisResult {
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
			// Stale: parse and store in cache
			result := parser.Parse(e.filename, string(e.content))
			ia.cache.Store(e.filename, e.hash, &result)
		} else if ia.cache.NeedsParse(e.filename) {
			// Not stale but needs parsing (loaded from disk index, no AST)
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

	// Phase 4: Collect all ASTs for full semantic analysis
	var allFiles []*ast.SourceFile
	var parseDiags []parser.ParseResult
	for _, e := range entries {
		if pr, ok := ia.cache.Load(e.filename); ok && pr != nil && pr.File != nil {
			allFiles = append(allFiles, pr.File)
			parseDiags = append(parseDiags, *pr)
		}
	}

	// Phase 5: Run full semantic analysis on all files
	// (v1: always full semantic pass; parse skip is the main win)
	analysisResult := analyzer.Analyze(allFiles, cfg)

	// Merge parse diagnostics into analysis result
	var allDiagList []interface{ String() string }
	_ = allDiagList // suppress unused
	// Collect parse diagnostics
	for _, pr := range parseDiags {
		if len(pr.Diags) > 0 {
			analysisResult.Diags = append(pr.Diags, analysisResult.Diags...)
		}
	}

	// Phase 6: Record stats and persist cache
	ia.stats = IncrStats{
		TotalFiles:   len(entries),
		StaleFiles:   len(staleFiles),
		SkippedFiles: len(entries) - len(staleFiles),
	}

	_ = ia.cache.SaveIndex(ia.cacheDir)

	return analysisResult
}
