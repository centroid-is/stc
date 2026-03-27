package checker

import (
	"github.com/centroid-is/stc/pkg/types"
)

// maxCandidates limits candidate set size to prevent explosion per RESEARCH.md.
const maxCandidates = 16

// allConcreteForConstraint returns all concrete TypeKinds that satisfy
// the given generic constraint function.
func allConcreteForConstraint(constraint func(types.TypeKind) bool) []types.TypeKind {
	candidates := make([]types.TypeKind, 0, maxCandidates)
	for _, k := range []types.TypeKind{
		types.KindBOOL, types.KindBYTE, types.KindWORD, types.KindDWORD, types.KindLWORD,
		types.KindSINT, types.KindINT, types.KindDINT, types.KindLINT,
		types.KindUSINT, types.KindUINT, types.KindUDINT, types.KindULINT,
		types.KindREAL, types.KindLREAL,
		types.KindSTRING, types.KindWSTRING,
		types.KindTIME, types.KindDATE, types.KindDT, types.KindTOD,
		types.KindCHAR, types.KindWCHAR,
	} {
		if constraint(k) {
			candidates = append(candidates, k)
			if len(candidates) >= maxCandidates {
				break
			}
		}
	}
	return candidates
}

// ResolveCandidates resolves generic parameters for functions with ANY_*
// constraints. Given a function type and actual argument types, it finds
// a consistent type assignment that satisfies all generic constraints.
//
// Returns: (resolvedReturnType, resolvedParamTypes, success)
func ResolveCandidates(fn *types.FunctionType, argTypes []types.Type) (types.Type, []types.Type, bool) {
	if fn == nil || len(fn.Params) == 0 {
		return fn.ReturnType, nil, true
	}

	// For each parameter with a generic constraint, find candidate types
	// that match both the constraint AND are compatible with the actual arg.
	resolvedParams := make([]types.Type, len(fn.Params))
	var commonKind types.TypeKind
	hasGeneric := false

	for i, param := range fn.Params {
		if i >= len(argTypes) {
			break
		}

		argType := argTypes[i]
		if argType == nil || argType == types.Invalid {
			resolvedParams[i] = param.Type
			continue
		}

		if param.GenericConstraint != nil {
			hasGeneric = true
			// Check if the actual argument satisfies the constraint
			if !param.GenericConstraint(argType.Kind()) {
				return types.Invalid, nil, false
			}

			// Track the common type across all generic params
			if commonKind == types.KindInvalid {
				commonKind = argType.Kind()
			} else {
				ct, ok := types.CommonType(commonKind, argType.Kind())
				if !ok {
					return types.Invalid, nil, false
				}
				commonKind = ct
			}
			resolvedParams[i] = argType
		} else if param.Type == nil {
			// ANY parameter (no constraint) - accept anything
			resolvedParams[i] = argType
		} else {
			resolvedParams[i] = param.Type
		}
	}

	// Determine the return type
	retType := fn.ReturnType
	if hasGeneric && commonKind != types.KindInvalid {
		// For generic functions, the return type matches the resolved common type
		if retType == nil || retType == fn.ReturnType {
			retType = &types.PrimitiveType{Kind_: commonKind}
		}
	}
	if retType == nil {
		// For functions with ANY return type, use the first argument's type
		if len(argTypes) > 0 && argTypes[0] != nil {
			retType = argTypes[0]
		} else {
			retType = types.Invalid
		}
	}

	return retType, resolvedParams, true
}
