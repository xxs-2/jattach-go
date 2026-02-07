/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/xxs-2/jattach-go"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: basic-example <java-pid>")
		os.Exit(1)
	}

	pid, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid PID: %v\n", err)
		os.Exit(1)
	}

	// Create a client
	client := jattach.NewClient()

	// Get JVM version
	fmt.Println("=== JVM Version ===")
	resp, err := client.ExecuteJCmd(pid, "VM.version")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Detected JVM type: %s\n", resp.JVMType)
	fmt.Printf("Response code: %d\n", resp.Code)
	fmt.Printf("Output:\n%s\n", resp.Output)

	// Get thread dump
	fmt.Println("\n=== Thread Dump ===")
	resp, err = client.ThreadDump(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Response code: %d\n", resp.Code)
	fmt.Printf("Output (first 500 chars):\n%s...\n", truncate(resp.Output, 500))

	// Get system properties
	fmt.Println("\n=== System Properties ===")
	resp, err = client.GetProperties(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Response code: %d\n", resp.Code)
	fmt.Printf("Output (first 500 chars):\n%s...\n", truncate(resp.Output, 500))

	fmt.Println("\n✅ All operations completed successfully!")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
