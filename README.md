# SSH MCP Server

**SSH for AI Agents: Let AI Run Your DevOps.**

A Model Context Protocol (MCP) server that enables your AI agents and language models to securely manage remote infrastructure over SSH.

## Overview

SSH MCP Server bridges the gap between AI agents and your infrastructure, providing a robust, simple interface for:
- Executing shell commands on remote machines
- Managing files and directories
- Monitoring system health and logs
- Running Docker operations
- Querying databases
- Managing package installations

## Features

- **Direct SSH Bridge** - No need for SSH libraries in your AI application
- **Managed Identity** - Auto-generated Ed25519 keys for secure authentication
- **Smart Sessions** - Supports both stateless HTTP and persistent shell sessions
- **Structured Output** - Clean JSON responses for easy LLM parsing
- **File Operations** - Read, write, edit, and sync files with atomic operations
- **DevOps Tools** - Docker, systemd logs, package management, and health diagnostics
- **Database Support** - Query SQL, MongoDB, and CQL databases
- **Safe Patching** - Built-in diff support for secure file modifications

## Quick Start

### Docker (Recommended)

```bash
docker run -d \
  --name ssh-mcp \
  -p 8000:8000 \
  -v ssh-mcp-data:/data \
  firstfinger/ssh-mcp:latest
```

### Local Python

```bash
pip install .
uvicorn ssh.server_all:app --host 0.0.0.0 --port 8000
```

## License

MIT License - See LICENSE file for details.
