# MCP Tools Reference

This document lists all MCP tools available in the SSH-MCP server, based on actual code analysis of `src/ssh/mcp_server.py` and tool modules.

## Core/Connection Tools

| Tool | Function | Parameters | Description |
|------|----------|------------|-------------|
| `connect` | Connect to SSH server | `host`, `username`, `port=22`, `private_key_path`, `password`, `alias`, `via` | Creates persistent SSH connection. Auto-generates alias as 'user@host' if not provided |
| `disconnect` | Disconnect session | `alias` | Disconnect one or all SSH connections |
| `identity` | Get public key | - | Returns server's public SSH key in markdown for authorized_keys |
| `sync` | Stream file between nodes | `source_node`, `source_path`, `dest_node`, `dest_path` | Efficiently streams file from one remote node to another |

## File Operations

| Tool | Function | Parameters | Description |
|------|----------|------------|-------------|
| `read` | Read remote file | `path`, `target="primary"` | Read file contents from remote host |
| `write` | Write remote file | `path`, `content`, `target="primary"` | Write/overwrite file on remote host |
| `edit` | Edit remote file | `path`, `old_text`, `new_text`, `target="primary"` | Smart text replacement. Errors if match is not unique |
| `list_dir` | List directory | `path`, `target="primary"` | List directory contents (JSON format) |

## System Tools

| Tool | Function | Parameters | Description |
|------|----------|------------|-------------|
| `run` | Execute command | `command`, `target="primary"`, `timeout` | Execute any shell command with optional timeout |
| `info` | Get system info | `target="primary"` | Returns OS, kernel, and shell information |

## Monitoring Tools

| Tool | Function | Parameters | Description |
|------|----------|------------|-------------|
| `usage` | Resource usage | `target="primary"` | Get CPU, RAM, disk usage (returns dict) |
| `logs` | Read log file | `path`, `lines=50`, `grep`, `target="primary"` | Safely read recent logs from file |
| `ps` | List processes | `sort_by="cpu"`, `limit=10`, `target="primary"` | List top processes by resource usage |

## Docker Tools

| Tool | Function | Parameters | Description |
|------|----------|------------|-------------|
| `docker_ps` | List containers | `all=False`, `target="primary"` | List Docker containers (structured JSON) |

**Note:** For other Docker operations, use `run` tool:
- Logs: `run("docker logs <container>")`
- Start/Stop: `run("docker start\|stop\|restart <container>")`
- Inspect: `run("docker inspect <container>")`
- Exec: `run("docker exec <container> <command>")`
- Networks: `run("docker network ls")`
- Copy: `run("docker cp <src> <dst>")`

## Network Tools

| Tool | Function | Parameters | Description |
|------|----------|------------|-------------|
| `net_stat` | Check ports | `port`, `target="primary"` | Check for listening ports (uses ss/netstat, returns JSON) |

**Note:** For other network operations, use `run` tool:
- Connectivity: `run("nc -zv host port")` or `run("ping host")`
- DNS: `run("dig domain")` or `run("nslookup domain")`
- Traffic: `run("tcpdump -i any -c 20")`
- Curl: `run("curl -s url")`

## Service Tools

| Tool | Function | Parameters | Description |
|------|----------|------------|-------------|
| `list_services` | List services | `failed_only=False`, `target="primary"` | List system services (Systemd/OpenRC, returns JSON) |

**Note:** For service operations, use `run` tool:
- Status: `run("systemctl status <service>")`
- Start/Stop: `run("systemctl start\|stop\|restart <service>")`
- Logs: `run("journalctl -u <service> -n 100")`
- Enable/Disable: `run("systemctl enable\|disable <service>")`

## Database Tools

| Tool | Function | Parameters | Description |
|------|----------|------------|-------------|
| `db_query` | Execute query | `container_name`, `db_type`, `query`, `database`, `username`, `password`, `target="primary"`, `timeout=60` | Execute SQL/CQL/MongoDB query inside Docker container |

**Supported DB Types:** `postgres`, `mysql`, `scylladb`, `cassandra`, `mongodb`

## Search Tools

| Tool | Function | Parameters | Description |
|------|----------|------------|-------------|
| `search_files` | Search files | `pattern`, `path="/"`, `max_depth=5`, `file_type`, `target="primary"` | Find files by name pattern using find (glob pattern) |
| `search_text` | Search text | `pattern`, `path`, `recursive=True`, `ignore_case=False`, `max_results=50`, `target="primary"` | Search text patterns in files using grep (regex) |

## Package Management

| Tool | Function | Parameters | Description |
|------|----------|------------|-------------|
| `package_manage` | Manage packages | `action`, `pkg`, `target="primary"` | Install, remove, or check packages |

**Actions:** `install`, `remove`, `check`
**Auto-detects:** apt (Debian/Ubuntu), apk (Alpine), dnf/yum (RHEL/Fedora)

## Log/Journal Tools

| Tool | Function | Parameters | Description |
|------|----------|------------|-------------|
| `journal_read` | Read system logs | `service`, `since`, `lines=100`, `priority`, `target="primary"` | Read journalctl or syslog (returns dict) |
| `dmesg_read` | Read kernel logs | `grep`, `lines=100`, `target="primary"` | Read kernel ring buffer (returns dict) |

**Priority options:** `emerg`, `alert`, `crit`, `err`, `warning`, `notice`, `info`, `debug`

## Diagnostic Tools

| Tool | Function | Parameters | Description |
|------|----------|------------|-------------|
| `diagnose_system` | Health check | `target="primary"` | Comprehensive SRE health check in one call |

**Checks:**
- Load average and high resource consumers
- OOM killer events in dmesg
- Disk pressure (partitions >90% full)
- Failed services

---

## Tool Categories Summary

- **Core Tools:** 4 (connect, disconnect, identity, sync)
- **File Tools:** 4 (read, write, edit, list_dir)
- **System Tools:** 2 (run, info)
- **Monitoring Tools:** 3 (usage, logs, ps)
- **Docker Tools:** 1 (docker_ps) + run tool extensions
- **Network Tools:** 1 (net_stat) + run tool extensions
- **Service Tools:** 1 (list_services) + run tool extensions
- **Database Tools:** 1 (db_query)
- **Search Tools:** 2 (search_files, search_text)
- **Package Management:** 1 (package_manage)
- **Log/Journal Tools:** 2 (journal_read, dmesg_read)
- **Diagnostic Tools:** 1 (diagnose_system)

**Total Registered MCP Tools: 22**

---

## Important Notes

1. **All tools accept a `target` parameter** (default: `"primary"`) to specify which SSH connection to use
2. **Error handling:** Tools return error messages prefixed with "Error:" on failure
3. **Structured data:** Some tools return `dict[str, Any]` for complex data, others return `str`
4. **Session management:** Tools use Smart Session Store based on `X-Session-Key` header or per-session isolation
5. **Extensibility:** Many Docker, Network, and Service operations are available via the generic `run` tool

---

## Internal Helper Functions (Not Exposed as MCP Tools)

These functions exist in the codebase but are not registered as MCP tools:

### Docker (docker.py)
- `docker_ip()` - Get container IP addresses
- `docker_find_by_ip()` - Find container by IP
- `docker_networks()` - List Docker networks
- `docker_cp_from_container()` - Copy from container to host
- `docker_cp_to_container()` - Copy from host to container

### Database (db.py)
- `db_schema()` - Get database/collection schema
- `db_describe_table()` - Describe table structure
- `list_db_containers()` - Find database containers

### Services (services_universal.py)
- `inspect_service()` - Inspect service/container status
- `fetch_logs()` - Fetch service logs
- `service_action()` - Perform service actions

These are used internally by registered MCP tools and can be added if needed.
