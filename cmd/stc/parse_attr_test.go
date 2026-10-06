package main

import (
	"encoding/json"
	"testing"
)

// findNodes walks decoded JSON and returns every object for which match is true.
func findNodes(v any, match func(map[string]any) bool) []map[string]any {
	var out []map[string]any
	switch x := v.(type) {
	case map[string]any:
		if match(x) {
			out = append(out, x)
		}
		for _, c := range x {
			out = append(out, findNodes(c, match)...)
		}
	case []any:
		for _, c := range x {
			out = append(out, findNodes(c, match)...)
		}
	}
	return out
}

// namedNode matches an object whose "name" is an Ident with the given name.
func namedNode(name string) func(map[string]any) bool {
	return func(m map[string]any) bool {
		n, ok := m["name"].(map[string]any)
		return ok && n["name"] == name
	}
}

func attrsOf(t *testing.T, node map[string]any) []map[string]any {
	t.Helper()
	raw, ok := node["attributes"].([]any)
	if !ok {
		t.Fatalf("node has no attributes: %v", node)
	}
	var out []map[string]any
	for _, a := range raw {
		out = append(out, a.(map[string]any))
	}
	return out
}

func parseJSON(t *testing.T, path string) any {
	t.Helper()
	stdout, stderr, _ := runStc(t, "parse", "--format", "json", path)
	var doc any
	if err := json.Unmarshal([]byte(stdout), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	return doc
}

func TestParseJSONAttributes(t *testing.T) {
	t.Run("struct member", func(t *testing.T) {
		doc := parseJSON(t, "../../tests/twincat_probes/structpragma.st")
		members := findNodes(doc, namedNode("I1"))
		if len(members) != 1 {
			t.Fatalf("expected one I1 node, got %d", len(members))
		}
		attrs := attrsOf(t, members[0])
		if len(attrs) != 1 || attrs[0]["name"] != "OPC.UA.DA.Access" || attrs[0]["value"] != "1" {
			t.Fatalf("unexpected attributes on I1: %v", attrs)
		}
		if attrs[0]["kind"] != "Attribute" {
			t.Errorf("attribute kind = %v, want Attribute", attrs[0]["kind"])
		}
		for _, other := range findNodes(doc, namedNode("I2")) {
			if _, has := other["attributes"]; has {
				t.Errorf("I2 should carry no attributes: %v", other)
			}
		}
	})

	t.Run("struct member with AT address", func(t *testing.T) {
		doc := parseJSON(t, "../../tests/twincat_probes/structat.st")
		members := findNodes(doc, namedNode("I1"))
		if len(members) == 0 {
			t.Fatal("no I1 node in structat.st JSON")
		}
		attrs := attrsOf(t, members[0])
		if attrs[0]["name"] != "OPC.UA.DA.Access" {
			t.Fatalf("unexpected attributes on I1: %v", attrs)
		}
	})

	t.Run("enum value and type", func(t *testing.T) {
		doc := parseJSON(t, "../../tests/twincat_probes/enum_attr.st")
		values := findNodes(doc, namedNode("rdy"))
		if len(values) != 1 {
			t.Fatalf("expected one rdy node, got %d", len(values))
		}
		attrs := attrsOf(t, values[0])
		if len(attrs) != 1 || attrs[0]["name"] != "OPC.UA.DA.Description" || attrs[0]["value"] != "Ready" {
			t.Fatalf("unexpected attributes on rdy: %v", attrs)
		}
		types := findNodes(doc, namedNode("E_State"))
		if len(types) != 1 {
			t.Fatalf("expected one E_State node, got %d", len(types))
		}
		tattrs := attrsOf(t, types[0])
		if len(tattrs) != 2 || tattrs[0]["name"] != "qualified_only" || tattrs[1]["name"] != "strict" {
			t.Fatalf("unexpected attributes on E_State: %v", tattrs)
		}
		if _, has := tattrs[0]["value"]; has {
			t.Errorf("valueless attribute should have no value key: %v", tattrs[0])
		}
	})
}
