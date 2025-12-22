/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

//go:build linux

package process

import (
	"fmt"
	"path/filepath"
	"strconv"
	"syscall"
)

// EnterNamespace switches to the namespace of the target process
// Returns: 1 if switched, 0 if already in same namespace, error if failed
func EnterNamespace(pid int, nsType string) (int, error) {
	selfPath := filepath.Join("/proc/self/ns", nsType)
	targetPath := filepath.Join("/proc", strconv.Itoa(pid), "ns", nsType)

	var selfStat, targetStat syscall.Stat_t
	if err := syscall.Stat(selfPath, &selfStat); err != nil {
		return -1, fmt.Errorf("failed to stat self namespace: %w", err)
	}
	if err := syscall.Stat(targetPath, &targetStat); err != nil {
		return -1, fmt.Errorf("failed to stat target namespace: %w", err)
	}

	// Already in the same namespace
	if selfStat.Ino == targetStat.Ino {
		return 0, nil
	}

	// Open the target namespace
	fd, err := syscall.Open(targetPath, syscall.O_RDONLY, 0)
	if err != nil {
		return -1, fmt.Errorf("failed to open namespace: %w", err)
	}
	defer syscall.Close(fd)

	// Switch to the namespace using setns syscall
	_, _, errno := syscall.Syscall(syscall.SYS_SETNS, uintptr(fd), 0, 0)
	if errno != 0 {
		return -1, fmt.Errorf("setns failed: %v", errno)
	}

	return 1, nil
}
