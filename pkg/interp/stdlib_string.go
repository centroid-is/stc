package interp

import (
	"strings"

	"github.com/centroid-is/stc/pkg/types"
)

func init() {
	registerStringFunctions()
}

func registerStringFunctions() {
	// LEN(IN) -> INT: length of string
	StdlibFunctions["LEN"] = func(args []Value) (Value, error) {
		if len(args) < 1 {
			return Value{}, &RuntimeError{Msg: "LEN requires 1 argument"}
		}
		return Value{Kind: ValInt, Int: int64(len(args[0].Str)), IECType: types.KindINT}, nil
	}

	// LEFT(IN, L) -> STRING: leftmost L characters
	StdlibFunctions["LEFT"] = func(args []Value) (Value, error) {
		if len(args) < 2 {
			return Value{}, &RuntimeError{Msg: "LEFT requires 2 arguments (IN, L)"}
		}
		s := args[0].Str
		l := int(args[1].Int)
		if l <= 0 {
			return StringValue(""), nil
		}
		if l > len(s) {
			l = len(s)
		}
		return StringValue(s[:l]), nil
	}

	// RIGHT(IN, L) -> STRING: rightmost L characters
	StdlibFunctions["RIGHT"] = func(args []Value) (Value, error) {
		if len(args) < 2 {
			return Value{}, &RuntimeError{Msg: "RIGHT requires 2 arguments (IN, L)"}
		}
		s := args[0].Str
		l := int(args[1].Int)
		if l <= 0 {
			return StringValue(""), nil
		}
		if l > len(s) {
			l = len(s)
		}
		return StringValue(s[len(s)-l:]), nil
	}

	// MID(IN, L, P) -> STRING: L characters starting at 1-based position P
	StdlibFunctions["MID"] = func(args []Value) (Value, error) {
		if len(args) < 3 {
			return Value{}, &RuntimeError{Msg: "MID requires 3 arguments (IN, L, P)"}
		}
		s := args[0].Str
		l := int(args[1].Int)
		p := int(args[2].Int) // 1-based

		// Edge cases
		if p < 1 || l <= 0 {
			return StringValue(""), nil
		}

		start := p - 1 // Convert to 0-based
		if start >= len(s) {
			return StringValue(""), nil
		}
		end := start + l
		if end > len(s) {
			end = len(s)
		}
		return StringValue(s[start:end]), nil
	}

	// CONCAT(IN1, IN2) -> STRING: concatenate two strings
	StdlibFunctions["CONCAT"] = func(args []Value) (Value, error) {
		if len(args) < 2 {
			return Value{}, &RuntimeError{Msg: "CONCAT requires 2 arguments (IN1, IN2)"}
		}
		return StringValue(args[0].Str + args[1].Str), nil
	}

	// FIND(IN1, IN2) -> INT: 1-based position of IN2 in IN1, 0 if not found
	StdlibFunctions["FIND"] = func(args []Value) (Value, error) {
		if len(args) < 2 {
			return Value{}, &RuntimeError{Msg: "FIND requires 2 arguments (IN1, IN2)"}
		}
		idx := strings.Index(args[0].Str, args[1].Str)
		if idx < 0 {
			return Value{Kind: ValInt, Int: 0, IECType: types.KindINT}, nil
		}
		// Convert 0-based to 1-based
		return Value{Kind: ValInt, Int: int64(idx + 1), IECType: types.KindINT}, nil
	}

	// INSERT(IN1, IN2, P) -> STRING: insert IN2 into IN1 at 1-based position P
	StdlibFunctions["INSERT"] = func(args []Value) (Value, error) {
		if len(args) < 3 {
			return Value{}, &RuntimeError{Msg: "INSERT requires 3 arguments (IN1, IN2, P)"}
		}
		s := args[0].Str
		ins := args[1].Str
		p := int(args[2].Int) // 1-based

		if p < 1 {
			return StringValue(s), nil
		}

		idx := p - 1 // Convert to 0-based
		if idx > len(s) {
			idx = len(s)
		}
		return StringValue(s[:idx] + ins + s[idx:]), nil
	}

	// DELETE(IN, L, P) -> STRING: delete L chars starting at 1-based position P
	StdlibFunctions["DELETE"] = func(args []Value) (Value, error) {
		if len(args) < 3 {
			return Value{}, &RuntimeError{Msg: "DELETE requires 3 arguments (IN, L, P)"}
		}
		s := args[0].Str
		l := int(args[1].Int)
		p := int(args[2].Int) // 1-based

		if p < 1 || l <= 0 {
			return StringValue(s), nil
		}

		idx := p - 1 // Convert to 0-based
		if idx >= len(s) {
			return StringValue(s), nil
		}
		end := idx + l
		if end > len(s) {
			end = len(s)
		}
		return StringValue(s[:idx] + s[end:]), nil
	}

	// REPLACE(IN1, IN2, L, P) -> STRING: replace L chars at 1-based position P with IN2
	StdlibFunctions["REPLACE"] = func(args []Value) (Value, error) {
		if len(args) < 4 {
			return Value{}, &RuntimeError{Msg: "REPLACE requires 4 arguments (IN1, IN2, L, P)"}
		}
		s := args[0].Str
		repl := args[1].Str
		l := int(args[2].Int)
		p := int(args[3].Int) // 1-based

		if p < 1 {
			return StringValue(s), nil
		}

		idx := p - 1 // Convert to 0-based
		if idx >= len(s) {
			return StringValue(s), nil
		}
		end := idx + l
		if end > len(s) {
			end = len(s)
		}
		return StringValue(s[:idx] + repl + s[end:]), nil
	}
}
