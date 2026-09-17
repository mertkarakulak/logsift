package summary

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mertkarakulak/logsift/internal/parser"
)

func makeEntry(status int, path, ip string, ts time.Time) parser.LogEntry {
	level := "info"
	if status >= 500 {
		level = "err"
	} else if status >= 400 {
		level = "warn"
	}
	return parser.LogEntry{
		Time:       ts,
		Level:      level,
		Status:     status,
		Path:       path,
		RemoteAddr: ip,
		Message:    "GET " + path + " → " + string(rune('0'+status/100)) + "xx",
		Format:     parser.FormatNginx,
		Bytes:       -1,
		UpstreamTime: -1,
	}
}

func TestStatsAdd(t *testing.T) {
	now := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	s := New()

	entries := []parser.LogEntry{
		makeEntry(200, "/api/users", "1.1.1.1", now),
		makeEntry(200, "/api/users", "1.1.1.1", now.Add(30*time.Second)),
		makeEntry(404, "/missing", "2.2.2.2", now.Add(time.Minute)),
		makeEntry(500, "/api/payment", "3.3.3.3", now.Add(2*time.Minute)),
		makeEntry(500, "/api/payment", "3.3.3.3", now.Add(3*time.Minute)),
		makeEntry(302, "/old", "1.1.1.1", now.Add(4*time.Minute)),
	}

	for _, e := range entries {
		s.Add(e, true)
	}
	// Add an unparsed line.
	s.Add(parser.LogEntry{Raw: "garbage line"}, false)

	if s.Total != 7 {
		t.Errorf("Total=%d want 7", s.Total)
	}
	if s.Unparsed != 1 {
		t.Errorf("Unparsed=%d want 1", s.Unparsed)
	}
	if s.S2xx != 2 {
		t.Errorf("S2xx=%d want 2", s.S2xx)
	}
	if s.S3xx != 1 {
		t.Errorf("S3xx=%d want 1", s.S3xx)
	}
	if s.S4xx != 1 {
		t.Errorf("S4xx=%d want 1", s.S4xx)
	}
	if s.S5xx != 2 {
		t.Errorf("S5xx=%d want 2", s.S5xx)
	}
	if s.PathBy5xx["/api/payment"] != 2 {
		t.Errorf("PathBy5xx[/api/payment]=%d want 2", s.PathBy5xx["/api/payment"])
	}
	if s.IPCounts["1.1.1.1"] != 3 {
		t.Errorf("IPCounts[1.1.1.1]=%d want 3 (2 nginx + 1 redirect)", s.IPCounts["1.1.1.1"])
	}
}

func TestTopN(t *testing.T) {
	m := map[string]int64{
		"a": 5,
		"b": 10,
		"c": 3,
		"d": 10,
		"e": 1,
	}
	got := topN(m, 3)
	if len(got) != 3 {
		t.Fatalf("len=%d want 3", len(got))
	}
	if got[0].v != 10 {
		t.Errorf("top[0].v=%d want 10", got[0].v)
	}
	// Both b and d have 10; they should be sorted by key.
	if got[0].k != "b" {
		t.Errorf("top[0].k=%q want b", got[0].k)
	}
	if got[1].k != "d" {
		t.Errorf("top[1].k=%q want d", got[1].k)
	}
}

func TestPeakRPM(t *testing.T) {
	s := New()
	base := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	// Add 5 entries in minute 0, 2 in minute 1.
	for i := 0; i < 5; i++ {
		s.Add(makeEntry(200, "/", "1.1.1.1", base.Add(time.Duration(i)*10*time.Second)), true)
	}
	for i := 0; i < 2; i++ {
		s.Add(makeEntry(200, "/", "1.1.1.1", base.Add(time.Minute+time.Duration(i)*10*time.Second)), true)
	}
	rpm, at := s.PeakRPM()
	if rpm != 5 {
		t.Errorf("PeakRPM=%d want 5", rpm)
	}
	if at.Before(base) || at.After(base.Add(time.Minute)) {
		t.Errorf("PeakRPM at=%v not in first minute", at)
	}
}

func TestPrint(t *testing.T) {
	s := New()
	now := time.Now()
	s.Add(makeEntry(200, "/", "1.1.1.1", now), true)
	s.Add(makeEntry(500, "/api/pay", "2.2.2.2", now), true)

	var buf bytes.Buffer
	Print(&buf, s, 10)
	out := buf.String()

	if !strings.Contains(out, "logsift summary") {
		t.Error("output missing header")
	}
	if !strings.Contains(out, "2xx") {
		t.Error("output missing 2xx")
	}
	if !strings.Contains(out, "5xx") {
		t.Error("output missing 5xx")
	}
}

func TestPrintJSON(t *testing.T) {
	s := New()
	now := time.Now()
	s.Add(makeEntry(200, "/", "1.1.1.1", now), true)
	s.Add(makeEntry(500, "/api/pay", "2.2.2.2", now), true)

	var buf bytes.Buffer
	if err := PrintJSON(&buf, s, 10); err != nil {
		t.Fatalf("PrintJSON: %v", err)
	}

	var out JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Total != 2 {
		t.Errorf("Total=%d want 2", out.Total)
	}
	if out.Status.S2xx != 1 {
		t.Errorf("S2xx=%d want 1", out.Status.S2xx)
	}
	if out.Status.S5xx != 1 {
		t.Errorf("S5xx=%d want 1", out.Status.S5xx)
	}
}
