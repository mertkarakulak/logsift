// logsift — read nginx, journald and syslog output; filter and summarise it.
// See README.md for usage and supported formats.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mertkarakulak/logsift/internal/cmdgrep"
	"github.com/mertkarakulak/logsift/internal/cmdsummary"
)

const usage = `logsift — filter and summarise nginx / journald / syslog output

Usage:
  logsift summary [flags] [file ...]
  logsift grep    [flags] [file ...]

Run "logsift <command> -help" for command-specific flags.

Supported input formats (auto-detected per line, or set with --format):
  nginx        nginx access log in default "combined" format
  nginx-error  nginx error log
  journald     journalctl -o json output
  syslog       RFC3164 plain syslog or PRI-prefixed syslog

Files may be glob patterns (e.g. /var/log/nginx/access.log*) and may be
gzip-compressed (detected by .gz suffix). If no files are given, stdin is read.
`

func main() {
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }

	if len(os.Args) < 2 {
		flag.Usage()
		os.Exit(2)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "summary":
		if err := cmdsummary.Run(args); err != nil {
			fmt.Fprintln(os.Stderr, "logsift summary:", err)
			os.Exit(2)
		}
	case "grep":
		rc, err := cmdgrep.Run(args)
		if err != nil {
			fmt.Fprintln(os.Stderr, "logsift grep:", err)
			os.Exit(2)
		}
		os.Exit(rc)
	case "-h", "--help", "help":
		flag.Usage()
	default:
		fmt.Fprintf(os.Stderr, "logsift: unknown command %q\n\n", cmd)
		flag.Usage()
		os.Exit(2)
	}
}
