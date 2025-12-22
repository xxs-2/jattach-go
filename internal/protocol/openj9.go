/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

package protocol

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const maxNotifFiles = 256

// AttachOpenJ9 performs the OpenJ9 attach sequence
func AttachOpenJ9(ctx context.Context, pid, nspid int, tmpPath string, cmd string, args []string, printOutput bool) (*Response, error) {
	// Acquire global attach lock
	attachLock, err := acquireLock(tmpPath, "", "_attachlock")
	if err != nil {
		return nil, fmt.Errorf("could not acquire attach lock: %w", err)
	}
	defer releaseLock(attachLock)

	// Create listening TCP socket
	listener, port, err := createAttachSocket()
	if err != nil {
		return nil, fmt.Errorf("failed to create attach socket: %w", err)
	}
	defer listener.Close()

	// Generate random authentication key
	key := randomKey()

	// Write replyInfo file with key and port
	replyInfoPath := filepath.Join(tmpPath, ".com_ibm_tools_attach", strconv.Itoa(nspid), "replyInfo")
	if err := writeReplyInfo(replyInfoPath, port, key); err != nil {
		return nil, fmt.Errorf("could not write replyInfo: %w", err)
	}
	defer os.Remove(replyInfoPath)

	// Lock notification files
	notifLocks, notifCount := lockNotificationFiles(tmpPath)
	defer unlockNotificationFiles(notifLocks, notifCount)

	// Notify semaphore to wake JVM threads
	if err := notifySemaphore(tmpPath, 1, notifCount); err != nil {
		// Not fatal, continue
		if printOutput {
			fmt.Printf("Warning: failed to notify semaphore: %v\n", err)
		}
	}
	defer notifySemaphore(tmpPath, -1, notifCount)

	// Accept connection from JVM with timeout
	conn, err := acceptClient(listener, key)
	if err != nil {
		return nil, fmt.Errorf("JVM did not connect: %w", err)
	}
	defer conn.Close()

	if printOutput {
		fmt.Println("Connected to remote JVM")
	}

	// Translate and send command
	translatedCmd := TranslateCommand(cmd, args)
	if err := writeCommandOpenJ9(conn, translatedCmd); err != nil {
		return nil, fmt.Errorf("error writing command: %w", err)
	}

	// Read response
	resp, err := readResponseOpenJ9(conn, translatedCmd, printOutput)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	// Detach cleanly
	if resp.Code != 1 {
		detach(conn)
	}

	return resp, nil
}

// acquireLock acquires a file lock
func acquireLock(tmpPath, subdir, filename string) (*os.File, error) {
	lockPath := filepath.Join(tmpPath, ".com_ibm_tools_attach", subdir, filename)

	// Ensure directory exists
	dir := filepath.Dir(lockPath)
	os.MkdirAll(dir, 0755)

	f, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}

	return f, nil
}

// releaseLock releases a file lock
func releaseLock(f *os.File) {
	if f != nil {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}
}

// createAttachSocket creates a TCP listen socket on a random port
// Tries IPv6 first, falls back to IPv4
func createAttachSocket() (net.Listener, int, error) {
	// Try IPv6 first
	listener, err := net.Listen("tcp6", "[::]:0")
	if err != nil {
		// Fall back to IPv4
		listener, err = net.Listen("tcp4", "0.0.0.0:0")
		if err != nil {
			return nil, 0, err
		}
	}

	addr := listener.Addr().(*net.TCPAddr)
	return listener, addr.Port, nil
}

// randomKey generates a 64-bit random key for authentication
func randomKey() uint64 {
	var key uint64

	// Start with time-based seed
	key = uint64(time.Now().UnixNano()) * 0xc6a4a7935bd1e995

	// Mix in random bytes from crypto/rand
	var randBytes [8]byte
	if _, err := rand.Read(randBytes[:]); err == nil {
		key ^= binary.LittleEndian.Uint64(randBytes[:])
	}

	return key
}

// writeReplyInfo writes the connection info for the JVM to read
func writeReplyInfo(path string, port int, key uint64) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	content := fmt.Sprintf("%016x\n%d\n", key, port)
	return os.WriteFile(path, []byte(content), 0600)
}

// lockNotificationFiles locks all attachNotificationSync files
func lockNotificationFiles(tmpPath string) ([]*os.File, int) {
	locks := make([]*os.File, 0, maxNotifFiles)
	attachDir := filepath.Join(tmpPath, ".com_ibm_tools_attach")

	dir, err := os.Open(attachDir)
	if err != nil {
		return locks, 0
	}
	defer dir.Close()

	entries, err := dir.Readdirnames(-1)
	if err != nil {
		return locks, 0
	}

	for _, entry := range entries {
		if len(entry) == 0 || entry[0] < '1' || entry[0] > '9' {
			continue
		}

		// Check if it's a directory (PID directory)
		entryPath := filepath.Join(attachDir, entry)
		info, err := os.Stat(entryPath)
		if err != nil || !info.IsDir() {
			continue
		}

		// Try to lock the notification file
		lock, err := acquireLock(tmpPath, entry, "attachNotificationSync")
		if err == nil {
			locks = append(locks, lock)
		}

		if len(locks) >= maxNotifFiles {
			break
		}
	}

	return locks, len(locks)
}

// unlockNotificationFiles releases all notification file locks
func unlockNotificationFiles(locks []*os.File, count int) {
	for i := 0; i < count && i < len(locks); i++ {
		if locks[i] != nil {
			releaseLock(locks[i])
		}
	}
}

// Note: notifySemaphore is implemented in openj9_posix.go (Linux/FreeBSD) and openj9_stub.go (other platforms)

// ftok generates a System V IPC key from a path
func ftok(path string, projID int) (int, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		// If file doesn't exist, create it
		f, err := os.Create(path)
		if err != nil {
			return 0, err
		}
		f.Close()

		if err := syscall.Stat(path, &st); err != nil {
			return 0, err
		}
	}

	// Generate key: (projID << 24) | (st_dev & 0xff) << 16 | (st_ino & 0xffff)
	key := (projID << 24) | (int(st.Dev&0xff) << 16) | int(st.Ino&0xffff)
	return key, nil
}

// acceptClient waits for the JVM to connect and validates the authentication key
func acceptClient(listener net.Listener, expectedKey uint64) (net.Conn, error) {
	// Set 5-second timeout for accept
	if tcpListener, ok := listener.(*net.TCPListener); ok {
		tcpListener.SetDeadline(time.Now().Add(5 * time.Second))
	}

	conn, err := listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("JVM did not respond: %w", err)
	}

	// Read authentication message: "ATTACH_CONNECTED {hex_key} "
	authBuf := make([]byte, 35)
	n := 0
	for n < len(authBuf) {
		read, err := conn.Read(authBuf[n:])
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("JVM connection was prematurely closed: %w", err)
		}
		n += read
	}

	// Validate authentication
	expected := fmt.Sprintf("ATTACH_CONNECTED %016x ", expectedKey)
	if string(authBuf) != expected {
		conn.Close()
		return nil, fmt.Errorf("unexpected JVM response")
	}

	// Reset timeout for command execution
	conn.SetDeadline(time.Time{})

	return conn, nil
}

// writeCommandOpenJ9 sends a null-terminated command to the JVM
func writeCommandOpenJ9(conn net.Conn, cmd string) error {
	data := []byte(cmd)
	data = append(data, 0) // Null terminator

	n := 0
	for n < len(data) {
		written, err := conn.Write(data[n:])
		if err != nil {
			return err
		}
		n += written
	}

	return nil
}

// readResponseOpenJ9 reads the JVM response with dynamic buffer allocation
func readResponseOpenJ9(conn net.Conn, cmd string, printOutput bool) (*Response, error) {
	bufSize := 8192
	buf := make([]byte, bufSize)
	offset := 0

	// Read until null terminator
	for {
		n, err := conn.Read(buf[offset:])
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("error reading response: %w", err)
		}
		if n == 0 {
			return nil, fmt.Errorf("unexpected EOF reading response")
		}

		offset += n

		// Check for null terminator
		if buf[offset-1] == 0 {
			break
		}

		// Grow buffer if needed
		if offset >= len(buf) {
			newBuf := make([]byte, len(buf)*2)
			copy(newBuf, buf)
			buf = newBuf
		}
	}

	response := string(buf[:offset-1]) // Exclude null terminator
	code := 0

	// Parse response based on command type
	if strings.HasPrefix(cmd, "ATTACH_LOADAGENT") {
		// Check for agent load success/failure
		if !strings.HasPrefix(response, "ATTACH_ACK") {
			code = -1
			// Parse error code from AgentInitializationException
			if strings.HasPrefix(response, "ATTACH_ERR AgentInitializationException") {
				codeStr := strings.TrimSpace(response[39:])
				fmt.Sscanf(codeStr, "%d", &code)
			}
		}
	} else if strings.HasPrefix(cmd, "ATTACH_DIAGNOSTICS:") && printOutput {
		// Extract diagnostic result
		if idx := strings.Index(response, "openj9_diagnostics.string_result="); idx != -1 {
			result := response[idx+33:]
			printUnescaped(result)
			return &Response{Code: code, Output: result}, nil
		}
	}

	if printOutput {
		fmt.Println(response)
	}

	return &Response{
		Code:   code,
		Output: response,
	}, nil
}

// printUnescaped prints a string with escape sequences unescaped
func printUnescaped(s string) {
	s = strings.TrimSuffix(s, "\n")

	// Unescape Java Properties format escape sequences
	s = strings.ReplaceAll(s, "\\f", "\f")
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\r", "\r")
	s = strings.ReplaceAll(s, "\\t", "\t")
	s = strings.ReplaceAll(s, "\\\\", "\\")

	fmt.Println(s)
}

// detach sends the ATTACH_DETACHED command
func detach(conn net.Conn) {
	writeCommandOpenJ9(conn, "ATTACH_DETACHED")

	// Read response until null terminator
	buf := make([]byte, 256)
	for {
		n, err := conn.Read(buf)
		if err != nil || n == 0 {
			break
		}
		if buf[n-1] == 0 {
			break
		}
	}
}
