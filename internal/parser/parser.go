// Package parser defines the LogEntry type, the Parser interface, and format
// auto-detection logic used across all logsift commands.
package parser

import (
	"strings"
	"time"
)

// Format names accepted by --format.
const (
	FormatNginx      = "nginx"
	FormatNginxError = "nginx-error"
	FormatJournald   = "journald"
	FormatSyslog     = "syslog"
	FormatUnknown    = "unknown"
)

// LogEntry is the normalised representation of a single log line regardless of
// origin format. Fields that are not applicable to a given format are left at
// their zero value.
type LogEntry struct {
	Time         time.Time
	Level        string // "info","notice","warn","err","crit","alert","emerg","debug"
	Status       int    // HTTP status code; 0 when not applicable
	Method       string // HTTP method
	Path         string // HTTP request path
	Bytes        int64  // response body bytes; -1 when absent
	UpstreamTime float64 // seconds; -1 when absent (custom log_format only)
	RemoteAddr   string
	UserAgent    string
	Unit         string // systemd unit from journald _SYSTEMD_UNIT
	Message      string // log message (may be combined from several fields)
	Raw          string // original untransformed line
	Format       string // which parser produced this entry
}

// Parser turns a raw log line into a LogEntry. The second return value is false
// when the line cannot be parsed by this parser.
type Parser interface {
	Parse(line string) (LogEntry, bool)
}

// Detect attempts to identify the format of line using cheap prefix heuristics
// so that callers can pick the right parser without full parsing. The returned
// string is one of the Format* constants.
func Detect(line string) string {
	line = strings.TrimSpace(line)
	if len(line) == 0 {
		return FormatUnknown
	}
	// journald JSON: starts with '{'
	if line[0] == '{' {
		return FormatJournald
	}
	// nginx error: "YYYY/MM/DD HH:MM:SS [level] ..."
	if len(line) >= 20 {
		if line[4] == '/' && line[7] == '/' && line[10] == ' ' {
			return FormatNginxError
		}
	}
	// syslog RFC3164: "Mon DD HH:MM:SS hostname ..."
	// Also syslog RFC5424 / rsyslog with PRI prefix: "<N>..."
	if line[0] == '<' {
		return FormatSyslog
	}
	months := [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun",
		"Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}
	if len(line) >= 3 {
		prefix := line[:3]
		for _, m := range months {
			if prefix == m {
				return FormatSyslog
			}
		}
	}
	// nginx access (combined): starts with an IP or IPv6 address followed by " - "
	if isDigitOrColon(line[0]) {
		return FormatNginx
	}
	return FormatUnknown
}

func isDigitOrColon(b byte) bool {
	return (b >= '0' && b <= '9') || b == ':'
}

// Auto returns a Parser that tries each of the four parsers in the order
// nginx-error, journald, syslog, nginx-access. Lines that match none are
// returned with Format == FormatUnknown and the original line in Raw.
func Auto() Parser { return &autoParser{} }

type autoParser struct{}

func (a *autoParser) Parse(line string) (LogEntry, bool) {
	switch Detect(line) {
	case FormatJournald:
		var p JournaldParser
		if e, ok := p.Parse(line); ok {
			return e, true
		}
	case FormatNginxError:
		var p NginxErrorParser
		if e, ok := p.Parse(line); ok {
			return e, true
		}
	case FormatSyslog:
		var p SyslogParser
		if e, ok := p.Parse(line); ok {
			return e, true
		}
	case FormatNginx:
		var p NginxParser
		if e, ok := p.Parse(line); ok {
			return e, true
		}
	}
	// Return an unparsed entry so callers still have the raw line.
	return LogEntry{Raw: line, Format: FormatUnknown, Bytes: -1, UpstreamTime: -1}, false
}

// ForName returns a Parser for the explicitly-named format, or nil when the
// name is not recognised.
func ForName(name string) Parser {
	switch name {
	case FormatNginx:
		return &NginxParser{}
	case FormatNginxError:
		return &NginxErrorParser{}
	case FormatJournald:
		return &JournaldParser{}
	case FormatSyslog:
		return &SyslogParser{}
	}
	return nil
}
