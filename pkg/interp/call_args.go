package interp

import (
	"fmt"
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
)

// paramSlot is one VAR_INPUT or VAR_IN_OUT parameter of a callee, in
// declaration order. Positional arguments bind to slots by index.
type paramSlot struct {
	name  string
	inOut bool
}

// outBinding is a `param => target` argument, written after the call returns.
type outBinding struct {
	param  string
	target ast.Expr
}

// inoutBinding is a VAR_IN_OUT argument whose value is copied back to target
// after the call returns.
type inoutBinding struct {
	param  string
	target ast.Expr
}

// boundCall is the result of binding a call's arguments to a callee's
// parameters.
type boundCall struct {
	inputs  map[string]Value // upper-case parameter name -> bound value
	outputs []outBinding
	inouts  []inoutBinding
}

// bindArgs binds the arguments of a FUNCTION or METHOD call to the
// parameters declared in blocks and defines every variable of blocks in
// callee, in declaration order.
//
// pos are the leading positional arguments; named holds the rest in source
// order. A positional entry (Name nil) binds the slot at its index in the
// full argument list, counting VAR_INPUT and VAR_IN_OUT parameters in
// declaration order (ruling A1). `name := v` binds an input or in-out by
// case-insensitive name, `name => target` binds an output. An empty argument
// (`name := ,`) leaves the parameter at its default. Arguments are evaluated
// in the caller's env; parameters without an argument and all other
// variables take their declared initial value, evaluated in callee, or the
// zero value of their type.
func (interp *Interpreter) bindArgs(env, callee *Env, blocks []*ast.VarBlock, pos []ast.Expr, named []*ast.CallArg, fname string) (*boundCall, error) {
	var slots []paramSlot
	outputs := map[string]string{}
	for _, vb := range blocks {
		for _, vd := range vb.Declarations {
			for _, n := range vd.Names {
				switch vb.Section {
				case ast.VarInput:
					slots = append(slots, paramSlot{name: n.Name})
				case ast.VarInOut:
					slots = append(slots, paramSlot{name: n.Name, inOut: true})
				case ast.VarOutput:
					outputs[strings.ToUpper(n.Name)] = n.Name
				}
			}
		}
	}

	all := make([]*ast.CallArg, 0, len(pos)+len(named))
	for _, e := range pos {
		all = append(all, &ast.CallArg{Value: e})
	}
	all = append(all, named...)

	bc := &boundCall{inputs: map[string]Value{}}
	seen := map[string]bool{}
	bindOnce := func(param string) error {
		key := strings.ToUpper(param)
		if seen[key] {
			return &RuntimeError{Msg: fmt.Sprintf("parameter '%s' of %s is bound more than once", param, fname)}
		}
		seen[key] = true
		return nil
	}

	for i, a := range all {
		if a.IsOutput {
			param, ok := outputs[strings.ToUpper(a.Name.Name)]
			if !ok {
				return nil, &RuntimeError{Msg: fmt.Sprintf("%s has no output parameter '%s'", fname, a.Name.Name)}
			}
			if err := bindOnce(param); err != nil {
				return nil, err
			}
			if a.Value != nil {
				bc.outputs = append(bc.outputs, outBinding{param: param, target: a.Value})
			}
			continue
		}

		var slot paramSlot
		if a.Name == nil {
			if i >= len(slots) {
				return nil, &RuntimeError{Msg: fmt.Sprintf("too many arguments to %s: it has %d input parameter(s)", fname, len(slots))}
			}
			slot = slots[i]
		} else {
			found := false
			for _, s := range slots {
				if strings.EqualFold(s.name, a.Name.Name) {
					slot, found = s, true
					break
				}
			}
			if !found {
				return nil, &RuntimeError{Msg: fmt.Sprintf("%s has no input parameter '%s'", fname, a.Name.Name)}
			}
		}
		if err := bindOnce(slot.name); err != nil {
			return nil, err
		}
		if a.Value == nil {
			// Empty argument: the parameter keeps its default.
			continue
		}
		v, err := interp.evalExpr(env, a.Value)
		if err != nil {
			return nil, err
		}
		if v.IsAggregate() {
			v = v.Clone()
		}
		bc.inputs[strings.ToUpper(slot.name)] = v
		if slot.inOut && isAssignable(a.Value) {
			bc.inouts = append(bc.inouts, inoutBinding{param: slot.name, target: a.Value})
		}
	}

	resolve := interp.TypeResolverFunc()
	for _, vb := range blocks {
		isParam := vb.Section == ast.VarInput || vb.Section == ast.VarInOut
		for _, vd := range vb.Declarations {
			for _, n := range vd.Names {
				if v, ok := bc.inputs[strings.ToUpper(n.Name)]; ok && isParam {
					if v.Kind == ValInt {
						v = adoptEnumTag(zeroFromTypeSpecWith(vd.Type, resolve, 0), v)
					}
					callee.Define(n.Name, v)
					continue
				}
				callee.Define(n.Name, interp.initialValue(callee, vd, resolve))
			}
		}
	}
	return bc, nil
}

// initialValue is a declaration's initial value evaluated in env, or the zero
// value of its type when it has none or the initialiser cannot be evaluated
// yet (aggregate initialisers are applied at runtime from Phase 22 on).
func (interp *Interpreter) initialValue(env *Env, vd *ast.VarDecl, resolve TypeResolver) Value {
	if vd.InitValue != nil {
		if iv, err := interp.evalExpr(env, vd.InitValue); err == nil {
			if iv.IsAggregate() {
				iv = iv.Clone()
			}
			return iv
		}
	}
	return zeroFromTypeSpecWith(vd.Type, resolve, 0)
}

// writeBack copies VAR_IN_OUT values and `=>` outputs from callee to their
// targets in the caller's env once the call has returned.
func (interp *Interpreter) writeBack(env, callee *Env, bc *boundCall) error {
	for _, b := range bc.inouts {
		if err := interp.copyOut(env, callee, b.param, b.target); err != nil {
			return err
		}
	}
	for _, b := range bc.outputs {
		if err := interp.copyOut(env, callee, b.param, b.target); err != nil {
			return err
		}
	}
	return nil
}

func (interp *Interpreter) copyOut(env, callee *Env, param string, target ast.Expr) error {
	v, _ := callee.GetLocal(param)
	if v.IsAggregate() {
		v = v.Clone()
	}
	return interp.assignToTarget(env, target, v)
}

// positionalArgs evaluates the arguments of a call to a built-in or
// test-runner function, which take positional arguments only.
func (interp *Interpreter) positionalArgs(env *Env, e *ast.CallExpr, name string) ([]Value, error) {
	if len(e.NamedArgs) > 0 {
		return nil, &RuntimeError{
			Msg: fmt.Sprintf("built-in function %s does not accept named arguments", name),
			Pos: e.Span().Start,
		}
	}
	args := make([]Value, 0, len(e.Args))
	for _, argExpr := range e.Args {
		v, err := interp.evalExpr(env, argExpr)
		if err != nil {
			return nil, err
		}
		args = append(args, v)
	}
	return args, nil
}
