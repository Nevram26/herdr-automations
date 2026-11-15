//go:build !windows

package main

import "syscall"

// detached starts a process in its own session, so it outlives the caller.
func detached() *syscall.SysProcAttr { return &syscall.SysProcAttr{Setsid: true} }
