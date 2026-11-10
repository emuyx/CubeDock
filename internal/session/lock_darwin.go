//go:build darwin

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
