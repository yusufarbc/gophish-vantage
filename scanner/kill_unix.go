//go:build !windows
// +build !windows

package scanner

import "syscall"

func killProcessGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}
