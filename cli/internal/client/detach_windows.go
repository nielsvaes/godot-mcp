//go:build windows

package client

import "syscall"

const (
	detachedProcess    = 0x00000008
	createNewProcGroup = 0x00000200
)

// detachSysProcAttr starts the daemon detached from the CLI console.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: detachedProcess | createNewProcGroup}
}
