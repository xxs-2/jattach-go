/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

//go:build darwin

package process

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/unix"
)

// GetProcessInfo retrieves process information for the given PID on macOS
// Uses sysctl with KERN_PROC to get process credentials
func GetProcessInfo(pid int) (*Info, error) {
	mib := []int32{1, 14, 1, int32(pid)} // CTL_KERN, KERN_PROC, KERN_PROC_PID, pid

	var kinfo unix.KinfoProc
	size := unsafe.Sizeof(kinfo)
	sizePtr := size

	_, _, errno := unix.Syscall6(
		unix.SYS___SYSCTL,
		uintptr(unsafe.Pointer(&mib[0])),
		uintptr(len(mib)),
		uintptr(unsafe.Pointer(&kinfo)),
		uintptr(unsafe.Pointer(&sizePtr)),
		0, 0,
	)

	if errno != 0 {
		return nil, fmt.Errorf("process %d not found: %v", pid, errno)
	}

	return &Info{
		UID:   kinfo.Eproc.Ucred.Uid,
		GID:   uint32(kinfo.Eproc.Pcred.P_svgid),
		NsPID: pid, // No namespaces on macOS
	}, nil
}

// getTmpPathPlatform returns the user-specific temp directory on macOS
// macOS uses a secure per-user temporary directory
func getTmpPathPlatform(pid int) (string, error) {
	// macOS uses TMPDIR environment variable for per-user temp
	tmpDir := os.Getenv("TMPDIR")
	if tmpDir == "" {
		tmpDir = "/tmp"
	}
	return tmpDir, nil
}
