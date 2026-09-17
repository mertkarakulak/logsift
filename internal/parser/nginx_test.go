package parser

import (
	"testing"
)

func TestNginxParser(t *testing.T) {
	p := &NginxParser{}

	tests := []struct {
		name       string
		line       string
		wantOK     bool
		wantStatus int
		wantMethod string
		wantPath   string
		wantIP     string
		wantBytes  int64
	}{
		{
			name:       "GET 200",
			line:       `192.168.1.1 - - [15/Jan/2024:10:00:01 +0000] "GET /api/users HTTP/1.1" 200 1234 "-" "Mozilla/5.0"`,
			wantOK:     true,
			wantStatus: 200,
			wantMethod: "GET",
			wantPath:   "/api/users",
			wantIP:     "192.168.1.1",
			wantBytes:  1234,
		},
		{
			name:       "POST 201",
			line:       `192.168.1.2 - bob [15/Jan/2024:10:00:15 +0000] "POST /api/orders HTTP/1.1" 201 512 "https://example.com/" "curl/7.88.1"`,
			wantOK:     true,
			wantStatus: 201,
			wantMethod: "POST",
			wantPath:   "/api/orders",
			wantIP:     "192.168.1.2",
			wantBytes:  512,
		},
		{
			name:       "304 zero bytes",
			line:       `10.0.0.5 - - [15/Jan/2024:10:00:30 +0000] "GET /static/app.js HTTP/1.1" 304 0 "-" "Mozilla/5.0"`,
			wantOK:     true,
			wantStatus: 304,
			wantBytes:  0,
		},
		{
			name:       "bytes dash",
			line:       `10.0.0.5 - - [15/Jan/2024:10:00:30 +0000] "GET /static/app.js HTTP/1.1" 304 - "-" "Mozilla/5.0"`,
			wantOK:     true,
			wantStatus: 304,
			wantBytes:  -1,
		},
		{
			name:       "500",
			line:       `203.0.113.10 - - [15/Jan/2024:10:01:00 +0000] "GET /api/payment HTTP/1.1" 500 256 "-" "Go-http-client/1.1"`,
			wantOK:     true,
			wantStatus: 500,
		},
		{
			name:   "not nginx",
			line:   `2024/01/15 10:00:45 [error] 1234#5678: *101 something failed`,
			wantOK: false,
		},
		{
			name:   "empty",
			line:   ``,
			wantOK: false,
		},
		{
			name:       "with upstream time",
			line:       `192.168.1.1 - - [15/Jan/2024:10:00:01 +0000] "GET /api/users HTTP/1.1" 200 1234 "-" "Mozilla/5.0" 0.045`,
			wantOK:     true,
			wantStatus: 200,
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
			if e.Status != tt.wantStatus {
				t.Errorf("Status=%d want %d", e.Status, tt.wantStatus)
			}
			if tt.wantMethod != "" && e.Method != tt.wantMethod {
				t.Errorf("Method=%q want %q", e.Method, tt.wantMethod)
			}
			if tt.wantPath != "" && e.Path != tt.wantPath {
				t.Errorf("Path=%q want %q", e.Path, tt.wantPath)
			}
			if tt.wantIP != "" && e.RemoteAddr != tt.wantIP {
				t.Errorf("RemoteAddr=%q want %q", e.RemoteAddr, tt.wantIP)
			}
			if tt.wantBytes != 0 && e.Bytes != tt.wantBytes {
				t.Errorf("Bytes=%d want %d", e.Bytes, tt.wantBytes)
			}
			if e.Time.IsZero() {
				t.Error("Time is zero")
			}
			if e.Format != FormatNginx {
				t.Errorf("Format=%q want %q", e.Format, FormatNginx)
			}
		})
	}
}

func TestNginxTimezone(t *testing.T) {
	p := &NginxParser{}
	line := `192.168.1.1 - - [15/Jan/2024:10:00:01 -0700] "GET / HTTP/1.1" 200 0 "-" "-"`
	e, ok := p.Parse(line)
	if !ok {
		t.Fatal("Parse failed")
	}
	_, offset := e.Time.Zone()
	if offset != -7*3600 {
		t.Errorf("offset=%d want %d", offset, -7*3600)
	}
}

func TestNginxUpstreamTime(t *testing.T) {
	p := &NginxParser{}
	line := `192.168.1.1 - - [15/Jan/2024:10:00:01 +0000] "GET / HTTP/1.1" 200 100 "-" "agent" 1.234`
	e, ok := p.Parse(line)
	if !ok {
		t.Fatal("Parse failed")
	}
	if e.UpstreamTime < 1.233 || e.UpstreamTime > 1.235 {
		t.Errorf("UpstreamTime=%f want ~1.234", e.UpstreamTime)
	}
}

func TestNginxFixture(t *testing.T) {
	lines := fixtureLines(t, "../../testdata/nginx_access.log")
	p := &NginxParser{}
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
		t.Error("no lines parsed from fixture")
	}
	t.Logf("parsed %d/%d lines", ok, ok+fail)
}

func TestDetect(t *testing.T) {
	tests := []struct {
		line string
		want string
	}{
		{`192.168.1.1 - - [15/Jan/2024:10:00:01 +0000] "GET / HTTP/1.1" 200 0 "-" "-"`, FormatNginx},
		{`2024/01/15 10:00:45 [error] 1234#5678: *101 failed`, FormatNginxError},
		{`{"__REALTIME_TIMESTAMP":"1705312801000000","MESSAGE":"hi","PRIORITY":"6"}`, FormatJournald},
		{`Jan 15 10:00:01 web01 systemd[1]: Starting`, FormatSyslog},
		{`<190>Jan 15 10:01:30 web01 sshd[2222]: Failed password`, FormatSyslog},
	}
	for _, tt := range tests {
		got := Detect(tt.line)
		if got != tt.want {
			t.Errorf("Detect(%q...) = %q want %q", tt.line[:min(30, len(tt.line))], got, tt.want)
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
