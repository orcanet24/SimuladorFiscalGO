// Package orderedmap provides a JSON object that preserves insertion order,
// matching Python dict semantics used by the reference implementation.
package orderedmap

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// Map is an insertion-ordered string-keyed map.
type Map[V any] struct {
	keys []string
	vals map[string]V
}

// New returns an empty ordered map.
func New[V any]() *Map[V] {
	return &Map[V]{vals: map[string]V{}}
}

// Len returns the number of entries.
func (m *Map[V]) Len() int { return len(m.keys) }

// Keys returns the keys in insertion order.
func (m *Map[V]) Keys() []string {
	out := make([]string, len(m.keys))
	copy(out, m.keys)
	return out
}

// Get returns the value and whether the key exists.
func (m *Map[V]) Get(k string) (V, bool) {
	v, ok := m.vals[k]
	return v, ok
}

// GetOr returns the value or a fallback.
func (m *Map[V]) GetOr(k string, def V) V {
	if v, ok := m.vals[k]; ok {
		return v
	}
	return def
}

// Set inserts or replaces a key, recording insertion order on first write.
func (m *Map[V]) Set(k string, v V) {
	if m.vals == nil {
		m.vals = map[string]V{}
	}
	if _, ok := m.vals[k]; !ok {
		m.keys = append(m.keys, k)
	}
	m.vals[k] = v
}

// Delete removes a key, preserving the order of the rest.
func (m *Map[V]) Delete(k string) {
	if _, ok := m.vals[k]; !ok {
		return
	}
	delete(m.vals, k)
	for i, key := range m.keys {
		if key == k {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			break
		}
	}
}

// Clear removes every entry.
func (m *Map[V]) Clear() {
	m.keys = nil
	m.vals = map[string]V{}
}

// Clone returns a deep-enough copy (values are copied by value).
func (m *Map[V]) Clone() *Map[V] {
	out := New[V]()
	for _, k := range m.keys {
		out.Set(k, m.vals[k])
	}
	return out
}

// ForEach visits entries in insertion order.
func (m *Map[V]) ForEach(fn func(k string, v V)) {
	for _, k := range m.keys {
		fn(k, m.vals[k])
	}
}

// MarshalJSON writes the object with keys in insertion order.
func (m *Map[V]) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, k := range m.keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		buf.Write(key)
		buf.WriteByte(':')
		val, err := json.Marshal(m.vals[k])
		if err != nil {
			return nil, err
		}
		buf.Write(val)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// UnmarshalJSON reads an object, preserving the order it appears in the file.
func (m *Map[V]) UnmarshalJSON(b []byte) error {
	m.Clear()
	if bytes.Equal(b, []byte("null")) {
		return nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	// Recover order by scanning the source object's keys.
	order, err := keyOrder(b)
	if err != nil {
		return err
	}
	for _, k := range order {
		var v V
		if err := json.Unmarshal(raw[k], &v); err != nil {
			return err
		}
		m.Set(k, v)
	}
	return nil
}

// keyOrder extracts object keys in the order they appear in the JSON source.
func keyOrder(b []byte) ([]string, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, nil
	}
	var order []string
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		k, ok := tok.(string)
		if !ok {
			continue
		}
		order = append(order, k)
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return nil, err
		}
	}
	return order, nil
}

// FloatString renders a float the way Python's json does for whole numbers
// (e.g. 1.0 -> "1.0"), used where byte-parity with the reference matters.
func FloatString(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
