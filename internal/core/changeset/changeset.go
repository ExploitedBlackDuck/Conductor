// Package changeset parses rclone's structured (--use-json-log) dry-run output
// into a domain.ChangeSet — the concrete creates/updates/deletes that back the
// destructive-op preview gate (ADR-0015, §7.4). It deliberately classifies
// rclone's emitted *events*, not its human-readable lines: the dry-run text is
// not a stable interface, so this parser is pinned to the rclone version via
// fixtures (§7.5 drift guard) and fails CI rather than silently mis-parsing
// when the event shape changes.
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

// Event is the subset of an rclone JSON-log record this parser reads. rclone
// emits one record per line on stderr under --use-json-log; a dry-run skip
// carries the affected path in Object and the action in Msg.
type Event struct {
	Level  string `json:"level"`
	Msg    string `json:"msg"`
	Object string `json:"object"`
}

// Parse reads newline-delimited rclone JSON-log records and classifies each
// dry-run skip into a domain.ChangeSet. Non-JSON lines and records that are not
// dry-run skips are ignored, so partial/interleaved output is tolerated.
func Parse(jsonLog []byte) (domain.ChangeSet, error) {
	var cs domain.ChangeSet
	sc := bufio.NewScanner(bytes.NewReader(jsonLog))
	// rclone can emit long lines for deeply nested paths; raise the line cap.
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var e Event
		if err := json.Unmarshal(line, &e); err != nil {
			continue // tolerate non-record lines rather than failing the whole parse
		}
		addEvent(&cs, e)
	}
	if err := sc.Err(); err != nil {
		return domain.ChangeSet{}, err
	}
	return cs, nil
}

// addEvent classifies one event and folds it into the change set.
func addEvent(cs *domain.ChangeSet, e Event) {
	kind, ok := classify(e.Msg)
	if !ok || e.Object == "" {
		return
	}
	fc := domain.FileChange{Kind: kind, Path: e.Object}
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

// classify maps a dry-run log message to a change kind. rclone reports skipped
// actions as "Skipped <action> as --dry-run is set"; only those records are
// change-set entries. Order matters: "delete" is checked first so a message is
// never misfiled as a create/update.
func classify(msg string) (domain.ChangeKind, bool) {
	if !strings.Contains(msg, "--dry-run is set") {
		return "", false
	}
	m := strings.ToLower(msg)
	switch {
	case strings.Contains(m, "delete"):
		return domain.ChangeDelete, true
	case strings.Contains(m, "update"):
		return domain.ChangeUpdate, true
	case strings.Contains(m, "copy"), strings.Contains(m, "move"):
		// A copied or server-side-moved object appears at the destination.
		return domain.ChangeCreate, true
	default:
		return "", false
	}
}
