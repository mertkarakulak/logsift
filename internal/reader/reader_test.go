package reader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinesFromFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.log")
	content := "line one\nline two\nline three\n"
	if err := os.WriteFile(f, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	ch, errc := Lines([]string{f})
	var got []string
	for line := range ch {
		got = append(got, line)
	}
	if err := <-errc; err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("got %d lines want 3: %v", len(got), got)
	}
}

func TestLinesGzip(t *testing.T) {
	ch, errc := Lines([]string{"../../testdata/nginx_access.log.gz"})
	var lines []string
	for line := range ch {
		lines = append(lines, line)
	}
	if err := <-errc; err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(lines) == 0 {
		t.Error("no lines from gz file")
	}
	// Every line from the gz should be a valid nginx access log line.
	for _, l := range lines {
		if !strings.Contains(l, " HTTP/") {
			t.Errorf("unexpected line: %q", l)
		}
	}
}

func TestLinesGlob(t *testing.T) {
	ch, errc := Lines([]string{"../../testdata/nginx_access.log*"})
	var count int
	for range ch {
		count++
	}
	if err := <-errc; err != nil {
		t.Fatalf("error: %v", err)
	}
	// Both the plain and .gz file expand, each with 13 lines = 26.
	if count < 13 {
		t.Errorf("got %d lines from glob, want at least 13", count)
	}
}

func TestLinesEmptyFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "empty.log")
	if err := os.WriteFile(f, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	ch, errc := Lines([]string{f})
	var count int
	for range ch {
		count++
	}
	if err := <-errc; err != nil {
		t.Fatalf("error: %v", err)
	}
	if count != 0 {
		t.Errorf("got %d lines from empty file", count)
	}
}

func TestLinesMissingFile(t *testing.T) {
	ch, errc := Lines([]string{"/tmp/does_not_exist_logsift_test.log"})
	for range ch {
	}
	err := <-errc
	if err == nil {
		t.Error("expected error for missing file, got nil")
	}
}
