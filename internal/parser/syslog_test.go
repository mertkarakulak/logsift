package parser

import (
	"testing"
)

func TestSyslogParser(t *testing.T) {
	p := &SyslogParser{}

	tests := []struct {
		name      string
		line      string
		wantOK    bool
		wantLevel string
		wantMsg   string
	}{
		{
			name:      "plain syslog",
			line:      `Jan 15 10:00:01 web01 systemd[1]: Starting nginx.service...`,
			wantOK:    true,
			wantLevel: "info",
			wantMsg:   "Starting nginx.service...",
		},
		{
			name:      "single-space day",
			line:      `Jan  5 10:00:01 web01 crond[1234]: (root) CMD (/usr/lib/cron/run-crons)`,
			wantOK:    true,
			wantLevel: "info",
		},
		{
			name:      "PRI-prefixed local7.info",
			line:      `<190>Jan 15 10:01:30 web01 sshd[2222]: Failed password for invalid user admin from 203.0.113.10`,
			wantOK:    true,
			// 190 = 23*8+6 → facility 23, severity 6 (info)
			wantLevel: "info",
		},
		{
			name:      "PRI-prefixed kern.err",
			line:      `<11>Jan 15 10:02:00 web01 kernel: something bad`,
			wantOK:    true,
			// 11 = 1*8+3 → facility 1, severity 3 (err)
			wantLevel: "err",
		},
		{
			name:   "not syslog",
			line:   `{"__REALTIME_TIMESTAMP":"1705312801000000","MESSAGE":"hi"}`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ok := p.Parse(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("Parse(%q) ok=%v want %v", tt.line, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if e.Level != tt.wantLevel {
				t.Errorf("Level=%q want %q", e.Level, tt.wantLevel)
			}
			if tt.wantMsg != "" && e.Message != tt.wantMsg {
				t.Errorf("Message=%q want %q", e.Message, tt.wantMsg)
			}
			if e.Time.IsZero() {
				t.Error("Time is zero")
			}
			if e.Format != FormatSyslog {
				t.Errorf("Format=%q want %q", e.Format, FormatSyslog)
			}
		})
	}
}

func TestSyslogFixture(t *testing.T) {
	lines := fixtureLines(t, "../../testdata/syslog.log")
	p := &SyslogParser{}
	var ok, fail int
	for _, line := range lines {
		_, parsed := p.Parse(line)
		if parsed {
			ok++
		} else {
			fail++
		}
	}
	if ok == 0 {
		t.Error("no lines parsed from syslog fixture")
	}
	t.Logf("parsed %d/%d lines", ok, ok+fail)
}
