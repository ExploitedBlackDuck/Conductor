//go:build !unix

package singleinstance

import "errors"

// acquire is unimplemented off unix. Conductor's supported platforms (macOS,
// Linux — see rclonebin.manifest) are all unix; this stub keeps a non-unix build
// honest by refusing rather than silently dropping the single-instance
// guarantee (§2.3).
func acquire(string) (func() error, error) {
	return nil, errors.New("single-instance lock is not supported on this platform")
}
