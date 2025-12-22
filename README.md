# jattach-go

Pure Go implementation of [jattach](https://github.com/apangin/jattach) - a utility to send commands to a JVM process via Dynamic Attach mechanism.

## Features

- **Pure Go**: No CGO dependencies, easy cross-compilation
- **Multi-platform**: Linux, macOS, FreeBSD
- **Multi-JVM**: Supports both HotSpot/OpenJDK and IBM OpenJ9
- **Container-aware**: Full Linux namespace support for Docker/Kubernetes
- **Library and CLI**: Use as a Go library or standalone command-line tool
- **Context support**: Cancellable operations with `context.Context`

## Installation

### As a library

```bash
go get github.com/jattach/jattach-go
```

### As a CLI tool

```bash
cd cmd/jattach
go build
sudo ./jattach <pid> <cmd> [args...]
```

## Usage

### Library Usage

```go
package main

import (
    "fmt"
    "github.com/jattach/jattach-go"
)

func main() {
    client := jattach.NewClient()

    // Get thread dump
    resp, err := client.ThreadDump(1234)
    if err != nil {
        panic(err)
    }
    fmt.Printf("JVM Type: %s\n", resp.JVMType)
    fmt.Printf("Output:\n%s\n", resp.Output)
}
```

### High-Level API

```go
// Load a native agent
resp, err := client.LoadAgent(pid, "/path/to/agent.so", true, "options")

// Load a Java agent
resp, err := client.LoadJavaAgent(pid, "/path/to/agent.jar", "options")

// Get thread dump
resp, err := client.ThreadDump(pid)

// Create heap dump
resp, err := client.HeapDump(pid, "/tmp/heap.hprof")

// Execute jcmd command
resp, err := client.ExecuteJCmd(pid, "VM.version")

// Get system properties
resp, err := client.GetProperties(pid)

// Get agent properties
resp, err := client.GetAgentProperties(pid)

// Set VM flag
resp, err := client.SetFlag(pid, "PrintGCDetails", "true")

// Print VM flag
resp, err := client.PrintFlag(pid, "MaxHeapSize")
```

### Low-Level API

```go
// Generic attach with custom command
resp, err := client.Attach(pid, "jcmd", "VM.flags", "-all")
```

### With Context (Cancellation)

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

resp, err := client.AttachWithContext(ctx, pid, "threaddump")
```

### Custom Options

```go
client := jattach.NewClientWithOptions(&jattach.Options{
    PrintOutput: true,                    // Print JVM responses to stdout
    TmpPath:     "/custom/tmp",          // Override temp directory
    Timeout:     10 * time.Second,       // Connection timeout
})
```

## CLI Usage

```bash
# Get JVM version
jattach 1234 jcmd VM.version

# Thread dump
jattach 1234 threaddump

# Heap dump
jattach 1234 dumpheap /tmp/heap.hprof

# Load native agent
jattach 1234 load /path/to/agent.so true options

# Load Java agent
jattach 1234 load instrument false myagent.jar=options

# System properties
jattach 1234 properties

# Execute jcmd
jattach 1234 jcmd GC.heap_info
```

## Supported Commands

| Command | Description |
|---------|-------------|
| `load` | Load native agent library (`Agent_OnAttach`) |
| `properties` | Print system properties |
| `agentProperties` | Print agent-specific properties |
| `datadump` | Show heap and thread summary |
| `threaddump` | Dump all stack traces |
| `dumpheap` | Dump heap to HPROF file |
| `inspectheap` | Heap histogram (class statistics) |
| `setflag` | Modify manageable VM flag |
| `printflag` | Print specific VM flag value |
| `jcmd` | Execute arbitrary jcmd command |

## Platform Support

| Platform | HotSpot/OpenJDK | OpenJ9 | Containers |
|----------|-----------------|--------|------------|
| Linux    | ✅              | ✅     | ✅         |
| macOS    | ✅              | ✅     | N/A        |
| FreeBSD  | ✅              | ✅     | N/A        |
| Windows  | ⚠️ Planned      | ❌     | N/A        |

## How It Works

### HotSpot/OpenJDK Protocol

1. Creates `.attach_pid<pid>` trigger file
2. Sends `SIGQUIT` to JVM process
3. Waits for Unix domain socket `.java_pid<pid>` to appear
4. Connects to socket and sends command
5. Reads response

### OpenJ9 Protocol

1. Creates TCP listen socket on random port
2. Generates random authentication key
3. Writes connection info to `replyInfo` file
4. Signals semaphore to wake JVM threads
5. Waits for JVM to connect back with authentication
6. Sends translated command
7. Reads response and detaches

## Container Support (Linux)

The library automatically handles Docker/Kubernetes containers:

- Detects PID namespace (uses `NStgid` from `/proc/[pid]/status`)
- Enters container namespaces (net, ipc, mnt) with `setns()`
- Accesses container-specific `/tmp` via `/proc/[pid]/root/tmp`
- Switches to target process UID/GID for security

## Error Handling

```go
resp, err := client.Attach(pid, cmd, args...)
if err != nil {
    // Check for specific errors
    if errors.Is(err, jattach.ErrProcessNotFound) {
        // Process doesn't exist
    } else if errors.Is(err, jattach.ErrPermissionDenied) {
        // Need root or same user as JVM
    } else if errors.Is(err, jattach.ErrTimeout) {
        // JVM didn't respond in time
    }

    // Get detailed context
    var attachErr *jattach.AttachError
    if errors.As(err, &attachErr) {
        fmt.Printf("Operation: %s, PID: %d\n", attachErr.Op, attachErr.PID)
    }
}

// Check JVM response code
if resp.Code != 0 {
    fmt.Printf("JVM returned error code: %d\n", resp.Code)
}
```

## Examples

See the [examples](examples/) directory:

- [basic](examples/basic/main.go) - Simple usage demonstrating common operations

## Comparison with C jattach

| Feature | jattach (C) | jattach-go |
|---------|-------------|------------|
| Language | C | Go |
| Dependencies | libc, POSIX APIs | Pure Go (stdlib only) |
| Cross-compilation | Difficult | Easy (`GOOS=linux go build`) |
| Library usage | Via CGO | Native Go import |
| Platforms | Linux, macOS, FreeBSD, Windows | Linux, macOS, FreeBSD |
| Size | ~40 KB | ~2-3 MB (Go runtime) |
| Performance | Instant | Instant (sub-ms overhead) |

## Building from Source

```bash
# Clone the repository
git clone https://github.com/jattach/jattach-go.git
cd jattach-go

# Build the library
go build

# Build the CLI tool
cd cmd/jattach
go build

# Cross-compile for different platforms
GOOS=linux GOARCH=amd64 go build
GOOS=darwin GOARCH=arm64 go build
```

## Requirements

- Go 1.21 or later
- Target JVM must support Dynamic Attach mechanism:
  - Oracle/OpenJDK HotSpot
  - IBM OpenJ9
  - Other JVMs implementing the standard attach protocol

## Security Notes

- Requires same UID as target JVM (or root)
- On Linux, may need `CAP_SYS_PTRACE` capability for namespace switching
- For containers, may need `CAP_SYS_ADMIN` for `setns()`

## License

Apache License 2.0

## Contributing

Contributions welcome! Please ensure:

1. Code follows Go conventions
2. All platforms tested (use build tags)
3. Tests pass
4. Documentation updated

## Credits

Based on the original [jattach](https://github.com/apangin/jattach) by Andrei Pangin.

Ported to Go with platform-specific implementations using Go build tags instead of C preprocessor directives.
