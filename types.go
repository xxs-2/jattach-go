/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

package jattach

import "time"

// JVMType indicates the detected JVM implementation
type JVMType int

const (
	// JVMTypeUnknown indicates the JVM type could not be determined
	JVMTypeUnknown JVMType = iota
	// JVMTypeHotSpot indicates Oracle HotSpot or OpenJDK
	JVMTypeHotSpot
	// JVMTypeOpenJ9 indicates IBM OpenJ9
	JVMTypeOpenJ9
)

func (t JVMType) String() string {
	switch t {
	case JVMTypeHotSpot:
		return "HotSpot"
	case JVMTypeOpenJ9:
		return "OpenJ9"
	default:
		return "Unknown"
	}
}

// Response contains the result from a JVM attach operation
type Response struct {
	// Code is the return code from the JVM operation (0 = success)
	Code int

	// Output contains the response text from the JVM
	Output string

	// JVMType indicates which JVM type was detected
	JVMType JVMType
}

// Options configures attach behavior
type Options struct {
	// PrintOutput controls whether JVM responses are printed to stdout
	PrintOutput bool

	// TmpPath overrides the default temporary directory path
	// Equivalent to JATTACH_PATH environment variable
	TmpPath string

	// Timeout for connection attempts (default: 6 seconds for HotSpot)
	Timeout time.Duration

	// Logger for diagnostic output (optional)
	Logger Logger
}

// Logger interface for diagnostic output
type Logger interface {
	Printf(format string, v ...interface{})
}

// Command constants for convenience
const (
	// CmdLoad loads a native agent library or Java agent
	CmdLoad = "load"

	// CmdThreadDump requests a thread dump
	CmdThreadDump = "threaddump"

	// CmdDumpHeap creates a heap dump file
	CmdDumpHeap = "dumpheap"

	// CmdInspectHeap shows heap histogram/class statistics
	CmdInspectHeap = "inspectheap"

	// CmdDataDump shows heap and thread summary
	CmdDataDump = "datadump"

	// CmdJCmd executes a jcmd command
	CmdJCmd = "jcmd"

	// CmdProperties prints system properties
	CmdProperties = "properties"

	// CmdAgentProperties prints agent-specific properties
	CmdAgentProperties = "agentProperties"

	// CmdSetFlag modifies a manageable VM flag
	CmdSetFlag = "setflag"

	// CmdPrintFlag prints a specific VM flag
	CmdPrintFlag = "printflag"
)
