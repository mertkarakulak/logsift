// Package cmdgrep implements the "logsift grep" subcommand.
//
// Exit codes follow grep convention:
//
//	0  one or more matching lines were found
//	1  no matching lines
//	2  usage or processing error
package cmdgrep

import (
	"flag"
	"fmt"
	"os"

	"github.com/mertkarakulak/logsift/internal/filter"
	"github.com/mertkarakulak/logsift/internal/parser"
	"github.com/mertkarakulak/logsift/internal/reader"
)

// Run parses flags from args, runs the grep command, and returns the exit code
// (0 = matches found, 1 = no matches, 2 = error).
func Run(args []string) (int, error) {
	fs := flag.NewFlagSet("grep", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		since  = fs.String("since", "", "only entries at or after this time (e.g. 1h, 30m, 2026-09-17T10:00:00Z)")
		until  = fs.String("until", "", "only entries before or at this time")
		status = fs.String("status", "", "filter by HTTP status class: 5xx, 4xx, 3xx, 2xx, or exact code")
		level  = fs.String("level", "", "filter by minimum severity level: emerg alert crit err warn notice info debug")
		unit   = fs.String("unit", "", "filter by systemd unit name (substring match)")
		match  = fs.String("match", "", "filter by regex applied to raw line")
		format = fs.String("format", "", "force parser: nginx, nginx-error, journald, syslog")
	)

	if err := fs.Parse(args); err != nil {
		return 2, err
	}
	files := fs.Args()

	var opts filter.Options
	var err error

	if *since != "" {
		opts.Since, err = filter.ParseSince(*since)
		if err != nil {
			return 2, fmt.Errorf("--since: %w", err)
		}
	}
	if *until != "" {
		opts.Until, err = filter.ParseSince(*until)
		if err != nil {
			return 2, fmt.Errorf("--until: %w", err)
		}
	}
	opts.Status = *status
	opts.Level = *level
	opts.Unit = *unit
	opts.Match = *match

	cf, err := filter.Compile(opts)
	if err != nil {
		return 2, err
	}

	var p parser.Parser
	if *format != "" {
		p = parser.ForName(*format)
		if p == nil {
			return 2, fmt.Errorf("unknown --format %q", *format)
		}
	} else {
		p = parser.Auto()
	}

	found := false
	lines, errc := reader.Lines(files)

	for line := range lines {
		e, ok := p.Parse(line)
		if !ok {
			// Unparsed lines: still apply --match against the raw line.
			// Other structured filters (status, level, unit, time) require a
			// parsed entry, so unparsed lines are excluded if any such filter
			// is active.
			if hasStructuredFilter(opts) {
				continue
			}
			if opts.Match != "" && !cf.Match(e) {
				continue
			}
			fmt.Println(line)
			found = true
			continue
		}
		if cf.Match(e) {
			fmt.Println(line)
			found = true
		}
	}
	if err := <-errc; err != nil {
		return 2, fmt.Errorf("reading input: %w", err)
	}

	if !found {
		return 1, nil
	}
	return 0, nil
}

// hasStructuredFilter returns true when any filter other than --match requires
// a parsed entry to evaluate.
func hasStructuredFilter(opts filter.Options) bool {
	return !opts.Since.IsZero() || !opts.Until.IsZero() ||
		opts.Status != "" || opts.Level != "" || opts.Unit != ""
}
