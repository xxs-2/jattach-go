/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

//go:build freebsd

package process

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

// GetProcessInfo retrieves process information for the given PID on FreeBSD
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

	// FreeBSD uses different field names than macOS
	return &Info{
		UID:   uint32(kinfo.Ki_uid),
		GID:   uint32(kinfo.Ki_groups[0]),
		NsPID: pid, // No namespaces on FreeBSD
	}, nil
}

// getTmpPathPlatform returns the default /tmp on FreeBSD
func getTmpPathPlatform(pid int) (string, error) {
	return "/tmp", nil
}
