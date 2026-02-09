//go:build linux

/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

package process

import (
	"fmt"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
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
	// 使用 unix.O_CLOEXEC 确保文件描述符不会在 exec 时泄露
	fd, err := unix.Open(targetPath, unix.O_RDONLY|unix.O_CLOEXEC, 0)
	if err != nil {
		return -1, fmt.Errorf("failed to open namespace: %w", err)
	}
	defer unix.Close(fd)

	// Switch to the namespace using unix.Setns
	// 替代原有的 syscall.Syscall(syscall.SYS_SETNS, ...)
	if err := unix.Setns(fd, 0); err != nil {
		return -1, fmt.Errorf("setns failed: %w", err)
	}

	return 1, nil
}
