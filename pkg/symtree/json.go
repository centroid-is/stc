package symtree

import (
	"bytes"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// jsonTree and jsonNode fix the field order of the JSON output; ordered
// data never goes through a Go map.
type jsonTree struct {
	Roots []*jsonNode `json:"roots"`
}

type jsonNode struct {
	Name        string      `json:"name"`
	Path        string      `json:"path"`
	Kind        string      `json:"kind"`
	Type        string      `json:"type,omitempty"`
	TypeName    string      `json:"type_name,omitempty"`
	Section     string      `json:"section,omitempty"`
	Constant    bool        `json:"constant,omitempty"`
	Retain      bool        `json:"retain,omitempty"`
	Persistent  bool        `json:"persistent,omitempty"`
	Pos         jsonPos     `json:"pos"`
	Attributes  []jsonAttr  `json:"attributes,omitempty"`
	EnumStrings enumStrings `json:"enum_strings,omitempty"`
	Low         *int        `json:"low,omitempty"`
	High        *int        `json:"high,omitempty"`
	Element     *jsonNode   `json:"element,omitempty"`
	Children    []*jsonNode `json:"children,omitempty"`
}

type jsonPos struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Col  int    `json:"col"`
}

type jsonAttr struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// enumStrings marshals as an object keyed by ordinal in ascending numeric
// order ({"-1":"c","2":"b","10":"a"}).
type enumStrings map[int64]string

func (e enumStrings) MarshalJSON() ([]byte, error) {
	keys := make([]int64, 0, len(e))
	for k := range e {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.Quote(strconv.FormatInt(k, 10)))
		b.WriteByte(':')
		v, _ := json.Marshal(e[k]) // a string always marshals
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// JSON renders the tree deterministically. An array emits its bounds and
// one element prototype labelled "[*]" instead of every element.
func (t *Tree) JSON() ([]byte, error) {
	out := jsonTree{Roots: make([]*jsonNode, 0, len(t.Roots))}
	for _, r := range t.Roots {
		out.Roots = append(out.Roots, toJSON(r, true))
	}
	return json.Marshal(out)
}

func toJSON(n *Node, root bool) *jsonNode {
	j := &jsonNode{
		Name: n.Name, Path: n.Path, Kind: n.Kind.String(), TypeName: n.TypeName,
		Constant: n.Constant, Retain: n.Retain, Persistent: n.Persistent,
		Pos: jsonPos{File: n.Pos.File, Line: n.Pos.Line, Col: n.Pos.Col},
	}
	if n.Type != nil {
		j.Type = n.Type.String()
	}
	if !root {
		j.Section = n.Section.String()
	}
	for _, a := range n.Attributes {
		j.Attributes = append(j.Attributes, jsonAttr{Name: a.Name, Value: a.Value})
	}
	if len(n.EnumStrings) > 0 {
		j.EnumStrings = enumStrings(n.EnumStrings)
	}
	if n.Kind == KindArray {
		if n.Bounded {
			low, high := n.Low, n.High
			j.Low, j.High = &low, &high
		}
		if n.elem != nil {
			j.Element = toJSON(n.elem("[*]"), false)
		}
		return j
	}
	for _, c := range n.children {
		j.Children = append(j.Children, toJSON(c, false))
	}
	return j
}

// Text renders the tree as indented lines "path : TYPE {attr, attr}",
// two spaces per level. Roots print their path only. Array elements are
// not expanded.
func (t *Tree) Text() string {
	var b strings.Builder
	for _, r := range t.Roots {
		writeText(&b, r, 0)
	}
	return b.String()
}

func writeText(b *strings.Builder, n *Node, depth int) {
	b.WriteString(strings.Repeat("  ", depth))
	b.WriteString(n.Path)
	if depth > 0 && n.TypeName != "" {
		b.WriteString(" : ")
		b.WriteString(n.TypeName)
	}
	if len(n.Attributes) > 0 {
		b.WriteString(" {")
		for i, a := range n.Attributes {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(a.Name)
			if a.HasValue {
				b.WriteString(" := '")
				b.WriteString(strings.ReplaceAll(a.Value, "'", "''"))
				b.WriteString("'")
			}
		}
		b.WriteString("}")
	}
	b.WriteByte('\n')
	if n.Kind == KindArray {
		return
	}
	for _, c := range n.children {
		writeText(b, c, depth+1)
	}
}
