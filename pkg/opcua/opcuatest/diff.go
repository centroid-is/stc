package opcuatest

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// DiffMode selects which side's extra nodes count as differences.
type DiffMode int

const (
	// Subset reports emulated nodes missing from the real snapshot and
	// ignores real nodes the emulated space does not have.
	Subset DiffMode = iota
	// Full also reports real nodes missing from the emulated space.
	Full
)

// Difference is one divergence between an emulated and a real snapshot.
// Missing nodes use Attribute "node" and the text "missing" on the side
// that lacks them. DataType definitions use the NodeID "DataType:<name>".
type Difference struct {
	NodeID    string `json:"nodeId"`
	Attribute string `json:"attribute"`
	Emulated  string `json:"emulated"`
	Real      string `json:"real"`
}

func (d Difference) String() string {
	return fmt.Sprintf("%s %s: emulated %s, real %s", d.NodeID, d.Attribute, d.Emulated, d.Real)
}

// Load reads a snapshot JSON file.
func Load(path string) (*Snapshot, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &s, nil
}

// DataTypeName normalizes a DataType reference. Standard ns=0 ids stay as
// they are. Other ids become their type name: the text after the last '.'
// or '>' of the identifier, so stc's "ns=4;s=DT.ST_Drive_HMI" equals any
// TF6100 id that ends in the same type name. Snapshots carry no DataType
// BrowseName, so the name is derived from the id.
func DataTypeName(id string) string {
	if id == "" || !strings.HasPrefix(id, "ns=") || strings.HasPrefix(id, "ns=0;") {
		return id
	}
	_, ident, ok := strings.Cut(id, ";")
	if !ok {
		return id
	}
	if rest, ok := strings.CutPrefix(ident, "s="); ok {
		ident = rest
	}
	if i := strings.LastIndexAny(ident, ".>"); i >= 0 && i < len(ident)-1 {
		ident = ident[i+1:]
	}
	return ident
}

// Diff compares node ids, node class, data type, value rank, array
// dimensions, access levels and DataType definitions. Description, Root,
// Parent, Reference and BrowseName are not compared. The result is sorted
// by node id, then attribute.
func Diff(emulated, real *Snapshot, mode DiffMode) []Difference {
	var out []Difference
	add := func(id, attr, e, r string) {
		out = append(out, Difference{NodeID: id, Attribute: attr, Emulated: e, Real: r})
	}
	realNodes := map[string]Node{}
	for _, n := range real.Nodes {
		realNodes[n.NodeID] = n
	}
	emuNodes := map[string]bool{}
	for _, e := range emulated.Nodes {
		emuNodes[e.NodeID] = true
		r, ok := realNodes[e.NodeID]
		if !ok {
			add(e.NodeID, "node", "present", "missing")
			continue
		}
		cmp := func(attr, ev, rv string) {
			if ev != rv {
				add(e.NodeID, attr, ev, rv)
			}
		}
		cmp("nodeClass", e.NodeClass, r.NodeClass)
		cmp("dataType", DataTypeName(e.DataType), DataTypeName(r.DataType))
		cmp("valueRank", fmtRank(e.ValueRank), fmtRank(r.ValueRank))
		cmp("arrayDimensions", fmt.Sprint(e.ArrayDimensions), fmt.Sprint(r.ArrayDimensions))
		cmp("accessLevel", fmtAccess(e.AccessLevel), fmtAccess(r.AccessLevel))
	}
	if mode == Full {
		for _, r := range real.Nodes {
			if !emuNodes[r.NodeID] {
				add(r.NodeID, "node", "missing", "present")
			}
		}
	}

	realDT := map[string]DataType{}
	for _, d := range real.DataTypes {
		realDT[DataTypeName(d.NodeID)] = d
	}
	emuDT := map[string]bool{}
	for _, e := range emulated.DataTypes {
		name := DataTypeName(e.NodeID)
		emuDT[name] = true
		id := "DataType:" + name
		r, ok := realDT[name]
		if !ok {
			add(id, "definition", "present", "missing")
			continue
		}
		if e.Kind != r.Kind {
			add(id, "kind", e.Kind, r.Kind)
			continue
		}
		ef, rf := fieldTexts(e.Fields), fieldTexts(r.Fields)
		for i := 0; i < max(len(ef), len(rf)); i++ {
			ev, rv := at(ef, i), at(rf, i)
			if ev != rv {
				add(id, fmt.Sprintf("field[%d]", i), ev, rv)
			}
		}
		ev, rv := enumTexts(e.Values), enumTexts(r.Values)
		for i := 0; i < max(len(ev), len(rv)); i++ {
			a, b := at(ev, i), at(rv, i)
			if a != b {
				add(id, fmt.Sprintf("value[%d]", i), a, b)
			}
		}
	}
	if mode == Full {
		for _, r := range real.DataTypes {
			if name := DataTypeName(r.NodeID); !emuDT[name] {
				add("DataType:"+name, "definition", "missing", "present")
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].NodeID != out[j].NodeID {
			return out[i].NodeID < out[j].NodeID
		}
		return out[i].Attribute < out[j].Attribute
	})
	return out
}

// FormatDiff renders at most max differences, one per line, followed by
// "... and N more" when some were cut.
func FormatDiff(ds []Difference, max int) string {
	var b strings.Builder
	for i, d := range ds {
		if max > 0 && i >= max {
			fmt.Fprintf(&b, "... and %d more\n", len(ds)-max)
			break
		}
		b.WriteString(d.String())
		b.WriteByte('\n')
	}
	return b.String()
}

func fmtRank(v *int32) string {
	if v == nil {
		return "unset"
	}
	return fmt.Sprint(*v)
}

func fmtAccess(v *uint8) string {
	if v == nil {
		return "unset"
	}
	return fmt.Sprint(*v)
}

func fieldTexts(fs []Field) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = fmt.Sprintf("%s:%s rank %d dims %v", f.Name, DataTypeName(f.DataType), f.ValueRank, f.ArrayDimensions)
	}
	return out
}

func enumTexts(vs []EnumValue) []string {
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = fmt.Sprintf("%s(%d)", v.Name, v.Value)
	}
	return out
}

func at(s []string, i int) string {
	if i < len(s) {
		return s[i]
	}
	return "missing"
}
