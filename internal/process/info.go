/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

package process

import "os"

// Info contains process information needed for attachment
type Info struct {
	UID   uint32 // Effective user ID
	GID   uint32 // Effective group ID
	NsPID int    // PID inside container namespace (or regular PID)
}

// GetTmpPath returns the temporary directory path for the given PID
// This may be container-specific on Linux
func GetTmpPath(pid int) (string, error) {
	path, err := getTmpPathPlatform(pid)
	if err != nil {
		// Check environment variable override
		if jattachPath := os.Getenv("JATTACH_PATH"); jattachPath != "" {
			return jattachPath, nil
		}
		return "/tmp", err
	}
	return path, nil
}
