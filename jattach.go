/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

package jattach

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jattach/jattach-go/internal/process"
	"github.com/jattach/jattach-go/internal/protocol"
)

// Client manages JVM attach operations
type Client struct {
	options *Options
}

// NewClient creates a new jattach client with default options
func NewClient() *Client {
	return &Client{
		options: &Options{
			PrintOutput: false,
			Timeout:     6 * time.Second,
		},
	}
}

// NewClientWithOptions creates a client with custom options
func NewClientWithOptions(opts *Options) *Client {
	if opts.Timeout == 0 {
		opts.Timeout = 6 * time.Second
	}
	return &Client{options: opts}
}

// Attach sends a command to the JVM process
// Returns the JVM's response and any error
func (c *Client) Attach(pid int, cmd string, args ...string) (*Response, error) {
	return c.AttachWithContext(context.Background(), pid, cmd, args...)
}

// AttachWithContext allows cancellation via context
func (c *Client) AttachWithContext(ctx context.Context, pid int, cmd string, args ...string) (*Response, error) {
	// Ignore SIGPIPE to prevent crashes on broken socket writes
	signal.Ignore(syscall.SIGPIPE)

	// Get process information (UID, GID, namespace PID)
	info, err := process.GetProcessInfo(pid)
	if err != nil {
		return nil, wrapError("get_process_info", pid, ErrProcessNotFound)
	}

	// Enter container namespaces if on Linux (net, ipc, mnt)
	mntChanged := 0
	for _, nsType := range []string{"net", "ipc", "mnt"} {
		result, err := process.EnterNamespace(pid, nsType)
		if err != nil {
			// Not fatal, continue
			if c.options.Logger != nil {
				c.options.Logger.Printf("Warning: failed to enter %s namespace: %v", nsType, err)
			}
		}
		if nsType == "mnt" && result > 0 {
			mntChanged = result
		}
	}

	// Switch to target process credentials (required by HotSpot security model)
	if err := syscall.Setgid(int(info.GID)); err != nil {
		return nil, wrapError("setgid", pid, ErrPermissionDenied)
	}
	if err := syscall.Setuid(int(info.UID)); err != nil {
		return nil, wrapError("setuid", pid, ErrPermissionDenied)
	}

	// Determine temporary path
	tmpPath := c.options.TmpPath
	if tmpPath == "" {
		tmpPath = os.Getenv("JATTACH_PATH")
	}
	if tmpPath == "" {
		var err error
		tmpPath, err = process.GetTmpPath(pid)
		if err != nil {
			tmpPath = "/tmp"
		}
	}

	// Detect JVM type (OpenJ9 vs HotSpot)
	jvmType := JVMTypeHotSpot
	if protocol.IsOpenJ9Process(tmpPath, info.NsPID) {
		jvmType = JVMTypeOpenJ9
	}

	// Dispatch to appropriate protocol handler
	var protoResp *protocol.Response
	if jvmType == JVMTypeOpenJ9 {
		protoResp, err = protocol.AttachOpenJ9(ctx, pid, info.NsPID, tmpPath, cmd, args, c.options.PrintOutput)
	} else {
		protoResp, err = protocol.AttachHotSpot(ctx, pid, info.NsPID, tmpPath, mntChanged, cmd, args, c.options.PrintOutput)
	}

	if err != nil {
		return nil, wrapError("attach", pid, err)
	}

	// Convert protocol.Response to jattach.Response
	resp := &Response{
		Code:    protoResp.Code,
		Output:  protoResp.Output,
		JVMType: jvmType,
	}

	return resp, nil
}

// LoadAgent loads a native agent library into the target JVM
// If absolute is true, uses the absolute path (ATTACH_LOADAGENTPATH)
// If absolute is false, searches java.library.path (ATTACH_LOADAGENT)
func (c *Client) LoadAgent(pid int, agentPath string, absolute bool, options string) (*Response, error) {
	args := []string{agentPath}
	if absolute {
		args = append(args, "true")
	} else {
		args = append(args, "false")
	}
	if options != "" {
		args = append(args, options)
	}
	return c.Attach(pid, CmdLoad, args...)
}

// LoadJavaAgent loads a Java agent (via the instrument library)
func (c *Client) LoadJavaAgent(pid int, jarPath string, options string) (*Response, error) {
	// instrument library is in java.library.path, so use non-absolute load
	instrumentArgs := jarPath
	if options != "" {
		instrumentArgs += "=" + options
	}
	return c.Attach(pid, CmdLoad, "instrument", "false", instrumentArgs)
}

// ThreadDump gets a thread dump from the target JVM
func (c *Client) ThreadDump(pid int) (*Response, error) {
	return c.Attach(pid, CmdThreadDump)
}

// HeapDump creates a heap dump file
func (c *Client) HeapDump(pid int, filepath string) (*Response, error) {
	return c.Attach(pid, CmdDumpHeap, filepath)
}

// ExecuteJCmd executes a jcmd command
func (c *Client) ExecuteJCmd(pid int, command string, args ...string) (*Response, error) {
	cmdArgs := append([]string{command}, args...)
	return c.Attach(pid, CmdJCmd, cmdArgs...)
}

// GetProperties retrieves system properties from the JVM
func (c *Client) GetProperties(pid int) (*Response, error) {
	return c.Attach(pid, CmdProperties)
}

// GetAgentProperties retrieves agent properties from the JVM
func (c *Client) GetAgentProperties(pid int) (*Response, error) {
	return c.Attach(pid, CmdAgentProperties)
}

// SetFlag modifies a manageable VM flag
func (c *Client) SetFlag(pid int, flag string, value string) (*Response, error) {
	return c.Attach(pid, CmdSetFlag, flag, value)
}

// PrintFlag prints a specific VM flag value
func (c *Client) PrintFlag(pid int, flag string) (*Response, error) {
	return c.Attach(pid, CmdPrintFlag, flag)
}

// Convenience function for simple one-off operations
// Attach performs a one-time attach operation with default options
func Attach(pid int, cmd string, args ...string) (*Response, error) {
	client := NewClient()
	return client.Attach(pid, cmd, args...)
}
