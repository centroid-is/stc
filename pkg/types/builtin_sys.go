package types

// convKinds are the BOOL, integer, bit-string and real kinds that have
// *_TO_* conversions between every pair.
var convKinds = []Type{
	TypeBOOL,
	TypeSINT, TypeINT, TypeDINT, TypeLINT,
	TypeUSINT, TypeUINT, TypeUDINT, TypeULINT,
	TypeBYTE, TypeWORD, TypeDWORD, TypeLWORD,
	TypeREAL, TypeLREAL,
}

// registerSystemBuiltins adds the CODESYS/TwinCAT system functions that
// production code uses beyond the IEC core: SIZEOF, ADR, the bit shifts and
// rotations, and every BOOL/integer/bit-string/real X_TO_Y conversion not
// already registered (UINT_TO_WORD, BOOL_TO_UINT, DWORD_TO_REAL, ...).
func registerSystemBuiltins() {
	// SIZEOF(any) -> UDINT: byte size of a variable (or type name, see the
	// checker).
	BuiltinFunctions["SIZEOF"] = &FunctionType{
		Name:       "SIZEOF",
		ReturnType: TypeUDINT,
		Params:     []Parameter{{Name: "IN", Direction: DirInput}},
	}
	// ADR(any) -> PVOID (POINTER TO BYTE in the Beckhoff stubs).
	BuiltinFunctions["ADR"] = &FunctionType{
		Name:       "ADR",
		ReturnType: &PointerType{BaseType: TypeBYTE},
		Params:     []Parameter{{Name: "IN", Direction: DirInput}},
	}
	// SHL/SHR/ROL/ROR(IN : ANY_BIT, N : ANY_INT) -> type of IN.
	for _, name := range []string{"SHL", "SHR", "ROL", "ROR"} {
		BuiltinFunctions[name] = &FunctionType{
			Name: name,
			Params: []Parameter{
				{Name: "IN", Type: TypeDWORD, Direction: DirInput, GenericConstraint: IsAnyBit},
				{Name: "N", Type: TypeINT, Direction: DirInput},
			},
		}
	}
	for _, from := range convKinds {
		for _, to := range convKinds {
			if from == to {
				continue
			}
			name := from.String() + "_TO_" + to.String()
			if _, ok := BuiltinFunctions[name]; ok {
				continue
			}
			BuiltinFunctions[name] = &FunctionType{
				Name:       name,
				ReturnType: to,
				Params:     []Parameter{{Name: "IN", Type: from, Direction: DirInput}},
			}
		}
	}
}
