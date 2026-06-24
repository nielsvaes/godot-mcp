//go:build !windows

package client

import "syscall"

// detachSysProcAttr starts the daemon in its own session so it survives the
// CLI process exiting.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
