package pyfmt

import (
	"strconv"
	"strings"
)

// PyFloat marshals a float the way Python's json module does: the shortest
// round-trip representation, always with a decimal point or exponent
// (1.0 -> "1.0", 0.5 -> "0.5", 1e21 -> "1e+21").
type PyFloat float64

// MarshalJSON implements json.Marshaler.
func (f PyFloat) MarshalJSON() ([]byte, error) {
	return []byte(FormatPyFloat(float64(f))), nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (f *PyFloat) UnmarshalJSON(b []byte) error {
	v, err := strconv.ParseFloat(string(b), 64)
	if err != nil {
		return err
	}
	*f = PyFloat(v)
	return nil
}

// FormatPyFloat renders v the way repr(v) does in Python.
func FormatPyFloat(v float64) string {
	s := strconv.FormatFloat(v, 'g', -1, 64)
	if strings.ContainsAny(s, ".eEn") {
		// 'n' covers "nan"/"-nan"; "inf" needs the suffix added below.
		if strings.ContainsAny(s, ".e") {
			return s
		}
	}
	if !strings.ContainsAny(s, ".eE") && !isSpecial(s) {
		return s + ".0"
	}
	return s
}

func isSpecial(s string) bool {
	switch strings.ToLower(strings.TrimPrefix(s, "-")) {
	case "nan", "+nan", "inf", "+inf", "-inf":
		return true
	}
	return false
}
