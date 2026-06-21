// Package changeset parses rclone's dry-run output into a domain.ChangeSet — the
// concrete creates/updates/deletes that back the destructive-op preview gate
// (ADR-0015, §7.4). It supports the two structured mechanisms the pinned rclone
// actually provides (validated against v1.74.3, fixtures in testdata/, §7.5
// drift guard), never human-readable log scraping:
//
//   - ParseCombined reads a `--combined` report (sync/copy/move/check): each
//     line is "<sigil> <path>" with + create, * update, - delete, = match,
//     ! error. This distinguishes all three change kinds.
//   - ParseJSONLog reads `--use-json-log` dry-run skip events (bisync, delete,
//     purge — which do not support --combined): the structured `skipped` field
//     gives copy vs delete. It cannot split create from update, so writes are
//     reported as creates; deletes (the dangerous kind) are exact.
//
// An rclone upgrade that changes either shape must refresh the fixtures and is
// caught by the drift guard rather than silently mis-parsing.
package changeset

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"

	"github.com/conductor-app/conductor/internal/core/domain"
)

// maxListed caps how many create/update paths are retained per kind; the exact
// counts remain accurate beyond it (Truncated is set). Deletes are never capped
// — the dangerous changes are always fully enumerable (§7.4).
const maxListed = 10000

// ParseCombined reads a `--combined` change report and classifies each line.
// Lines that are not "<sigil> <path>" (blank, or a match/error marker) are
// ignored, so interleaved noise is tolerated.
func ParseCombined(report []byte) (domain.ChangeSet, error) {
	var cs domain.ChangeSet
	sc := newScanner(report)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) < 3 || line[1] != ' ' {
			continue
		}
		path := string(bytes.TrimRight(line[2:], "\r"))
		if path == "" {
			continue
		}
		switch line[0] {
		case '+':
			add(&cs, domain.ChangeCreate, path)
		case '*':
			add(&cs, domain.ChangeUpdate, path)
		case '-':
			add(&cs, domain.ChangeDelete, path)
			// '=' (match) and '!' (error) are not change-set entries.
		}
	}
	return cs, sc.Err()
}

// jsonEvent is the subset of an rclone JSON-log record ParseJSONLog reads. A
// dry-run skip sets `skipped` to the action and `object` to the path;
// `objectType` distinguishes a file ("*local.Object") from a filesystem-level
// summary ("*local.Fs", e.g. a server-side directory move) that must be excluded.
type jsonEvent struct {
	Skipped    string `json:"skipped"`
	Object     string `json:"object"`
	ObjectType string `json:"objectType"`
}

// ParseJSONLog reads newline-delimited rclone `--use-json-log` records and folds
// each dry-run skip into a ChangeSet. Used for bisync/delete/purge, which do not
// support --combined; deletes are exact, writes are reported as creates.
func ParseJSONLog(jsonLog []byte) (domain.ChangeSet, error) {
	var cs domain.ChangeSet
	sc := newScanner(jsonLog)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var e jsonEvent
		if err := json.Unmarshal(line, &e); err != nil {
			continue // tolerate non-record lines rather than failing the whole parse
		}
		if e.Skipped == "" || e.Object == "" || strings.HasSuffix(e.ObjectType, ".Fs") {
			continue
		}
		s := strings.ToLower(e.Skipped)
		switch {
		case strings.Contains(s, "delete"):
			add(&cs, domain.ChangeDelete, e.Object)
		case strings.Contains(s, "copy"), strings.Contains(s, "move"):
			add(&cs, domain.ChangeCreate, e.Object) // create-or-overwrite, undistinguished
		}
	}
	return cs, sc.Err()
}

// add folds one change into the set, capping create/update lists (but never
// deletes) and keeping exact counts.
func add(cs *domain.ChangeSet, kind domain.ChangeKind, path string) {
	fc := domain.FileChange{Kind: kind, Path: path}
	switch kind {
	case domain.ChangeDelete:
		cs.DeleteCount++
		cs.Deletes = append(cs.Deletes, fc) // never capped
	case domain.ChangeUpdate:
		cs.UpdateCount++
		if len(cs.Updates) < maxListed {
			cs.Updates = append(cs.Updates, fc)
		} else {
			cs.Truncated = true
		}
	case domain.ChangeCreate:
		cs.CreateCount++
		if len(cs.Creates) < maxListed {
			cs.Creates = append(cs.Creates, fc)
		} else {
			cs.Truncated = true
		}
	}
}

// newScanner returns a line scanner with a raised buffer for deeply nested paths.
func newScanner(b []byte) *bufio.Scanner {
	sc := bufio.NewScanner(bytes.NewReader(b))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	return sc
}
