package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// syslogRFC3164Re parses classic BSD syslog lines:
//
//	Jan 15 12:34:56 hostname program[pid]: message
//	Jan  5 12:34:56 hostname program: message
var syslogRFC3164Re = regexp.MustCompile(
	`^([A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+(\S+)\s+(\S+?)(?:\[(\d+)\])?:\s*(.*)$`,
)

// syslogPRIRe parses lines with a PRI prefix <N>:
//
//	<190>Jan 15 12:34:56 hostname program[pid]: message
//	<190>2024-01-15T12:34:56.000Z hostname program[pid]: message
var syslogPRIRe = regexp.MustCompile(`^<(\d+)>(.+)$`)

// syslogLevelNames maps syslog priority (0-7) to canonical level names.
var syslogLevelNames = [8]string{
	"emerg", "alert", "crit", "err", "warn", "notice", "info", "debug",
}

// SyslogParser parses plain syslog lines (RFC3164 and PRI-prefixed variants).
//
// Timestamps carry no year and no offset; they are interpreted in time.Local.
// The year is inferred as the current year; if that produces a time more than
// 24 hours in the future the previous year is used instead.
type SyslogParser struct{}

// Parse implements Parser.
func (p *SyslogParser) Parse(line string) (LogEntry, bool) {
	line = strings.TrimSpace(line)

	var pri int = -1
	rest := line

	// Strip optional PRI prefix.
	if m := syslogPRIRe.FindStringSubmatch(line); m != nil {
		n, err := strconv.Atoi(m[1])
		if err == nil {
			pri = n & 7 // low 3 bits = severity
		}
		rest = m[2]
	}

	m := syslogRFC3164Re.FindStringSubmatch(rest)
	if m == nil {
		return LogEntry{}, false
	}
	// m[1]=timestamp m[2]=host m[3]=program m[4]=pid(opt) m[5]=message

	ts, err := parseSyslogTimestamp(m[1])
	if err != nil {
		return LogEntry{}, false
	}

	level := "info"
	if pri >= 0 && pri <= 7 {
		level = syslogLevelNames[pri]
	}

	return LogEntry{
		Time:         ts,
		Level:        level,
		Bytes:        -1,
		UpstreamTime: -1,
		Message:      m[5],
		Raw:          line,
		Format:       FormatSyslog,
	}, true
}

// parseSyslogTimestamp parses "Jan 15 12:34:56" style timestamps using
// time.Local, inferring the year.
func parseSyslogTimestamp(s string) (time.Time, error) {
	// Normalise varying-width day: "Jan  5" → "Jan 05"
	s = strings.TrimSpace(s)
	now := time.Now()

	candidate := fmt.Sprintf("%d %s", now.Year(), s)
	ts, err := time.ParseInLocation("2006 Jan 2 15:04:05", candidate, time.Local)
	if err != nil {
		// try two-digit day
		ts, err = time.ParseInLocation("2006 Jan 02 15:04:05", candidate, time.Local)
		if err != nil {
			return time.Time{}, err
		}
	}
	// If the parsed time is more than 24 h in the future, use previous year.
	if ts.Sub(now) > 24*time.Hour {
		ts = ts.AddDate(-1, 0, 0)
	}
	return ts, nil
}
