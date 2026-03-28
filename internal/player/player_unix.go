//go:build !windows

package player

import "os/exec"

// setDetached is a no-op on non-Windows platforms.
func setDetached(cmd *exec.Cmd) {}
