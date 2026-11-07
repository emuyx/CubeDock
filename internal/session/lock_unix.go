//go:build linux || darwin

package session

import "golang.org/x/sys/unix"

// lockPlatform takes a non-blocking POSIX record lock over the first byte of
// the session file. F_SETLK reports contention as EAGAIN/EACCES.
//
// The record lock is released automatically when the descriptor closes, so the
// caller does not need an explicit unlock path on shutdown.
func lockPlatform(fd uintptr) error {
	request := &unix.Flock_t{Type: unix.F_WRLCK}
	if err := unix.FcntlFlock(fd, unix.F_SETLK, request); err != nil {
		if err == unix.EAGAIN || err == unix.EACCES {
			return ErrTokenBusy
		}
		return err
	}
	return nil
}
