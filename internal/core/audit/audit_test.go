package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/domain"
)

// memStore is an in-memory audit Store for tests, mirroring the atomic
// read-tail-then-append contract the sqlitestore adapter provides.
type memStore struct {
	entries []domain.AuditEntry
	anchors []domain.AuditAnchor
}

func (m *memStore) InsertAnchor(_ context.Context, a domain.AuditAnchor) error {
	m.anchors = append(m.anchors, a)
	return nil
}

func (m *memStore) LatestAnchor(_ context.Context) (domain.AuditAnchor, bool, error) {
	if len(m.anchors) == 0 {
		return domain.AuditAnchor{}, false, nil
	}
	return m.anchors[len(m.anchors)-1], true, nil
}

func (m *memStore) AppendEntry(_ context.Context, build func(prev *domain.AuditEntry) (domain.AuditEntry, error)) (domain.AuditEntry, error) {
	var prev *domain.AuditEntry
	if len(m.entries) > 0 {
		prev = &m.entries[len(m.entries)-1]
	}
	e, err := build(prev)
	if err != nil {
		return domain.AuditEntry{}, err
	}
	m.entries = append(m.entries, e)
	return e, nil
}

func (m *memStore) Entries(_ context.Context) ([]domain.AuditEntry, error) {
	out := make([]domain.AuditEntry, len(m.entries))
	copy(out, m.entries)
	return out, nil
}

// fixedClock returns a deterministic, advancing time.
type fixedClock struct{ t time.Time }

func (c *fixedClock) Now() time.Time {
	c.t = c.t.Add(time.Second)
	return c.t
}

func newService(opts ...Option) (*Service, *memStore) {
	store := &memStore{}
	clk := &fixedClock{t: time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)}
	return New(store, clk, opts...), store
}

// signingKey is a fixed 32-byte test key.
var signingKey = bytes.Repeat([]byte{0x55}, 32)

func TestRecordChainsEntries(t *testing.T) {
	t.Parallel()
	svc, store := newService()
	ctx := context.Background()

	first, err := svc.Record(ctx, domain.ActionOperationStart, "op-1", map[string]string{"src": "example-s3:bucket"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), first.Seq)
	assert.Empty(t, first.PrevHash, "genesis entry has empty prev-hash")
	assert.NotEmpty(t, first.Hash)

	second, err := svc.Record(ctx, domain.ActionDestructiveConfirmed, "op-1", map[string]bool{"acknowledged": true})
	require.NoError(t, err)
	assert.Equal(t, int64(2), second.Seq)
	assert.Equal(t, first.Hash, second.PrevHash, "each entry links to its predecessor")

	require.Len(t, store.entries, 2)
}

func TestVerifyIntactChain(t *testing.T) {
	t.Parallel()
	svc, _ := newService()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, err := svc.Record(ctx, domain.ActionOperationStart, "op", map[string]int{"i": i})
		require.NoError(t, err)
	}

	res, err := svc.Verify(ctx)
	require.NoError(t, err)
	assert.True(t, res.Intact)
	assert.Equal(t, 5, res.Count)
	assert.Zero(t, res.BrokenAtSeq)
}

func TestVerifyEmptyChainIsIntact(t *testing.T) {
	t.Parallel()
	svc, _ := newService()

	res, err := svc.Verify(context.Background())
	require.NoError(t, err)
	assert.True(t, res.Intact)
	assert.Equal(t, 0, res.Count)
}

func TestVerifyDetectsTampering(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		tamper func(entries []domain.AuditEntry)
	}{
		{
			name: "altered detail without rehash",
			tamper: func(entries []domain.AuditEntry) {
				entries[1].Detail = json.RawMessage(`{"acknowledged":false}`)
			},
		},
		{
			name: "altered subject",
			tamper: func(entries []domain.AuditEntry) {
				entries[2].Subject = "op-evil"
			},
		},
		{
			name: "deleted middle entry breaks the link",
			tamper: func(entries []domain.AuditEntry) {
				// Caller replaces the slice; see below.
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			svc, store := newService()
			ctx := context.Background()
			for i := 0; i < 4; i++ {
				_, err := svc.Record(ctx, domain.ActionOperationStart, "op", map[string]int{"i": i})
				require.NoError(t, err)
			}

			if tc.name == "deleted middle entry breaks the link" {
				store.entries = append(store.entries[:1], store.entries[2:]...)
			} else {
				tc.tamper(store.entries)
			}

			res, err := svc.Verify(ctx)
			require.NoError(t, err)
			assert.False(t, res.Intact, "tampering must be detected")
			assert.NotZero(t, res.BrokenAtSeq)

			_, verr := svc.VerifyOrError(ctx)
			require.Error(t, verr)
			code, ok := coreerr.CodeOf(verr)
			require.True(t, ok)
			assert.Equal(t, coreerr.CodeAuditChainBroken, code)
		})
	}
}

func TestSignHeadAndVerifySignature(t *testing.T) {
	t.Parallel()
	svc, _ := newService(WithSigningKey(signingKey))
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := svc.Record(ctx, domain.ActionOperationStart, "op", map[string]int{"i": i})
		require.NoError(t, err)
	}
	anchor, err := svc.SignHead(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(3), anchor.Seq)
	assert.NotEmpty(t, anchor.Signature)

	res, err := svc.Verify(ctx)
	require.NoError(t, err)
	assert.True(t, res.Intact)
	assert.True(t, res.HeadSigned)
	assert.True(t, res.SignatureValid)
	assert.True(t, res.Trustworthy())
}

// TestSignedHeadDetectsFullRecompute is the ADR-0010 property: an attacker who
// rewrites every entry and re-hashes the whole chain (so the links stay
// internally consistent) is still caught, because the signed head no longer
// matches the chain and cannot be re-signed without the key.
func TestSignedHeadDetectsFullRecompute(t *testing.T) {
	t.Parallel()
	svc, store := newService(WithSigningKey(signingKey))
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		_, err := svc.Record(ctx, domain.ActionOperationStart, "op", map[string]int{"i": i})
		require.NoError(t, err)
	}
	_, err := svc.SignHead(ctx)
	require.NoError(t, err)

	// Simulate a full recompute: alter entry 2's subject and re-derive every hash
	// from genesis so the chain links verify cleanly.
	store.entries[1].Subject = "op-evil"
	var prev string
	for i := range store.entries {
		store.entries[i].PrevHash = prev
		store.entries[i].Hash = chainHash(prev, store.entries[i])
		prev = store.entries[i].Hash
	}

	res, err := svc.Verify(ctx)
	require.NoError(t, err)
	assert.True(t, res.Intact, "the recomputed chain links are internally consistent")
	assert.True(t, res.HeadSigned)
	assert.False(t, res.SignatureValid, "the signed head no longer matches the rewritten chain")
	assert.False(t, res.Trustworthy())

	_, verr := svc.VerifyOrError(ctx)
	require.Error(t, verr)
	code, _ := coreerr.CodeOf(verr)
	assert.Equal(t, coreerr.CodeAuditSignatureInvalid, code)
}

// TestTamperedAnchorIsDetected proves an attacker who edits the anchor to match
// a rewritten head still fails, lacking the key to re-sign it.
func TestTamperedAnchorIsDetected(t *testing.T) {
	t.Parallel()
	svc, store := newService(WithSigningKey(signingKey))
	ctx := context.Background()
	_, err := svc.Record(ctx, domain.ActionOperationStart, "op", nil)
	require.NoError(t, err)
	_, err = svc.SignHead(ctx)
	require.NoError(t, err)

	store.anchors[0].HeadHash = "deadbeef" // forged head, but signature not re-derivable
	res, err := svc.Verify(ctx)
	require.NoError(t, err)
	assert.False(t, res.SignatureValid)
}

// TestVerifyWithoutSigningKeyIgnoresHead proves that with signing disabled the
// chain still verifies on its links alone (no signature assertion).
func TestVerifyWithoutSigningKeyIgnoresHead(t *testing.T) {
	t.Parallel()
	svc, _ := newService() // no signing key
	ctx := context.Background()
	_, err := svc.Record(ctx, domain.ActionOperationStart, "op", nil)
	require.NoError(t, err)

	res, err := svc.Verify(ctx)
	require.NoError(t, err)
	assert.True(t, res.Intact)
	assert.False(t, res.HeadSigned)
	assert.True(t, res.Trustworthy())
}
