//go:build windows

package session

import "golang.org/x/sys/windows"

// lockPlatform takes an exclusive, non-blocking byte-range lock on the first
// byte of the session file.
//
// Windows releases the range lock when the file handle closes; the zeroed
// Overlapped value anchors the range at offset zero.
func lockPlatform(fd uintptr) error {
	err := windows.LockFileEx(
		windows.Handle(fd),
		windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY,
		0, 1, 0, &windows.Overlapped{},
	)
	if err == windows.ERROR_LOCK_VIOLATION {
		return ErrTokenBusy
	}
	return err
}
