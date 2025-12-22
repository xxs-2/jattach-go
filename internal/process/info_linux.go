/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

//go:build linux

package process

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// GetProcessInfo retrieves process information for the given PID on Linux
// Parses /proc/[pid]/status for UID, GID, and NStgid fields
func GetProcessInfo(pid int) (*Info, error) {
	statusPath := filepath.Join("/proc", strconv.Itoa(pid), "status")
	f, err := os.Open(statusPath)
	if err != nil {
		return nil, fmt.Errorf("process %d not found: %w", pid, err)
	}
	defer f.Close()

	info := &Info{NsPID: pid}
	scanner := bufio.NewScanner(f)
	nspidFound := false

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		switch fields[0] {
		case "Uid:":
			// Effective UID is the second value (index 2 in fields)
			if len(fields) >= 3 {
				uid, _ := strconv.ParseUint(fields[2], 10, 32)
				info.UID = uint32(uid)
			}
		case "Gid:":
			// Effective GID is the second value (index 2 in fields)
			if len(fields) >= 3 {
				gid, _ := strconv.ParseUint(fields[2], 10, 32)
				info.GID = uint32(gid)
			}
		case "NStgid:":
			// Last value is the innermost namespace PID
			if len(fields) >= 2 {
				nspid, _ := strconv.Atoi(fields[len(fields)-1])
				info.NsPID = nspid
				nspidFound = true
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading /proc/%d/status: %w", pid, err)
	}

	// Fallback for kernels < 4.1 that don't have NStgid field
	if !nspidFound {
		info.NsPID = altLookupNsPID(pid)
	}

	return info, nil
}

// altLookupNsPID finds the container PID for old kernels < 4.1
// that don't export NStgid in /proc/pid/status
func altLookupNsPID(pid int) int {
	pidNsPath := filepath.Join("/proc", strconv.Itoa(pid), "ns", "pid")

	// Check if we're in the same PID namespace
	var oldNsStat, newNsStat syscall.Stat_t
	if syscall.Stat("/proc/self/ns/pid", &oldNsStat) == nil &&
		syscall.Stat(pidNsPath, &newNsStat) == nil {
		if oldNsStat.Ino == newNsStat.Ino {
			return pid // Same namespace
		}
	}

	// Browse all PIDs in the namespace of the target process
	// trying to find which one corresponds to the host PID
	procPath := filepath.Join("/proc", strconv.Itoa(pid), "root", "proc")
	dir, err := os.Open(procPath)
	if err != nil {
		return pid
	}
	defer dir.Close()

	entries, err := dir.Readdirnames(-1)
	if err != nil {
		return pid
	}

	for _, entry := range entries {
		if len(entry) == 0 || entry[0] < '1' || entry[0] > '9' {
			continue
		}

		// Check if /proc/<container-pid>/sched points back to <host-pid>
		schedPath := filepath.Join("/proc", strconv.Itoa(pid), "root", "proc", entry, "sched")
		if schedGetHostPID(schedPath) == pid {
			nspid, _ := strconv.Atoi(entry)
			return nspid
		}
	}

	return pid
}

// schedGetHostPID extracts the host PID from /proc/pid/sched
// The first line looks like: java (1234, #threads: 12)
// where 1234 is the host PID
func schedGetHostPID(path string) int {
	f, err := os.Open(path)
	if err != nil {
		return -1
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	if !scanner.Scan() {
		return -1
	}

	line := scanner.Text()
	// Find the last '(' and extract the number after it
	idx := strings.LastIndex(line, "(")
	if idx == -1 {
		return -1
	}

	pidStr := strings.TrimSpace(line[idx+1:])
	// Extract just the number before the comma
	if commaIdx := strings.Index(pidStr, ","); commaIdx != -1 {
		pidStr = pidStr[:commaIdx]
	}

	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return -1
	}

	return pid
}

// getTmpPathPlatform returns the /tmp path for the given PID on Linux
// For containerized processes, this is /proc/[pid]/root/tmp
func getTmpPathPlatform(pid int) (string, error) {
	path := filepath.Join("/proc", strconv.Itoa(pid), "root", "tmp")
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		return "/tmp", err
	}
	return path, nil
}
