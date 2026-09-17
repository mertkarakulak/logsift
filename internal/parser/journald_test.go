package parser

import (
	"testing"
	"time"
)

func TestJournaldParser(t *testing.T) {
	p := &JournaldParser{}

	tests := []struct {
		name      string
		line      string
		wantOK    bool
		wantLevel string
		wantUnit  string
		wantMsg   string
		wantTimeUS int64 // expected unix microseconds
	}{
		{
			name:       "info line",
			line:       `{"__REALTIME_TIMESTAMP":"1705312801000000","MESSAGE":"Started nginx.service","PRIORITY":"6","_SYSTEMD_UNIT":"nginx.service"}`,
			wantOK:     true,
			wantLevel:  "info",
			wantUnit:   "nginx.service",
			wantMsg:    "Started nginx.service",
			wantTimeUS: 1705312801000000,
		},
		{
			name:      "error line",
			line:      `{"__REALTIME_TIMESTAMP":"1705312805000000","MESSAGE":"connect() failed","PRIORITY":"3","_SYSTEMD_UNIT":"nginx.service"}`,
			wantOK:    true,
			wantLevel: "err",
			wantUnit:  "nginx.service",
		},
		{
			name:      "crit line priority 2",
			line:      `{"__REALTIME_TIMESTAMP":"1705312815000000","MESSAGE":"OOM kill","PRIORITY":"2","_SYSTEMD_UNIT":"kernel"}`,
			wantOK:    true,
			wantLevel: "crit",
		},
		{
			name:   "no timestamp",
			line:   `{"MESSAGE":"no timestamp","PRIORITY":"6"}`,
			wantOK: false,
		},
		{
			name:   "invalid json",
			line:   `not json at all`,
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, ok := p.Parse(tt.line)
			if ok != tt.wantOK {
				t.Fatalf("Parse() ok=%v want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if e.Level != tt.wantLevel {
				t.Errorf("Level=%q want %q", e.Level, tt.wantLevel)
			}
			if tt.wantUnit != "" && e.Unit != tt.wantUnit {
				t.Errorf("Unit=%q want %q", e.Unit, tt.wantUnit)
			}
			if tt.wantMsg != "" && e.Message != tt.wantMsg {
				t.Errorf("Message=%q want %q", e.Message, tt.wantMsg)
			}
			if tt.wantTimeUS != 0 {
				want := time.UnixMicro(tt.wantTimeUS)
				if !e.Time.Equal(want) {
					t.Errorf("Time=%v want %v", e.Time, want)
				}
			}
			if e.Format != FormatJournald {
				t.Errorf("Format=%q want %q", e.Format, FormatJournald)
			}
		})
	}
}

func TestJournaldPriorityThreshold(t *testing.T) {
	tests := []struct {
		name string
		want int
	}{
		{"emerg", 0},
		{"alert", 1},
		{"crit", 2},
		{"err", 3},
		{"error", 3},
		{"warn", 4},
		{"warning", 4},
		{"notice", 5},
		{"info", 6},
		{"debug", 7},
		{"unknown", -1},
	}
	for _, tt := range tests {
		got := PriorityThreshold(tt.name)
		if got != tt.want {
			t.Errorf("PriorityThreshold(%q)=%d want %d", tt.name, got, tt.want)
		}
	}
}

func TestJournaldFixture(t *testing.T) {
	lines := fixtureLines(t, "../../testdata/journald.jsonl")
	p := &JournaldParser{}
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
		t.Error("no lines parsed from journald fixture")
	}
	t.Logf("parsed %d/%d lines", ok, ok+fail)
}
