// Package lsp provides a Language Server Protocol implementation for
// IEC 61131-3 Structured Text, built on the GLSP library.
package lsp

import (
	"net/url"
	"strings"
	"sync"

	"github.com/centroid-is/stc/pkg/analyzer"
	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/parser"
	"github.com/centroid-is/stc/pkg/pipeline"
)

// Document represents an open text document with its current content,
// parse results, and analysis results.
type Document struct {
	URI            string
	Content        string
	Version        int32
	ParseResult    *parser.ParseResult
	AnalysisResult *analyzer.AnalysisResult
}

// DocumentStore manages open documents with thread-safe access.
type DocumentStore struct {
	mu   sync.RWMutex
	docs map[string]*Document
}

// NewDocumentStore creates a new empty document store.
func NewDocumentStore() *DocumentStore {
	return &DocumentStore{
		docs: make(map[string]*Document),
	}
}

// Open adds a new document to the store and triggers parsing and analysis.
func (s *DocumentStore) Open(uri, content string, version int32) *Document {
	doc := &Document{
		URI:     uri,
		Content: content,
		Version: version,
	}
	s.analyzeDocument(doc)

	s.mu.Lock()
	s.docs[uri] = doc
	s.mu.Unlock()

	return doc
}

// Update replaces the content of an existing document and re-analyzes.
func (s *DocumentStore) Update(uri, content string, version int32) *Document {
	s.mu.Lock()
	doc, ok := s.docs[uri]
	if !ok {
		doc = &Document{URI: uri}
		s.docs[uri] = doc
	}
	doc.Content = content
	doc.Version = version
	s.mu.Unlock()

	s.analyzeDocument(doc)

	return doc
}

// Close removes a document from the store.
func (s *DocumentStore) Close(uri string) {
	s.mu.Lock()
	delete(s.docs, uri)
	s.mu.Unlock()
}

// Get retrieves a document by URI, returning nil if not found.
func (s *DocumentStore) Get(uri string) *Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.docs[uri]
}

// analyzeDocument parses the changed document and performs cross-file
// analysis across all open documents for multi-file symbol resolution.
func (s *DocumentStore) analyzeDocument(doc *Document) {
	filename := uriToFilename(doc.URI)
	pipeResult := pipeline.Parse(filename, doc.Content, nil)
	result := pipeResult.ParseResult
	doc.ParseResult = &result

	// Collect all open documents for cross-file analysis
	s.mu.RLock()
	var allFiles []*ast.SourceFile
	for _, d := range s.docs {
		if d.ParseResult != nil {
			allFiles = append(allFiles, d.ParseResult.File)
		}
	}
	s.mu.RUnlock()

	// Include the current document (may not be in docs yet during Open)
	found := false
	for _, f := range allFiles {
		if f == result.File {
			found = true
			break
		}
	}
	if !found {
		allFiles = append(allFiles, result.File)
	}

	analysisResult := analyzer.Analyze(allFiles, nil)

	// Store full analysis result on all open documents so cross-file
	// features (go-to-def, hover) work across files
	s.mu.RLock()
	for _, d := range s.docs {
		d.AnalysisResult = &analysisResult
	}
	s.mu.RUnlock()
	doc.AnalysisResult = &analysisResult
}

// uriToFilename converts a file:// URI to a local filesystem path.
func uriToFilename(uri string) string {
	parsed, err := url.Parse(uri)
	if err != nil {
		// Fallback: strip file:// prefix manually
		return strings.TrimPrefix(uri, "file://")
	}
	if parsed.Scheme == "file" {
		return parsed.Path
	}
	return uri
}
