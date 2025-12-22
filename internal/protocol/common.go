/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

package protocol

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
)

// IsOpenJ9Process checks if the target process is an OpenJ9 JVM
// by looking for the attachInfo file
func IsOpenJ9Process(tmpPath string, pid int) bool {
	attachInfoPath := filepath.Join(tmpPath, ".com_ibm_tools_attach", strconv.Itoa(pid), "attachInfo")
	var st syscall.Stat_t
	return syscall.Stat(attachInfoPath, &st) == nil
}

// getFileOwner returns the owner UID of a file
func getFileOwner(path string) (uint32, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return 0, err
	}
	return st.Uid, nil
}

// processIsAlive checks if a process is still running
func processIsAlive(pid int) bool {
	// Send signal 0 to check if process exists
	err := syscall.Kill(pid, syscall.Signal(0))
	return err == nil
}

// checkFileOwnership verifies that the file is owned by the current effective user
func checkFileOwnership(path string) bool {
	owner, err := getFileOwner(path)
	if err != nil {
		return false
	}
	return owner == uint32(os.Geteuid())
}

// Response wraps a JVM response
type Response struct {
	Code   int
	Output string
}

// formatError creates an error with context
func formatError(format string, args ...interface{}) error {
	return fmt.Errorf(format, args...)
}
