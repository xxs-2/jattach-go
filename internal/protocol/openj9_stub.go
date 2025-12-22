/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

//go:build !linux && !freebsd

package protocol

// notifySemaphore is a no-op on macOS and other platforms
func notifySemaphore(tmpPath string, value int, count int) error {
	// OpenJ9 semaphore operations not supported on this platform
	return nil
}
