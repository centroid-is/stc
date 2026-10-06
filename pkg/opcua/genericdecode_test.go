package opcua

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/awcullen/opcua/client"
	"github.com/awcullen/opcua/ua"
)

// genericDecoder decodes an ExtensionObject body using only the
// StructureDefinitions the server serves (28-RESEARCH decode procedure
// steps 3-6): no compile-time or registered Go type is involved.
type genericDecoder struct {
	t *testing.T
	c *client.Client
}

// builtin resolves a DataType to its ns=0 builtin, Enumeration (i=29) or
// Structure (i=22) by walking inverse HasSubtype references.
func (g genericDecoder) builtin(dt ua.NodeID) ua.NodeID {
	for i := 0; i < 16; i++ {
		if n, ok := dt.(ua.NodeIDNumeric); ok && n.NamespaceIndex == 0 &&
			(n.ID <= 25 || dt == ua.DataTypeIDEnumeration || dt == ua.DataTypeIDStructure) {
			return dt
		}
		sup := browseRefs(g.t, g.c, dt, ua.ReferenceTypeIDHasSubtype, ua.BrowseDirectionInverse)
		if len(sup) == 0 {
			g.t.Fatalf("no supertype for %v", dt)
		}
		dt = expandedID(sup[0].NodeID)
	}
	g.t.Fatalf("supertype chain of %v too deep", dt)
	return nil
}

func (g genericDecoder) walk(dec *ua.BinaryDecoder, def ua.StructureDefinition) (map[string]any, error) {
	out := map[string]any{}
	for _, f := range def.Fields {
		b := g.builtin(f.DataType)
		if f.ValueRank == ua.ValueRankOneDimension {
			var n int32
			if err := dec.ReadInt32(&n); err != nil {
				return nil, err
			}
			arr := make([]any, 0, n)
			for j := int32(0); j < n; j++ {
				v, err := g.scalar(dec, b, f)
				if err != nil {
					return nil, err
				}
				arr = append(arr, v)
			}
			out[f.Name] = arr
			continue
		}
		v, err := g.scalar(dec, b, f)
		if err != nil {
			return nil, err
		}
		out[f.Name] = v
	}
	return out, nil
}

func (g genericDecoder) scalar(dec *ua.BinaryDecoder, b ua.NodeID, f ua.StructureField) (any, error) {
	switch b {
	case ua.DataTypeIDBoolean:
		var v bool
		return v, dec.ReadBoolean(&v)
	case ua.DataTypeIDInt16:
		var v int16
		return v, dec.ReadInt16(&v)
	case ua.DataTypeIDInt32, ua.DataTypeIDEnumeration:
		var v int32
		return v, dec.ReadInt32(&v)
	case ua.DataTypeIDFloat:
		var v float32
		return v, dec.ReadFloat(&v)
	case ua.DataTypeIDString:
		var v string
		return v, dec.ReadString(&v)
	case ua.DataTypeIDStructure: // nested: inline, no ExtensionObject header
		return g.walk(dec, structDefOf(g.t, g.c, f.DataType))
	}
	return nil, fmt.Errorf("field %s: builtin %v not handled", f.Name, b)
}

func TestGenericDecode(t *testing.T) {
	t.Parallel()
	s := startServer(t, nil)
	c := dialAnon(t, s)
	d, err := s.ensureStruct(stOuter())
	if err != nil {
		t.Fatal(err)
	}

	// The server side composes a value of the runtime Go type and awcullen
	// encodes the body, exactly as a structured Variable's read does.
	v := reflect.New(d.GoType).Elem()
	inner := reflect.New(d.Fields[0].Nested.GoType).Elem()
	inner.Field(0).Set(reflect.ValueOf(int32(-7)))
	inner.Field(1).Set(reflect.ValueOf(true))
	v.Field(0).Set(inner)
	v.Field(1).Set(reflect.ValueOf([]float32{1.5, 2.5, 3.5}))
	v.Field(2).Set(reflect.ValueOf("héllo"))
	v.Field(3).Set(reflect.ValueOf(int32(2)))
	items := reflect.MakeSlice(d.Fields[4].GoType, 2, 2)
	items.Index(1).Field(0).Set(reflect.ValueOf(int32(9)))
	v.Field(4).Set(items)
	var body bytes.Buffer
	if err := ua.NewBinaryEncoder(&body, ua.NewEncodingContext()).Encode(v.Interface()); err != nil {
		t.Fatalf("encode: %v", err)
	}

	// The client side knows only the Variable's DataType node id.
	g := genericDecoder{t: t, c: c}
	def := structDefOf(t, c, d.DTID)
	if def.DefaultEncodingID != d.EncID {
		t.Fatalf("DefaultEncodingID %v", def.DefaultEncodingID)
	}
	r := bytes.NewReader(body.Bytes())
	got, err := g.walk(ua.NewBinaryDecoder(r, ua.NewEncodingContext()), def)
	if err != nil {
		t.Fatal(err)
	}
	if r.Len() != 0 {
		t.Fatalf("%d trailing bytes", r.Len())
	}
	want := map[string]any{
		"inner": map[string]any{"a": int32(-7), "b": true},
		"vals":  []any{float32(1.5), float32(2.5), float32(3.5)},
		"s":     "héllo",
		"state": int32(2),
		"items": []any{
			map[string]any{"a": int32(0), "b": false},
			map[string]any{"a": int32(9), "b": false},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded\n got %#v\nwant %#v", got, want)
	}
}
