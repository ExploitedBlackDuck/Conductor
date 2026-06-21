package app

import (
	"context"
	"time"

	"github.com/conductor-app/conductor/internal/core/domain"
)

// VerificationDTO is one integrity-check result for the frontend (§7.12).
type VerificationDTO struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Src        string `json:"src"`
	Dst        string `json:"dst"`
	StartedAt  string `json:"startedAt"`
	EndedAt    string `json:"endedAt"`
	Match      int    `json:"match"`
	Differ     int    `json:"differ"`
	Missing    int    `json:"missing"`
	ErrorCount int    `json:"errorCount"`
	Result     string `json:"result"`
}

// VerifyResultDTO is a completed verification with the offending paths for live
// display (the paths are shown but not persisted, §7.12).
type VerifyResultDTO struct {
	Verification VerificationDTO `json:"verification"`
	Differ       []string        `json:"differ"`
	MissingOnSrc []string        `json:"missingOnSrc"`
	MissingOnDst []string        `json:"missingOnDst"`
	Errors       []string        `json:"errors"`
	Error        *ErrorDTO       `json:"error"`
}

// VerificationsResultDTO is a list of past verifications or a typed error.
type VerificationsResultDTO struct {
	Verifications []VerificationDTO `json:"verifications"`
	Error         *ErrorDTO         `json:"error"`
}

// RunVerify runs a check or cryptcheck of src against dst and records the result
// (§7.12). It mutates nothing; a mismatch is a result, not an error.
func (a *App) RunVerify(kind string, src, dst EndpointDTO, oneway bool) VerifyResultDTO {
	res, err := a.verify.Run(context.Background(), domain.VerificationKind(kind),
		domain.Endpoint{Remote: src.Remote, Path: src.Path},
		domain.Endpoint{Remote: dst.Remote, Path: dst.Path}, oneway)
	if err != nil {
		return VerifyResultDTO{Error: errorToDTO(err)}
	}
	return VerifyResultDTO{
		Verification: toVerificationDTO(res.Verification),
		Differ:       orEmpty(res.Differ),
		MissingOnSrc: orEmpty(res.MissingOnSrc),
		MissingOnDst: orEmpty(res.MissingOnDst),
		Errors:       orEmpty(res.Errors),
	}
}

// ListVerifications returns the most recent verifications (§7.12).
func (a *App) ListVerifications() VerificationsResultDTO {
	vs, err := a.verify.Recent(context.Background(), 200)
	if err != nil {
		return VerificationsResultDTO{Error: errorToDTO(err)}
	}
	out := make([]VerificationDTO, 0, len(vs))
	for _, v := range vs {
		out = append(out, toVerificationDTO(v))
	}
	return VerificationsResultDTO{Verifications: out}
}

func toVerificationDTO(v domain.Verification) VerificationDTO {
	dto := VerificationDTO{
		ID: v.ID, Kind: string(v.Kind), Src: v.Src, Dst: v.Dst,
		Match: v.Match, Differ: v.Differ, Missing: v.Missing, ErrorCount: v.ErrorCount,
		Result: string(v.Result),
	}
	if !v.StartedAt.IsZero() {
		dto.StartedAt = v.StartedAt.Format(time.RFC3339)
	}
	if !v.EndedAt.IsZero() {
		dto.EndedAt = v.EndedAt.Format(time.RFC3339)
	}
	return dto
}

// orEmpty returns a non-nil slice so the JSON encodes [] rather than null.
func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
