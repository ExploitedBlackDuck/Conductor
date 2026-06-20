// Package audit implements Conductor's append-only, hash-chained audit log
// (ADR-0010, §7.8). Each entry's hash is SHA-256 over the previous entry's hash
// concatenated with a canonical encoding of the entry, so any insertion,
// deletion, or modification breaks verification downstream. This log is durable
// and tamper-evident — distinct from the disposable operational slog (§2.4).
package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/ports"
)

// Store is the persistence port the audit service requires (a consumer-defined
// interface, §2.1). The sqlitestore adapter implements it. AppendEntry must be
// atomic: it reads the current tail and inserts the built entry within a single
// transaction so the chain cannot fork under concurrent writers.
type Store interface {
	// AppendEntry reads the current last entry (prev is nil if the log is
	// empty), invokes build to produce the next entry, and persists it
	// atomically.
	AppendEntry(ctx context.Context, build func(prev *domain.AuditEntry) (domain.AuditEntry, error)) (domain.AuditEntry, error)
	// Entries returns all audit entries in ascending Seq order.
	Entries(ctx context.Context) ([]domain.AuditEntry, error)
}

// Service records and verifies audit entries.
type Service struct {
	store Store
	clock ports.Clock
	// mu serialises Record calls in-process; AppendEntry provides the
	// cross-transaction guarantee at the store.
	mu sync.Mutex
}

// New constructs an audit Service.
func New(store Store, clock ports.Clock) *Service {
	return &Service{store: store, clock: clock}
}

// Entries returns the full audit chain in ascending Seq order, for the audit
// viewer and history exports (§7.11.7–7.11.8).
func (s *Service) Entries(ctx context.Context) ([]domain.AuditEntry, error) {
	return s.store.Entries(ctx)
}

// Record appends an entry for action concerning subject, with detail serialised
// to canonical JSON. It returns the persisted entry including its chain hash.
func (s *Service) Record(ctx context.Context, action domain.AuditAction, subject string, detail any) (domain.AuditEntry, error) {
	detailJSON, err := canonicalJSON(detail)
	if err != nil {
		return domain.AuditEntry{}, fmt.Errorf("encoding audit detail: %w", err)
	}
	at := s.clock.Now().UTC()

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.store.AppendEntry(ctx, func(prev *domain.AuditEntry) (domain.AuditEntry, error) {
		var seq int64 = 1
		var prevHash string
		if prev != nil {
			seq = prev.Seq + 1
			prevHash = prev.Hash
		}
		entry := domain.AuditEntry{
			Seq:      seq,
			At:       at,
			Action:   action,
			Subject:  subject,
			Detail:   detailJSON,
			PrevHash: prevHash,
		}
		entry.Hash = chainHash(prevHash, entry)
		return entry, nil
	})
}

// Result reports the outcome of verifying the chain.
type Result struct {
	// Intact is true when every entry's hash and link verify.
	Intact bool
	// Count is the number of entries examined.
	Count int
	// BrokenAtSeq is the Seq of the first entry that failed, or 0 when intact.
	BrokenAtSeq int64
	// Reason describes the first failure, or "" when intact.
	Reason string
}

// Verify walks the chain from genesis, recomputing each hash and checking each
// link. An intact empty log verifies. A broken chain yields Intact=false; the
// caller surfaces ERR_AUDIT_CHAIN_BROKEN (§8.4).
func (s *Service) Verify(ctx context.Context) (Result, error) {
	entries, err := s.store.Entries(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("loading audit entries: %w", err)
	}

	var prevHash string
	for i, e := range entries {
		expectedSeq := int64(i + 1)
		if e.Seq != expectedSeq {
			return brokenResult(len(entries), e.Seq, fmt.Sprintf("seq gap: expected %d, got %d", expectedSeq, e.Seq)), nil
		}
		if e.PrevHash != prevHash {
			return brokenResult(len(entries), e.Seq, "prev-hash does not match preceding entry"), nil
		}
		if got := chainHash(prevHash, e); got != e.Hash {
			return brokenResult(len(entries), e.Seq, "hash mismatch (entry altered)"), nil
		}
		prevHash = e.Hash
	}

	return Result{Intact: true, Count: len(entries)}, nil
}

// VerifyOrError is Verify with the broken chain mapped to a coded error, for
// callers that want an error rather than a Result to branch on.
func (s *Service) VerifyOrError(ctx context.Context) (Result, error) {
	res, err := s.Verify(ctx)
	if err != nil {
		return res, err
	}
	if !res.Intact {
		return res, coreerr.New(coreerr.CodeAuditChainBroken,
			fmt.Sprintf("audit chain broken at entry %d: %s", res.BrokenAtSeq, res.Reason), nil)
	}
	return res, nil
}

func brokenResult(count int, seq int64, reason string) Result {
	return Result{Intact: false, Count: count, BrokenAtSeq: seq, Reason: reason}
}

// canonicalEntry is the deterministic shape hashed into the chain. Field order
// is fixed by declaration; Detail is emitted verbatim from its canonical bytes.
type canonicalEntry struct {
	Seq     int64              `json:"seq"`
	At      string             `json:"at"`
	Action  domain.AuditAction `json:"action"`
	Subject string             `json:"subject"`
	Detail  json.RawMessage    `json:"detail"`
}

// chainHash computes hex(SHA256(prevHash || canonical(entry))).
func chainHash(prevHash string, e domain.AuditEntry) string {
	canonical, err := json.Marshal(canonicalEntry{
		Seq:     e.Seq,
		At:      e.At.UTC().Format(timeLayout),
		Action:  e.Action,
		Subject: e.Subject,
		Detail:  e.Detail,
	})
	if err != nil {
		// canonicalEntry contains only JSON-encodable fields with Detail already
		// validated as JSON; marshaling cannot fail in practice. Fold any
		// impossible error into the hash input so it surfaces as a mismatch
		// rather than panicking in library code (§2.2).
		canonical = []byte("unencodable:" + err.Error())
	}
	h := sha256.New()
	h.Write([]byte(prevHash))
	h.Write(canonical)
	return hex.EncodeToString(h.Sum(nil))
}

// canonicalJSON encodes detail to compact, canonical JSON. A nil detail encodes
// as JSON null.
func canonicalJSON(detail any) (json.RawMessage, error) {
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(raw), nil
}

// timeLayout is the timestamp format embedded in the chain hash; it must match
// what the store round-trips so verification is reproducible.
const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"
