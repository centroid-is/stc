package opcua

import (
	"testing"

	"github.com/centroid-is/stc/pkg/ast"
)

func TestAttr(t *testing.T) {
	n := scalar("x", nil, at("opc.ua.da", "1"), at("OPC.UA.DA.Access", "1"), at("OPC.UA.DA", "0"))
	if v, ok := attr(n, "OPC.UA.DA"); !ok || v != "0" {
		t.Errorf("last occurrence: got %q %v, want \"0\" true", v, ok)
	}
	if v, ok := attr(n, "opc.ua.da.access"); !ok || v != "1" {
		t.Errorf("case-insensitive name: got %q %v", v, ok)
	}
	if _, ok := attr(n, "OPC.UA.DA.Description"); ok {
		t.Error("missing attribute reported present")
	}
	q := scalar("q", nil, at("OPC.UA.DA", "'1'"))
	if v, _ := attr(q, "OPC.UA.DA"); v != "1" {
		t.Errorf("leftover quotes not stripped: %q", v)
	}
	dq := scalar("dq", nil, ast.Attribute{Name: "OPC.UA.DA", Value: "1", HasValue: true, DoubleQuoted: true})
	if _, ok := attr(dq, "OPC.UA.DA"); ok {
		t.Error("double-quoted attribute name must be ignored")
	}
	if _, ok := attr(nil, "OPC.UA.DA"); ok {
		t.Error("nil node has no attributes")
	}
}

func TestUnescapeIEC(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Motor $'A$' $$5$N", "Motor 'A' $5\n"},
		{"a$nb$Lc$ld", "a\nb\nc\nd"},
		{"$R$r$T$t$P$p", "\r\r\t\t\f\f"},
		{"$41$4a$\"", "AJ\""},
		{"plain", "plain"},
		{"end$", "end$"},
		{"$q$", "$q$"},
		{"$4", "$4"},
		{"$4z", "$4z"},
	}
	for _, c := range cases {
		if got := unescapeIEC(c.in); got != c.want {
			t.Errorf("unescapeIEC(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
