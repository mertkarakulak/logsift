// Package summary aggregates parsed log entries and renders a human-readable or
// JSON summary report.
package summary

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/mertkarakulak/logsift/internal/normalize"
	"github.com/mertkarakulak/logsift/internal/parser"
)

// Stats accumulates aggregated data from a stream of log entries.
//
// Memory note: Stats uses unbounded maps keyed by path, IP, and normalised
// message. For very large log files with high-cardinality paths or IPs the
// maps may grow proportionally to the number of distinct values. This is an
// accepted trade-off documented in the README.
type Stats struct {
	First    time.Time
	Last     time.Time
	Total    int64
	Unparsed int64

	// Status class counts
	S2xx int64
	S3xx int64
	S4xx int64
	S5xx int64

	FormatCounts map[string]int64

	// Top-K counters (full maps; see memory note above)
	PathBy5xx    map[string]int64
	IPCounts     map[string]int64
	ErrorMsgs    map[string]int64 // keyed by normalised message

	// RPM tracking: request count per minute bucket
	minuteBuckets map[int64]int64 // unix minute → count
}

// New returns a zero-value Stats ready to accept entries.
func New() *Stats {
	return &Stats{
		FormatCounts:  make(map[string]int64),
		PathBy5xx:     make(map[string]int64),
		IPCounts:      make(map[string]int64),
		ErrorMsgs:     make(map[string]int64),
		minuteBuckets: make(map[int64]int64),
	}
}

// Add incorporates one log entry (which may be unparsed).
func (s *Stats) Add(e parser.LogEntry, parsed bool) {
	s.Total++
	if !parsed {
		s.Unparsed++
		return
	}

	if s.First.IsZero() || e.Time.Before(s.First) {
		s.First = e.Time
	}
	if e.Time.After(s.Last) {
		s.Last = e.Time
	}

	s.FormatCounts[e.Format]++

	switch {
	case e.Status >= 500:
		s.S5xx++
		if e.Path != "" {
			s.PathBy5xx[e.Path]++
		}
	case e.Status >= 400:
		s.S4xx++
	case e.Status >= 300:
		s.S3xx++
	case e.Status >= 200:
		s.S2xx++
	}

	if e.RemoteAddr != "" {
		s.IPCounts[e.RemoteAddr]++
	}

	if e.Level == "err" || e.Level == "crit" || e.Level == "alert" || e.Level == "emerg" ||
		e.Status >= 400 {
		if e.Message != "" {
			s.ErrorMsgs[normalize.Message(e.Message)]++
		}
	}

	if !e.Time.IsZero() {
		bucket := e.Time.Unix() / 60
		s.minuteBuckets[bucket]++
	}
}

// PeakRPM returns the peak requests-per-minute and the minute it occurred.
func (s *Stats) PeakRPM() (rpm int64, at time.Time) {
	for bucket, count := range s.minuteBuckets {
		if count > rpm {
			rpm = count
			at = time.Unix(bucket*60, 0)
		}
	}
	return
}

// topN returns the top n entries from a map[string]int64, sorted descending by value.
func topN(m map[string]int64, n int) []kv {
	kvs := make([]kv, 0, len(m))
	for k, v := range m {
		kvs = append(kvs, kv{k, v})
	}
	sort.Slice(kvs, func(i, j int) bool {
		if kvs[i].v != kvs[j].v {
			return kvs[i].v > kvs[j].v
		}
		return kvs[i].k < kvs[j].k
	})
	if n > 0 && len(kvs) > n {
		kvs = kvs[:n]
	}
	return kvs
}

type kv struct {
	k string
	v int64
}

// Print writes a human-readable summary to w.
func Print(w io.Writer, s *Stats, topK int) {
	fmt.Fprintln(w, "=== logsift summary ===")

	if !s.First.IsZero() {
		fmt.Fprintf(w, "Time range : %s → %s\n",
			s.First.Format("2006-01-02 15:04:05"),
			s.Last.Format("2006-01-02 15:04:05"),
		)
	}
	fmt.Fprintf(w, "Total lines: %d\n", s.Total)
	if s.Unparsed > 0 {
		fmt.Fprintf(w, "Unparsed   : %d\n", s.Unparsed)
	}

	if len(s.FormatCounts) > 0 {
		parts := make([]string, 0, len(s.FormatCounts))
		for f, n := range s.FormatCounts {
			parts = append(parts, fmt.Sprintf("%s(%d)", f, n))
		}
		sort.Strings(parts)
		fmt.Fprintf(w, "Formats    : %s\n", strings.Join(parts, "  "))
	}

	httpTotal := s.S2xx + s.S3xx + s.S4xx + s.S5xx
	if httpTotal > 0 {
		fmt.Fprintln(w, "\nStatus classes:")
		printBar(w, "2xx", s.S2xx, httpTotal)
		printBar(w, "3xx", s.S3xx, httpTotal)
		printBar(w, "4xx", s.S4xx, httpTotal)
		printBar(w, "5xx", s.S5xx, httpTotal)
	}

	if len(s.PathBy5xx) > 0 {
		fmt.Fprintf(w, "\nTop %d paths by 5xx:\n", topK)
		for _, item := range topN(s.PathBy5xx, topK) {
			fmt.Fprintf(w, "  %-50s %d\n", item.k, item.v)
		}
	}

	if len(s.IPCounts) > 0 {
		fmt.Fprintf(w, "\nTop %d IPs by requests:\n", topK)
		for _, item := range topN(s.IPCounts, topK) {
			fmt.Fprintf(w, "  %-45s %d\n", item.k, item.v)
		}
	}

	if len(s.ErrorMsgs) > 0 {
		fmt.Fprintf(w, "\nTop %d error messages (normalised):\n", topK)
		for _, item := range topN(s.ErrorMsgs, topK) {
			msg := item.k
			if len(msg) > 80 {
				msg = msg[:77] + "..."
			}
			fmt.Fprintf(w, "  %-80s %d\n", msg, item.v)
		}
	}

	if rpm, at := s.PeakRPM(); rpm > 0 {
		fmt.Fprintf(w, "\nPeak RPM   : %d (at %s)\n", rpm, at.Format("15:04"))
	}
}

func printBar(w io.Writer, label string, count, total int64) {
	pct := 0.0
	if total > 0 {
		pct = float64(count) / float64(total) * 100
	}
	fmt.Fprintf(w, "  %s  %7d  (%5.1f%%)\n", label, count, pct)
}

// JSONOutput is the JSON representation of the summary.
type JSONOutput struct {
	TimeRange struct {
		First string `json:"first"`
		Last  string `json:"last"`
	} `json:"time_range"`
	Total    int64            `json:"total_lines"`
	Unparsed int64            `json:"unparsed,omitempty"`
	Formats  map[string]int64 `json:"formats,omitempty"`
	Status   struct {
		S2xx int64 `json:"2xx"`
		S3xx int64 `json:"3xx"`
		S4xx int64 `json:"4xx"`
		S5xx int64 `json:"5xx"`
	} `json:"status_classes"`
	TopPaths   []kvJSON `json:"top_paths_by_5xx,omitempty"`
	TopIPs     []kvJSON `json:"top_ips,omitempty"`
	TopErrors  []kvJSON `json:"top_error_messages,omitempty"`
	PeakRPM    int64    `json:"peak_rpm"`
	PeakRPMAt  string   `json:"peak_rpm_at,omitempty"`
}

type kvJSON struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

// PrintJSON writes a JSON summary to w.
func PrintJSON(w io.Writer, s *Stats, topK int) error {
	out := JSONOutput{
		Total:    s.Total,
		Unparsed: s.Unparsed,
		Formats:  s.FormatCounts,
	}
	if !s.First.IsZero() {
		out.TimeRange.First = s.First.Format(time.RFC3339)
		out.TimeRange.Last = s.Last.Format(time.RFC3339)
	}
	out.Status.S2xx = s.S2xx
	out.Status.S3xx = s.S3xx
	out.Status.S4xx = s.S4xx
	out.Status.S5xx = s.S5xx

	for _, item := range topN(s.PathBy5xx, topK) {
		out.TopPaths = append(out.TopPaths, kvJSON{item.k, item.v})
	}
	for _, item := range topN(s.IPCounts, topK) {
		out.TopIPs = append(out.TopIPs, kvJSON{item.k, item.v})
	}
	for _, item := range topN(s.ErrorMsgs, topK) {
		out.TopErrors = append(out.TopErrors, kvJSON{item.k, item.v})
	}

	rpm, at := s.PeakRPM()
	out.PeakRPM = rpm
	if !at.IsZero() {
		out.PeakRPMAt = at.Format(time.RFC3339)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
