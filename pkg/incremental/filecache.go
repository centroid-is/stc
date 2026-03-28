package incremental

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/centroid-is/stc/pkg/parser"
)

// ContentHash returns a deterministic hex-encoded SHA-256 hash of the content.
func ContentHash(content []byte) string {
	h := sha256.Sum256(content)
	return hex.EncodeToString(h[:])
}

// CacheEntry is an in-memory cache entry for a single source file.
type CacheEntry struct {
	Hash        string
	ParseResult *parser.ParseResult
}

// FileCache tracks file content hashes and cached ParseResults.
// It supports persisting the hash index to disk for cross-invocation reuse.
type FileCache struct {
	entries map[string]*CacheEntry // keyed by filename
}

// NewFileCache creates an empty file cache.
func NewFileCache() *FileCache {
	return &FileCache{
		entries: make(map[string]*CacheEntry),
	}
}

// IsStale returns true if the file is not in the cache or its hash differs
// from currentHash.
func (c *FileCache) IsStale(filename, currentHash string) bool {
	entry, ok := c.entries[filename]
	if !ok {
		return true
	}
	return entry.Hash != currentHash
}

// Store stores or replaces a cache entry for the given file.
func (c *FileCache) Store(filename, hash string, result *parser.ParseResult) {
	c.entries[filename] = &CacheEntry{
		Hash:        hash,
		ParseResult: result,
	}
}

// Load returns the cached ParseResult for the given file.
// Returns nil and false if not cached.
func (c *FileCache) Load(filename string) (*parser.ParseResult, bool) {
	entry, ok := c.entries[filename]
	if !ok {
		return nil, false
	}
	return entry.ParseResult, true
}

// Remove removes a cache entry for the given file.
func (c *FileCache) Remove(filename string) {
	delete(c.entries, filename)
}

// Files returns all cached filenames, sorted for determinism.
func (c *FileCache) Files() []string {
	result := make([]string, 0, len(c.entries))
	for f := range c.entries {
		result = append(result, f)
	}
	sort.Strings(result)
	return result
}

// NeedsParse returns true if the file exists in the cache but has no
// ParseResult (i.e., it was loaded from a disk index and hasn't been
// parsed yet this session).
func (c *FileCache) NeedsParse(filename string) bool {
	entry, ok := c.entries[filename]
	if !ok {
		return false
	}
	return entry.ParseResult == nil
}

// IndexEntry is the serializable form of a cache entry for the disk index.
type IndexEntry struct {
	Hash string `json:"hash"`
}

// DiskIndex is the JSON-serializable on-disk cache index.
type DiskIndex struct {
	Version int                    `json:"version"`
	Files   map[string]IndexEntry  `json:"files"`
}

const (
	cacheDir   = ".stc-cache"
	indexFile  = "index.json"
	indexVersion = 1
)

// SaveIndex writes a JSON index file mapping filenames to their content hashes
// inside the given directory at .stc-cache/index.json.
func (c *FileCache) SaveIndex(dir string) error {
	cachePath := filepath.Join(dir, cacheDir)
	if err := os.MkdirAll(cachePath, 0o755); err != nil {
		return err
	}

	idx := DiskIndex{
		Version: indexVersion,
		Files:   make(map[string]IndexEntry, len(c.entries)),
	}
	for filename, entry := range c.entries {
		idx.Files[filename] = IndexEntry{Hash: entry.Hash}
	}

	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(cachePath, indexFile), data, 0o644)
}

// LoadIndex reads .stc-cache/index.json from the given directory and populates
// the cache entries with hashes only (ParseResult is nil). This means IsStale
// returns false for matching hashes even before a Store call.
// Returns nil if the index file does not exist.
func (c *FileCache) LoadIndex(dir string) error {
	indexPath := filepath.Join(dir, cacheDir, indexFile)
	data, err := os.ReadFile(indexPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // No index file is not an error
		}
		return err
	}

	var idx DiskIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		return err
	}

	// Only load version 1 indices
	if idx.Version != indexVersion {
		return nil
	}

	for filename, entry := range idx.Files {
		c.entries[filename] = &CacheEntry{
			Hash:        entry.Hash,
			ParseResult: nil, // Needs re-parse this session
		}
	}

	return nil
}
