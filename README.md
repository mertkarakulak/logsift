# logsift

**Answer "what went wrong in the last hour" in one command.**

logsift reads nginx access/error logs, journald JSON output and plain syslog
lines, then filters and summarises them — streaming, low-overhead, no
dependencies beyond the Go standard library.

## Why

Running Linux servers means spending time staring at log files spread across
`/var/log/nginx/`, `journalctl`, and `/var/log/syslog`. logsift lets you pipe
them together, ask a time-bounded question (`--since 1h`) and get a ranked
summary of what actually broke — without loading the whole file into memory or
installing an ELK stack for a single VPS.

## Install

```sh
go install github.com/mertkarakulak/logsift@latest
```

Or build from source:

```sh
git clone https://github.com/mertkarakulak/logsift
cd logsift
go build -o logsift .
```

## Commands

### `logsift summary`

Parse one or more log files (or stdin) and print totals, status breakdown,
top paths/IPs/errors and peak RPM.

```
logsift summary [flags] [file ...]
```

**Flags**

| Flag | Description |
|---|---|
| `--since 1h` | Only entries at or after this time. Accepts a Go duration (`1h`, `30m`) or RFC3339 timestamp. |
| `--until <time>` | Only entries before or at this time. |
| `--status 5xx\|4xx\|3xx\|2xx\|404` | Filter by HTTP status class or exact code. |
| `--level err` | Filter by minimum severity: `emerg alert crit err warn notice info debug`. |
| `--unit nginx.service` | Filter by systemd unit (substring match; journald only). |
| `--match <regex>` | Filter by regex applied to raw line. |
| `--format nginx\|nginx-error\|journald\|syslog` | Force a parser instead of auto-detection. |
| `--top N` | Number of top entries per section (default 10). |
| `--json` | Output the summary as JSON. |

### `logsift grep`

Print matching log lines. Accepts the same filter flags as `summary`.
Exit codes follow grep convention: 0 = matches found, 1 = no matches, 2 = error.

```
logsift grep [flags] [file ...]
```

## Usage examples

### Summarise the last hour of nginx traffic

```sh
logsift summary --since 1h /var/log/nginx/access.log*
```

### What went wrong on this server today

```sh
journalctl -o json --since today | logsift summary --level err
```

### Find all 5xx lines from the last 30 minutes

```sh
logsift grep --status 5xx --since 30m /var/log/nginx/access.log*
```

### Combine nginx access and journald in one summary

```sh
cat /var/log/nginx/access.log <(journalctl -o json --since "1 hour ago") | logsift summary
```

### JSON output for dashboards or scripts

```sh
logsift summary --json /var/log/nginx/access.log | jq '.top_paths_by_5xx'
```

## Real sample output

**`logsift summary testdata/nginx_access.log`**

```
=== logsift summary ===
Time range : 2024-01-15 10:00:01 → 2024-01-15 10:03:00
Total lines: 13
Formats    : nginx(13)

Status classes:
  2xx        6  ( 46.2%)
  3xx        2  ( 15.4%)
  4xx        1  (  7.7%)
  5xx        4  ( 30.8%)

Top 10 paths by 5xx:
  /api/payment                                       3
  /api/orders                                        1

Top 10 IPs by requests:
  192.168.1.1                                   4
  10.0.0.5                                      3
  192.168.1.2                                   3
  203.0.113.10                                  3

Top 10 error messages (normalised):
  GET /api/payment HTTP/<NUM> → <NUM>                                              3
  DELETE /api/users/<NUM> HTTP/<NUM> → <NUM>                                       1
  GET /api/orders HTTP/<NUM> → <NUM>                                               1

Peak RPM   : 5 (at 13:01)
```

**`logsift grep --status 5xx testdata/nginx_access.log`**

```
203.0.113.10 - - [15/Jan/2024:10:01:00 +0000] "GET /api/payment HTTP/1.1" 500 256 "-" "Go-http-client/1.1"
203.0.113.10 - - [15/Jan/2024:10:01:05 +0000] "GET /api/payment HTTP/1.1" 500 256 "-" "Go-http-client/1.1"
192.168.1.1 - - [15/Jan/2024:10:01:10 +0000] "GET /api/orders HTTP/1.1" 500 128 "-" "Mozilla/5.0"
203.0.113.10 - - [15/Jan/2024:10:02:30 +0000] "GET /api/payment HTTP/1.1" 502 64 "-" "Go-http-client/1.1"
```

**`logsift grep --level err testdata/journald.jsonl`**

```
{"__REALTIME_TIMESTAMP":"1705312805000000",...,"MESSAGE":"connect() failed (111: Connection refused) for upstream","PRIORITY":"3","_SYSTEMD_UNIT":"nginx.service"}
{"__REALTIME_TIMESTAMP":"1705312815000000",...,"MESSAGE":"Out of memory: Kill process 8765 (php-fpm) score 892","PRIORITY":"2","_SYSTEMD_UNIT":"kernel"}
{"__REALTIME_TIMESTAMP":"1705312825000000",...,"MESSAGE":"connect() failed (111: Connection refused) for upstream","PRIORITY":"3","_SYSTEMD_UNIT":"nginx.service"}
```

**`logsift summary --json testdata/nginx_access.log`** (excerpt)

```json
{
  "time_range": { "first": "2024-01-15T10:00:01Z", "last": "2024-01-15T10:03:00Z" },
  "total_lines": 13,
  "formats": { "nginx": 13 },
  "status_classes": { "2xx": 6, "3xx": 2, "4xx": 1, "5xx": 4 },
  "top_paths_by_5xx": [
    { "key": "/api/payment", "count": 3 },
    { "key": "/api/orders",  "count": 1 }
  ],
  "peak_rpm": 5
}
```

## Supported formats

### nginx combined (access log)

The default `log_format combined`:

```
$remote_addr - $remote_user [$time_local] "$request" $status $body_bytes_sent "$http_referer" "$http_user_agent"
```

Fields parsed: remote address, status, method, path, bytes, user agent, timestamp (with UTC offset).

An optional trailing `$upstream_response_time` field is supported when your site uses a custom `log_format` that appends it; it is **not** present in the default combined format.

### nginx error log

```
2024/01/15 12:34:56 [error] 1234#5678: *890 message, client: 1.2.3.4, ...
```

Timestamp is interpreted in the local timezone (no offset in this format).

### journald JSON (`journalctl -o json`)

```json
{"__REALTIME_TIMESTAMP":"1705312801000000","MESSAGE":"...","PRIORITY":"3","_SYSTEMD_UNIT":"nginx.service"}
```

`__REALTIME_TIMESTAMP` is **microseconds** since the Unix epoch (as a string).
`PRIORITY` is the syslog numeric severity (0=emerg … 7=debug).
`--level err` matches priority ≤ 3 (err and more severe).

### Syslog (RFC3164 / PRI-prefixed)

```
Jan 15 12:34:56 hostname program[pid]: message
<190>Jan 15 12:34:56 hostname sshd[1234]: Failed password for ...
```

Timestamp carries no year and no offset; it is interpreted in the local timezone.
The year is inferred as the current calendar year; if that produces a timestamp
more than 24 hours in the future the previous year is used.

`--level` filtering requires a PRI-prefix (`<N>`); plain syslog lines without
one are always included when a level filter is active.

## Auto-detection

Format is detected per line using cheap prefix heuristics (no full parse):

| Prefix | Format |
|---|---|
| `{` | journald JSON |
| `YYYY/MM/DD HH:MM:SS [` | nginx error |
| `<N>` or `MonDD` | syslog |
| digits or `:` (IPv4/IPv6) | nginx access |

Override with `--format nginx|nginx-error|journald|syslog`.

## Memory

logsift processes logs in a streaming fashion (line by line) and never loads
the whole file. The `summary` command does use unbounded maps for top-K counters
keyed by path, IP, and normalised error message. For very large log files with
millions of distinct URLs or IPs the map memory grows proportionally to the
number of distinct values. This is acceptable for typical server log files.

## Limitations

- nginx upstream response time is only available with a custom `log_format`; it is absent from the default `combined` format.
- `--level` filtering only works for journald and PRI-prefixed syslog lines. Plain syslog lines without a PRI prefix are considered to have no level.
- gzip decompression is triggered by a `.gz` file suffix. A gzip-compressed file without the suffix is not auto-detected.
- Syslog timestamps have no timezone; they are interpreted in the system local timezone. Running on a host in a different timezone than the log producer will shift the `--since`/`--until` windows.
- IPv6 addresses in log lines may be partially normalised by the message normaliser (pattern match is heuristic).

## Development

```sh
git clone https://github.com/mertkarakulak/logsift
cd logsift
go test ./...     # run all tests
go vet ./...      # vet
go build .        # build the binary
```

Test fixtures live in `testdata/`. The normalisation replacement order (UUID →
IPv6 → IPv4 → hex → numbers) is documented in `internal/normalize/normalize.go`.

## License

MIT — see [LICENSE](LICENSE).
