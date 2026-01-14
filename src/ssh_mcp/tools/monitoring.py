from ..ssh_manager import SSHManager
import json

async def usage(manager: SSHManager, target: str | None = None) -> str:
    """
    Get system resource usage (CPU, RAM, Disk).
    Uses standard Linux commands available on almost all distros.
    """
    # 1. CPU Load (via uptime)
    uptime_output = await manager.execute("uptime", target=target)
    # Output ex: " 14:00:00 up 10 days,  4:00,  2 users,  load average: 0.05, 0.10, 0.15"
    
    # 2. Memory (via free -m or /proc/meminfo)
    # We prefer /proc/meminfo for parsing stability
    mem_output = await manager.execute("cat /proc/meminfo | grep -E 'MemTotal|MemAvailable|MemFree'", target=target)
    
    # 3. Disk (via df -h /)
    disk_output = await manager.execute("df -h / | tail -n 1", target=target)
    
    return f"""
System Status:
---
[Load Average]
{uptime_output.strip()}

[Memory Stats]
{mem_output.strip()}

[Disk Usage (/)]
{disk_output.strip()}
"""

async def logs(manager: SSHManager, path: str, lines: int = 50, grep: str | None = None, target: str | None = None) -> str:
    """
    Safely read the end of a log file.
    Args:
        lines: Number of lines to read (max 500).
        grep: Optional string to filter lines.
    """
    # Safety limit
    if lines > 500: lines = 500
    
    cmd = f"tail -n {lines} {path}"
    if grep:
        # Simple grep filtering
        cmd += f" | grep '{grep}'"
        
    return await manager.execute(cmd, target=target)

async def ps(manager: SSHManager, sort_by: str = "cpu", limit: int = 10, target: str | None = None) -> str:
    """
    List top processes.
    Args:
        sort_by: 'cpu' or 'mem'
        limit: Number of processes to show.
    """
    sort_flag = "-%cpu" if sort_by == "cpu" else "-%mem"
    
    # ps options:
    # -e: all processes
    # -o: output format
    # --sort: sorting order
    cmd = f"ps -eo pid,user,%cpu,%mem,comm --sort={sort_flag} | head -n {limit + 1}"
    
    return await manager.execute(cmd, target=target)
