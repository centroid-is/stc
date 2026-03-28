package incremental

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/centroid-is/stc/pkg/parser"
)

func TestContentHash(t *testing.T) {
	h1 := ContentHash([]byte("hello"))
	h2 := ContentHash([]byte("hello"))
	h3 := ContentHash([]byte("world"))

	if h1 != h2 {
		t.Fatalf("same content should produce same hash: %q != %q", h1, h2)
	}
	if h1 == h3 {
		t.Fatal("different content should produce different hash")
	}
	if len(h1) != 64 { // SHA-256 hex = 64 chars
		t.Fatalf("hash length = %d, want 64", len(h1))
	}
}

func TestFileCacheStoreAndLoad(t *testing.T) {
	c := NewFileCache()

	result := &parser.ParseResult{}
	c.Store("motor.st", "abc123", result)

	got, ok := c.Load("motor.st")
	if !ok {
		t.Fatal("Load(motor.st) should return true after Store")
	}
	if got != result {
		t.Fatal("Load should return the same ParseResult that was stored")
	}

	_, ok = c.Load("nonexistent.st")
	if ok {
		t.Fatal("Load(nonexistent.st) should return false")
	}
}

func TestFileCacheIsStale(t *testing.T) {
	c := NewFileCache()

	// Not in cache -> stale
	if !c.IsStale("motor.st", "hash1") {
		t.Fatal("IsStale should return true for uncached file")
	}

	c.Store("motor.st", "hash1", &parser.ParseResult{})

	// Same hash -> not stale
	if c.IsStale("motor.st", "hash1") {
		t.Fatal("IsStale should return false for matching hash")
	}

	// Different hash -> stale
	if !c.IsStale("motor.st", "hash2") {
		t.Fatal("IsStale should return true for different hash")
	}
}

func TestFileCacheRemove(t *testing.T) {
	c := NewFileCache()
	c.Store("motor.st", "hash1", &parser.ParseResult{})

	c.Remove("motor.st")

	_, ok := c.Load("motor.st")
	if ok {
		t.Fatal("Load should return false after Remove")
	}
}

func TestFileCacheFiles(t *testing.T) {
	c := NewFileCache()
	c.Store("b.st", "h1", &parser.ParseResult{})
	c.Store("a.st", "h2", &parser.ParseResult{})

	files := c.Files()
	if len(files) != 2 {
		t.Fatalf("Files() returned %d entries, want 2", len(files))
	}
	// Should be sorted
	if files[0] != "a.st" || files[1] != "b.st" {
		t.Fatalf("Files() = %v, want [a.st b.st]", files)
	}
}

func TestDiskIndexRoundTrip(t *testing.T) {
	dir := t.TempDir()

	// Create and store
	c1 := NewFileCache()
	c1.Store("motor.st", "hash_motor", &parser.ParseResult{})
	c1.Store("main.st", "hash_main", &parser.ParseResult{})

	if err := c1.SaveIndex(dir); err != nil {
		t.Fatalf("SaveIndex failed: %v", err)
	}

	// Verify file exists
	indexPath := filepath.Join(dir, ".stc-cache", "index.json")
	if _, err := os.Stat(indexPath); err != nil {
		t.Fatalf("index.json not created: %v", err)
	}

	// Load into new cache
	c2 := NewFileCache()
	if err := c2.LoadIndex(dir); err != nil {
		t.Fatalf("LoadIndex failed: %v", err)
	}

	// Same hashes should not be stale
	if c2.IsStale("motor.st", "hash_motor") {
		t.Fatal("motor.st should not be stale after LoadIndex with same hash")
	}
	if c2.IsStale("main.st", "hash_main") {
		t.Fatal("main.st should not be stale after LoadIndex with same hash")
	}

	// Different hash should be stale
	if !c2.IsStale("motor.st", "different_hash") {
		t.Fatal("motor.st should be stale with different hash")
	}

	// ParseResult should be nil (needs re-parse)
	if c2.NeedsParse("motor.st") != true {
		t.Fatal("NeedsParse should return true for index-loaded entry")
	}
}

func TestFileCacheNeedsParse(t *testing.T) {
	c := NewFileCache()

	// Not in cache -> false (no entry at all)
	if c.NeedsParse("motor.st") {
		t.Fatal("NeedsParse should return false for missing file")
	}

	// Stored with ParseResult -> false
	c.Store("motor.st", "hash1", &parser.ParseResult{})
	if c.NeedsParse("motor.st") {
		t.Fatal("NeedsParse should return false after Store with result")
	}
}

func TestDiskIndexLoadMissing(t *testing.T) {
	dir := t.TempDir()

	c := NewFileCache()
	// Loading from dir without index should not error
	if err := c.LoadIndex(dir); err != nil {
		t.Fatalf("LoadIndex should not error on missing index: %v", err)
	}
}
