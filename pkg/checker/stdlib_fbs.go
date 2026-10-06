package checker

import (
	"strings"

	"github.com/centroid-is/stc/pkg/ast"
	"github.com/centroid-is/stc/pkg/source"
	"github.com/centroid-is/stc/pkg/symbols"
	"github.com/centroid-is/stc/pkg/types"
)

// stdFBSig is the checker signature of one IEC 61131-3 standard function
// block. Inputs lists the Tc2_Standard (CODESYS) names in declaration
// order; aliases lists the IEC names the interpreter implements (R, LD, S1,
// R1, S). Aliases are appended after the canonical inputs, so positional
// order is unchanged and named arguments accept both spellings.
//
// Tc2_Standard declares PV and CV of the counters as WORD; stc types them
// INT, matching the interpreter and the IEC standard (phase 20 ruling A2).
type stdFBSig struct {
	name    string
	inputs  []types.Parameter
	aliases []types.Parameter
	// aliasOf names, for each alias, the canonical input it sets.
	aliasOf []string
	outputs []types.Parameter
}

func stdParams(dir types.ParamDirection, pairs ...any) []types.Parameter {
	out := make([]types.Parameter, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, types.Parameter{Name: pairs[i].(string), Type: pairs[i+1].(types.Type), Direction: dir})
	}
	return out
}

func stdFBSigs() []stdFBSig {
	in := func(p ...any) []types.Parameter { return stdParams(types.DirInput, p...) }
	out := func(p ...any) []types.Parameter { return stdParams(types.DirOutput, p...) }
	b, t, i := types.TypeBOOL, types.TypeTIME, types.TypeINT
	timer := func(name string) stdFBSig {
		return stdFBSig{name: name, inputs: in("IN", b, "PT", t), outputs: out("Q", b, "ET", t)}
	}
	edge := func(name string) stdFBSig {
		return stdFBSig{name: name, inputs: in("CLK", b), outputs: out("Q", b)}
	}
	return []stdFBSig{
		timer("TON"), timer("TOF"), timer("TP"),
		{name: "CTU", inputs: in("CU", b, "RESET", b, "PV", i), aliases: in("R", b), aliasOf: []string{"RESET"},
			outputs: out("Q", b, "CV", i)},
		{name: "CTD", inputs: in("CD", b, "LOAD", b, "PV", i), aliases: in("LD", b), aliasOf: []string{"LOAD"},
			outputs: out("Q", b, "CV", i)},
		{name: "CTUD", inputs: in("CU", b, "CD", b, "RESET", b, "LOAD", b, "PV", i), aliases: in("R", b, "LD", b),
			aliasOf: []string{"RESET", "LOAD"}, outputs: out("QU", b, "QD", b, "CV", i)},
		edge("R_TRIG"), edge("F_TRIG"),
		{name: "SR", inputs: in("SET1", b, "RESET", b), aliases: in("S1", b, "R", b), aliasOf: []string{"SET1", "RESET"},
			outputs: out("Q1", b)},
		{name: "RS", inputs: in("SET", b, "RESET1", b), aliases: in("S", b, "R1", b), aliasOf: []string{"SET", "RESET1"},
			outputs: out("Q1", b)},
	}
}

// registerStdFBs inserts the ten standard FBs as library symbols with a POU
// scope holding their parameters, so instance calls and member access check
// like user FBs. Library stubs and user code may redeclare any of them; the
// redeclaration replaces the standard entry without a diagnostic (see
// isStdFB).
func (r *Resolver) registerStdFBs() {
	r.std = make(map[string]*symbols.Symbol)
	// No source position: the LSP treats an empty file as "no location".
	var pos source.Pos
	for _, sig := range stdFBSigs() {
		fb := &types.FunctionBlockType{
			Name:    sig.name,
			Inputs:  append(append([]types.Parameter{}, sig.inputs...), sig.aliases...),
			Outputs: sig.outputs,
		}
		if len(sig.aliases) > 0 {
			fb.ParamAliases = make(map[string]string, len(sig.aliases))
			for k, a := range sig.aliases {
				fb.ParamAliases[a.Name] = sig.aliasOf[k]
			}
		}
		scope := r.table.RegisterPOU(sig.name, symbols.KindFunctionBlock, pos)
		sym := r.table.LookupGlobal(sig.name)
		sym.Type = fb
		sym.IsLibrary = true
		r.std[sig.name] = sym
		for _, p := range fb.Inputs {
			_ = scope.Insert(&symbols.Symbol{Name: p.Name, Kind: symbols.KindVariable, Pos: pos, ParamDir: ast.VarInput, Type: p.Type})
		}
		for _, p := range fb.Outputs {
			_ = scope.Insert(&symbols.Symbol{Name: p.Name, Kind: symbols.KindVariable, Pos: pos, ParamDir: ast.VarOutput, Type: p.Type})
		}
	}
}

// isStdFB reports whether sym is a standard FB registered by registerStdFBs.
func (r *Resolver) isStdFB(sym *symbols.Symbol) bool {
	return sym != nil && r.std[strings.ToUpper(sym.Name)] == sym
}

// yieldStdFB removes the standard FB registered under name, if any, so a
// user or library global of any kind (GVL variable, enum value, POU, TYPE)
// takes the name without a redeclaration diagnostic. Standard FBs never
// shadow a declaration: they are the lowest-priority globals.
func (r *Resolver) yieldStdFB(name string) {
	if existing := r.table.GlobalScope().LookupLocal(name); r.isStdFB(existing) {
		r.table.RemovePOU(existing.Name)
		delete(r.std, strings.ToUpper(existing.Name))
	}
}
