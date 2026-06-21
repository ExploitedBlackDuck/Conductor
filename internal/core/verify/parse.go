// Package verify runs integrity checks (check/cryptcheck) and records their
// results as tamper-evident evidence (§7.12). It mutates nothing, so it carries
// no destructive gate. The check/cryptcheck comparison is captured through
// rclone's structured `--combined` report — over rc for check (operations/check
// returns the report directly), and as a one-shot CLI subprocess for cryptcheck
// (which rclone does not expose over rc, validated against the pinned binary).
package verify

import (
	"bufio"
	"bytes"
	"strings"
)

// Report is the parsed result of a check's `--combined` report: counts of
// matching files and the offending paths by category. The sigils are
// = match, * differ, + missing on destination, - missing on source, ! error.
type Report struct {
	Match        int
	Differ       []string
	MissingOnSrc []string
	MissingOnDst []string
	Errors       []string
}

// Missing is the total number of files present on only one side.
func (r Report) Missing() int { return len(r.MissingOnSrc) + len(r.MissingOnDst) }

// DifferCount is the number of files that exist on both sides but do not match.
func (r Report) DifferCount() int { return len(r.Differ) }

// ErrorCount is the number of files that could not be compared.
func (r Report) ErrorCount() int { return len(r.Errors) }

// ParseCombined classifies each "<sigil> <path>" line of a check's combined
// report. Lines that are not in that form are ignored, so interleaved noise
// (e.g. captured stderr on the CLI path) is tolerated.
func ParseCombined(lines []string) Report {
	var r Report
	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if len(line) < 3 || line[1] != ' ' {
			continue
		}
		path := line[2:]
		switch line[0] {
		case '=':
			r.Match++
		case '*':
			r.Differ = append(r.Differ, path)
		case '+':
			r.MissingOnDst = append(r.MissingOnDst, path)
		case '-':
			r.MissingOnSrc = append(r.MissingOnSrc, path)
		case '!':
			r.Errors = append(r.Errors, path)
		}
	}
	return r
}

// ParseCombinedBytes splits a raw combined report (e.g. CLI stdout) into lines
// and parses it.
func ParseCombinedBytes(out []byte) Report {
	var lines []string
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return ParseCombined(lines)
}
