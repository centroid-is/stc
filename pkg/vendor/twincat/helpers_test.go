package twincat

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/centroid-is/stc/pkg/diag"
)

// crlfBOM returns raw rewritten with CRLF line ends and a UTF-8 BOM.
func crlfBOM(raw []byte) []byte {
	lf := bytes.ReplaceAll(raw, []byte("\r\n"), []byte("\n"))
	out := bytes.ReplaceAll(lf, []byte("\n"), []byte("\r\n"))
	return append([]byte("\xEF\xBB\xBF"), out...)
}

// copyTreeCRLF copies the directory src into dst, rewriting every file with CRLF and a BOM.
func copyTreeCRLF(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, crlfBOM(raw), 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func findCode(ds []diag.Diagnostic, code string) *diag.Diagnostic {
	for i := range ds {
		if ds[i].Code == code {
			return &ds[i]
		}
	}
	return nil
}

func abs(t *testing.T, p string) string {
	t.Helper()
	a, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func mustRead(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func writeBytes(p string, b []byte) error { return os.WriteFile(p, b, 0o644) }
