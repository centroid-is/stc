package interp

import (
	"fmt"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
)

// RefStep is one step of a reference path: a struct member or an array index.
type RefStep struct {
	Member  string // upper-case member name when !IsIndex
	Index   int
	IsIndex bool
}

// RefPath is the target of a REFERENCE TO value bound with REF= or REF():
// the variable Var in Env, then Steps into its members and elements. Index
// expressions are evaluated once, when the path is built, so a later change
// of an index variable does not move the reference. Reads and writes walk
// the path from the root each time, so they always see the current value.
type RefPath struct {
	Env   *Env
	Var   string
	Steps []RefStep
}

// String renders the path as ST, e.g. S.M[2].
func (p *RefPath) String() string {
	var b strings.Builder
	b.WriteString(p.Var)
	for _, st := range p.Steps {
		if st.IsIndex {
			fmt.Fprintf(&b, "[%d]", st.Index)
		} else {
			b.WriteString("." + st.Member)
		}
	}
	return b.String()
}

// with returns a copy of p extended by step.
func (p *RefPath) with(step RefStep) *RefPath {
	steps := make([]RefStep, 0, len(p.Steps)+1)
	steps = append(steps, p.Steps...)
	return &RefPath{Env: p.Env, Var: p.Var, Steps: append(steps, step)}
}

// refPathOf returns the target of a bound reference value. A REF() of a
// plain variable keeps the older PtrEnv/PtrVar form.
func refPathOf(v Value) (*RefPath, bool) {
	if v.Kind != ValReference {
		return nil, false
	}
	if v.Ref != nil {
		return v.Ref, true
	}
	if v.PtrEnv != nil && v.PtrVar != "" {
		return &RefPath{Env: v.PtrEnv, Var: v.PtrVar}, true
	}
	return nil, false
}

// referenceTo returns the reference value for path p.
func referenceTo(p *RefPath) Value {
	if len(p.Steps) == 0 {
		return Value{Kind: ValReference, PtrEnv: p.Env, PtrVar: p.Var}
	}
	return Value{Kind: ValReference, Ref: p}
}

func danglingRef(p *RefPath) error {
	return &RuntimeError{Msg: fmt.Sprintf("dangling reference: %s no longer resolves", p)}
}

// stepInto follows one step from v.
func stepInto(v Value, st RefStep) (Value, bool) {
	if st.IsIndex {
		if v.Kind == ValArray && st.Index >= 0 && st.Index < len(v.Array) {
			return v.Array[st.Index], true
		}
		return Value{}, false
	}
	if v.Kind == ValStruct {
		f, ok := v.Struct[st.Member]
		return f, ok
	}
	return Value{}, false
}

// readRef returns the current value at p.
func readRef(p *RefPath) (Value, error) {
	v, ok := p.Env.GetLocal(p.Var)
	if !ok {
		return Value{}, danglingRef(p)
	}
	for _, st := range p.Steps {
		if v, ok = stepInto(v, st); !ok {
			return Value{}, danglingRef(p)
		}
	}
	return v, nil
}

// writeRef stores val at p. Arrays and structs share their backing store
// with the root variable, so setting the element in place updates the root,
// which is then written back to its owning env.
func writeRef(p *RefPath, val Value) error {
	root, ok := p.Env.GetLocal(p.Var)
	if !ok {
		return danglingRef(p)
	}
	if len(p.Steps) == 0 {
		p.Env.Set(p.Var, val)
		return nil
	}
	cur := root
	for _, st := range p.Steps[:len(p.Steps)-1] {
		if cur, ok = stepInto(cur, st); !ok {
			return danglingRef(p)
		}
	}
	last := p.Steps[len(p.Steps)-1]
	if _, ok := stepInto(cur, last); !ok {
		return danglingRef(p)
	}
	if last.IsIndex {
		cur.Array[last.Index] = val
	} else {
		cur.Struct[last.Member] = val
	}
	p.Env.Set(p.Var, root)
	return nil
}

// buildRefPath resolves the lvalue e to a reference path, evaluating index
// expressions now. A reference variable resolves to its own target, so
// `r2 REF= r;` binds r2 to what r refers to. A member of a user FB instance
// (inst.x, THIS^.x) roots the path in the instance env.
func (interp *Interpreter) buildRefPath(env *Env, e ast.Expr) (*RefPath, error) {
	switch x := e.(type) {
	case *ast.Ident:
		v, ok := env.Get(x.Name)
		if !ok {
			return nil, &RuntimeError{Msg: fmt.Sprintf("reference target '%s' is not a variable", x.Name), Pos: x.Span().Start}
		}
		if p, bound := refPathOf(v); bound {
			return p, nil
		}
		return &RefPath{Env: env.FindOwner(x.Name), Var: strings.ToUpper(x.Name)}, nil
	case *ast.ParenExpr:
		return interp.buildRefPath(env, x.Inner)
	case *ast.MemberAccessExpr:
		if g, gvl := interp.gvlRoot(env, x.Object); g != nil {
			if _, ok := g.GetLocal(x.Member.Name); !ok {
				return nil, gvlMemberError(gvl, x.Member)
			}
			return &RefPath{Env: g, Var: strings.ToUpper(x.Member.Name)}, nil
		}
		base, err := interp.buildRefPath(env, x.Object)
		var obj Value
		if err == nil {
			if obj, err = readRef(base); err != nil {
				return nil, err
			}
		} else if v, evErr := interp.evalExpr(env, x.Object); evErr == nil && v.Kind == ValFBInstance {
			// THIS^.x: the object is an FB instance, not a variable path.
			obj, err = v, nil
		} else {
			return nil, err
		}
		if obj.Kind == ValFBInstance {
			return fbMemberRoot(obj.FBRef, x.Member)
		}
		return base.with(RefStep{Member: strings.ToUpper(x.Member.Name)}), nil
	case *ast.IndexExpr:
		base, err := interp.buildRefPath(env, x.Object)
		if err != nil {
			return nil, err
		}
		idx, err := interp.evalExpr(env, x.Indices[0])
		if err != nil {
			return nil, err
		}
		if idx.Kind != ValInt {
			return nil, &RuntimeError{Msg: fmt.Sprintf("array index must be integer, got %s", idx.Kind), Pos: x.Span().Start}
		}
		return base.with(RefStep{IsIndex: true, Index: int(idx.Int)}), nil
	case *ast.DerefExpr:
		ptr, err := interp.evalExpr(env, x.Operand)
		if err != nil {
			return nil, err
		}
		if ptr.Kind != ValPointer || ptr.PtrEnv == nil || ptr.PtrVar == "" {
			return nil, &RuntimeError{Msg: fmt.Sprintf("cannot dereference %s", ptr.Kind), Pos: x.Span().Start}
		}
		return &RefPath{Env: ptr.PtrEnv, Var: ptr.PtrVar}, nil
	default:
		return nil, &RuntimeError{Msg: fmt.Sprintf("reference target is not a variable: %T", e), Pos: e.Span().Start}
	}
}

// fbMemberRoot roots a reference path at member of a user FB instance.
func fbMemberRoot(inst *FBInstance, member *ast.Ident) (*RefPath, error) {
	if inst == nil || inst.Env == nil {
		return nil, &RuntimeError{Msg: fmt.Sprintf("cannot reference member '%s' of a standard function block", member.Name), Pos: member.Span().Start}
	}
	if _, ok := inst.Env.GetLocal(member.Name); !ok {
		return nil, &RuntimeError{Msg: fmt.Sprintf("FB '%s' has no member '%s'", inst.TypeName, member.Name), Pos: member.Span().Start}
	}
	return &RefPath{Env: inst.Env, Var: strings.ToUpper(member.Name)}, nil
}

// refTo builds the reference value for the lvalue e and checks that its
// target exists now.
func (interp *Interpreter) refTo(env *Env, e ast.Expr) (Value, error) {
	p, err := interp.buildRefPath(env, e)
	if err != nil {
		return Value{}, err
	}
	if _, err := readRef(p); err != nil {
		return Value{}, &RuntimeError{Msg: fmt.Sprintf("reference target %s does not exist", p), Pos: e.Span().Start}
	}
	return referenceTo(p), nil
}

// execRefAssign executes `target REF= value;`: target is rebound to the
// lvalue value, without writing through its current target.
func (interp *Interpreter) execRefAssign(env *Env, s *ast.RefAssignStmt) error {
	ref, err := interp.refTo(env, s.Value)
	if err != nil {
		return err
	}
	if id, ok := s.Target.(*ast.Ident); ok {
		owner := env.FindOwner(id.Name)
		if owner == nil {
			return &RuntimeError{Msg: fmt.Sprintf("undefined variable: %s", id.Name), Pos: id.Span().Start}
		}
		owner.Define(id.Name, ref)
		return nil
	}
	return interp.assignToTarget(env, s.Target, ref)
}
