package vendor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/pipeline"
	"github.com/centroid-is/stc/pkg/project"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/centroid-is/stc/pkg/types"
)

// LoadMocks reads and parses .st mock files from all configured mock paths.
// Each path in cfg.Test.MockPaths is a directory containing .st mock files.
// Relative paths are resolved against projectDir. Returns all parsed
// SourceFiles, or an error if any configured path does not exist.
func LoadMocks(cfg *project.Config, projectDir string) ([]*ast.SourceFile, error) {
	if len(cfg.Test.MockPaths) == 0 {
		return nil, nil
	}

	var result []*ast.SourceFile

	for _, mockPath := range cfg.Test.MockPaths {
		// Resolve relative paths against project directory
		if !filepath.IsAbs(mockPath) {
			mockPath = filepath.Join(projectDir, mockPath)
		}

		// Verify directory exists
		info, err := os.Stat(mockPath)
		if err != nil {
			return nil, fmt.Errorf("mock path %q does not exist: %w", mockPath, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("mock path %q is not a directory", mockPath)
		}

		// Glob for .st files (non-recursive)
		pattern := filepath.Join(mockPath, "*.st")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("mock path %q: glob error: %w", mockPath, err)
		}

		for _, match := range matches {
			content, err := os.ReadFile(match)
			if err != nil {
				return nil, fmt.Errorf("mock path %q: reading %q: %w", mockPath, match, err)
			}

			parsed := pipeline.Parse(match, string(content), nil)
			if parsed.File != nil {
				result = append(result, parsed.File)
			}
		}
	}

	return result, nil
}

// ValidateMockSignatures checks that mock function blocks have the same
// parameter signatures (inputs, outputs, in-outs) as their corresponding
// library stubs. Returns a slice of errors describing any mismatches.
// FBs in mock files that are not found in the symbol table are silently
// skipped (they may be entirely new test-only FBs).
func ValidateMockSignatures(mockFiles []*ast.SourceFile, table *symbols.Table) []error {
	var errs []error

	for _, file := range mockFiles {
		for _, decl := range file.Declarations {
			fbDecl, ok := decl.(*ast.FunctionBlockDecl)
			if !ok || fbDecl.Name == nil {
				continue
			}
			name := fbDecl.Name.Name

			// Look up the symbol in the table
			sym := table.LookupGlobal(name)
			if sym == nil {
				// Not in library -- skip validation
				continue
			}

			libFBType, ok := sym.Type.(*types.FunctionBlockType)
			if !ok {
				continue
			}

			// Build mock's parameter lists from var blocks
			mockFBType := extractFBParams(fbDecl)

			// Validate inputs
			if inputErrs := validateParams(name, "input", libFBType.Inputs, mockFBType.Inputs); len(inputErrs) > 0 {
				errs = append(errs, inputErrs...)
			}

			// Validate outputs
			if outputErrs := validateParams(name, "output", libFBType.Outputs, mockFBType.Outputs); len(outputErrs) > 0 {
				errs = append(errs, outputErrs...)
			}

			// Validate in-outs
			if inoutErrs := validateParams(name, "in-out", libFBType.InOuts, mockFBType.InOuts); len(inoutErrs) > 0 {
				errs = append(errs, inoutErrs...)
			}
		}
	}

	return errs
}

// extractFBParams builds a FunctionBlockType with parameter lists from
// an AST FunctionBlockDecl's var blocks.
func extractFBParams(d *ast.FunctionBlockDecl) *types.FunctionBlockType {
	fbt := &types.FunctionBlockType{Name: d.Name.Name}
	for _, vb := range d.VarBlocks {
		for _, vd := range vb.Declarations {
			typeName := typeSpecName(vd.Type)
			for _, n := range vd.Names {
				param := types.Parameter{
					Name: n.Name,
					Type: &types.PrimitiveType{Kind_: lookupKind(typeName)},
				}
				switch vb.Section {
				case ast.VarInput:
					param.Direction = types.DirInput
					fbt.Inputs = append(fbt.Inputs, param)
				case ast.VarOutput:
					param.Direction = types.DirOutput
					fbt.Outputs = append(fbt.Outputs, param)
				case ast.VarInOut:
					param.Direction = types.DirInOut
					fbt.InOuts = append(fbt.InOuts, param)
				}
			}
		}
	}
	return fbt
}

// typeSpecName extracts a type name string from a TypeSpec.
func typeSpecName(ts ast.TypeSpec) string {
	if nt, ok := ts.(*ast.NamedType); ok && nt.Name != nil {
		return nt.Name.Name
	}
	return ""
}

// lookupKind resolves a type name to a TypeKind for comparison.
func lookupKind(name string) types.TypeKind {
	if typ, ok := types.LookupElementaryType(strings.ToUpper(name)); ok {
		return typ.Kind()
	}
	return types.KindInvalid
}

// validateParams compares library and mock parameter lists, returning errors
// for count or type mismatches.
func validateParams(fbName, dir string, lib, mock []types.Parameter) []error {
	var errs []error
	if len(lib) != len(mock) {
		errs = append(errs, fmt.Errorf(
			"mock %q: %s parameter count mismatch: library has %d, mock has %d",
			fbName, dir, len(lib), len(mock)))
		return errs
	}
	for i := range lib {
		libType := lib[i].Type.String()
		mockType := mock[i].Type.String()
		if !strings.EqualFold(libType, mockType) {
			errs = append(errs, fmt.Errorf(
				"mock %q: %s parameter %d (%s) type mismatch: library has %s, mock has %s",
				fbName, dir, i+1, lib[i].Name, libType, mockType))
		}
	}
	return errs
}
