// Package reader provides streaming log line access from files (including
// .gz-compressed and glob patterns) and from stdin.
package reader

import (
	"bufio"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Lines returns a channel that yields every non-empty line from the given
// paths (which may contain glob patterns). Files ending in ".gz" are
// transparently decompressed. Reading the channel blocks; close is performed
// automatically when all paths are exhausted or an early exit is triggered by
// the context. Errors are sent to the returned error channel (one value or nil
// at close).
//
// If paths is empty, Lines reads from stdin instead.
func Lines(paths []string) (<-chan string, <-chan error) {
	out := make(chan string, 256)
	errc := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errc)

		if len(paths) == 0 {
			errc <- scanReader(os.Stdin, out)
			return
		}

		for _, pattern := range paths {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				errc <- err
				return
			}
			if len(matches) == 0 {
				// treat non-glob pattern as a literal path
				matches = []string{pattern}
			}
			for _, path := range matches {
				if err := readFile(path, out); err != nil {
					errc <- err
					return
				}
			}
		}
	}()

	return out, errc
}

func readFile(path string, out chan<- string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var r io.Reader = f
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gz.Close()
		r = gz
	}

	return scanReader(r, out)
}

func scanReader(r io.Reader, out chan<- string) error {
	sc := bufio.NewScanner(r)
	// Allow lines up to 1 MiB (some nginx lines with large user-agents can be long).
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line != "" {
			out <- line
		}
	}
	return sc.Err()
}
