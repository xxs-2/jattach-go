/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

package protocol

import (
	"bytes"
	"context"
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

// AttachHotSpot performs the HotSpot/OpenJDK attach sequence
func AttachHotSpot(ctx context.Context, pid, nspid int, tmpPath string, mntChanged int, cmd string, args []string, printOutput bool) (*Response, error) {
	socketPath := filepath.Join(tmpPath, fmt.Sprintf(".java_pid%d", nspid))

	// Check if socket already exists
	if !checkSocket(socketPath) {
		// Start attach mechanism (create trigger file + SIGQUIT + wait)
		if err := startAttachMechanism(ctx, pid, nspid, tmpPath, mntChanged, socketPath); err != nil {
			return nil, fmt.Errorf("could not start attach mechanism: %w", err)
		}
	}

	// Connect to Unix domain socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return nil, fmt.Errorf("could not connect to socket: %w", err)
	}
	defer conn.Close()

	if printOutput {
		fmt.Println("Connected to remote JVM")
	}

	// Write command
	if err := writeCommand(conn, cmd, args); err != nil {
		return nil, fmt.Errorf("error writing command: %w", err)
	}

	// Read response
	resp, err := readResponse(conn, cmd, args, printOutput)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %w", err)
	}

	return resp, nil
}

// checkSocket verifies that a socket file exists and is actually a socket
func checkSocket(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSocket != 0
}

// startAttachMechanism triggers the JVM attach listener
// Creates .attach_pid file, sends SIGQUIT, and polls for socket
func startAttachMechanism(ctx context.Context, pid, nspid int, tmpPath string, mntChanged int, socketPath string) error {
	// Determine attach trigger file path
	var attachPath string
	if mntChanged > 0 {
		attachPath = filepath.Join("/proc", strconv.Itoa(nspid), "cwd", fmt.Sprintf(".attach_pid%d", nspid))
	} else {
		attachPath = filepath.Join("/proc", strconv.Itoa(pid), "cwd", fmt.Sprintf(".attach_pid%d", nspid))
	}

	// Create attach trigger file
	f, err := os.OpenFile(attachPath, os.O_CREATE|os.O_WRONLY, 0660)
	useFallback := false

	if err != nil || !checkFileOwnership(attachPath) {
		// Some mounted filesystems may change the ownership of the file.
		// JVM will not trust such file, so try a different path
		os.Remove(attachPath)
		useFallback = true
	}

	if useFallback {
		// Fallback to /tmp
		attachPath = filepath.Join(tmpPath, fmt.Sprintf(".attach_pid%d", nspid))
		f, err = os.OpenFile(attachPath, os.O_CREATE|os.O_WRONLY, 0660)
		if err != nil {
			return fmt.Errorf("failed to create attach trigger file: %w", err)
		}
	}

	f.Close()
	defer os.Remove(attachPath)

	// Send SIGQUIT to trigger attach listener (use host PID, not namespace PID)
	if err := syscall.Kill(pid, syscall.SIGQUIT); err != nil {
		return fmt.Errorf("failed to send SIGQUIT: %w", err)
	}

	// Poll for socket with exponential backoff
	delay := 20 * time.Millisecond
	maxDelay := 500 * time.Millisecond
	timeout := 6 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Check if socket appeared
		if checkSocket(socketPath) {
			return nil
		}

		// Check if process is still alive
		if !processIsAlive(pid) {
			return fmt.Errorf("process %d died while waiting for attach", pid)
		}

		// Sleep with exponential backoff
		time.Sleep(delay)
		delay += 20 * time.Millisecond
		if delay > maxDelay {
			delay = maxDelay
		}
	}

	return fmt.Errorf("timeout waiting for socket to appear")
}

// writeCommand sends a command to the JVM via the socket
// Protocol: "1\x00" + cmd + "\x00" + arg1 + "\x00" + arg2 + "\x00" + arg3 + "\x00"
func writeCommand(conn net.Conn, cmd string, args []string) error {
	buf := &bytes.Buffer{}

	// Protocol version
	buf.WriteString("1")
	buf.WriteByte(0)

	// Handle jcmd special case: max 2 arguments, merge extras
	cmdArgs := args
	if cmd == "jcmd" && len(args) > 1 {
		// Merge all args after the first one
		merged := strings.Join(args[1:], " ")
		cmdArgs = []string{args[0], merged}
	} else if len(args) > 3 {
		// For other commands: max 3 arguments, merge extras into last
		merged := strings.Join(args[3:], " ")
		cmdArgs = append(args[:3], merged)
	}

	// Write command
	buf.WriteString(cmd)
	buf.WriteByte(0)

	// Write arguments (pad to 3 args with null bytes)
	for i := 0; i < 3; i++ {
		if i < len(cmdArgs) {
			buf.WriteString(cmdArgs[i])
		}
		buf.WriteByte(0)
	}

	// Send to socket
	_, err := conn.Write(buf.Bytes())
	return err
}

// readResponse reads and parses the JVM response
// Special handling for 'load' command to extract Agent_OnAttach result
func readResponse(conn net.Conn, cmd string, args []string, printOutput bool) (*Response, error) {
	buf := make([]byte, 8192)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("error reading response: %w", err)
	}
	if n == 0 {
		return nil, fmt.Errorf("unexpected EOF reading response")
	}

	data := buf[:n]

	// Parse return code from first line
	code := 0
	lines := bytes.SplitN(data, []byte{'\n'}, 2)
	if len(lines) > 0 {
		codeStr := string(bytes.TrimSpace(lines[0]))
		code, _ = strconv.Atoi(codeStr)
	}

	// Special handling for 'load' command
	if cmd == "load" {
		// Read all remaining data
		remaining := &bytes.Buffer{}
		remaining.Write(data)
		io.Copy(remaining, conn)

		output := remaining.String()

		// Parse Agent_OnAttach return code
		if code == 0 && len(output) >= 2 {
			secondLine := ""
			if lines := strings.SplitN(output, "\n", 3); len(lines) >= 2 {
				secondLine = strings.TrimSpace(lines[1])
			}

			if strings.HasPrefix(secondLine, "return code: ") {
				// JDK 9+: Agent_OnAttach result after "return code: "
				codeStr := strings.TrimSpace(secondLine[13:])
				code, _ = strconv.Atoi(codeStr)
			} else if len(secondLine) > 0 && (secondLine[0] >= '0' && secondLine[0] <= '9' || secondLine[0] == '-') {
				// JDK 8: Agent_OnAttach result on second line alone
				code, _ = strconv.Atoi(secondLine)
			} else if len(secondLine) > 0 {
				// JDK 21+: load command always returns 0; rest is error message
				code = -1
			}
		}

		// Print error message if load failed
		if code != 0 && !printOutput {
			if len(lines) >= 2 {
				fmt.Fprint(os.Stderr, string(lines[1]))
			} else if len(args) > 0 {
				fmt.Fprintf(os.Stderr, "Target JVM failed to load %s\n", args[0])
			}
		}

		if printOutput {
			fmt.Print("JVM response code = ")
			fmt.Print(output)
		}

		return &Response{
			Code:   code,
			Output: output,
		}, nil
	}

	// For other commands, just read remaining data
	if printOutput {
		fmt.Print("JVM response code = ")
		fmt.Print(string(data))

		// Read and print remaining output
		buf2 := make([]byte, 8192)
		for {
			n, err := conn.Read(buf2)
			if n > 0 {
				fmt.Print(string(buf2[:n]))
			}
			if err != nil {
				break
			}
		}
		fmt.Println()
	}

	return &Response{
		Code:   code,
		Output: string(data),
	}, nil
}
