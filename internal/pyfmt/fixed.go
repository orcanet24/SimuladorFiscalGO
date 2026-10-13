package pyfmt

import (
	"strconv"
	"strings"
)

// Fixed renders v with `prec` decimals right-aligned in `width`, like Python's
// f"{v:8.3f}". Overflow is allowed (Python never truncates).
func Fixed(v float64, width, prec int) string {
	return padLeft(FixedPrec(v, prec), width)
}

// FixedPrec formats v with `prec` decimals, no padding.
func FixedPrec(v float64, prec int) string {
	return strconv.FormatFloat(v, 'f', prec, 64)
}

func padLeft(s string, width int) string {
	n := Count(s)
	if n >= width {
		return s
	}
	return strings.Repeat(" ", width-n) + s
}

// Money formats v the way the paper renderer does: thousands separated by
// "." and decimals by "," — equivalent to f"{v:,.2f}" with ',' and '.' swapped.
//
// Matches Python for the full documented matrix, including negatives.
func Money(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	dot := strings.IndexByte(s, '.')
	intPart := s
	frac := ""
	if dot >= 0 {
		intPart = s[:dot]
		frac = s[dot:] // keeps the '.'
	}
	var b strings.Builder
	for i, c := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	out := b.String() + "," + strings.TrimPrefix(frac, ".")
	if neg {
		out = "-" + out
	}
	return out
}
