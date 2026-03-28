package lint

import (
	"fmt"
	"regexp"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/diag"
)

var (
	// PascalCase: segments starting with uppercase, optionally separated by underscore.
	// Allows FB_Motor, MyMotor, IO_Handler, etc.
	rePascalCase = regexp.MustCompile(`^[A-Z][a-zA-Z0-9]*(_[A-Z][a-zA-Z0-9]*)*$`)

	// UPPER_SNAKE_CASE: all uppercase with underscores.
	reUpperSnake = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$`)
)

// isPascalCase checks if a name follows PascalCase convention (with optional underscore segments).
func isPascalCase(s string) bool {
	return rePascalCase.MatchString(s)
}

// isLowerStart checks if the first character is lowercase.
func isLowerStart(s string) bool {
	if len(s) == 0 {
		return false
	}
	return s[0] >= 'a' && s[0] <= 'z'
}

// isUpperSnake checks if a name follows UPPER_SNAKE_CASE convention.
func isUpperSnake(s string) bool {
	return reUpperSnake.MatchString(s)
}

// checkNaming checks naming conventions based on the specified convention.
func checkNaming(file *ast.SourceFile, convention string) []diag.Diagnostic {
	if convention == "none" {
		return nil
	}

	var diags []diag.Diagnostic

	for _, decl := range file.Declarations {
		// Check POU names
		checkPOUNaming(decl, &diags)

		// Check variable names in var blocks
		varBlocks := pouVarBlocks(decl)
		for _, vb := range varBlocks {
			for _, vd := range vb.Declarations {
				for _, name := range vd.Names {
					if name == nil {
						continue
					}
					if vb.IsConstant {
						// Constants should be UPPER_SNAKE_CASE
						if !isUpperSnake(name.Name) {
							diags = append(diags, diag.Diagnostic{
								Severity: diag.Warning,
								Pos:      spanPos(name),
								Code:     CodeNamingConstant,
								Message:  fmt.Sprintf("constant %q should be UPPER_SNAKE_CASE", name.Name),
							})
						}
					} else {
						// Variables should start with lowercase
						if !isLowerStart(name.Name) {
							diags = append(diags, diag.Diagnostic{
								Severity: diag.Warning,
								Pos:      spanPos(name),
								Code:     CodeNamingVar,
								Message:  fmt.Sprintf("variable %q should start with a lowercase letter", name.Name),
							})
						}
					}
				}
			}
		}
	}

	return diags
}

// checkPOUNaming checks that POU names follow PascalCase convention.
func checkPOUNaming(decl ast.Declaration, diags *[]diag.Diagnostic) {
	var name *ast.Ident

	switch d := decl.(type) {
	case *ast.ProgramDecl:
		name = d.Name
	case *ast.FunctionBlockDecl:
		name = d.Name
	case *ast.FunctionDecl:
		name = d.Name
	default:
		return
	}

	if name == nil {
		return
	}
	if !isPascalCase(name.Name) {
		*diags = append(*diags, diag.Diagnostic{
			Severity: diag.Warning,
			Pos:      spanPos(name),
			Code:     CodeNamingPOU,
			Message:  fmt.Sprintf("POU name %q should be PascalCase", name.Name),
		})
	}
}
