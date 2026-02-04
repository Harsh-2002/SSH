# SSH MCP Server

A high-performance SSH connection management server implementing the [Model Context Protocol (MCP)](https://modelcontextprotocol.io/). Enables AI agents to execute commands, manage files, and perform DevOps operations across remote infrastructure via persistent SSH sessions.

[![Go Version](https://img.shields.io/badge/go-1.25+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

## Features

- **42 Production-Ready Tools** - Core SSH, file operations, Docker, database, monitoring, and VoIP
- **Persistent Sessions** - SSH connections survive across multiple MCP requests
- **Jump Host Support** - Connect through bastion hosts with the `via` parameter
- **Session Isolation** - Per-client connection pools with automatic cleanup
- **SFTP Native** - Direct file operations without external binaries
- **Auto-Reconnect** - Transparent reconnection on connection loss
- **Zero Dependencies** - Single static binary, no runtime requirements

## Quick Start

### Build from Source

```bash
git clone https://github.com/harsh-2002/SSH-MCP.git
cd SSH-MCP
go build -o ssh-mcp ./cmd/server
```

### Run

```bash
# Stdio mode (for local MCP hosts like Claude Desktop)
./ssh-mcp

# HTTP mode (for remote access)
./ssh-mcp -mode http -port 8000
```

### Docker

```bash
docker build -t ssh-mcp .
docker run -v /path/to/keys:/data ssh-mcp
```

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-mode` | `stdio` | Transport mode: `stdio` or `http` |
| `-port` | `8000` | HTTP server port (http mode only) |
| `-debug` | `false` | Enable debug logging |
| `-global` | `false` | Use single shared SSH manager for all sessions |

## Tools Reference

### Core (5 tools)

| Tool | Description |
|------|-------------|
| `connect` | Establish SSH connection with optional jump host support |
| `disconnect` | Close one or all SSH connections |
| `run` | Execute shell command with configurable timeout |
| `identity` | Get server's public SSH key for authorized_keys |
| `info` | Get remote system information |

### File Operations (5 tools)

| Tool | Description |
|------|-------------|
| `read` | Read remote file contents via SFTP |
| `write` | Write content to remote file |
| `edit` | Find and replace text in file |
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

## Session Management

### Session Isolation

Each MCP client gets its own isolated connection pool:

- **Session-based**: Automatic isolation by MCP session ID
- **Header-based**: Use `X-Session-Key` header for custom grouping
- **Global mode**: Single shared pool with `-global` flag

### Session Lifecycle

| Event | Behavior |
|-------|----------|
| Client connects | New session pool created |
| Idle timeout (5 min) | Header-based sessions cleaned up |
| Client disconnects | Session pool destroyed |
| Server shutdown | All connections closed gracefully |

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                     MCP Server                          │
│  ┌─────────────────────────────────────────────────┐   │
│  │                   Pool                           │   │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐         │   │
│  │  │ Session │  │ Session │  │ Session │  ...    │   │
│  │  │ Manager │  │ Manager │  │ Manager │         │   │
│  │  └────┬────┘  └────┬────┘  └────┬────┘         │   │
│  └───────┼───────────┼───────────┼─────────────────┘   │
│          │           │           │                      │
│  ┌───────▼───────────▼───────────▼─────────────────┐   │
│  │                SSH Clients                       │   │
│  │  ┌────────┐  ┌────────┐  ┌────────┐             │   │
│  │  │ host-a │  │ host-b │  │ via:   │   ...       │   │
│  │  │        │  │        │  │ jump   │             │   │
│  │  └────────┘  └────────┘  └────────┘             │   │
│  └─────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

## Project Structure

```
.
├── cmd/server/           # Application entry point
│   └── main.go
├── internal/
│   ├── ssh/              # SSH connection management
│   │   ├── client.go     # Single SSH connection with SFTP
│   │   ├── manager.go    # Multi-connection pool per session
│   │   ├── pool.go       # Session isolation and cleanup
│   │   └── keys.go       # Ed25519 key generation
│   ├── sip/              # VoIP packet parsing
│   │   └── parser.go     # PCAP/SIP/RTP with gopacket
│   └── tools/            # MCP tool implementations
│       ├── core.go       # connect, disconnect, run
│       ├── files.go      # read, write, edit, sync
│       ├── docker.go     # Container operations
│       ├── monitoring.go # System diagnostics
│       ├── db.go         # Database queries
│       ├── network.go    # Network utilities
│       └── voip.go       # SIP/RTP tools
├── Dockerfile            # Multi-stage distroless build
├── go.mod
└── README.md
```

## Development

### Requirements

- Go 1.25+
- libpcap-dev (for VoIP tests with gopacket)

### Build

```bash
go build -o ssh-mcp ./cmd/server
```

### Test

```bash
go test ./... -v
```

### Docker Build

```bash
docker build -t ssh-mcp .
```

## Load Balancing

When running multiple instances, use hash-based load balancing:

```nginx
upstream mcp_servers {
    hash $http_x_session_key consistent;
    server mcp1:8000;
    server mcp2:8000;
    server mcp3:8000;
}
```

## Security Considerations

- **Key Management**: Mount `/data` volume for persistent SSH keys
- **Host Key Verification**: Currently set to `InsecureIgnoreHostKey()` (customize for production)
- **Path Validation**: All file operations validate against allowed root
- **Session Timeout**: Idle sessions automatically cleaned up after 5 minutes

## Contributing

Contributions welcome! Please read the contributing guidelines and submit PRs.

## License

MIT License - see [LICENSE](LICENSE) for details.
