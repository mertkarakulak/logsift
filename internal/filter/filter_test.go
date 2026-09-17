package filter

import (
	"testing"
	"time"

	"github.com/mertkarakulak/logsift/internal/parser"
)

func TestMatchStatus(t *testing.T) {
	tests := []struct {
		code int
		spec string
		want bool
	}{
		{200, "2xx", true},
		{201, "2xx", true},
		{301, "3xx", true},
		{404, "4xx", true},
		{500, "5xx", true},
		{502, "5xx", true},
		{200, "5xx", false},
		{404, "404", true},
		{405, "404", false},
		{0, "2xx", false},
	}
	for _, tt := range tests {
		got := matchStatus(tt.code, tt.spec)
		if got != tt.want {
			t.Errorf("matchStatus(%d,%q)=%v want %v", tt.code, tt.spec, got, tt.want)
		}
	}
}

func TestCompiledMatch(t *testing.T) {
	now := time.Now()
	base := parser.LogEntry{
		Time:       now,
		Level:      "err",
		Status:     500,
		RemoteAddr: "1.2.3.4",
		Unit:       "nginx.service",
		Message:    "something failed",
		Raw:        "raw line text",
	}

	t.Run("no filters passes everything", func(t *testing.T) {
		c, err := Compile(Options{})
		if err != nil {
			t.Fatal(err)
		}
		if !c.Match(base) {
			t.Error("expected match")
		}
	})

	t.Run("since filter future excludes", func(t *testing.T) {
		c, _ := Compile(Options{Since: now.Add(1 * time.Hour)})
		if c.Match(base) {
			t.Error("expected no match")
		}
	})

	t.Run("since filter past includes", func(t *testing.T) {
		c, _ := Compile(Options{Since: now.Add(-1 * time.Hour)})
		if !c.Match(base) {
			t.Error("expected match")
		}
	})

	t.Run("status 5xx matches 500", func(t *testing.T) {
		c, _ := Compile(Options{Status: "5xx"})
		if !c.Match(base) {
			t.Error("expected match")
		}
	})

	t.Run("status 4xx excludes 500", func(t *testing.T) {
		c, _ := Compile(Options{Status: "4xx"})
		if c.Match(base) {
			t.Error("expected no match")
		}
	})

	t.Run("level err matches err entry", func(t *testing.T) {
		c, _ := Compile(Options{Level: "err"})
		if !c.Match(base) {
			t.Error("expected match")
		}
	})

	t.Run("level err also matches crit (higher severity)", func(t *testing.T) {
		c, _ := Compile(Options{Level: "err"})
		crit := base
		crit.Level = "crit"
		if !c.Match(crit) {
			t.Error("expected crit to pass --level err filter")
		}
	})

	t.Run("level err excludes info", func(t *testing.T) {
		c, _ := Compile(Options{Level: "err"})
		info := base
		info.Level = "info"
		if c.Match(info) {
			t.Error("expected info to be excluded by --level err")
		}
	})

	t.Run("unit substring match", func(t *testing.T) {
		c, _ := Compile(Options{Unit: "nginx"})
		if !c.Match(base) {
			t.Error("expected match")
		}
	})

	t.Run("unit no match", func(t *testing.T) {
		c, _ := Compile(Options{Unit: "postgres"})
		if c.Match(base) {
			t.Error("expected no match")
		}
	})

	t.Run("match regex hit", func(t *testing.T) {
		c, _ := Compile(Options{Match: "raw.*text"})
		if !c.Match(base) {
			t.Error("expected match")
		}
	})

	t.Run("match regex miss", func(t *testing.T) {
		c, _ := Compile(Options{Match: "^notfound"})
		if c.Match(base) {
			t.Error("expected no match")
		}
	})

	t.Run("invalid regex returns error", func(t *testing.T) {
		_, err := Compile(Options{Match: "["})
		if err == nil {
			t.Error("expected error for invalid regex")
		}
	})
}

func TestParseSince(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"1h", false},
		{"30m", false},
		{"2026-09-17T10:00:00Z", false},
		{"2026-09-17T10:00:00+03:00", false},
		{"2026-09-17T10:00:00", false},
		{"2026-09-17", false},
		{"notadate", true},
		{"1x", true},
	}
	for _, tt := range tests {
		_, err := ParseSince(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseSince(%q) err=%v wantErr=%v", tt.input, err, tt.wantErr)
		}
	}
}
