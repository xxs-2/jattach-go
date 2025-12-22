/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

//go:build linux || freebsd

package protocol

import (
	"fmt"
	"syscall"
)

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
	semID, _, errno := syscall.Syscall(syscall.SYS_SEMGET, uintptr(semKey), 1, 0666|syscall.IPC_CREAT)
	if errno != 0 {
		return fmt.Errorf("semget failed: %v", errno)
	}

	// Perform semop for each notification
	op := syscall.Sembuf{
		SemNum: 0,
		SemOp:  int16(value),
	}
	if value < 0 {
		op.SemFlg = syscall.IPC_NOWAIT
	}

	for i := 0; i < count; i++ {
		syscall.Semop(int(semID), []syscall.Sembuf{op})
	}

	return nil
}
