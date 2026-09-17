package parser

import (
	"bufio"
	"os"
	"testing"
)

// fixtureLines reads all non-empty lines from a file for use in parser tests.
func fixtureLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("fixtureLines: %v", err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if l := sc.Text(); l != "" {
			lines = append(lines, l)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("fixtureLines scan: %v", err)
	}
	return lines
}
