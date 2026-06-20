package domain

import "time"

// Mount is an active rclone mount (§7.1), mapped from mount/listmounts.
type Mount struct {
	// Fs is the mounted remote:path.
	Fs string
	// MountPoint is the local directory the remote is mounted on.
	MountPoint string
	// MountedOn is when the mount was established.
	MountedOn time.Time
}
