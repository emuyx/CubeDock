//go:build linux

package session

import "golang.org/x/sys/unix"

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
