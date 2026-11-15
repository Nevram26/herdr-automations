package main

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// detached starts a process with no console and in its own process group, so
// it outlives the caller and no console window opens for it.
func detached() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP, HideWindow: true}
}
