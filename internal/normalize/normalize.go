// Package normalize provides message normalization for log deduplication.
// Similar log messages that differ only in dynamic values (IPs, IDs, counters)
// are collapsed into a single representative form so that summary counts are
// meaningful.
package normalize

import "regexp"

// Replacements are applied in order. More-specific patterns must come before
// less-specific ones to avoid shredding longer tokens (e.g. a UUID) with a
// digit-replacement that runs first.
var replacements = []struct {
	re   *regexp.Regexp
	repl string
}{
	// UUID (8-4-4-4-12 hex digits, case-insensitive)
	{regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`), `<UUID>`},
	// IPv6 address (simplified: colon-separated hex groups)
	{regexp.MustCompile(`(?i)(?:[0-9a-f]{0,4}:){2,7}[0-9a-f]{0,4}`), `<IPv6>`},
	// IPv4 address (with optional port :N)
	{regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(?::\d+)?\b`), `<IP>`},
	// Long hex strings (8+ hex chars, e.g. object IDs, hashes)
	{regexp.MustCompile(`(?i)0x[0-9a-f]{4,}`), `<HEX>`},
	{regexp.MustCompile(`(?i)\b[0-9a-f]{8,}\b`), `<HEX>`},
	// Numbers (integers and floats), after the structural patterns above
	{regexp.MustCompile(`\b\d+(?:\.\d+)?\b`), `<NUM>`},
}

// Message normalizes a log message string by replacing dynamic tokens with
// stable placeholders, making similar messages sort together for top-K counting.
func Message(s string) string {
	for _, r := range replacements {
		s = r.re.ReplaceAllString(s, r.repl)
	}
	return s
}
