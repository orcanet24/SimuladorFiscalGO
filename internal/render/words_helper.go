package render

import (
	"fmt"
	"strconv"
	"strings"
)

// fmtSscan parses a Python-style float literal from the golden corpus, which
// may be written as "45.7" or "5e-05".
func fmtSscan(s string, v *float64) (int, error) {
	s = strings.TrimSpace(s)
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("%w", err)
	}
	*v = f
	return 1, nil
}
