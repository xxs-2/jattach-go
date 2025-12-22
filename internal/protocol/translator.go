/*
 * Copyright The jattach authors
 * SPDX-License-Identifier: Apache-2.0
 */

package protocol

import (
	"fmt"
	"strings"
)

// TranslateCommand converts HotSpot command syntax to OpenJ9 equivalent
func TranslateCommand(cmd string, args []string) string {
	switch cmd {
	case "load":
		// load <path> [absolute] [options]
		if len(args) == 0 {
			return "ATTACH_LOADAGENT(,)"
		}
		path := args[0]
		absolute := len(args) > 1 && args[1] == "true"
		options := ""
		if len(args) > 2 {
			options = args[2]
		}

		if absolute {
			return fmt.Sprintf("ATTACH_LOADAGENTPATH(%s,%s)", path, options)
		}
		return fmt.Sprintf("ATTACH_LOADAGENT(%s,%s)", path, options)

	case "jcmd":
		// jcmd <command> [args...]
		if len(args) == 0 {
			return "ATTACH_DIAGNOSTICS:help"
		}
		// Join all args with commas
		return "ATTACH_DIAGNOSTICS:" + strings.Join(args, ",")

	case "threaddump":
		// threaddump [options]
		opts := ""
		if len(args) > 0 {
			opts = args[0]
		}
		return fmt.Sprintf("ATTACH_DIAGNOSTICS:Thread.print,%s", opts)

	case "dumpheap":
		// dumpheap <path>
		path := ""
		if len(args) > 0 {
			path = args[0]
		}
		return fmt.Sprintf("ATTACH_DIAGNOSTICS:Dump.heap,%s", path)

	case "inspectheap":
		// inspectheap [options]
		opts := ""
		if len(args) > 0 {
			opts = args[0]
		}
		return fmt.Sprintf("ATTACH_DIAGNOSTICS:GC.class_histogram,%s", opts)

	case "datadump":
		// datadump [options]
		opts := ""
		if len(args) > 0 {
			opts = args[0]
		}
		return fmt.Sprintf("ATTACH_DIAGNOSTICS:Dump.java,%s", opts)

	case "properties":
		return "ATTACH_GETSYSTEMPROPERTIES"

	case "agentProperties":
		return "ATTACH_GETAGENTPROPERTIES"

	default:
		// Unknown command, pass through
		return cmd
	}
}
