//go:build darwin

package session

import "golang.org/x/sys/unix"

// lockPlatform uses BSD flock on macOS. The lock belongs to the open file
// description, so a second open in the same process still conflicts.
func lockPlatform(fd uintptr) error {
	return classifyLock(unix.Flock(int(fd), unix.LOCK_EX|unix.LOCK_NB))
}

func classifyLock(err error) error {
	switch {
	case err == nil:
		return nil
	case err == unix.EWOULDBLOCK, err == unix.EAGAIN:
		return ErrTokenBusy
	default:
		return err
	}
}
