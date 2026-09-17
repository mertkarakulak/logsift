package parser

import (
	"encoding/json"
	"strconv"
	"time"
)

// journaldPriorities maps syslog priority names to their numeric level (0=emerg).
var journaldPriorities = map[string]int{
	"emerg": 0, "emergency": 0,
	"alert":   1,
	"crit":    2, "critical": 2,
	"err":     3, "error": 3,
	"warn":    4, "warning": 4,
	"notice":  5,
	"info":    6,
	"debug":   7,
}

// journaldLevelNames maps numeric priority → canonical level name.
var journaldLevelNames = [8]string{
	"emerg", "alert", "crit", "err", "warn", "notice", "info", "debug",
}

// JournaldParser parses journald JSON output produced by `journalctl -o json`.
type JournaldParser struct{}

// Parse implements Parser.
func (p *JournaldParser) Parse(line string) (LogEntry, bool) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(line), &obj); err != nil {
		return LogEntry{}, false
	}

	var ts time.Time
	if raw, ok := obj["__REALTIME_TIMESTAMP"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			if us, err := strconv.ParseInt(s, 10, 64); err == nil {
				ts = time.UnixMicro(us)
			}
		}
	}
	if ts.IsZero() {
		return LogEntry{}, false
	}

	level := "info"
	if raw, ok := obj["PRIORITY"]; ok {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			if n, err := strconv.Atoi(s); err == nil && n >= 0 && n <= 7 {
				level = journaldLevelNames[n]
			}
		}
	}

	var msg string
	if raw, ok := obj["MESSAGE"]; ok {
		_ = json.Unmarshal(raw, &msg)
	}

	var unit string
	if raw, ok := obj["_SYSTEMD_UNIT"]; ok {
		_ = json.Unmarshal(raw, &unit)
	}

	return LogEntry{
		Time:         ts,
		Level:        level,
		Bytes:        -1,
		UpstreamTime: -1,
		Unit:         unit,
		Message:      msg,
		Raw:          line,
		Format:       FormatJournald,
	}, true
}

// PriorityThreshold returns the numeric priority threshold for a level name,
// e.g. "err" → 3. Returns -1 if the name is unrecognised.
func PriorityThreshold(name string) int {
	n, ok := journaldPriorities[name]
	if !ok {
		return -1
	}
	return n
}
