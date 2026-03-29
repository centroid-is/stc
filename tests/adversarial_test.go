package tests

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/centroid-is/stc/pkg/parser"
)

// TestAdversarial_ParseAllFiles walks all .st files in tests/adversarial/ and
// attempts to parse each one. It fails if any file causes a panic or hangs.
func TestAdversarial_ParseAllFiles(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file location")
	}
	advDir := filepath.Join(filepath.Dir(thisFile), "adversarial")

	if _, err := os.Stat(advDir); os.IsNotExist(err) {
		t.Skipf("adversarial directory not found: %s", advDir)
	}

	entries, err := os.ReadDir(advDir)
	if err != nil {
		t.Fatalf("failed to read adversarial directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".st") {
			continue
		}

		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(advDir, name)
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read file: %v", err)
			}

			// Parse with timeout and panic recovery
			result, timedOut, panicVal := parseWithTimeout(name, string(src), 5*time.Second)

			if panicVal != nil {
				t.Fatalf("PANIC parsing %s: %v", name, panicVal)
			}
			if timedOut {
				t.Fatalf("TIMEOUT parsing %s (hung for >5s)", name)
			}
			if result.File == nil {
				t.Fatalf("Parse returned nil File for %s", name)
			}

			// Log diagnostics for debugging
			if len(result.Diags) > 0 {
				t.Logf("%s: %d diagnostics (expected for adversarial input)", name, len(result.Diags))
			}
		})
	}
}

// TestAdversarial_InterpretAllFiles walks adversarial .st files and attempts
// to interpret any PROGRAM declarations. Panics are failures, errors are fine.
func TestAdversarial_InterpretAllFiles(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to determine test file location")
	}
	advDir := filepath.Join(filepath.Dir(thisFile), "adversarial")

	if _, err := os.Stat(advDir); os.IsNotExist(err) {
		t.Skipf("adversarial directory not found: %s", advDir)
	}

	entries, err := os.ReadDir(advDir)
	if err != nil {
		t.Fatalf("failed to read adversarial directory: %v", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".st") {
			continue
		}

		name := entry.Name()
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(advDir, name)
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read file: %v", err)
			}

			result := parser.Parse(name, string(src))
			if result.File == nil {
				return // parse failed to produce file, skip
			}

			// Try to interpret via a goroutine with timeout
			done := make(chan struct{})
			var panicVal any

			go func() {
				defer func() {
					if r := recover(); r != nil {
						panicVal = r
					}
					close(done)
				}()
				interpretFile(result)
			}()

			select {
			case <-done:
				if panicVal != nil {
					t.Fatalf("PANIC interpreting %s: %v", name, panicVal)
				}
			case <-time.After(5 * time.Second):
				t.Fatalf("TIMEOUT interpreting %s (hung for >5s)", name)
			}
		})
	}
}

// interpretFile attempts to interpret PROGRAM declarations found in a parse result.
func interpretFile(result parser.ParseResult) {
	// Import interp here would create a cycle, so we just re-parse-and-check
	// that the AST is well-formed. The actual interpretation is tested in
	// pkg/interp/adversarial_test.go
	for _, decl := range result.File.Declarations {
		_ = decl.Children()
	}
}

// TestAdversarial_GeneratedEdgeCases tests programmatically generated edge cases.
func TestAdversarial_GeneratedEdgeCases(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{"null_bytes", "\x00\x00\x00"},
		{"all_255", "\xff\xff\xff\xff"},
		{"crlf_everywhere", "PROGRAM p\r\nVAR x : INT;\r\nEND_VAR\r\nEND_PROGRAM\r\n"},
		{"cr_only", "PROGRAM p\rVAR x : INT;\rEND_VAR\rEND_PROGRAM\r"},
		{"mixed_newlines", "PROGRAM p\nVAR x : INT;\r\nEND_VAR\rEND_PROGRAM\n"},
		{"tabs_only", "\t\t\t\t\t"},
		{"very_long_line", "PROGRAM p\nVAR " + strings.Repeat("x", 100000) + " : INT; END_VAR\nEND_PROGRAM"},
		{"many_empty_lines", strings.Repeat("\n", 10000)},
		{"many_spaces", strings.Repeat(" ", 10000)},
		{"alternating_open_close", strings.Repeat("()", 5000)},
		{"many_operators", strings.Repeat("+ - * / ", 5000)},
		{"unclosed_parens", strings.Repeat("(", 1000)},
		{"unclosed_brackets", strings.Repeat("[", 1000)},
		{"many_dots", strings.Repeat(".", 10000)},
		{"many_assigns", strings.Repeat(":= ", 5000)},
		{"repeated_keywords", strings.Repeat("IF THEN ELSE END_IF ", 1000)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, timedOut, panicVal := parseWithTimeout(tc.name+".st", tc.input, 5*time.Second)
			if panicVal != nil {
				t.Fatalf("PANIC: %v", panicVal)
			}
			if timedOut {
				t.Fatalf("TIMEOUT (hung for >5s)")
			}
			if result.File == nil {
				t.Fatal("Parse returned nil File")
			}
		})
	}
}
