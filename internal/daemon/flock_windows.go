package daemon

import (
	"os"

	"golang.org/x/sys/windows"
)

// errLocked is what tryLock returns while another process holds the lock.
var errLocked = windows.ERROR_LOCK_VIOLATION

// Windows has no flock; LockFileEx is the equivalent, and Windows also drops it
// when the holder exits. Its locks are mandatory, though: locking the pid's own
// bytes would keep holderPID from reading them. So the lock covers one byte
// 4 GiB into the file, past anything ever written there.
func region() *windows.Overlapped { return &windows.Overlapped{OffsetHigh: 1} }

func tryLock(f *os.File, exclusive bool) error {
	flags := uint32(windows.LOCKFILE_FAIL_IMMEDIATELY)
	if exclusive {
		flags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}
	return windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, region())
}

func unlock(f *os.File) { _ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, region()) }
