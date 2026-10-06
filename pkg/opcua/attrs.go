package opcua

import (
	"strconv"
	"strings"
)

// TF6100 attribute names (IEC attribute names compare case-insensitively).
const (
	attrDA          = "OPC.UA.DA"
	attrAccess      = "OPC.UA.DA.Access"
	attrDescription = "OPC.UA.DA.Description"
	attrStructured  = "OPC.UA.DA.StructuredType"
)

// attr returns the value of the LAST attribute named name on n, so an
// instance declaration overrides the type-level attributes listed before
// it. Names written in double quotes are ignored, as TwinCAT does. One
// leftover pair of single quotes around the value is stripped.
func attr(n SymbolNode, name string) (string, bool) {
	if n == nil {
		return "", false
	}
	val, found := "", false
	for _, a := range n.Attributes() {
		if a.DoubleQuoted || !strings.EqualFold(a.Name, name) {
			continue
		}
		val, found = a.Value, true
	}
	if len(val) >= 2 && val[0] == '\'' && val[len(val)-1] == '\'' {
		val = val[1 : len(val)-1]
	}
	return val, found
}

// unescapeIEC resolves the IEC 61131-3 string escapes $$, $', $", $L/$l,
// $N/$n, $P/$p, $R/$r, $T/$t and $hh (two hex digits). An escape that is
// not one of these is kept verbatim.
func unescapeIEC(s string) string {
	if !strings.Contains(s, "$") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '$' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		switch n := s[i+1]; n {
		case '$', '\'', '"':
			b.WriteByte(n)
		case 'L', 'l', 'N', 'n':
			b.WriteByte('\n')
		case 'P', 'p':
			b.WriteByte('\f')
		case 'R', 'r':
			b.WriteByte('\r')
		case 'T', 't':
			b.WriteByte('\t')
		default:
			if i+2 < len(s) {
				if v, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
					b.WriteByte(byte(v))
					i += 2
					continue
				}
			}
			b.WriteByte(c)
			continue
		}
		i++
	}
	return b.String()
}
