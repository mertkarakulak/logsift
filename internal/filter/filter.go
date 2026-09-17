// Package filter provides predicate functions for log entries.
package filter

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mertkarakulak/logsift/internal/parser"
)

// Options holds all user-supplied filter criteria. Zero values mean "no filter
// on that dimension".
type Options struct {
	Since  time.Time // only entries at or after this time
	Until  time.Time // only entries before or at this time
	Status string    // "5xx", "4xx", "3xx", "2xx", "404", etc.
	Level  string    // "err", "warn", "info", … maps to priority threshold
	Unit   string    // systemd unit substring match
	Match  string    // regex applied to raw line
}

// Compiled holds pre-compiled filter state derived from Options.
type Compiled struct {
	opts      Options
	matchRe   *regexp.Regexp
	priThresh int // -1 means no level filter
}

// Compile pre-compiles regex in opts. Returns an error if opts.Match is not a
// valid regular expression.
func Compile(opts Options) (*Compiled, error) {
	c := &Compiled{opts: opts, priThresh: -1}
	if opts.Match != "" {
		re, err := regexp.Compile(opts.Match)
		if err != nil {
			return nil, fmt.Errorf("--match: %w", err)
		}
		c.matchRe = re
	}
	if opts.Level != "" {
		c.priThresh = parser.PriorityThreshold(opts.Level)
		if c.priThresh < 0 {
			return nil, fmt.Errorf("--level: unknown level %q (valid: emerg alert crit err warn notice info debug)", opts.Level)
		}
	}
	return c, nil
}

// Match returns true when entry passes all active filter criteria.
func (c *Compiled) Match(e parser.LogEntry) bool {
	if !c.opts.Since.IsZero() && e.Time.Before(c.opts.Since) {
		return false
	}
	if !c.opts.Until.IsZero() && e.Time.After(c.opts.Until) {
		return false
	}
	if c.opts.Status != "" && !matchStatus(e.Status, c.opts.Status) {
		return false
	}
	if c.priThresh >= 0 {
		// --level err means priority <= 3 (err and worse).
		// Entries with no level (empty string) are excluded when level filter is set.
		if e.Level == "" {
			return false
		}
		entryPri := parser.PriorityThreshold(e.Level)
		if entryPri < 0 || entryPri > c.priThresh {
			return false
		}
	}
	if c.opts.Unit != "" && !strings.Contains(e.Unit, c.opts.Unit) {
		return false
	}
	if c.matchRe != nil && !c.matchRe.MatchString(e.Raw) {
		return false
	}
	return true
}

// matchStatus checks whether statusCode satisfies the spec string.
// Spec may be "5xx", "4xx", "3xx", "2xx", or a specific code like "404".
func matchStatus(code int, spec string) bool {
	if code == 0 {
		return false
	}
	switch strings.ToLower(spec) {
	case "5xx":
		return code >= 500 && code < 600
	case "4xx":
		return code >= 400 && code < 500
	case "3xx":
		return code >= 300 && code < 400
	case "2xx":
		return code >= 200 && code < 300
	default:
		return fmt.Sprintf("%d", code) == spec
	}
}

// ParseSince parses a --since/--until value: a Go duration (e.g. "1h") is
// subtracted from now; an RFC3339 timestamp is returned as-is.
func ParseSince(s string) (time.Time, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return time.Now().Add(-d), nil
	}
	// Try RFC3339 with offset
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	// Try RFC3339 without timezone (treat as local)
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", s, time.Local); err == nil {
		return t, nil
	}
	// Try plain date
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q (use e.g. 1h, 30m, 2026-09-17T10:00:00Z)", s)
}
