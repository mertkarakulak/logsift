package parser

import (
	"regexp"
	"strings"
	"time"
)

// nginxErrorRe parses nginx error log lines:
//
//	2024/01/15 12:34:56 [error] 1234#5678: *123 message, client: 1.2.3.4, ...
var nginxErrorRe = regexp.MustCompile(
	`^(\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}) \[(\w+)\] \d+#\d+: (.+)$`,
)

// nginxErrorClientRe extracts the client IP from the trailing metadata fields.
var nginxErrorClientRe = regexp.MustCompile(`client: ([^,\s]+)`)

// NginxErrorParser parses nginx error log lines.
type NginxErrorParser struct{}

// Parse implements Parser.
func (p *NginxErrorParser) Parse(line string) (LogEntry, bool) {
	m := nginxErrorRe.FindStringSubmatch(line)
	if m == nil {
		return LogEntry{}, false
	}
	// m[1] = timestamp, m[2] = level, m[3] = rest

	ts, err := time.ParseInLocation("2006/01/02 15:04:05", m[1], time.Local)
	if err != nil {
		return LogEntry{}, false
	}

	level := mapNginxLevel(m[2])
	msg := m[3]

	var remoteAddr string
	if cm := nginxErrorClientRe.FindStringSubmatch(msg); cm != nil {
		remoteAddr = cm[1]
	}

	return LogEntry{
		Time:         ts,
		Level:        level,
		Bytes:        -1,
		UpstreamTime: -1,
		RemoteAddr:   remoteAddr,
		Message:      msg,
		Raw:          line,
		Format:       FormatNginxError,
	}, true
}

// mapNginxLevel maps nginx level names to logsift canonical level names.
func mapNginxLevel(s string) string {
	switch strings.ToLower(s) {
	case "emerg":
		return "emerg"
	case "alert":
		return "alert"
	case "crit":
		return "crit"
	case "error":
		return "err"
	case "warn":
		return "warn"
	case "notice":
		return "notice"
	case "info":
		return "info"
	case "debug":
		return "debug"
	}
	return s
}
