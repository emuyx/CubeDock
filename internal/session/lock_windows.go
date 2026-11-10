//go:build windows

package session

import (
	"os"

	"golang.org/x/sys/windows"
)

// openLockFile opens (creating if needed) the lease file. Opening a reparse
// point without FILE_FLAG_OPEN_REPARSE_POINT would require CreateFile; the
// lease only creates a missing file, so os.OpenFile is sufficient here.
func openLockFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
}

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
