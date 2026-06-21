package app

import (
	"context"

	"github.com/conductor-app/conductor/internal/core/rclonebin"
)

// OnboardingDTO tells the frontend whether first-run setup is needed and which
// rclone version Conductor pins, so the binary-missing/mismatched degraded state
// (§7.11.9) can route into the acquisition wizard (ADR-0008).
type OnboardingDTO struct {
	// PinnedVersion is the exact rclone version Conductor acquires and verifies.
	PinnedVersion string `json:"pinnedVersion"`
}

// Onboarding returns static onboarding facts the wizard needs.
func (a *App) Onboarding() OnboardingDTO {
	return OnboardingDTO{PinnedVersion: rclonebin.PinnedVersion}
}

// AcquireRclone downloads and verifies the pinned rclone into the configured
// binary path (ADR-0008), then retries bringing up the daemon so the app
// becomes ready without a restart. It is operator-initiated, from the wizard or
// the degraded "binary missing" state. The binary is re-verified on launch
// regardless, so a failed/partial acquisition fails closed.
func (a *App) AcquireRclone() *ErrorDTO {
	ctx := context.Background()
	if err := rclonebin.Acquire(ctx, a.binaryPath, rclonebin.HTTPFetcher); err != nil {
		return errorToDTO(err)
	}
	// The daemon refused to start without a verified binary; bring it up now that
	// one is in place. control.Start records its own error into Status, so a
	// retry failure surfaces through the normal degraded path.
	if a.stats != nil {
		if err := a.control.Start(ctx, a.stats); err != nil {
			return errorToDTO(err)
		}
	}
	return nil
}
