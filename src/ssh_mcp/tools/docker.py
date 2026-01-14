from ..ssh_manager import SSHManager

async def check_tool_availability(manager: SSHManager, tool: str, target: str | None = None) -> bool:
    """Checks if a command-line tool is available on the remote system."""
    check_cmd = f"command -v {tool} >/dev/null 2>&1 && echo 'present' || echo 'missing'"
    output = await manager.execute(check_cmd, target=target)
    return "present" in output

async def docker_ps(manager: SSHManager, all: bool = False, target: str | None = None) -> str:
    """
    List Docker containers.
    Returns structured JSON output if possible, or standard table otherwise.
    """
    if not await check_tool_availability(manager, "docker", target=target):
        return "Error: Docker command not found."

    flag = "-a" if all else ""
    
    # We use a custom format to make it easier for the LLM to parse
    # Format: ID | Image | Status | Names
    cmd = f"docker ps {flag} --format 'table {{.ID}}\t{{.Image}}\t{{.Status}}\t{{.Names}}'"
    
    return await manager.execute(cmd, target=target)

async def docker_logs(manager: SSHManager, container_id: str, lines: int = 50, target: str | None = None) -> str:
    """Get logs for a specific container."""
    if not await check_tool_availability(manager, "docker", target=target):
        return "Error: Docker command not found."
        
    cmd = f"docker logs --tail {lines} {container_id}"
    return await manager.execute(cmd, target=target)

async def docker_op(manager: SSHManager, container_id: str, action: str, target: str | None = None) -> str:
    """
    Perform a lifecycle action on a container.
    Args:
        action: start, stop, restart
    """
    if action not in ["start", "stop", "restart"]:
        return "Error: Invalid action. Use start, stop, or restart."
        
    if not await check_tool_availability(manager, "docker", target=target):
        return "Error: Docker command not found."

    cmd = f"docker {action} {container_id}"
    return await manager.execute(cmd, target=target)
