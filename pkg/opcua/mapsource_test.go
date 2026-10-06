package opcua

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestKindString(t *testing.T) {
	want := map[Kind]string{
		KindRoot: "Root", KindGVL: "GVL", KindProgram: "Program", KindFBInstance: "FBInstance",
		KindStruct: "Struct", KindArray: "Array", KindScalar: "Scalar", KindEnum: "Enum",
		KindReference: "Reference", KindPointer: "Pointer", Kind(42): "Kind(42)", Kind(-1): "Kind(-1)",
	}
	for k, s := range want {
		if got := k.String(); got != s {
			t.Errorf("Kind(%d).String() = %q, want %q", int(k), got, s)
		}
	}
}

func TestMapSourceRead(t *testing.T) {
	init := map[string]any{"GVL.a": int16(1)}
	m := NewMapSource(init)
	init["GVL.a"] = int16(99) // NewMapSource copies
	v, err := m.Read("GVL.a")
	if err != nil || v != int16(1) {
		t.Fatalf("Read hit = %v, %v", v, err)
	}
	_, err = m.Read("GVL.missing")
	if !errors.Is(err, ErrUnknownSymbol) {
		t.Fatalf("Read miss err = %v, want ErrUnknownSymbol", err)
	}
	if got, ok := m.Get("GVL.a"); !ok || got != int16(1) {
		t.Fatalf("Get = %v, %v", got, ok)
	}
	if _, ok := m.Get("GVL.missing"); ok {
		t.Fatal("Get miss returned ok")
	}
}

func TestMapSourceWrite(t *testing.T) {
	m := NewMapSource(map[string]any{"a": true, "b": int32(0)})
	if err := m.Write("a", false); err != nil {
		t.Fatal(err)
	}
	if v, _ := m.Read("a"); v != false {
		t.Fatalf("after write a = %v", v)
	}
	if err := m.Write("nope", 1); !errors.Is(err, ErrUnknownSymbol) {
		t.Fatalf("unknown write err = %v", err)
	}
	m.FailWrite("b", ErrNotWritable)
	if err := m.Write("b", int32(5)); !errors.Is(err, ErrNotWritable) {
		t.Fatalf("injected err = %v", err)
	}
	if v, _ := m.Read("b"); v != int32(0) {
		t.Fatalf("failed write changed value to %v", v)
	}
	m.FailWrite("b", nil)
	if err := m.Write("b", int32(7)); err != nil {
		t.Fatal(err)
	}
	m.Set("c", "set") // no WriteRecord
	w := m.Writes()
	want := []WriteRecord{{"a", false}, {"b", int32(7)}}
	if len(w) != len(want) {
		t.Fatalf("Writes = %v, want %v", w, want)
	}
	for i := range want {
		if w[i] != want[i] {
			t.Fatalf("Writes[%d] = %v, want %v", i, w[i], want[i])
		}
	}
	w[0].Path = "mutated"
	if m.Writes()[0].Path != "a" {
		t.Fatal("Writes did not return a copy")
	}
}

func TestMapSourceSnapshot(t *testing.T) {
	m := NewMapSource(map[string]any{"a": 1, "b": 2})
	s, err := m.Snapshot([]string{"a", "b"})
	if err != nil || len(s) != 2 || s["a"] != 1 || s["b"] != 2 {
		t.Fatalf("Snapshot = %v, %v", s, err)
	}
	s, err = m.Snapshot([]string{"a", "x"})
	if !errors.Is(err, ErrUnknownSymbol) || s != nil {
		t.Fatalf("Snapshot with unknown = %v, %v", s, err)
	}
}

func TestMapSourceConcurrent(t *testing.T) {
	m := NewMapSource(map[string]any{"x": 0})
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 200; i++ {
				switch i % 4 {
				case 0:
					_ = m.Write("x", g*1000+i)
				case 1:
					_, _ = m.Read("x")
				case 2:
					_, _ = m.Snapshot([]string{"x"})
				default:
					m.Set(fmt.Sprintf("p%d", g), i)
				}
			}
		}(g)
	}
	wg.Wait()
	if n := len(m.Writes()); n != 8*50 {
		t.Fatalf("write log length = %d, want 400", n)
	}
}
