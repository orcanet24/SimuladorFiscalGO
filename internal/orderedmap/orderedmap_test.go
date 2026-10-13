package orderedmap

import (
	"encoding/json"
	"testing"
)

func TestInsertionOrderPreserved(t *testing.T) {
	m := New[float64]()
	m.Set("cash", 75.37)
	m.Set("card", 9.02)
	m.Set("transfer", 1.5)
	if got, want := m.Keys(), []string{"cash", "card", "transfer"}; !equal(got, want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"cash":75.37,"card":9.02,"transfer":1.5}` {
		t.Errorf("marshal = %s", b)
	}
}

func TestReplaceKeepsPosition(t *testing.T) {
	m := New[float64]()
	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("a", 3)
	if got, want := m.Keys(), []string{"a", "b"}; !equal(got, want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
	if v, _ := m.Get("a"); v != 3 {
		t.Errorf("a = %v", v)
	}
}

func TestUnmarshalPreservesFileOrder(t *testing.T) {
	m := New[float64]()
	if err := json.Unmarshal([]byte(`{"z":1,"a":2,"m":3}`), m); err != nil {
		t.Fatal(err)
	}
	if got, want := m.Keys(), []string{"z", "a", "m"}; !equal(got, want) {
		t.Fatalf("keys = %v, want %v", got, want)
	}
}

func TestClone(t *testing.T) {
	m := New[float64]()
	m.Set("x", 1)
	c := m.Clone()
	c.Set("y", 2)
	if m.Len() != 1 || c.Len() != 2 {
		t.Errorf("clone leaked: %d vs %d", m.Len(), c.Len())
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
