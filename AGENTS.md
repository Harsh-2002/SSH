# AGENTS.md - SSH-MCP Repository Guide

This document provides build commands and code style guidelines for agentic coding agents working on this repository.

## Build, Lint, and Test Commands

### Installation
```bash
pip install .
```

### Running the Server
```bash
# Stdio mode (for local MCP hosts)
python -m ssh

# HTTP server (Streamable HTTP transport)
uvicorn ssh.server_all:app --host 0.0.0.0 --port 8000

# Using installed commands
ssh-mcp                    # stdio mode
ssh-mcp-server             # HTTP server
```

### Docker
```bash
docker compose up -d
# or
docker run -d --name ssh-mcp -p 8000:8000 -v ssh-mcp-data:/data firstfinger/ssh-mcp:latest
```

### Testing
This project does not currently have test infrastructure. When adding tests, create them in a `tests/` directory and use pytest.

### Building Distribution
```bash
# Build wheel (uses hatchling)
pip install hatchling
python -m build
```

## Code Style Guidelines

### Python Version and Type Hints
- Requires Python 3.10+
- Use modern type hints with `|` union operator: `str | None` instead of `Optional[str]`
- Include `from __future__ import annotations` at the top of files using forward references
- All functions and methods should have return type annotations

### Imports
- Standard library imports first, then third-party, then local imports
- Use relative imports within the `src/ssh` package: `from ..ssh_manager import SSHManager`
- Group imports with blank lines between standard library, third-party, and local imports

### Naming Conventions
- **Functions and methods**: `snake_case` (e.g., `get_system_info`, `run_command`, `_validate_path`)
- **Classes**: `PascalCase` (e.g., `SSHManager`, `SessionStore`)
- **Constants**: `UPPER_SNAKE_CASE` (e.g., `_SESSION_TIMEOUT`, `_GLOBAL_STATE`)
- **Private members**: Prefix with underscore (e.g., `_lock`, `_alias_locks`, `_ensure_connection`)
- **Module-level private variables**: Prefix with underscore (e.g., `_GLOBAL_MANAGER`, `_SESSION_STORE`)

### Async/Await Patterns
- All SSH operations are asynchronous
- Use `async with` context managers for SFTP operations and locks
- Use `asynccontextmanager` for custom async contexts
- Implement auto-reconnect logic with `retry: bool = True` parameter pattern

### Error Handling
- Use specific exception types: `ValueError`, `PermissionError`, `ConnectionError`, `RuntimeError`
- Log errors with context: `logger.error(f"Connection failed: {e}")`
- Raise descriptive error messages that help users understand and fix issues
- Wrap external library exceptions in appropriate custom exceptions
- Handle connection loss gracefully with retry logic
- Validate inputs early and fail fast with clear error messages

### Docstrings
- Use triple-quoted docstrings for all public functions and classes
- Document parameters, return values, and notable behavior
- Keep docstrings concise but informative

### Logging
- Use the `logging` module with logger name "ssh-mcp"
- Configure log format with timestamps and log levels
- Use appropriate log levels: `INFO` for normal operations, `WARNING` for recoverable issues, `ERROR` for failures
- Include context in log messages: which target/alias, what operation, what failed

### Tool Implementation Pattern
Tools are organized in `src/ssh/tools/` as async functions that take a `SSHManager` instance and `target` parameter:

```python
async def tool_function(manager: SSHManager, target: str | None = None) -> str:
    """Brief description."""
    return await manager.run("command", target=target)
```

Tools registered in `mcp_server.py` follow this pattern:
```python
@mcp.tool()
async def tool_name(ctx: Context, param: str, target: str = "primary") -> str:
    """Tool description."""
    manager = await get_session_manager(ctx)
    if not manager: return "Error: Not connected."
    try:
        return await tools.tool_function(manager, param, target)
    except Exception as e:
        logger.error(f"Tool failed: {e}")
        return f"Error: {str(e)}"
```

### File and Directory Structure
- `src/ssh/__main__.py`: Entry point for stdio mode
- `src/ssh/server_all.py`: HTTP server entry point
- `src/ssh/ssh_manager.py`: Core SSH connection management
- `src/ssh/mcp_server.py`: MCP tool registration and routing
- `src/ssh/tools/*.py`: Tool implementations organized by category
- `src/ssh/session_store.py`: Session caching logic

### Environment Variables
Configure behavior with environment variables:
- `PORT=8000`: HTTP server port
- `SSH_MCP_SESSION_HEADER=X-Session-Key`: Header for session caching
- `SSH_MCP_SESSION_TIMEOUT=300`: Session idle timeout
- `SSH_MCP_GLOBAL_STATE=false`: Use shared global manager
- `SSH_MCP_COMMAND_TIMEOUT=120.0`: SSH command timeout in seconds
- `SSH_MCP_MAX_OUTPUT=51200`: Maximum output bytes returned
- `SSH_MCP_DEBUG_ASYNCSSH=false`: Enable verbose asyncssh logging

### Security Considerations
- Always validate file paths against `allowed_root` before operations
- Path validation prevents directory traversal attacks
- Sensitive credentials (passwords, keys) should not be logged
- SSH keys stored in `/data/id_ed25519` with 0o600 permissions
- System keys are generated automatically and persisted

### Code Organization
- Keep tool implementations focused and single-purpose
- Use helper functions for parsing and transformation
- Separate business logic from MCP tool registration
- Maintain clean separation between SSH operations and tool interface
