/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

//go:build !linux

package process

// EnterNamespace is a no-op on non-Linux platforms
// Returns 0 (no switch needed)
func EnterNamespace(pid int, nsType string) (int, error) {
	return 0, nil
}
