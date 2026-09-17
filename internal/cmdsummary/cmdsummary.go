// Package cmdsummary implements the "logsift summary" subcommand.
package cmdsummary

import (
	"flag"
	"fmt"
	"os"

	"github.com/mertkarakulak/logsift/internal/filter"
	"github.com/mertkarakulak/logsift/internal/parser"
	"github.com/mertkarakulak/logsift/internal/reader"
	"github.com/mertkarakulak/logsift/internal/summary"
)

// Run parses flags from args and runs the summary command.
func Run(args []string) error {
	fs := flag.NewFlagSet("summary", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var (
		since   = fs.String("since", "", "only entries at or after this time (e.g. 1h, 30m, 2026-09-17T10:00:00Z)")
		until   = fs.String("until", "", "only entries before or at this time")
		status  = fs.String("status", "", "filter by HTTP status class: 5xx, 4xx, 3xx, 2xx, or exact code")
		level   = fs.String("level", "", "filter by minimum severity level: emerg alert crit err warn notice info debug")
		unit    = fs.String("unit", "", "filter by systemd unit name (substring match)")
		match   = fs.String("match", "", "filter by regex applied to raw line")
		format  = fs.String("format", "", "force parser: nginx, nginx-error, journald, syslog")
		topK    = fs.Int("top", 10, "number of top entries to show in each section")
		asJSON  = fs.Bool("json", false, "output summary as JSON")
	)

	if err := fs.Parse(args); err != nil {
		return err
	}
	files := fs.Args()

	opts, err := buildFilterOpts(*since, *until, *status, *level, *unit, *match)
	if err != nil {
		return err
	}

	cf, err := filter.Compile(opts)
	if err != nil {
		return err
	}

	var p parser.Parser
	if *format != "" {
		p = parser.ForName(*format)
		if p == nil {
			return fmt.Errorf("unknown --format %q", *format)
		}
	} else {
		p = parser.Auto()
	}

	filterActive := !opts.Since.IsZero() || !opts.Until.IsZero() ||
		opts.Status != "" || opts.Level != "" || opts.Unit != "" || opts.Match != ""

	stats := summary.New()
	lines, errc := reader.Lines(files)

	for line := range lines {
		e, ok := p.Parse(line)
		if filterActive && (!ok || !cf.Match(e)) {
			continue
		}
		stats.Add(e, ok)
	}
	if err := <-errc; err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	if *asJSON {
		return summary.PrintJSON(os.Stdout, stats, *topK)
	}
	summary.Print(os.Stdout, stats, *topK)
	return nil
}

func buildFilterOpts(since, until, status, level, unit, match string) (filter.Options, error) {
	var opts filter.Options
	var err error

	if since != "" {
		opts.Since, err = filter.ParseSince(since)
		if err != nil {
			return opts, fmt.Errorf("--since: %w", err)
		}
	}
	if until != "" {
		opts.Until, err = filter.ParseSince(until)
		if err != nil {
			return opts, fmt.Errorf("--until: %w", err)
		}
	}
	opts.Status = status
	opts.Level = level
	opts.Unit = unit
	opts.Match = match
	return opts, nil
}
