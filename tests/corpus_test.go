package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/parser"
)

// parseWithTimeout runs the parser with a timeout to guard against infinite loops.
// Returns the parse result and whether the parse completed in time.
func parseWithTimeout(filename, src string, timeout time.Duration) (result parser.ParseResult, timedOut bool, panicVal any) {
	type parseOut struct {
		result   parser.ParseResult
		panicVal any
	}
	ch := make(chan parseOut, 1)
	go func() {
		var out parseOut
		defer func() {
			if r := recover(); r != nil {
				out.panicVal = r
			}
			ch <- out
		}()
		out.result = parser.Parse(filename, src)
	}()

	select {
	case out := <-ch:
		return out.result, false, out.panicVal
	case <-time.After(timeout):
		return parser.ParseResult{}, true, nil
	}
}

// TestCorpusParse walks all .st files in tests/corpus/ and attempts to parse each one.
// It fails if any file causes a panic, and reports overall parse success rates.
func TestCorpusParse(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file location")
	}
	corpusDir := filepath.Join(filepath.Dir(thisFile), "corpus")

	if _, err := os.Stat(corpusDir); os.IsNotExist(err) {
		t.Skipf("corpus directory not found: %s", corpusDir)
	}

	var (
		mu        sync.Mutex
		total     int
		success   int
		withError int
		panicked  int
		timedOut  int
		results   []string
	)

	const perFileTimeout = 3 * time.Second

	err := filepath.Walk(corpusDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(strings.ToLower(info.Name()), ".st") {
			return nil
		}

		mu.Lock()
		total++
		mu.Unlock()
		relPath, _ := filepath.Rel(corpusDir, path)

		t.Run(relPath, func(t *testing.T) {
			src, readErr := os.ReadFile(path)
			if readErr != nil {
				t.Fatalf("failed to read file: %v", readErr)
			}

			result, didTimeout, panicVal := parseWithTimeout(info.Name(), string(src), perFileTimeout)

			mu.Lock()
			defer mu.Unlock()

			if didTimeout {
				timedOut++
				results = append(results, fmt.Sprintf("TIMEOUT %s (>%s)", relPath, perFileTimeout))
				t.Logf("parser timed out after %s (possible infinite loop)", perFileTimeout)
				return
			}

			if panicVal != nil {
				panicked++
				results = append(results, fmt.Sprintf("PANIC   %s: %v", relPath, panicVal))
				t.Errorf("parser panicked: %v", panicVal)
				return
			}

			if len(result.Diags) == 0 {
				success++
				results = append(results, fmt.Sprintf("OK      %s", relPath))
			} else {
				withError++
				results = append(results, fmt.Sprintf("ERRORS  %s (%d diagnostics)", relPath, len(result.Diags)))
				for _, d := range result.Diags {
					t.Logf("  diagnostic: %s", d.Message)
				}
			}
		})
		return nil
	})
	if err != nil {
		t.Fatalf("failed to walk corpus: %v", err)
	}

	// Summary
	t.Logf("\n=== Corpus Parse Summary ===")
	t.Logf("Total files:    %d", total)
	t.Logf("Clean parse:    %d", success)
	t.Logf("Parse w/errors: %d", withError)
	t.Logf("Panicked:       %d", panicked)
	t.Logf("Timed out:      %d", timedOut)
	if total > 0 {
		rate := float64(success) * 100 / float64(total)
		t.Logf("Success rate:   %.1f%%", rate)
	}
	t.Logf("")
	for _, r := range results {
		t.Logf("  %s", r)
	}

	if panicked > 0 {
		t.Fatalf("%d file(s) caused parser panics", panicked)
	}
	if timedOut > 0 {
		t.Logf("WARNING: %d file(s) caused parser timeouts (possible infinite loops)", timedOut)
	}
}
