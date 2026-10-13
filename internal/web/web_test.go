package web

import (
	"testing"

	"simuladorfiscalgo/internal/pyfmt"
	"simuladorfiscalgo/internal/state"
)

// TestMarshalPySeparators checks byte parity with
// json.dumps(payload, ensure_ascii=False): ", " and ": " separators,
// no trailing newline, no HTML escaping, strings left untouched.
func TestMarshalPySeparators(t *testing.T) {
	type payload struct {
		A int               `json:"a"`
		B []int             `json:"b"`
		S string            `json:"s"`
		M map[string]string `json:"m"`
	}
	got, err := marshalPy(payload{
		A: 8,
		B: []int{1, 2},
		S: "x, y: z",
		M: map[string]string{"k": "v"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"a": 8, "b": [1, 2], "s": "x, y: z", "m": {"k": "v"}}`
	if string(got) != want {
		t.Errorf("marshalPy = %q, want %q", got, want)
	}
}

// TestMarshalPyNoHTMLEscape: Python does not escape < > & (ensure_ascii only
// affects non-ASCII, which is emitted raw as UTF-8).
func TestMarshalPyNoHTMLEscape(t *testing.T) {
	type payload struct {
		S string `json:"s"`
	}
	got, err := marshalPy(payload{S: "REPORTE Z N° 0001 <A> & \"B\""})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"s": "REPORTE Z N° 0001 <A> & \"B\""}`
	if string(got) != want {
		t.Errorf("marshalPy = %q, want %q", got, want)
	}
}

// TestMarshalPyEscapedQuotes: quotes inside strings must not receive the
// extra space (only separators outside strings are padded).
func TestMarshalPyEscapedQuotes(t *testing.T) {
	type payload struct {
		S string `json:"s"`
	}
	got, err := marshalPy(payload{S: "a\"b,c:d"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"s": "a\"b,c:d"}`
	if string(got) != want {
		t.Errorf("marshalPy = %q, want %q", got, want)
	}
}

// TestPayloadKeyOrder: history and preview keep the reference dict literal
// order (entries/count, open/lines), not Go's alphabetical map order.
func TestPayloadKeyOrder(t *testing.T) {
	h, err := marshalPy(historyPayload{Entries: []state.HistoryEntry{}, Count: 3})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"entries": [], "count": 3}`; string(h) != want {
		t.Errorf("history = %q, want %q", h, want)
	}
	p, err := marshalPy(previewPayload{Open: false, Lines: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"open": false, "lines": []}`; string(p) != want {
		t.Errorf("preview = %q, want %q", p, want)
	}
}

// TestMarshalPyFloatParity: PyFloat values keep Python repr formatting.
func TestMarshalPyFloatParity(t *testing.T) {
	type fp struct {
		A pyfmt.PyFloat `json:"a"`
	}
	got, err := marshalPy(fp{A: pyfmt.PyFloat(220)})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"a": 220.0}`; string(got) != want {
		t.Errorf("float = %q, want %q", got, want)
	}
}
