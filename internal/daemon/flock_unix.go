//go:build !windows

package daemon

import (
	"os"
	"syscall"
)

// errLocked is what tryLock returns while another process holds the lock.
var errLocked = syscall.EWOULDBLOCK

func tryLock(f *os.File, exclusive bool) error {
	how := syscall.LOCK_SH
	if exclusive {
		how = syscall.LOCK_EX
	}
	return syscall.Flock(int(f.Fd()), how|syscall.LOCK_NB)
}

func unlock(f *os.File) { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN) }
