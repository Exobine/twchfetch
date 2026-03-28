//go:build windows

package player

import (
	"os/exec"
	"syscall"
)

// setDetached sets the DETACHED_PROCESS flag so the player runs independently
// of the terminal that owns the TUI.
func setDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x00000008, // DETACHED_PROCESS
	}
}
