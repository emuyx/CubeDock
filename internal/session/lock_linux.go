//go:build linux

package session

import (
	"os"

	"golang.org/x/sys/unix"
)

// openLockFile opens (creating if needed) the lease file without following a
// symlink, so a same-user attacker cannot redirect the lock to another inode.
func openLockFile(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_RDWR|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), path), nil
}

// lockPlatform uses an open-file-description lock on Linux. Unlike classic
// POSIX F_SETLK (per-process), OFD locks conflict between separate opens even
// inside one process, which is what the account lease requires.
func lockPlatform(fd uintptr) error {
	lock := unix.Flock_t{Type: unix.F_WRLCK}
	return classifyLock(unix.FcntlFlock(fd, unix.F_OFD_SETLK, &lock))
}

func classifyLock(err error) error {
	switch {
	case err == nil:
		return nil
	case err == unix.EAGAIN, err == unix.EACCES:
		return ErrTokenBusy
	default:
		return err
	}
}
