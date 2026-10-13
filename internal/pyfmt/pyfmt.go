// Package pyfmt reproduces the exact formatting semantics of the Python
// reference implementation (f-strings, str.center/ljust/slice, round()).
//
// The fiscal paper renderer depends on these being byte-identical to Python,
// otherwise the golden tests would diverge.
package pyfmt

import (
	"errors"
	"math"
	"strconv"
	"strings"
)

// Float parses a Python-style float literal (allows 1_000 and .5).
func Float(s string) (float64, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, errors.New("could not convert string to float: ''")
	}
	clean := strings.ReplaceAll(t, "_", "")
	if v, err := strconv.ParseFloat(clean, 64); err == nil {
		return v, nil
	}
	// Python accepts 'inf', '-inf', 'nan' with any case.
	lower := strings.ToLower(clean)
	switch lower {
	case "inf", "+inf", "infinity", "+infinity":
		return math.Inf(1), nil
	case "-inf", "-infinity":
		return math.Inf(-1), nil
	case "nan":
		return math.NaN(), nil
	}
	return 0, errors.New("could not convert string to float: '" + t + "'")
}

// Int parses a Python-style int literal (allows 1_000).
func Int(s string) (int, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, errors.New("invalid literal for int() with base 10: ''")
	}
	clean := strings.ReplaceAll(t, "_", "")
	v, err := strconv.ParseInt(clean, 10, 64)
	if err != nil {
		return 0, errors.New("invalid literal for int() with base 10: '" + t + "'")
	}
	return int(v), nil
}

// PyRound mirrors Python's round(x, ndigits): correctly rounded from the
// exact binary value using banker's rounding, then returned as a float.
func PyRound(x float64, ndigits int) float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return x
	}
	if ndigits > 30 {
		ndigits = 30
	}
	if ndigits < 0 {
		// Python rounds to a decimal place left of the point.
		pow := math.Pow(10, float64(-ndigits))
		v := roundHalfEven(x/pow, -ndigits) * pow
		return v
	}
	s := strconv.FormatFloat(x, 'f', ndigits, 64)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return x
	}
	return v
}

func roundHalfEven(x float64, ndigits int) float64 {
	s := strconv.FormatFloat(x, 'f', ndigits, 64)
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// F2 formats with two decimals, like Python's f"{v:.2f}".
func F2(v float64) string {
	return strconv.FormatFloat(v, 'f', 2, 64)
}

// F3 formats with three decimals, like Python's f"{v:.3f}".
func F3(v float64) string {
	return strconv.FormatFloat(v, 'f', 3, 64)
}

// PyIsDigit matches Python's str.isdigit() for a single rune.
func PyIsDigit(r rune) bool {
	return r >= '0' && r <= '9'
}
