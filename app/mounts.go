package app

import (
	"context"
	"time"

	"github.com/conductor-app/conductor/internal/core/domain"
)

// MountDTO is one active mount for the frontend.
type MountDTO struct {
	Fs         string `json:"fs"`
	MountPoint string `json:"mountPoint"`
	MountedOn  string `json:"mountedOn"`
}

// MountsResultDTO is the list of active mounts or a typed error.
type MountsResultDTO struct {
	Mounts []MountDTO `json:"mounts"`
	Error  *ErrorDTO  `json:"error"`
}

// ListMounts returns the active mounts (§7.11.6).
func (a *App) ListMounts() MountsResultDTO {
	ms, err := a.mounts.List(context.Background())
	if err != nil {
		return MountsResultDTO{Error: errorToDTO(err)}
	}
	out := make([]MountDTO, 0, len(ms))
	for _, m := range ms {
		out = append(out, toMountDTO(m))
	}
	return MountsResultDTO{Mounts: out}
}

// MountFs mounts fs at mountPoint. mountType may be "".
func (a *App) MountFs(fs, mountPoint, mountType string) *ErrorDTO {
	if err := a.mounts.Mount(context.Background(), fs, mountPoint, mountType); err != nil {
		return errorToDTO(err)
	}
	return nil
}

// UnmountFs unmounts the mount at mountPoint.
func (a *App) UnmountFs(mountPoint string) *ErrorDTO {
	if err := a.mounts.Unmount(context.Background(), mountPoint); err != nil {
		return errorToDTO(err)
	}
	return nil
}

func toMountDTO(m domain.Mount) MountDTO {
	dto := MountDTO{Fs: m.Fs, MountPoint: m.MountPoint}
	if !m.MountedOn.IsZero() {
		dto.MountedOn = m.MountedOn.Format(time.RFC3339)
	}
	return dto
}
