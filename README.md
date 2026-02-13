# SSH MCP Server

A high-performance SSH connection management server implementing the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/). Enables AI agents to execute commands, manage files, and perform DevOps operations across remote infrastructure via persistent SSH sessions.

[![Go Version](https://img.shields.io/badge/go-1.25+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

## Features

- **43 Production Tools** - SSH, SFTP, Docker, databases, system monitoring, and VoIP diagnostics
- **Persistent SSH Sessions** - Connection pooling with automatic lifecycle management
- **Jump Host/Bastion Support** - Multi-hop SSH tunneling via `via` parameter
- **Session Isolation** - Thread-safe per-client connection pools with configurable cleanup
- **Auto-Sudo Detection** - Automatically prefixes `sudo` for non-root users when needed
- **Server-Side Syntax Validation** - Go-native validators for YAML, JSON, TOML, XML, INI, Dockerfile, ENV — zero remote host dependencies. Validates before writes and after edits
- **Sed-like File Editing** - Regex, insert, append, delete, replace operations on any file
- **Single Static Binary** - Zero runtime dependencies, distroless container deployment

## Quick Start

### Using Pre-built Docker Image (Recommended)

Multi-architecture support: **AMD64** and **ARM64**

```bash
# Pull the latest image from Docker Hub
docker pull firstfinger/ssh-mcp:latest

# Run HTTP server (production - keys in /data volume)
docker run -v ssh-keys:/data -p 8000:8000 firstfinger/ssh-mcp:latest

# Run with custom port
docker run -v ssh-keys:/data -p 9090:8000 -e PORT=8000 firstfinger/ssh-mcp:latest

# Run in stdio mode (for local MCP hosts)
docker run -v ssh-keys:/data firstfinger/ssh-mcp:latest -mode stdio
```

### Local Development

```bash
# Build from source
go build -o ssh-mcp ./cmd/server

# Run HTTP server (development - keys in ./data/)
./ssh-mcp                    # Default: :8000
PORT=9090 ./ssh-mcp          # Custom port

# Run stdio mode
./ssh-mcp -mode stdio

# Build your own Docker image
docker build -t ssh-mcp .
docker run -v ssh-keys:/data -p 8000:8000 ssh-mcp
```

**SSH Key Storage**:
- **Development**: `./data/id_ed25519` (auto-created on first connection)
- **Production**: `/data/id_ed25519` (requires mounted volume, fails if not writable)

## Configuration

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `-mode` | `SSH_MCP_MODE` | `http` | Transport: `stdio` or `http` |
| `-port` | `PORT` | `8000` | HTTP port |
| `-debug` | `SSH_MCP_DEBUG` | `false` | Verbose logging |
| `-global` | `SSH_MCP_GLOBAL` | `false` | Shared SSH manager (single-user mode) |

## Tools Reference

### Core (5 tools)

| Tool | Description |
|------|-------------|
| `connect` | Establish SSH connection with optional jump host support |
| `disconnect` | Close one or all SSH connections |
| `run` | Execute shell command with configurable timeout |
| `identity` | Get server's public SSH key for authorized_keys |
| `info` | Get remote system information |

### File Operations (6 tools)

| Tool | Description |
|------|-------------|
| `read` | Read remote file contents via SFTP |
| `write` | Write content to remote file — validates syntax BEFORE writing (server-side) |
| `edit` | Sed-like file editor: replace, regex, insert, append, prepend, delete, replace_line |
| `validate` | Validate file syntax server-side (JSON, YAML, TOML, XML, INI, Dockerfile, ENV) |
| `list_dir` | List directory contents with metadata |
| `sync` | Stream file between two remote nodes |

### Docker (8 tools)

| Tool | Description |
|------|-------------|
| `docker_ps` | List containers |
| `docker_logs` | Get container logs |
| `docker_op` | Start/stop/restart containers |
| `docker_ip` | Get container IP addresses |
| `docker_find_by_ip` | Find container by IP address |
| `docker_networks` | List Docker networks |
| `docker_cp_from` | Copy file from container |
| `docker_cp_to` | Copy file to container |

### Monitoring (7 tools)

| Tool | Description |
|------|-------------|
| `usage` | CPU, memory, disk usage |
| `ps` | Process listing |
| `logs` | Read log files with tail/head |
| `journal_read` | Read systemd journal |
| `dmesg_read` | Read kernel ring buffer |
| `diagnose_system` | Comprehensive system diagnostics |
| `list_services` | List systemd services |

### Database (3 tools)

| Tool | Description |
|------|-------------|
| `db_query` | Execute SQL on PostgreSQL/MySQL in Docker |
| `db_schema` | Get database schema |
| `list_db_containers` | Find database containers |

### Network (4 tools)

| Tool | Description |
|------|-------------|
| `net_stat` | Network statistics (ss/netstat) |
| `search_files` | Find files by name pattern |
| `search_text` | Search file contents (grep) |
| `package_manage` | Install/remove packages |

### VoIP SIP/RTP (10 tools)

| Tool | Description |
|------|-------------|
| `voip_discover_containers` | Find VoIP containers |
| `voip_sip_capture` | Capture SIP packets |
| `voip_call_flow` | Parse SIP call flow from PCAP |
| `voip_registrations` | Extract SIP registrations |
| `voip_call_stats` | Call statistics summary |
| `voip_extract_sdp` | Extract SDP from SIP |
| `voip_packet_check` | Check for RTP/SIP packets |
| `voip_network_capture` | Raw network packet capture |
| `voip_rtp_capture` | Capture RTP streams |
| `voip_network_diagnostics` | Ping, traceroute, port checks |

## Usage Examples

### Basic Connection

```json
// Connect to a server
{"tool": "connect", "arguments": {"host": "10.0.0.1", "username": "admin"}}

// Run a command
{"tool": "run", "arguments": {"command": "hostname"}}

// Read a file
{"tool": "read", "arguments": {"path": "/etc/hostname"}}
```

### Jump Host (Bastion)

```json
// Connect to bastion first
{"tool": "connect", "arguments": {"host": "bastion.example.com", "username": "admin", "alias": "bastion"}}

// Connect to internal server through bastion
{"tool": "connect", "arguments": {"host": "internal-server", "username": "admin", "via": "bastion"}}
```

### File Sync Between Hosts

```json
// Connect to both servers
{"tool": "connect", "arguments": {"host": "server-a", "username": "admin", "alias": "A"}}
{"tool": "connect", "arguments": {"host": "server-b", "username": "admin", "alias": "B"}}

// Copy file from A to B (streams through MCP server)
{"tool": "sync", "arguments": {"source_node": "A", "source_path": "/data/file.txt", "dest_node": "B", "dest_path": "/data/file.txt"}}
```

### Long-Running Commands

```json
// Run with custom timeout (default: 120s)
{"tool": "run", "arguments": {"command": "apt update && apt upgrade -y", "timeout": 600}}
```

### Sed-like File Editing

```json
// Simple find-and-replace
{"tool": "edit", "arguments": {"path": "/etc/nginx/nginx.conf", "old_text": "worker_connections 512", "new_text": "worker_connections 1024"}}

// Regex replace (change any timeout value)
{"tool": "edit", "arguments": {"path": "/etc/app/config.yaml", "operation": "regex", "pattern": "timeout:\\s*\\d+", "replacement": "timeout: 30"}}

// Insert a line at position 5
{"tool": "edit", "arguments": {"path": "/etc/hosts", "operation": "insert", "line": 5, "content": "10.0.0.5 myhost"}}

// Append after a matching section header
{"tool": "edit", "arguments": {"path": "/etc/app.ini", "operation": "append", "pattern": "\\[database\\]", "content": "pool_size = 20"}}

// Delete lines matching a pattern
{"tool": "edit", "arguments": {"path": "/etc/config.conf", "operation": "delete", "pattern": "^#.*deprecated"}}

// Delete a line range
{"tool": "edit", "arguments": {"path": "/tmp/data.txt", "operation": "delete", "start_line": 10, "end_line": 15}}
```

### Syntax Validation (Server-Side)

All validation runs on the MCP server using Go-native parsers — **zero dependencies on remote host tools** (no python3, jq, xmllint needed).

```json
// Validate a YAML file (reads via SFTP, validates in Go)
{"tool": "validate", "arguments": {"path": "/etc/app/config.yaml"}}

// Validate JSON with forced type
{"tool": "validate", "arguments": {"path": "/data/config", "type": "json"}}

// Write with pre-write validation (blocks write on syntax error)
{"tool": "write", "arguments": {"path": "/etc/app/config.json", "content": "{\"port\": 8080}"}}

// Edit automatically validates after changes
{"tool": "edit", "arguments": {"path": "/etc/app/config.yaml", "operation": "replace", "find": "port: 80", "replace": "port: 443"}}

// Write without validation
{"tool": "write", "arguments": {"path": "/etc/app/config.json", "content": "{\"port\": 8080}", "skip_validate": true}}
```

## Session Management

### Isolation Modes

- **Session-based** (default): Auto-generated UUIDv7 per client connection
- **Header-based**: `X-Session-Key` header for sticky routing and load balancer affinity
- **Global mode**: Single shared manager (`-global` flag) for single-user environments

### Architecture

- Thread-safe pool isolation with mutex protection (verified with `-race`)
- Independent SSH connection managers per session/key
- Idle timeout: 5 minutes (header-based sessions)
- Graceful shutdown with connection cleanup

## System Architecture

```
MCP Client → [Session Pool] → SSH Connection Manager → Remote Hosts
             │                                       ↓
             ├─ Session A (UUIDv7)              [Host A, Host B]
             ├─ Session B (X-Session-Key: X)    [Host C via Bastion]
             └─ Session C (X-Session-Key: Y)    [Host D]
```

Each session maintains isolated SSH connections with independent lifecycle management.

## Architecture

### Concurrency & Safety

- Native goroutine-based concurrency with fine-grained locking
- Race detector validation (`go test -race`) under high load
- Lock-free atomic operations for session metadata
- Per-alias mutex synchronization

### Performance

- Single 11MB static binary with zero runtime dependencies
- Zero-copy SFTP streaming
- Native PCAP parsing (pure Go, no CGO)
- Adaptive session cleanup (5-minute idle timeout)

### Security

- No artificial path restrictions — uses the SSH user’s native OS permissions
- Auto-sudo for non-root users (`SudoPrefix` helper)
- Session pool isolation (thread-safe, no cross-contamination)
- Ed25519 key generation with 0600 permissions
- Distroless container runtime

## Project Structure

```
.
├── cmd/server/           # Application entry point
│   └── main.go
├── internal/
│   ├── ssh/              # Core SSH logic
│   │   ├── manager.go    # Thread-safe connection management
│   │   ├── pool.go       # Session isolation & cleanup
│   │   └── client.go     # SFTP & Exec client
│   ├── sip/              # VoIP packet parsing
│   │   └── parser.go     # PCAP/SIP/RTP analysis
│   └── tools/            # Tool implementations
│       ├── core.go       # Connection tools
│       ├── files.go      # File operations
│       └── voip.go       # VoIP diagnostics
├── Dockerfile            # Distroless production build
├── go.mod                # Go module definition
└── README.md             # Documentation
```

## Development

```bash
# Build from source
go build -o ssh-mcp ./cmd/server

# Build Docker image locally
docker build --build-arg COMMIT_SHA=$(git rev-parse --short HEAD) -t ssh-mcp .

# Or use the official multi-arch image (AMD64/ARM64)
docker pull firstfinger/ssh-mcp:latest
```

**Requirements**: Go 1.25+

## Deployment

### Production Recommendations

- Deploy behind reverse proxy (TLS termination)
- Private network or VPN-restricted access

### Endpoints

- `/mcp` - MCP protocol (POST/GET, SSE support)
- Session persistence: `X-Session-Key` header

### Load Balancing

Use consistent hashing on `X-Session-Key` for sticky routing.

## Contributing

Contributions welcome.

## License

MIT
