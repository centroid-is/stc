package interp

import (
	"errors"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
)

// RegisterFiles registers every declaration of files on the interpreter in
// the same order as the test runner: user FUNCTIONs, TYPEs (with their
// defaults), FUNCTION_BLOCKs, enums, and finally all GVLs through
// RegisterGVLs in file order. Pass library files first so library GVLs are
// registered before project GVLs. PROGRAMs are not instantiated here; see
// NewRuntime and NewScanCycleEngineWith.
//
// It returns the initialiser and array bound errors recorded so far
// (InitErrors), joined, or nil. Nil files are skipped.
func (interp *Interpreter) RegisterFiles(files []*ast.SourceFile) error {
	if interp.TypeDecls == nil {
		interp.TypeDecls = make(map[string]ast.TypeSpec)
	}
	if interp.TypeInits == nil {
		interp.TypeInits = make(map[string]ast.Expr)
	}
	if interp.FBDecls == nil {
		interp.FBDecls = make(map[string]*ast.FunctionBlockDecl)
	}
	var enums []*ast.TypeDecl
	var gvls []*ast.GVLDecl
	for _, f := range files {
		if f == nil {
			continue
		}
		for _, d := range f.Declarations {
			switch d := d.(type) {
			case *ast.FunctionDecl:
				interp.RegisterFunctionDecl(d)
			case *ast.TypeDecl:
				if d.Name == nil {
					continue
				}
				name := strings.ToUpper(d.Name.Name)
				interp.TypeDecls[name] = d.Type
				if d.InitValue != nil {
					interp.TypeInits[name] = d.InitValue
				}
				if _, ok := d.Type.(*ast.EnumType); ok {
					enums = append(enums, d)
				}
			case *ast.FunctionBlockDecl:
				if d.Name != nil {
					interp.FBDecls[strings.ToUpper(d.Name.Name)] = d
				}
			case *ast.GVLDecl:
				gvls = append(gvls, d)
			}
		}
	}
	for _, d := range enums {
		interp.RegisterEnumDecl(d.Name.Name, d.Type.(*ast.EnumType), d.Attributes)
	}
	interp.RegisterGVLs(gvls)
	return errors.Join(interp.InitErrors()...)
}
