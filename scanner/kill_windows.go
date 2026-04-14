//go:build windows
// +build windows

package scanner

import (
	"log"
	"os/exec"
	"strconv"
)

func killProcessGroup(pid int) error {
	killCmd := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	if err := killCmd.Run(); err != nil {
		log.Printf("[WARN] taskkill failed for PID %d: %v", pid, err)
		return err
	}
	return nil
}
