package twincat

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// BadXMLError reports a project or object file that is not valid XML (VEND027).
type BadXMLError struct {
	Path string
	Err  error
}

func (e *BadXMLError) Error() string {
	return fmt.Sprintf("%s: %s: invalid XML: %v", CodeBadXML, e.Path, e.Err)
}

func (e *BadXMLError) Unwrap() error { return e.Err }

// normPath converts a TwinCAT (Windows) relative path to the host form.
func normPath(p string) string {
	return filepath.FromSlash(strings.ReplaceAll(p, "\\", "/"))
}

// slashPath converts a TwinCAT relative path to forward slashes.
func slashPath(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

// readAbs reads path and returns its absolute form and contents.
func readAbs(path string) (string, []byte, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", nil, err
	}
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return "", nil, err
	}
	return absPath, raw, nil
}

// decodeXML unmarshals raw into v, wrapping failures as BadXMLError.
func decodeXML(path string, raw []byte, v any) error {
	if err := xml.NewDecoder(bytes.NewReader(raw)).Decode(v); err != nil {
		return &BadXMLError{Path: path, Err: err}
	}
	return nil
}

// lineCol returns the 1-based line and byte column of offset off in raw.
func lineCol(raw []byte, off int64) (int, int) {
	if off > int64(len(raw)) {
		off = int64(len(raw))
	}
	head := raw[:off]
	line := 1 + bytes.Count(head, []byte{'\n'})
	col := int(off) - (bytes.LastIndexByte(head, '\n') + 1) + 1
	return line, col
}
