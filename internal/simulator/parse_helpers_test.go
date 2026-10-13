package simulator

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func readLines(t *testing.T, file string) []string {
	t.Helper()
	f, err := os.Open(filepath.Join("..", "..", "testdata", "golden", file))
	if err != nil {
		t.Fatalf("open %s: %v", file, err)
	}
	defer f.Close()

	var out []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func fmtSscanInt(s string, v *int) (int, error) {
	s = strings.TrimSpace(s)
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	*v = n
	return 1, nil
}
