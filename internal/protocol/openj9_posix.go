//go:build linux || freebsd

/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

package protocol

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Sembuf matched the System V sembuf structure
type sembuf struct {
	SemNum uint16
	SemOp  int16
	SemFlg int16
}

// notifySemaphore signals the OpenJ9 semaphore (Linux/FreeBSD only)
func notifySemaphore(tmpPath string, value int, count int) error {
	if count == 0 {
		return nil
	}

	notifierPath := tmpPath + "/.com_ibm_tools_attach/_notifier"

	// Generate semaphore key using ftok
	semKey, err := ftok(notifierPath, 0xa1)
	if err != nil {
		return err
	}

	// Get or create semaphore
	semID, _, errno := syscall.Syscall(unix.SYS_SEMGET, uintptr(semKey), 1, 0666|unix.IPC_CREAT)
	if errno != 0 {
		return fmt.Errorf("semget failed: %v", errno)
	}

	// Perform semop for each notification
	op := sembuf{
		SemNum: 0,
		SemOp:  int16(value),
	}
	if value < 0 {
		op.SemFlg = int16(unix.IPC_NOWAIT)
	}

	for i := 0; i < count; i++ {
		_, _, errno := syscall.Syscall(unix.SYS_SEMOP, semID, uintptr(unsafe.Pointer(&op)), 1)
		if errno != 0 {
			// Maintain original behavior of continuing on failure
			continue
		}
	}

	return nil
}
