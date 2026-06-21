package verify

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/daemon"
	"github.com/conductor-app/conductor/internal/core/domain"
)

// combined mirrors the real operations/check report captured from rclone
// v1.74.3: = match, * differ, + missing on dst, - missing on src.
var combined = []string{"- onlydst.txt", "+ onlysrc.txt", "= match.txt", "* differ.txt"}

func TestParseCombinedClassifiesSigils(t *testing.T) {
	t.Parallel()
	r := ParseCombined(combined)
	assert.Equal(t, 1, r.Match)
	assert.Equal(t, []string{"differ.txt"}, r.Differ)
	assert.Equal(t, []string{"onlysrc.txt"}, r.MissingOnDst)
	assert.Equal(t, []string{"onlydst.txt"}, r.MissingOnSrc)
	assert.Equal(t, 2, r.Missing())
	assert.Empty(t, r.Errors)
}

func TestParseCombinedAllMatch(t *testing.T) {
	t.Parallel()
	r := ParseCombined([]string{"= a.txt", "= b.txt"})
	assert.Equal(t, 2, r.Match)
	assert.Zero(t, r.Missing()+r.DifferCount()+r.ErrorCount())
}

// fakeRC returns a canned combined report.
type fakeRC struct {
	combined []string
	oneway   bool
}

func (f *fakeRC) OperationsCheck(_ context.Context, _, _ string, oneway bool) ([]string, error) {
	f.oneway = oneway
	return f.combined, nil
}

type fakeStore struct{ inserted []domain.Verification }

func (s *fakeStore) InsertVerification(_ context.Context, v domain.Verification) error {
	s.inserted = append(s.inserted, v)
	return nil
}

func (s *fakeStore) Verifications(context.Context, int) ([]domain.Verification, error) {
	return s.inserted, nil
}

type fakeAudit struct{ actions []domain.AuditAction }

func (a *fakeAudit) Record(_ context.Context, action domain.AuditAction, _ string, _ any) (domain.AuditEntry, error) {
	a.actions = append(a.actions, action)
	return domain.AuditEntry{}, nil
}

type fakeRunner struct {
	out []byte
	err error
}

func (r *fakeRunner) Output(context.Context, daemon.Spec) ([]byte, error) { return r.out, r.err }

func newService(t *testing.T, rc *fakeRC, store *fakeStore, audit *fakeAudit, runner Runner) *Service {
	t.Helper()
	return New(Config{
		RC:         func() (RC, error) { return rc, nil },
		Runner:     runner,
		BinaryPath: "/bin/rclone",
		Store:      store,
		Audit:      audit,
		Clock:      fixedClock{},
		NewID:      func() string { return "ver-1" },
	})
}

type fixedClock struct{}

func (fixedClock) Now() time.Time { return time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC) }

func TestRunCheckRecordsMismatchAndAudits(t *testing.T) {
	t.Parallel()
	rc := &fakeRC{combined: combined}
	store := &fakeStore{}
	audit := &fakeAudit{}
	svc := newService(t, rc, store, audit, nil)

	res, err := svc.Run(context.Background(), domain.VerifyCheck,
		domain.Endpoint{Path: "/src"}, domain.Endpoint{Remote: "s3", Path: "dst"}, false)
	require.NoError(t, err)

	assert.Equal(t, domain.VerifyMismatch, res.Verification.Result)
	assert.Equal(t, 1, res.Verification.Match)
	assert.Equal(t, 1, res.Verification.Differ)
	assert.Equal(t, 2, res.Verification.Missing)
	assert.Equal(t, []string{"differ.txt"}, res.Differ)

	// Persisted and hash-chained into the audit log (§7.12).
	require.Len(t, store.inserted, 1)
	assert.Equal(t, domain.VerifyCheck, store.inserted[0].Kind)
	assert.Contains(t, audit.actions, domain.ActionVerification)
}

func TestRunCheckAllMatch(t *testing.T) {
	t.Parallel()
	rc := &fakeRC{combined: []string{"= a.txt", "= b.txt"}}
	svc := newService(t, rc, &fakeStore{}, &fakeAudit{}, nil)

	res, err := svc.Run(context.Background(), domain.VerifyCheck,
		domain.Endpoint{Path: "/a"}, domain.Endpoint{Path: "/b"}, true)
	require.NoError(t, err)
	assert.Equal(t, domain.VerifyMatch, res.Verification.Result)
	assert.True(t, rc.oneway, "oneway flag is threaded to the check")
}

// TestRunCryptcheckCLIParsesDespiteNonZeroExit proves the CLI path tolerates the
// non-zero exit rclone returns on a mismatch and still records the result.
func TestRunCryptcheckCLIParsesDespiteNonZeroExit(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{
		out: []byte("* differ.txt\n= match.txt\n"),
		err: errors.New("exit status 1"), // rclone exits non-zero on differences
	}
	store := &fakeStore{}
	svc := newService(t, &fakeRC{}, store, &fakeAudit{}, runner)

	res, err := svc.Run(context.Background(), domain.VerifyCryptcheck,
		domain.Endpoint{Remote: "crypt", Path: ""}, domain.Endpoint{Path: "/plain"}, false)
	require.NoError(t, err, "a non-zero exit with parseable output is a result, not a failure")
	assert.Equal(t, domain.VerifyMismatch, res.Verification.Result)
	assert.Equal(t, 1, res.Verification.Differ)
	require.Len(t, store.inserted, 1)
}

// TestRunCryptcheckRealFailureSurfaces proves an empty output with an error is a
// real failure, not a silent pass.
func TestRunCryptcheckRealFailureSurfaces(t *testing.T) {
	t.Parallel()
	runner := &fakeRunner{out: nil, err: errors.New("exit status 2")}
	svc := newService(t, &fakeRC{}, &fakeStore{}, &fakeAudit{}, runner)

	_, err := svc.Run(context.Background(), domain.VerifyCryptcheck,
		domain.Endpoint{Remote: "crypt"}, domain.Endpoint{Path: "/plain"}, false)
	require.Error(t, err)
}
