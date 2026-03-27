package types

// wideningRules encodes the IEC 61131-3 implicit type widening rules.
// Each entry maps a source type to the list of types it can be implicitly widened to.
// These are IEC-strict rules only: no CODESYS permissive widening.
//
// Rules:
//   Signed integers: SINT -> INT -> DINT -> LINT
//   Unsigned integers: USINT -> UINT -> UDINT -> ULINT
//   Reals: REAL -> LREAL
//   Cross-category (precision-preserving only):
//     SINT/INT -> REAL, SINT/INT/DINT -> LREAL
//     USINT/UINT -> REAL, USINT/UINT/UDINT -> LREAL
//   Bit types: BYTE -> WORD -> DWORD -> LWORD
//
//   NOT allowed: signed<->unsigned, BIT<->INT, BOOL->anything, DATE/STRING/TIME conversions
var wideningRules = map[TypeKind][]TypeKind{
	// Signed integers
	KindSINT: {KindINT, KindDINT, KindLINT, KindREAL, KindLREAL},
	KindINT:  {KindDINT, KindLINT, KindREAL, KindLREAL},
	KindDINT: {KindLINT, KindLREAL},
	// KindLINT: no implicit widening (LINT->LREAL loses precision)

	// Unsigned integers
	KindUSINT: {KindUINT, KindUDINT, KindULINT, KindREAL, KindLREAL},
	KindUINT:  {KindUDINT, KindULINT, KindREAL, KindLREAL},
	KindUDINT: {KindULINT, KindLREAL},
	// KindULINT: no implicit widening (ULINT->LREAL loses precision)

	// Reals
	KindREAL: {KindLREAL},

	// Bit types
	KindBYTE:  {KindWORD, KindDWORD, KindLWORD},
	KindWORD:  {KindDWORD, KindLWORD},
	KindDWORD: {KindLWORD},
}

// CanWiden reports whether type 'from' can be implicitly widened to type 'to'
// according to IEC 61131-3 rules. Identity (from == to) is NOT considered widening.
func CanWiden(from, to TypeKind) bool {
	targets, ok := wideningRules[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}

// CommonType finds the smallest common supertype for two TypeKinds.
// Returns (kind, true) if compatible, or (KindInvalid, false) if incompatible.
func CommonType(a, b TypeKind) (TypeKind, bool) {
	if a == b {
		return a, true
	}

	// Direct widening check
	if CanWiden(a, b) {
		return b, true
	}
	if CanWiden(b, a) {
		return a, true
	}

	// Search for smallest common supertype:
	// Collect all types that 'a' can widen to, then find the first one 'b' can also widen to.
	aTargets := allWideningTargets(a)
	bTargets := allWideningTargets(b)

	// Find the smallest common target (first in a's list that is also in b's list,
	// since wideningRules are ordered from smallest to largest).
	for _, at := range aTargets {
		for _, bt := range bTargets {
			if at == bt {
				return at, true
			}
		}
	}

	return KindInvalid, false
}

// allWideningTargets returns all types that k can be widened to (direct targets only,
// which already encode the transitive closure in the wideningRules table).
func allWideningTargets(k TypeKind) []TypeKind {
	return wideningRules[k]
}

// --- Category membership functions ---
// These follow the IEC 61131-3 ANY type hierarchy.
// Note: Category membership is separate from widening.
// BOOL is in ANY_BIT but cannot be widened to BYTE.

// IsAnySigned reports whether k is in the ANY_SIGNED category (SINT, INT, DINT, LINT).
func IsAnySigned(k TypeKind) bool {
	return k >= KindSINT && k <= KindLINT
}

// IsAnyUnsigned reports whether k is in the ANY_UNSIGNED category (USINT, UINT, UDINT, ULINT).
func IsAnyUnsigned(k TypeKind) bool {
	return k >= KindUSINT && k <= KindULINT
}

// IsAnyInt reports whether k is in the ANY_INT category (ANY_SIGNED + ANY_UNSIGNED).
func IsAnyInt(k TypeKind) bool {
	return IsAnySigned(k) || IsAnyUnsigned(k)
}

// IsAnyReal reports whether k is in the ANY_REAL category (REAL, LREAL).
func IsAnyReal(k TypeKind) bool {
	return k == KindREAL || k == KindLREAL
}

// IsAnyNum reports whether k is in the ANY_NUM category (ANY_INT + ANY_REAL).
func IsAnyNum(k TypeKind) bool {
	return IsAnyInt(k) || IsAnyReal(k)
}

// IsAnyBit reports whether k is in the ANY_BIT category (BOOL, BYTE, WORD, DWORD, LWORD).
func IsAnyBit(k TypeKind) bool {
	return k >= KindBOOL && k <= KindLWORD
}

// IsAnyString reports whether k is in the ANY_STRING category (STRING, WSTRING).
func IsAnyString(k TypeKind) bool {
	return k == KindSTRING || k == KindWSTRING
}

// IsAnyDate reports whether k is in the ANY_DATE category (DATE, DT, TOD).
// Note: TIME is in ANY_MAGNITUDE but not ANY_DATE per IEC hierarchy.
func IsAnyDate(k TypeKind) bool {
	return k == KindDATE || k == KindDT || k == KindTOD
}

// IsAnyChar reports whether k is in the ANY_CHAR category (CHAR, WCHAR).
func IsAnyChar(k TypeKind) bool {
	return k == KindCHAR || k == KindWCHAR
}
