package pyfmt

import "testing"

func TestCenterMatchesPython(t *testing.T) {
	// (input, width, expected) captured from CPython.
	cases := []struct {
		in   string
		width int
		want string
	}{
		{"ab", 1, "ab"},
		{"ab", 2, "ab"},
		{"ab", 3, " ab"},
		{"ab", 4, " ab "},
		{"ab", 5, "  ab "},
		{"ab", 6, "  ab  "},
		{"ab", 7, "   ab  "},
		{"ab", 8, "   ab   "},
		{"ab", 9, "    ab   "},
		{"ab", 10, "    ab    "},
		{"x", 4, " x  "},
		{"x", 5, "  x  "},
		{"abcdef", 7, " abcdef"},
		{"abcdef", 8, " abcdef "},
		{"abcdef", 9, "  abcdef "},
		{"abc", 6, " abc  "},
		{"abc", 7, "  abc  "},
	}
	for _, c := range cases {
		if got := Center(c.in, c.width); got != c.want {
			t.Errorf("Center(%q, %d) = %q, want %q", c.in, c.width, got, c.want)
		}
	}
}

// Exhaustive parity check of the left/right split formula.
func TestCenterFormulaExhaustive(t *testing.T) {
	for n := 1; n <= 12; n++ {
		for width := 1; width <= 32; width++ {
			fill := width - n
			var wantLeft int
			if fill <= 0 {
				wantLeft = 0
			} else if n%2 == 0 {
				wantLeft = (fill + 1) / 2
			} else {
				wantLeft = fill / 2
			}
			got := Center(repeat("a", n), width)
			if fill <= 0 {
				if Count(got) != n {
					t.Fatalf("n=%d width=%d: %q", n, width, got)
				}
				continue
			}
			left := Count(got) - Count(trimLeftSpaces(got))
			if left != wantLeft {
				t.Errorf("n=%d width=%d: left=%d want %d (%q)", n, width, left, wantLeft, got)
			}
			if Count(got) != width {
				t.Errorf("n=%d width=%d: len=%d want %d", n, width, Count(got), width)
			}
		}
	}
}

func repeat(s string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += s
	}
	return out
}

func trimLeftSpaces(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	return s
}

func TestMoneyMatchesPython(t *testing.T) {
	cases := map[float64]string{
		1234.5:      "1.234,50",
		1234567.891: "1.234.567,89",
		0.5:         "0,50",
		-1234.5:     "-1.234,50",
		100.0:       "100,00",
		0.0:         "0,00",
		-0.5:        "-0,50",
		999.99:      "999,99",
		1000.0:      "1.000,00",
	}
	for v, want := range cases {
		if got := Money(v); got != want {
			t.Errorf("Money(%v) = %q, want %q", v, got, want)
		}
	}
}

func TestSliceMatchesPython(t *testing.T) {
	cases := []struct {
		in         string
		start, end int
		want       string
	}{
		{"abcdef", 1, 4, "bcd"},
		{"áéió", 0, 2, "áé"},
		{"abcdef", -3, 100, "def"},
		{"abcdef", 10, 20, ""},
		{"abcdef", 4, 2, ""},
	}
	for _, c := range cases {
		if got := Slice(c.in, c.start, c.end); got != c.want {
			t.Errorf("Slice(%q,%d,%d) = %q, want %q", c.in, c.start, c.end, got, c.want)
		}
	}
	// The 40-char header cap used by ProgramCompany.
	long := repeat("MUYLARGOCOMO", 5)
	if got := Slice(long, 0, 40); got != "MUYLARGOCOMOMUYLARGOCOMOMUYLARGOCOMOMUYL" {
		t.Errorf("header slice = %q", got)
	}
}

func TestFixedMatchesPython(t *testing.T) {
	cases := []struct {
		v            float64
		width, prec  int
		want         string
	}{
		{6.0, 8, 3, "   6.000"},
		{123456.789, 8, 3, "123456.789"},
		{0.5, 8, 3, "   0.500"},
		{-12.5, 8, 3, " -12.500"},
		{2.5, 10, 2, "      2.50"},
		{1234567.89, 10, 2, "1234567.89"},
		{-1.5, 10, 2, "     -1.50"},
	}
	for _, c := range cases {
		if got := Fixed(c.v, c.width, c.prec); got != c.want {
			t.Errorf("Fixed(%v,%d,%d) = %q, want %q", c.v, c.width, c.prec, got, c.want)
		}
	}
}

func TestF2MatchesPython(t *testing.T) {
	cases := map[float64]string{
		0.1 + 0.2:             "0.30",
		7.805:                 "7.80",
		-0.005:                "-0.01",
		1e15:                  "1000000000000000.00",
		1.0 / 3.0:             "0.33",
		12345678901234.5:      "12345678901234.50",
	}
	for v, want := range cases {
		if got := F2(v); got != want {
			t.Errorf("F2(%v) = %q, want %q", v, got, want)
		}
	}
}

func TestPyRoundMatchesPython(t *testing.T) {
	// round(x, 2) from CPython
	cases := []struct {
		in   float64
		want float64
	}{
		{2.675, 2.67},
		{0.125, 0.12},
		{-0.125, -0.12},
		{1.005, 1.0},
		{2.5, 2.5},
	}
	for _, c := range cases {
		if got := PyRound(c.in, 2); got != c.want {
			t.Errorf("PyRound(%v,2) = %v, want %v", c.in, got, c.want)
		}
	}
}
