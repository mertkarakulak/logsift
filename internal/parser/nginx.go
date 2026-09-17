package parser

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// nginxCombinedRe parses the default nginx "combined" log_format:
//
//	'$remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent
//	 "$http_referer" "$http_user_agent"'
//
// An optional trailing field "$upstream_response_time" is supported when the
// site uses a custom log_format that appends it — it is NOT present in the
// default combined format.
var nginxCombinedRe = regexp.MustCompile(
	`^(\S+) - (\S+) \[([^\]]+)\] "([^"]*)" (\d{3}) (\d+|-) "([^"]*)" "([^"]*)"(?:\s+(\d+\.\d+|-|))?`,
)

// NginxParser parses nginx access log lines in the default "combined" format.
type NginxParser struct{}

// Parse implements Parser.
func (p *NginxParser) Parse(line string) (LogEntry, bool) {
	m := nginxCombinedRe.FindStringSubmatch(line)
	if m == nil {
		return LogEntry{}, false
	}
	// m indices:
	// 1 remote_addr  2 remote_user  3 time_local  4 request
	// 5 status       6 bytes        7 referer      8 user_agent
	// 9 upstream_time (optional, may be empty)

	ts, err := time.Parse("02/Jan/2006:15:04:05 -0700", m[3])
	if err != nil {
		return LogEntry{}, false
	}

	status, _ := strconv.Atoi(m[5])

	var bytes int64 = -1
	if m[6] != "-" {
		bytes, _ = strconv.ParseInt(m[6], 10, 64)
	}

	var upstream float64 = -1
	if m[9] != "" && m[9] != "-" {
		upstream, _ = strconv.ParseFloat(m[9], 64)
	}

	method, path := splitRequest(m[4])

	level := "info"
	if status >= 500 {
		level = "err"
	} else if status >= 400 {
		level = "warn"
	}

	msg := m[4] // "METHOD /path HTTP/x.y"
	if status > 0 {
		msg += " → " + m[5]
	}

	return LogEntry{
		Time:         ts,
		Level:        level,
		Status:       status,
		Method:       method,
		Path:         path,
		Bytes:        bytes,
		UpstreamTime: upstream,
		RemoteAddr:   m[1],
		UserAgent:    m[8],
		Message:      msg,
		Raw:          line,
		Format:       FormatNginx,
	}, true
}

// splitRequest splits a request field like "GET /path HTTP/1.1" into method
// and path. Returns empty strings on malformed input.
func splitRequest(req string) (method, path string) {
	parts := strings.SplitN(req, " ", 3)
	if len(parts) >= 2 {
		method = parts[0]
		path = parts[1]
	}
	return
}
