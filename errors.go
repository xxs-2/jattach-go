/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

package jattach

import (
	"errors"
	"fmt"
)

var (
	// ErrProcessNotFound indicates the target process doesn't exist
	ErrProcessNotFound = errors.New("process not found")

	// ErrPermissionDenied indicates insufficient permissions to attach
	ErrPermissionDenied = errors.New("permission denied")

	// ErrNotJavaProcess indicates the target is not a JVM process
	ErrNotJavaProcess = errors.New("not a Java process")

	// ErrConnectionFailed indicates connection to JVM failed
	ErrConnectionFailed = errors.New("connection failed")

	// ErrTimeout indicates the operation timed out
	ErrTimeout = errors.New("timeout")

	// ErrBitnessMatch indicates 32/64-bit mismatch (Windows)
	ErrBitnessMatch = errors.New("bitness mismatch")

	// ErrAgentLoadFailed indicates agent loading failed
	ErrAgentLoadFailed = errors.New("agent load failed")
)

// AttachError wraps errors with context about the attach operation
type AttachError struct {
	Op  string // Operation that failed
	PID int    // Target PID
	Err error  // Underlying error
}

func (e *AttachError) Error() string {
	return fmt.Sprintf("jattach: %s (pid=%d): %v", e.Op, e.PID, e.Err)
}

func (e *AttachError) Unwrap() error {
	return e.Err
}

// wrapError creates a wrapped error with context
func wrapError(op string, pid int, err error) error {
	if err == nil {
		return nil
	}
	return &AttachError{Op: op, PID: pid, Err: err}
}
