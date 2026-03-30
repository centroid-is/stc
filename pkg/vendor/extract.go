// Package vendor provides extraction of FB declarations from TwinCAT project files.
package vendor

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// TcPlcObject represents the root XML element of a .TcPOU file.
type TcPlcObject struct {
	XMLName xml.Name `xml:"TcPlcObject"`
	POU     TcPOU    `xml:"POU"`
}

// TcPOU represents a POU element within a TcPlcObject.
type TcPOU struct {
	Name           string           `xml:"Name,attr"`
	Declaration    string           `xml:"Declaration"`
	Implementation TcImplementation `xml:"Implementation"`
}

// TcImplementation holds the implementation section of a TcPOU.
type TcImplementation struct {
	ST string `xml:"ST"`
}

// PlcProject represents a .plcproj MSBuild XML file.
type PlcProject struct {
	XMLName    xml.Name       `xml:"Project"`
	ItemGroups []PlcItemGroup `xml:"ItemGroup"`
}

// PlcItemGroup represents an ItemGroup element within a .plcproj.
type PlcItemGroup struct {
	Compiles []PlcCompile `xml:"Compile"`
}

// PlcCompile represents a Compile element (file reference) in a .plcproj.
type PlcCompile struct {
	Include string `xml:"Include,attr"`
}

// ParseTcPOU parses a .TcPOU XML file and returns the POU.
func ParseTcPOU(r io.Reader) (*TcPOU, error) {
	var obj TcPlcObject
	if err := xml.NewDecoder(r).Decode(&obj); err != nil {
		return nil, fmt.Errorf("parsing TcPOU XML: %w", err)
	}
	return &obj.POU, nil
}

// ParseTcPOUFile parses a .TcPOU file from the filesystem.
func ParseTcPOUFile(path string) (*TcPOU, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseTcPOU(f)
}

// ParsePlcProj parses a .plcproj MSBuild XML file and returns the paths of
// all .TcPOU files referenced via Compile elements. Paths are relative to
// the .plcproj file location.
func ParsePlcProj(r io.Reader) ([]string, error) {
	var proj PlcProject
	if err := xml.NewDecoder(r).Decode(&proj); err != nil {
		return nil, fmt.Errorf("parsing plcproj XML: %w", err)
	}

	var pouFiles []string
	for _, ig := range proj.ItemGroups {
		for _, c := range ig.Compiles {
			if strings.HasSuffix(strings.ToLower(c.Include), ".tcpou") {
				pouFiles = append(pouFiles, c.Include)
			}
		}
	}
	return pouFiles, nil
}

// ParsePlcProjFile parses a .plcproj file from the filesystem.
func ParsePlcProjFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParsePlcProj(f)
}

// ExtractStub extracts the declaration section from a TcPOU, stripping the
// implementation body. Returns the declaration as a stub string suitable for
// writing to a .st file.
func ExtractStub(pou *TcPOU) string {
	decl := strings.TrimSpace(pou.Declaration)
	if decl == "" {
		return ""
	}
	return decl + "\n"
}

// ExtractProject reads a .plcproj file, finds all .TcPOU references,
// parses each one, and returns a map of POU name -> stub declaration text.
func ExtractProject(plcprojPath string) (map[string]string, error) {
	pouFiles, err := ParsePlcProjFile(plcprojPath)
	if err != nil {
		return nil, fmt.Errorf("reading project file: %w", err)
	}

	projDir := filepath.Dir(plcprojPath)
	stubs := make(map[string]string)

	for _, relPath := range pouFiles {
		// Normalize backslashes from Windows paths
		relPath = filepath.FromSlash(strings.ReplaceAll(relPath, "\\", "/"))
		absPath := filepath.Join(projDir, relPath)

		pou, err := ParseTcPOUFile(absPath)
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", relPath, err)
		}

		stub := ExtractStub(pou)
		if stub != "" {
			stubs[pou.Name] = stub
		}
	}

	return stubs, nil
}
