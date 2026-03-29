package interp

import (
	"strings"
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/types"
)

// ---------------------------------------------------------------------------
// 1. RegisterFunction / RegisterEnumType  (assertions.go)
// ---------------------------------------------------------------------------

func TestRegisterFunction(t *testing.T) {
	interp := New()
	called := false
	interp.RegisterFunction("MY_FUNC", func(args []Value, pos ast.Pos) (Value, error) {
		called = true
		return IntValue(99), nil
	})
	if interp.LocalFunctions == nil {
		t.Fatal("LocalFunctions should be initialised")
	}
	fn, ok := interp.LocalFunctions["MY_FUNC"]
	if !ok {
		t.Fatal("MY_FUNC not registered")
	}
	v, err := fn(nil, ast.Pos{})
	if err != nil {
		t.Fatal(err)
	}
	if !called || v.Int != 99 {
		t.Fatalf("unexpected result: called=%v, v=%v", called, v)
	}
}

func TestRegisterFunction_NilMap(t *testing.T) {
	interp := New()
	interp.LocalFunctions = nil
	interp.RegisterFunction("F", func(args []Value, pos ast.Pos) (Value, error) {
		return BoolValue(true), nil
	})
	if _, ok := interp.LocalFunctions["F"]; !ok {
		t.Fatal("expected F registered on nil map")
	}
}

func TestRegisterEnumType(t *testing.T) {
	interp := New()
	interp.RegisterEnumType("Color", map[string]int64{"RED": 0, "GREEN": 1, "BLUE": 2})
	if interp.EnumTypes == nil {
		t.Fatal("EnumTypes should be initialised")
	}
	m, ok := interp.EnumTypes["COLOR"]
	if !ok {
		t.Fatal("COLOR not found (should be upper-cased)")
	}
	if m["GREEN"] != 1 {
		t.Fatalf("expected GREEN=1, got %d", m["GREEN"])
	}
}

func TestRegisterEnumType_NilMap(t *testing.T) {
	interp := New()
	interp.EnumTypes = nil
	interp.RegisterEnumType("Status", map[string]int64{"OK": 0})
	if _, ok := interp.EnumTypes["STATUS"]; !ok {
		t.Fatal("expected STATUS registered on nil map")
	}
}

// ---------------------------------------------------------------------------
// 2. DefineSubrange / CheckSubrange / FindOwner  (env.go)
// ---------------------------------------------------------------------------

func TestDefineSubrange_And_CheckSubrange(t *testing.T) {
	env := NewEnv(nil)
	env.Define("x", IntValue(5))
	env.DefineSubrange("x", 0, 10)

	// In range
	if msg := env.CheckSubrange("x", IntValue(5)); msg != "" {
		t.Fatalf("expected no error, got: %s", msg)
	}
	// At boundaries
	if msg := env.CheckSubrange("x", IntValue(0)); msg != "" {
		t.Fatalf("expected no error at low bound, got: %s", msg)
	}
	if msg := env.CheckSubrange("x", IntValue(10)); msg != "" {
		t.Fatalf("expected no error at high bound, got: %s", msg)
	}
	// Out of range (low)
	if msg := env.CheckSubrange("x", IntValue(-1)); msg == "" {
		t.Fatal("expected error for -1")
	}
	// Out of range (high)
	if msg := env.CheckSubrange("x", IntValue(11)); msg == "" {
		t.Fatal("expected error for 11")
	}
	// Non-integer value: no constraint violation
	if msg := env.CheckSubrange("x", BoolValue(true)); msg != "" {
		t.Fatalf("expected no error for non-int value, got: %s", msg)
	}
}

func TestCheckSubrange_NoConstraint(t *testing.T) {
	env := NewEnv(nil)
	env.Define("y", IntValue(999))
	if msg := env.CheckSubrange("y", IntValue(999)); msg != "" {
		t.Fatalf("expected no error for unconstrained var, got: %s", msg)
	}
}

func TestCheckSubrange_ParentScope(t *testing.T) {
	parent := NewEnv(nil)
	parent.Define("x", IntValue(5))
	parent.DefineSubrange("x", 0, 10)

	child := NewEnv(parent)
	// Check from child should walk up to parent
	if msg := child.CheckSubrange("x", IntValue(11)); msg == "" {
		t.Fatal("expected error from parent scope constraint")
	}
	if msg := child.CheckSubrange("x", IntValue(5)); msg != "" {
		t.Fatalf("expected no error, got: %s", msg)
	}
}

func TestFindOwner(t *testing.T) {
	parent := NewEnv(nil)
	parent.Define("x", IntValue(1))

	child := NewEnv(parent)
	child.Define("y", IntValue(2))

	// y is in child
	if owner := child.FindOwner("y"); owner != child {
		t.Fatal("expected child to own y")
	}
	// x is in parent, found from child
	if owner := child.FindOwner("x"); owner != parent {
		t.Fatal("expected parent to own x")
	}
	// z not found
	if owner := child.FindOwner("z"); owner != nil {
		t.Fatal("expected nil for undefined variable")
	}
}

func TestFindOwner_CaseInsensitive(t *testing.T) {
	env := NewEnv(nil)
	env.Define("MyVar", IntValue(42))
	if owner := env.FindOwner("myvar"); owner != env {
		t.Fatal("FindOwner should be case-insensitive")
	}
}

// ---------------------------------------------------------------------------
// 3. zeroStruct / zeroFromTypeSpec for PointerType/ReferenceType (fb_instance.go)
// ---------------------------------------------------------------------------

func TestZeroStruct(t *testing.T) {
	st := &ast.StructType{
		Members: []*ast.StructMember{
			{Name: ident("x"), Type: &ast.NamedType{Name: ident("INT")}},
			{Name: ident("y"), Type: &ast.NamedType{Name: ident("BOOL")}},
			{Name: ident("s"), Type: &ast.NamedType{Name: ident("STRING")}},
		},
	}
	v := zeroStruct(st)
	if v.Kind != ValStruct {
		t.Fatalf("expected ValStruct, got %v", v.Kind)
	}
	if len(v.Struct) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(v.Struct))
	}
	if v.Struct["X"].Kind != ValInt {
		t.Fatalf("expected X=ValInt, got %v", v.Struct["X"].Kind)
	}
	if v.Struct["Y"].Kind != ValBool {
		t.Fatalf("expected Y=ValBool, got %v", v.Struct["Y"].Kind)
	}
	if v.Struct["S"].Kind != ValString {
		t.Fatalf("expected S=ValString, got %v", v.Struct["S"].Kind)
	}
}

func TestZeroStruct_NilName(t *testing.T) {
	// StructMember with nil name should be skipped
	st := &ast.StructType{
		Members: []*ast.StructMember{
			{Name: nil, Type: &ast.NamedType{Name: ident("INT")}},
		},
	}
	v := zeroStruct(st)
	if len(v.Struct) != 0 {
		t.Fatalf("expected 0 fields for nil-name member, got %d", len(v.Struct))
	}
}

func TestZeroFromTypeSpec_PointerType(t *testing.T) {
	ts := &ast.PointerType{BaseType: &ast.NamedType{Name: ident("INT")}}
	v := zeroFromTypeSpec(ts)
	if v.Kind != ValPointer {
		t.Fatalf("expected ValPointer, got %v", v.Kind)
	}
	if v.PtrEnv != nil || v.PtrVar != "" {
		t.Fatal("expected null pointer (nil env, empty var)")
	}
}

func TestZeroFromTypeSpec_ReferenceType(t *testing.T) {
	ts := &ast.ReferenceType{BaseType: &ast.NamedType{Name: ident("INT")}}
	v := zeroFromTypeSpec(ts)
	if v.Kind != ValReference {
		t.Fatalf("expected ValReference, got %v", v.Kind)
	}
}

func TestZeroFromTypeSpec_SubrangeType(t *testing.T) {
	ts := &ast.SubrangeType{
		BaseType: &ast.NamedType{Name: ident("INT")},
		Low:      intLit("0"),
		High:     intLit("100"),
	}
	v := zeroFromTypeSpec(ts)
	// Should use base type's zero
	if v.Kind != ValInt {
		t.Fatalf("expected ValInt, got %v", v.Kind)
	}
}

func TestZeroFromTypeSpec_StructType(t *testing.T) {
	ts := &ast.StructType{
		Members: []*ast.StructMember{
			{Name: ident("a"), Type: &ast.NamedType{Name: ident("REAL")}},
		},
	}
	v := zeroFromTypeSpec(ts)
	if v.Kind != ValStruct {
		t.Fatalf("expected ValStruct, got %v", v.Kind)
	}
}

func TestZeroFromTypeSpec_StringType(t *testing.T) {
	ts := &ast.StringType{}
	v := zeroFromTypeSpec(ts)
	if v.Kind != ValString {
		t.Fatalf("expected ValString, got %v", v.Kind)
	}
}

func TestZeroFromTypeSpec_NilTypeSpec(t *testing.T) {
	// nil falls to default
	v := zeroFromTypeSpec(nil)
	if v.Kind != ValInt {
		t.Fatalf("expected ValInt (default), got %v", v.Kind)
	}
}

// ---------------------------------------------------------------------------
// 4. evalMethodCall / findMethod / findProperty / execPropertyGetter / execPropertySetter
// ---------------------------------------------------------------------------

// helper: make an FB instance with a simple method
func makeFBWithMethod(methodName string, retType ast.TypeSpec, body []ast.Statement) *FBInstance {
	decl := &ast.FunctionBlockDecl{
		Name: ident("TestFB"),
		Methods: []*ast.MethodDecl{
			{
				Name:       ident(methodName),
				ReturnType: retType,
				Body:       body,
			},
		},
	}
	env := NewEnv(nil)
	return &FBInstance{
		TypeName: "TestFB",
		Decl:     decl,
		Env:      env,
	}
}

func TestEvalMethodCall(t *testing.T) {
	interp := New()
	env := NewEnv(nil)

	// Create FB with method GetValue that returns 42
	fbInst := makeFBWithMethod("GetValue",
		&ast.NamedType{Name: ident("INT")},
		[]ast.Statement{
			&ast.AssignStmt{
				Target: ident("GetValue"),
				Value:  intLit("42"),
			},
		},
	)
	env.Define("fb", Value{Kind: ValFBInstance, FBRef: fbInst})

	// Call fb.GetValue()
	memberAccess := &ast.MemberAccessExpr{
		Object: ident("fb"),
		Member: ident("GetValue"),
	}
	v, err := interp.evalMethodCall(env, memberAccess, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != 42 {
		t.Fatalf("expected 42, got %d", v.Int)
	}
}

func TestEvalMethodCall_WithArgs(t *testing.T) {
	interp := New()
	env := NewEnv(nil)

	// Method Add(a, b : INT) : INT
	fbDecl := &ast.FunctionBlockDecl{
		Name: ident("MathFB"),
		Methods: []*ast.MethodDecl{
			{
				Name:       ident("Add"),
				ReturnType: &ast.NamedType{Name: ident("INT")},
				VarBlocks: []*ast.VarBlock{
					{
						Section: ast.VarInput,
						Declarations: []*ast.VarDecl{
							{
								Names: []*ast.Ident{ident("a"), ident("b")},
								Type:  &ast.NamedType{Name: ident("INT")},
							},
						},
					},
				},
				Body: []ast.Statement{
					&ast.AssignStmt{
						Target: ident("Add"),
						Value:  binExpr(ident("a"), "+", ident("b")),
					},
				},
			},
		},
	}
	fbEnv := NewEnv(nil)
	fbInst := &FBInstance{TypeName: "MathFB", Decl: fbDecl, Env: fbEnv}
	env.Define("fb", Value{Kind: ValFBInstance, FBRef: fbInst})

	memberAccess := &ast.MemberAccessExpr{
		Object: ident("fb"),
		Member: ident("Add"),
	}
	v, err := interp.evalMethodCall(env, memberAccess, []ast.Expr{intLit("3"), intLit("7")})
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != 10 {
		t.Fatalf("expected 10, got %d", v.Int)
	}
}

func TestEvalMethodCall_NotFB(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(5))

	memberAccess := &ast.MemberAccessExpr{
		Object: ident("x"),
		Member: ident("Foo"),
	}
	_, err := interp.evalMethodCall(env, memberAccess, nil)
	if err == nil {
		t.Fatal("expected error calling method on non-FB")
	}
}

func TestEvalMethodCall_MethodNotFound(t *testing.T) {
	interp := New()
	env := NewEnv(nil)

	fbInst := makeFBWithMethod("GetValue", &ast.NamedType{Name: ident("INT")}, nil)
	env.Define("fb", Value{Kind: ValFBInstance, FBRef: fbInst})

	memberAccess := &ast.MemberAccessExpr{
		Object: ident("fb"),
		Member: ident("NonExistent"),
	}
	_, err := interp.evalMethodCall(env, memberAccess, nil)
	if err == nil {
		t.Fatal("expected error for undefined method")
	}
}

func TestFindMethod_NilDecl(t *testing.T) {
	inst := &FBInstance{TypeName: "T", Decl: nil}
	if m := findMethod(inst, "Foo"); m != nil {
		t.Fatal("expected nil for nil decl")
	}
}

func TestFindMethod_InheritsFromParent(t *testing.T) {
	parentDecl := &ast.FunctionBlockDecl{
		Name: ident("Parent"),
		Methods: []*ast.MethodDecl{
			{Name: ident("BaseMethod"), ReturnType: &ast.NamedType{Name: ident("INT")}},
		},
	}
	childDecl := &ast.FunctionBlockDecl{
		Name:    ident("Child"),
		Extends: ident("Parent"),
	}
	inst := &FBInstance{
		TypeName:   "Child",
		Decl:       childDecl,
		ParentDecl: parentDecl,
		Env:        NewEnv(nil),
	}
	m := findMethod(inst, "BaseMethod")
	if m == nil {
		t.Fatal("expected to find inherited method")
	}
	if m.Name.Name != "BaseMethod" {
		t.Fatalf("wrong method name: %s", m.Name.Name)
	}
}

func TestFindProperty(t *testing.T) {
	decl := &ast.FunctionBlockDecl{
		Name: ident("MyFB"),
		Properties: []*ast.PropertyDecl{
			{
				Name: ident("Count"),
				Type: &ast.NamedType{Name: ident("INT")},
			},
		},
	}
	inst := &FBInstance{TypeName: "MyFB", Decl: decl, Env: NewEnv(nil)}

	p := findProperty(inst, "Count")
	if p == nil {
		t.Fatal("expected to find Count property")
	}
	p = findProperty(inst, "NonExist")
	if p != nil {
		t.Fatal("expected nil for non-existent property")
	}
}

func TestFindProperty_NilDecl(t *testing.T) {
	inst := &FBInstance{TypeName: "T", Decl: nil}
	if p := findProperty(inst, "Foo"); p != nil {
		t.Fatal("expected nil for nil decl")
	}
}

func TestFindProperty_InheritsFromParent(t *testing.T) {
	parentDecl := &ast.FunctionBlockDecl{
		Name: ident("Parent"),
		Properties: []*ast.PropertyDecl{
			{Name: ident("BaseProp"), Type: &ast.NamedType{Name: ident("INT")}},
		},
	}
	childDecl := &ast.FunctionBlockDecl{
		Name:    ident("Child"),
		Extends: ident("Parent"),
	}
	inst := &FBInstance{
		TypeName:   "Child",
		Decl:       childDecl,
		ParentDecl: parentDecl,
		Env:        NewEnv(nil),
	}
	p := findProperty(inst, "BaseProp")
	if p == nil {
		t.Fatal("expected to find inherited property")
	}
}

func TestExecPropertyGetter(t *testing.T) {
	interp := New()
	fbEnv := NewEnv(nil)
	fbEnv.Define("internalCount", IntValue(42))

	prop := &ast.PropertyDecl{
		Name: ident("Count"),
		Type: &ast.NamedType{Name: ident("INT")},
		Getter: &ast.MethodDecl{
			Name:       ident("GET"),
			ReturnType: &ast.NamedType{Name: ident("INT")},
			Body: []ast.Statement{
				&ast.AssignStmt{
					Target: ident("GET"),
					Value:  ident("internalCount"),
				},
			},
		},
	}
	inst := &FBInstance{TypeName: "MyFB", Decl: &ast.FunctionBlockDecl{Name: ident("MyFB")}, Env: fbEnv}

	v, err := interp.execPropertyGetter(inst, prop)
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != 42 {
		t.Fatalf("expected 42, got %d", v.Int)
	}
}

func TestExecPropertySetter(t *testing.T) {
	interp := New()
	fbEnv := NewEnv(nil)
	fbEnv.Define("internalCount", IntValue(0))

	prop := &ast.PropertyDecl{
		Name: ident("Count"),
		Type: &ast.NamedType{Name: ident("INT")},
		Setter: &ast.MethodDecl{
			Name: ident("SET"),
			VarBlocks: []*ast.VarBlock{
				{
					Section: ast.VarInput,
					Declarations: []*ast.VarDecl{
						{
							Names: []*ast.Ident{ident("val")},
							Type:  &ast.NamedType{Name: ident("INT")},
						},
					},
				},
			},
			Body: []ast.Statement{
				&ast.AssignStmt{
					Target: ident("internalCount"),
					Value:  ident("Count"),
				},
			},
		},
	}
	inst := &FBInstance{TypeName: "MyFB", Decl: &ast.FunctionBlockDecl{Name: ident("MyFB")}, Env: fbEnv}

	err := interp.execPropertySetter(inst, prop, IntValue(99))
	if err != nil {
		t.Fatal(err)
	}
	v, _ := fbEnv.Get("internalCount")
	if v.Int != 99 {
		t.Fatalf("expected internalCount=99, got %d", v.Int)
	}
}

// Test property getter via evalMemberAccess
func TestEvalMemberAccess_PropertyGetter(t *testing.T) {
	interp := New()
	env := NewEnv(nil)

	fbEnv := NewEnv(nil)
	fbEnv.Define("stored", IntValue(77))

	decl := &ast.FunctionBlockDecl{
		Name: ident("PropFB"),
		Properties: []*ast.PropertyDecl{
			{
				Name: ident("Value"),
				Type: &ast.NamedType{Name: ident("INT")},
				Getter: &ast.MethodDecl{
					Name: ident("GET"),
					Body: []ast.Statement{
						&ast.AssignStmt{
							Target: ident("GET"),
							Value:  ident("stored"),
						},
					},
				},
			},
		},
	}
	inst := &FBInstance{TypeName: "PropFB", Decl: decl, Env: fbEnv}
	env.Define("fb", Value{Kind: ValFBInstance, FBRef: inst})

	memberExpr := &ast.MemberAccessExpr{
		Object: ident("fb"),
		Member: ident("Value"),
	}
	v, err := interp.evalMemberAccess(env, memberExpr)
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != 77 {
		t.Fatalf("expected 77, got %d", v.Int)
	}
}

// Test property setter via execAssignMember
func TestExecAssignMember_PropertySetter(t *testing.T) {
	interp := New()
	env := NewEnv(nil)

	fbEnv := NewEnv(nil)
	fbEnv.Define("stored", IntValue(0))

	decl := &ast.FunctionBlockDecl{
		Name: ident("PropFB"),
		Properties: []*ast.PropertyDecl{
			{
				Name: ident("Value"),
				Type: &ast.NamedType{Name: ident("INT")},
				Setter: &ast.MethodDecl{
					Name: ident("SET"),
					Body: []ast.Statement{
						&ast.AssignStmt{
							Target: ident("stored"),
							Value:  ident("Value"),
						},
					},
				},
			},
		},
	}
	inst := &FBInstance{TypeName: "PropFB", Decl: decl, Env: fbEnv}
	env.Define("fb", Value{Kind: ValFBInstance, FBRef: inst})

	target := &ast.MemberAccessExpr{
		Object: ident("fb"),
		Member: ident("Value"),
	}
	err := interp.execAssignMember(env, target, IntValue(55))
	if err != nil {
		t.Fatal(err)
	}
	v, _ := fbEnv.Get("stored")
	if v.Int != 55 {
		t.Fatalf("expected stored=55, got %d", v.Int)
	}
}

// ---------------------------------------------------------------------------
// 5. ADR / REF builtins in evalCall
// ---------------------------------------------------------------------------

func TestEvalCall_ADR(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("myVar", IntValue(10))

	callExpr := &ast.CallExpr{
		Callee: ident("ADR"),
		Args:   []ast.Expr{ident("myVar")},
	}
	v, err := interp.evalCall(env, callExpr)
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != ValPointer {
		t.Fatalf("expected ValPointer, got %v", v.Kind)
	}
	if v.PtrVar != "MYVAR" {
		t.Fatalf("expected PtrVar=MYVAR, got %s", v.PtrVar)
	}
	if v.PtrEnv != env {
		t.Fatal("expected PtrEnv to be the current env")
	}
}

func TestEvalCall_ADR_Errors(t *testing.T) {
	interp := New()
	env := NewEnv(nil)

	// Wrong arg count
	_, err := interp.evalCall(env, &ast.CallExpr{
		Callee: ident("ADR"),
		Args:   []ast.Expr{},
	})
	if err == nil {
		t.Fatal("expected error for 0 args")
	}

	// Non-ident arg
	_, err = interp.evalCall(env, &ast.CallExpr{
		Callee: ident("ADR"),
		Args:   []ast.Expr{intLit("5")},
	})
	if err == nil {
		t.Fatal("expected error for non-ident arg")
	}

	// Undefined variable
	_, err = interp.evalCall(env, &ast.CallExpr{
		Callee: ident("ADR"),
		Args:   []ast.Expr{ident("undefined")},
	})
	if err == nil {
		t.Fatal("expected error for undefined variable")
	}
}

func TestEvalCall_REF(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("myVar", IntValue(20))

	callExpr := &ast.CallExpr{
		Callee: ident("REF"),
		Args:   []ast.Expr{ident("myVar")},
	}
	v, err := interp.evalCall(env, callExpr)
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != ValReference {
		t.Fatalf("expected ValReference, got %v", v.Kind)
	}
	if v.PtrVar != "MYVAR" {
		t.Fatalf("expected PtrVar=MYVAR, got %s", v.PtrVar)
	}
}

func TestEvalCall_REF_Errors(t *testing.T) {
	interp := New()
	env := NewEnv(nil)

	// Wrong arg count
	_, err := interp.evalCall(env, &ast.CallExpr{
		Callee: ident("REF"),
		Args:   []ast.Expr{},
	})
	if err == nil {
		t.Fatal("expected error for 0 args")
	}

	// Non-ident arg
	_, err = interp.evalCall(env, &ast.CallExpr{
		Callee: ident("REF"),
		Args:   []ast.Expr{intLit("5")},
	})
	if err == nil {
		t.Fatal("expected error for non-ident arg")
	}

	// Undefined variable
	_, err = interp.evalCall(env, &ast.CallExpr{
		Callee: ident("REF"),
		Args:   []ast.Expr{ident("undefined")},
	})
	if err == nil {
		t.Fatal("expected error for undefined variable")
	}
}

// Test ADR in parent scope (uses FindOwner)
func TestEvalCall_ADR_ParentScope(t *testing.T) {
	interp := New()
	parent := NewEnv(nil)
	parent.Define("x", IntValue(10))
	child := NewEnv(parent)

	callExpr := &ast.CallExpr{
		Callee: ident("ADR"),
		Args:   []ast.Expr{ident("x")},
	}
	v, err := interp.evalCall(child, callExpr)
	if err != nil {
		t.Fatal(err)
	}
	if v.PtrEnv != parent {
		t.Fatal("expected PtrEnv to be the parent env")
	}
}

// ---------------------------------------------------------------------------
// 6. evalDeref / execAssignDeref
// ---------------------------------------------------------------------------

func TestEvalDeref(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("target", IntValue(42))

	// Create a pointer to target
	ptr := Value{Kind: ValPointer, PtrEnv: env, PtrVar: "TARGET"}
	env.Define("p", ptr)

	derefExpr := &ast.DerefExpr{
		Operand: ident("p"),
	}
	v, err := interp.evalDeref(env, derefExpr)
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != 42 {
		t.Fatalf("expected 42, got %d", v.Int)
	}
}

func TestEvalDeref_NonPointer(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(5))

	derefExpr := &ast.DerefExpr{Operand: ident("x")}
	_, err := interp.evalDeref(env, derefExpr)
	if err == nil {
		t.Fatal("expected error for non-pointer deref")
	}
	if !strings.Contains(err.Error(), "cannot dereference") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvalDeref_NilPointer(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	// Null pointer (no env/var set)
	env.Define("p", Value{Kind: ValPointer})

	derefExpr := &ast.DerefExpr{Operand: ident("p")}
	_, err := interp.evalDeref(env, derefExpr)
	if err == nil {
		t.Fatal("expected nil pointer dereference error")
	}
}

func TestEvalDeref_DanglingPointer(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	// Pointer to a variable that doesn't exist in the target env
	targetEnv := NewEnv(nil)
	env.Define("p", Value{Kind: ValPointer, PtrEnv: targetEnv, PtrVar: "GONE"})

	derefExpr := &ast.DerefExpr{Operand: ident("p")}
	_, err := interp.evalDeref(env, derefExpr)
	if err == nil {
		t.Fatal("expected dangling pointer error")
	}
}

func TestExecAssignDeref(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("target", IntValue(0))
	env.Define("p", Value{Kind: ValPointer, PtrEnv: env, PtrVar: "TARGET"})

	target := &ast.DerefExpr{Operand: ident("p")}
	err := interp.execAssignDeref(env, target, IntValue(99))
	if err != nil {
		t.Fatal(err)
	}
	v, _ := env.Get("target")
	if v.Int != 99 {
		t.Fatalf("expected target=99, got %d", v.Int)
	}
}

func TestExecAssignDeref_NonPointer(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(5))

	target := &ast.DerefExpr{Operand: ident("x")}
	err := interp.execAssignDeref(env, target, IntValue(10))
	if err == nil {
		t.Fatal("expected error for non-pointer assign deref")
	}
}

func TestExecAssignDeref_NilPointer(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("p", Value{Kind: ValPointer})

	target := &ast.DerefExpr{Operand: ident("p")}
	err := interp.execAssignDeref(env, target, IntValue(10))
	if err == nil {
		t.Fatal("expected nil pointer dereference error")
	}
}

func TestExecAssignDeref_DanglingPointer(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	targetEnv := NewEnv(nil) // empty env, no variable "GONE"
	env.Define("p", Value{Kind: ValPointer, PtrEnv: targetEnv, PtrVar: "GONE"})

	target := &ast.DerefExpr{Operand: ident("p")}
	err := interp.execAssignDeref(env, target, IntValue(10))
	if err == nil {
		t.Fatal("expected dangling pointer error")
	}
}

// Test assign through deref via execAssign (full path)
func TestExecAssign_DerefPath(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("target", IntValue(0))
	env.Define("p", Value{Kind: ValPointer, PtrEnv: env, PtrVar: "TARGET"})

	stmt := &ast.AssignStmt{
		Target: &ast.DerefExpr{Operand: ident("p")},
		Value:  intLit("77"),
	}
	err := interp.execAssign(env, stmt)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := env.Get("target")
	if v.Int != 77 {
		t.Fatalf("expected target=77, got %d", v.Int)
	}
}

// ---------------------------------------------------------------------------
// 7. parseLitTyped enum resolution
// ---------------------------------------------------------------------------

func TestParseLitTyped_Enum(t *testing.T) {
	interp := New()
	interp.RegisterEnumType("Color", map[string]int64{"RED": 0, "GREEN": 1, "BLUE": 2})

	v, err := interp.parseLitTyped("Green", "Color")
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != ValInt || v.Int != 1 {
		t.Fatalf("expected int 1, got %v", v)
	}
}

func TestParseLitTyped_EnumUnknownValue(t *testing.T) {
	interp := New()
	interp.RegisterEnumType("Color", map[string]int64{"RED": 0})

	_, err := interp.parseLitTyped("PURPLE", "Color")
	if err == nil {
		t.Fatal("expected error for unknown enum value")
	}
}

func TestParseLitTyped_UnknownPrefix(t *testing.T) {
	interp := New()
	_, err := interp.parseLitTyped("something", "UNKNOWN_TYPE")
	if err == nil {
		t.Fatal("expected error for unknown type prefix")
	}
}

func TestParseLitTyped_BOOL(t *testing.T) {
	interp := New()
	v, err := interp.parseLitTyped("TRUE", "BOOL")
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != ValBool || !v.Bool {
		t.Fatalf("expected true, got %v", v)
	}
}

func TestParseLitTyped_REAL(t *testing.T) {
	interp := New()
	v, err := interp.parseLitTyped("3.14", "REAL")
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != ValReal {
		t.Fatalf("expected ValReal, got %v", v.Kind)
	}
	if v.IECType != types.KindREAL {
		t.Fatalf("expected IECType REAL, got %v", v.IECType)
	}
}

func TestParseLitTyped_LREAL(t *testing.T) {
	interp := New()
	v, err := interp.parseLitTyped("2.718", "LREAL")
	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != ValReal {
		t.Fatalf("expected ValReal, got %v", v.Kind)
	}
}

// Test the full path: evalLiteral -> parseLitTyped for typed enum literal
func TestEvalLiteral_TypedEnum(t *testing.T) {
	interp := New()
	interp.RegisterEnumType("Status", map[string]int64{"OK": 0, "ERROR": 1})

	lit := &ast.Literal{
		LitKind:    ast.LitTyped,
		Value:      "ERROR",
		TypePrefix: "Status",
	}
	v, err := interp.evalLiteral(lit)
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != 1 {
		t.Fatalf("expected 1, got %d", v.Int)
	}
}

// ---------------------------------------------------------------------------
// Additional coverage: evalIdent with REFERENCE auto-deref
// ---------------------------------------------------------------------------

func TestEvalIdent_ReferenceAutoDeref(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("target", IntValue(42))

	// Create a reference to target
	ref := Value{Kind: ValReference, PtrEnv: env, PtrVar: "TARGET"}
	env.Define("r", ref)

	v, err := interp.evalIdent(env, ident("r"))
	if err != nil {
		t.Fatal(err)
	}
	// Auto-deref should give us the target value
	if v.Int != 42 {
		t.Fatalf("expected 42, got %d", v.Int)
	}
}

func TestEvalIdent_DanglingReference(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	targetEnv := NewEnv(nil) // empty env
	ref := Value{Kind: ValReference, PtrEnv: targetEnv, PtrVar: "GONE"}
	env.Define("r", ref)

	_, err := interp.evalIdent(env, ident("r"))
	if err == nil {
		t.Fatal("expected dangling reference error")
	}
}

// ---------------------------------------------------------------------------
// Additional coverage: evalCall via method call (CallExpr with MemberAccessExpr callee)
// ---------------------------------------------------------------------------

func TestEvalCall_MethodDispatch(t *testing.T) {
	interp := New()
	env := NewEnv(nil)

	fbInst := makeFBWithMethod("DoSomething",
		&ast.NamedType{Name: ident("INT")},
		[]ast.Statement{
			&ast.AssignStmt{
				Target: ident("DoSomething"),
				Value:  intLit("100"),
			},
		},
	)
	env.Define("fb", Value{Kind: ValFBInstance, FBRef: fbInst})

	callExpr := &ast.CallExpr{
		Callee: &ast.MemberAccessExpr{
			Object: ident("fb"),
			Member: ident("DoSomething"),
		},
	}
	v, err := interp.evalCall(env, callExpr)
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != 100 {
		t.Fatalf("expected 100, got %d", v.Int)
	}
}

// ---------------------------------------------------------------------------
// Additional coverage: Value.String for Pointer/Reference kinds
// ---------------------------------------------------------------------------

func TestValueString_PointerAndReference(t *testing.T) {
	p := Value{Kind: ValPointer, PtrVar: "X"}
	if s := p.String(); s != "PTR(X)" {
		t.Fatalf("expected PTR(X), got %s", s)
	}
	r := Value{Kind: ValReference, PtrVar: "Y"}
	if s := r.String(); s != "REF(Y)" {
		t.Fatalf("expected REF(Y), got %s", s)
	}
}

// ---------------------------------------------------------------------------
// Additional: execAssign with reference write-through and REF reassignment
// ---------------------------------------------------------------------------

func TestExecAssign_ReferenceWriteThrough(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("target", IntValue(10))
	ref := Value{Kind: ValReference, PtrEnv: env, PtrVar: "TARGET"}
	env.Define("r", ref)

	// Assign through reference: r := 50 should write to target
	stmt := &ast.AssignStmt{
		Target: ident("r"),
		Value:  intLit("50"),
	}
	err := interp.execAssign(env, stmt)
	if err != nil {
		t.Fatal(err)
	}
	v, _ := env.Get("target")
	if v.Int != 50 {
		t.Fatalf("expected target=50, got %d", v.Int)
	}
}

func TestExecAssign_ReferenceReassignment(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("a", IntValue(1))
	env.Define("b", IntValue(2))

	// Create reference to a
	refA := Value{Kind: ValReference, PtrEnv: env, PtrVar: "A"}
	env.Define("r", refA)

	// Reassign reference: r := REF(b)
	stmt := &ast.AssignStmt{
		Target: ident("r"),
		Value:  &ast.CallExpr{Callee: ident("REF"), Args: []ast.Expr{ident("b")}},
	}
	err := interp.execAssign(env, stmt)
	if err != nil {
		t.Fatal(err)
	}
	// r should now point to b
	rVal, _ := env.Get("r")
	if rVal.Kind != ValReference || rVal.PtrVar != "B" {
		t.Fatalf("expected reference to B, got %v", rVal)
	}
}

// ---------------------------------------------------------------------------
// Additional: subrange check in execAssign
// ---------------------------------------------------------------------------

func TestExecAssign_SubrangeViolation(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("x", IntValue(5))
	env.DefineSubrange("x", 0, 10)

	stmt := &ast.AssignStmt{
		Target: ident("x"),
		Value:  intLit("11"),
	}
	err := interp.execAssign(env, stmt)
	if err == nil {
		t.Fatal("expected subrange violation error")
	}
	if !strings.Contains(err.Error(), "subrange") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Additional: evalExpr dispatching to DerefExpr and CallExpr
// ---------------------------------------------------------------------------

func TestEvalExpr_DerefExpr(t *testing.T) {
	interp := New()
	env := NewEnv(nil)
	env.Define("target", IntValue(33))
	env.Define("p", Value{Kind: ValPointer, PtrEnv: env, PtrVar: "TARGET"})

	v, err := interp.evalExpr(env, &ast.DerefExpr{Operand: ident("p")})
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != 33 {
		t.Fatalf("expected 33, got %d", v.Int)
	}
}

// ---------------------------------------------------------------------------
// Additional: method with VAR (non-input) locals
// ---------------------------------------------------------------------------

func TestEvalMethodCall_WithLocalVars(t *testing.T) {
	interp := New()
	env := NewEnv(nil)

	fbDecl := &ast.FunctionBlockDecl{
		Name: ident("FB1"),
		Methods: []*ast.MethodDecl{
			{
				Name:       ident("Compute"),
				ReturnType: &ast.NamedType{Name: ident("INT")},
				VarBlocks: []*ast.VarBlock{
					{
						Section: ast.VarLocal,
						Declarations: []*ast.VarDecl{
							{
								Names:     []*ast.Ident{ident("temp")},
								Type:      &ast.NamedType{Name: ident("INT")},
								InitValue: intLit("10"),
							},
						},
					},
				},
				Body: []ast.Statement{
					&ast.AssignStmt{
						Target: ident("Compute"),
						Value:  ident("temp"),
					},
				},
			},
		},
	}
	fbEnv := NewEnv(nil)
	fbInst := &FBInstance{TypeName: "FB1", Decl: fbDecl, Env: fbEnv}
	env.Define("fb", Value{Kind: ValFBInstance, FBRef: fbInst})

	memberAccess := &ast.MemberAccessExpr{
		Object: ident("fb"),
		Member: ident("Compute"),
	}
	v, err := interp.evalMethodCall(env, memberAccess, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != 10 {
		t.Fatalf("expected 10, got %d", v.Int)
	}
}

// ---------------------------------------------------------------------------
// Additional coverage: property getter with local vars
// ---------------------------------------------------------------------------

func TestExecPropertyGetter_WithLocalVars(t *testing.T) {
	interp := New()
	fbEnv := NewEnv(nil)
	fbEnv.Define("base", IntValue(100))

	prop := &ast.PropertyDecl{
		Name: ident("Total"),
		Type: &ast.NamedType{Name: ident("INT")},
		Getter: &ast.MethodDecl{
			Name:       ident("GET"),
			ReturnType: &ast.NamedType{Name: ident("INT")},
			VarBlocks: []*ast.VarBlock{
				{
					Section: ast.VarLocal,
					Declarations: []*ast.VarDecl{
						{
							Names:     []*ast.Ident{ident("offset")},
							Type:      &ast.NamedType{Name: ident("INT")},
							InitValue: intLit("5"),
						},
					},
				},
			},
			Body: []ast.Statement{
				&ast.AssignStmt{
					Target: ident("GET"),
					Value:  binExpr(ident("base"), "+", ident("offset")),
				},
			},
		},
	}
	inst := &FBInstance{TypeName: "MyFB", Decl: &ast.FunctionBlockDecl{Name: ident("MyFB")}, Env: fbEnv}

	v, err := interp.execPropertyGetter(inst, prop)
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != 105 {
		t.Fatalf("expected 105, got %d", v.Int)
	}
}

// property getter that returns via property name (not getter name)
func TestExecPropertyGetter_ReturnViaPropertyName(t *testing.T) {
	interp := New()
	fbEnv := NewEnv(nil)

	prop := &ast.PropertyDecl{
		Name: ident("Count"),
		Type: &ast.NamedType{Name: ident("INT")},
		Getter: &ast.MethodDecl{
			// No Name set (nil) - return via property name
			Body: []ast.Statement{
				&ast.AssignStmt{
					Target: ident("Count"),
					Value:  intLit("33"),
				},
			},
		},
	}
	inst := &FBInstance{TypeName: "MyFB", Decl: &ast.FunctionBlockDecl{Name: ident("MyFB")}, Env: fbEnv}

	v, err := interp.execPropertyGetter(inst, prop)
	if err != nil {
		t.Fatal(err)
	}
	if v.Int != 33 {
		t.Fatalf("expected 33, got %d", v.Int)
	}
}

// property setter with non-input local vars
func TestExecPropertySetter_WithLocalVars(t *testing.T) {
	interp := New()
	fbEnv := NewEnv(nil)
	fbEnv.Define("stored", IntValue(0))

	prop := &ast.PropertyDecl{
		Name: ident("Count"),
		Type: &ast.NamedType{Name: ident("INT")},
		Setter: &ast.MethodDecl{
			Name: ident("SET"),
			VarBlocks: []*ast.VarBlock{
				{
					Section: ast.VarInput,
					Declarations: []*ast.VarDecl{
						{
							Names: []*ast.Ident{ident("val")},
							Type:  &ast.NamedType{Name: ident("INT")},
						},
					},
				},
				{
					Section: ast.VarLocal,
					Declarations: []*ast.VarDecl{
						{
							Names:     []*ast.Ident{ident("temp")},
							Type:      &ast.NamedType{Name: ident("INT")},
							InitValue: intLit("1"),
						},
					},
				},
			},
			Body: []ast.Statement{
				// stored := Count + temp
				&ast.AssignStmt{
					Target: ident("stored"),
					Value:  binExpr(ident("Count"), "+", ident("temp")),
				},
			},
		},
	}
	inst := &FBInstance{TypeName: "MyFB", Decl: &ast.FunctionBlockDecl{Name: ident("MyFB")}, Env: fbEnv}

	err := interp.execPropertySetter(inst, prop, IntValue(50))
	if err != nil {
		t.Fatal(err)
	}
	v, _ := fbEnv.Get("stored")
	if v.Int != 51 {
		t.Fatalf("expected stored=51, got %d", v.Int)
	}
}

// ---------------------------------------------------------------------------
// Additional: evalConstInt edge cases (fb_instance.go)
// ---------------------------------------------------------------------------

func TestEvalConstInt_UnaryMinus(t *testing.T) {
	expr := &ast.UnaryExpr{
		Op:      ast.Token{Text: "-"},
		Operand: intLit("7"),
	}
	result := evalConstInt(expr)
	if result != -7 {
		t.Fatalf("expected -7, got %d", result)
	}
}

func TestEvalConstInt_NonLiteral(t *testing.T) {
	// Non-literal should return 0
	result := evalConstInt(ident("x"))
	if result != 0 {
		t.Fatalf("expected 0, got %d", result)
	}
}

func TestEvalConstInt_NegativeLiteral(t *testing.T) {
	// Literal with leading minus
	lit := intLit("-5")
	result := evalConstInt(lit)
	if result != -5 {
		t.Fatalf("expected -5, got %d", result)
	}
}

// ---------------------------------------------------------------------------
// Additional: zeroArray edge cases
// ---------------------------------------------------------------------------

func TestZeroArray_NoRanges(t *testing.T) {
	at := &ast.ArrayType{
		Ranges:      []*ast.SubrangeSpec{},
		ElementType: &ast.NamedType{Name: ident("INT")},
	}
	v := zeroArray(at)
	if v.Kind != ValArray || len(v.Array) != 0 {
		t.Fatalf("expected empty array, got %v", v)
	}
}

func TestZeroArray_LargeRange(t *testing.T) {
	// Safety cap at 10000
	at := &ast.ArrayType{
		Ranges: []*ast.SubrangeSpec{
			{Low: intLit("0"), High: intLit("20000")},
		},
		ElementType: &ast.NamedType{Name: ident("INT")},
	}
	v := zeroArray(at)
	if len(v.Array) != 10000 {
		t.Fatalf("expected 10000 (capped), got %d", len(v.Array))
	}
}

// ---------------------------------------------------------------------------
// Additional: NewUserFBInstance with PointerType/SubrangeType init
// ---------------------------------------------------------------------------

func TestNewUserFBInstance_PointerVar(t *testing.T) {
	decl := &ast.FunctionBlockDecl{
		Name: ident("PtrFB"),
		VarBlocks: []*ast.VarBlock{
			{
				Section: ast.VarLocal,
				Declarations: []*ast.VarDecl{
					{
						Names: []*ast.Ident{ident("p")},
						Type:  &ast.PointerType{BaseType: &ast.NamedType{Name: ident("INT")}},
					},
				},
			},
		},
	}
	inst := NewUserFBInstance("PtrFB", decl, nil, nil)
	v, ok := inst.Env.Get("p")
	if !ok {
		t.Fatal("expected p to be defined")
	}
	if v.Kind != ValPointer {
		t.Fatalf("expected ValPointer, got %v", v.Kind)
	}
}

func TestNewUserFBInstance_InitValue(t *testing.T) {
	interp := New()
	decl := &ast.FunctionBlockDecl{
		Name: ident("InitFB"),
		VarBlocks: []*ast.VarBlock{
			{
				Section: ast.VarInput,
				Declarations: []*ast.VarDecl{
					{
						Names:     []*ast.Ident{ident("x")},
						Type:      &ast.NamedType{Name: ident("INT")},
						InitValue: intLit("42"),
					},
				},
			},
			{
				Section: ast.VarOutput,
				Declarations: []*ast.VarDecl{
					{
						Names: []*ast.Ident{ident("y")},
						Type:  &ast.NamedType{Name: ident("INT")},
					},
				},
			},
		},
	}
	inst := NewUserFBInstance("InitFB", decl, interp, nil)
	v, _ := inst.Env.Get("x")
	if v.Int != 42 {
		t.Fatalf("expected x=42, got %d", v.Int)
	}
	// Check input/output name tracking
	if len(inst.inputNames) != 1 || inst.inputNames[0] != "X" {
		t.Fatalf("unexpected inputNames: %v", inst.inputNames)
	}
	if len(inst.outputNames) != 1 || inst.outputNames[0] != "Y" {
		t.Fatalf("unexpected outputNames: %v", inst.outputNames)
	}
}
