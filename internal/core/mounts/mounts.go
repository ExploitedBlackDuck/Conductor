// Package mounts manages rclone mounts through the rc daemon (§7.3, §7.10 P6):
// mount, unmount, and list active mounts. Every mount and unmount is recorded
// as an audit entry (ADR-0010), since a mount exposes a remote's data to the
// local filesystem.
package mounts

import (
	"context"
	"fmt"

	"github.com/conductor-app/conductor/internal/core/domain"
)

// RC is the subset of the rc client the mounts service needs.
type RC interface {
	MountMount(ctx context.Context, fs, mountPoint, mountType string) error
	MountUnmount(ctx context.Context, mountPoint string) error
	ListMounts(ctx context.Context) ([]domain.Mount, error)
}

// Provider builds an RC client bound to the live daemon session.
type Provider func() (RC, error)

// Auditor records audit entries.
type Auditor interface {
	Record(ctx context.Context, action domain.AuditAction, subject string, detail any) (domain.AuditEntry, error)
}

// Service mounts, unmounts, and lists mounts.
type Service struct {
	rc    Provider
	audit Auditor
}

// New constructs the mounts Service.
func New(rc Provider, audit Auditor) *Service {
	return &Service{rc: rc, audit: audit}
}

// Mount mounts fs at mountPoint and records an audit entry. mountType is
// optional.
func (s *Service) Mount(ctx context.Context, fs, mountPoint, mountType string) error {
	rc, err := s.rc()
	if err != nil {
		return err
	}
	if err := rc.MountMount(ctx, fs, mountPoint, mountType); err != nil {
		return err
	}
	if _, err := s.audit.Record(ctx, domain.ActionMount, mountPoint, map[string]any{"fs": fs, "mountType": mountType}); err != nil {
		return fmt.Errorf("recording mount: %w", err)
	}
	return nil
}

// Unmount unmounts mountPoint and records an audit entry.
func (s *Service) Unmount(ctx context.Context, mountPoint string) error {
	rc, err := s.rc()
	if err != nil {
		return err
	}
	if err := rc.MountUnmount(ctx, mountPoint); err != nil {
		return err
	}
	if _, err := s.audit.Record(ctx, domain.ActionUnmount, mountPoint, nil); err != nil {
		return fmt.Errorf("recording unmount: %w", err)
	}
	return nil
}

// List returns the active mounts.
func (s *Service) List(ctx context.Context) ([]domain.Mount, error) {
	rc, err := s.rc()
	if err != nil {
		return nil, err
	}
	return rc.ListMounts(ctx)
}
