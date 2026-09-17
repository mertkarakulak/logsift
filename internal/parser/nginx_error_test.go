package parser

import (
	"testing"
)

func TestNginxErrorParser(t *testing.T) {
	p := &NginxErrorParser{}

	tests := []struct {
		name      string
		line      string
		wantOK    bool
		wantLevel string
		wantIP    string
	}{
		{
			name:      "error with client",
			line:      `2024/01/15 10:00:45 [error] 1234#5678: *101 connect() failed (111: Connection refused) while connecting to upstream, client: 203.0.113.10, server: example.com`,
			wantOK:    true,
			wantLevel: "err",
			wantIP:    "203.0.113.10",
		},
		{
			name:      "warn",
			line:      `2024/01/15 10:01:30 [warn] 1234#5678: *103 upstream response time 2.345 exceeds configured timeout, client: 10.0.0.5`,
			wantOK:    true,
			wantLevel: "warn",
			wantIP:    "10.0.0.5",
		},
		{
			name:      "crit",
			line:      `2024/01/15 10:03:00 [crit] 1234#5678: *105 SSL_read() failed (104: Connection reset by peer), client: 198.51.100.1`,
			wantOK:    true,
			wantLevel: "crit",
		},
		{
			name:      "info no client",
			line:      `2024/01/15 10:02:00 [info] 1234#5678: signal process started`,
			wantOK:    true,
			wantLevel: "info",
			wantIP:    "",
		},
		{
			name:   "not nginx-error",
			line:   `192.168.1.1 - - [15/Jan/2024:10:00:01 +0000] "GET / HTTP/1.1" 200 0 "-" "-"`,
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
			if tt.wantIP != "" && e.RemoteAddr != tt.wantIP {
				t.Errorf("RemoteAddr=%q want %q", e.RemoteAddr, tt.wantIP)
			}
			if e.Time.IsZero() {
				t.Error("Time is zero")
			}
			if e.Format != FormatNginxError {
				t.Errorf("Format=%q want %q", e.Format, FormatNginxError)
			}
		})
	}
}

func TestNginxErrorFixture(t *testing.T) {
	lines := fixtureLines(t, "../../testdata/nginx_error.log")
	p := &NginxErrorParser{}
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
		t.Error("no lines parsed from nginx_error fixture")
	}
	t.Logf("parsed %d/%d lines", ok, ok+fail)
}
