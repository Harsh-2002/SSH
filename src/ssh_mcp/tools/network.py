from ..ssh_manager import SSHManager

async def check_tool_availability(manager: SSHManager, tool: str, target: str = "primary") -> bool:
    """Checks if a command-line tool is available on the remote system."""
    check_cmd = f"command -v {tool} >/dev/null 2>&1 && echo 'present' || echo 'missing'"
    output = await manager.execute(check_cmd, target=target)
    return "present" in output

async def net_stat(manager: SSHManager, port: int = None, target: str = "primary") -> str:
    """
    Check for listening ports.
    Adaptively uses ss (modern), netstat (legacy), or lsof (fallback).
    """
    # 1. Try ss (Socket Statistics) - Most modern, widely available
    if await check_tool_availability(manager, "ss", target):
        # -l: listening, -t: tcp, -u: udp, -n: numeric ports, -p: show process
        cmd = "ss -ltunp"
        if port:
            cmd += f" | grep ':{port}'"
        return await manager.execute(cmd, target=target)
    
    # 2. Try netstat - Legacy
    if await check_tool_availability(manager, "netstat", target):
        cmd = "netstat -ltunp"
        if port:
            cmd += f" | grep ':{port}'"
        return await manager.execute(cmd, target=target)
        
    return "Error: No suitable network monitoring tool (ss, netstat) found."

async def net_dump(manager: SSHManager, interface: str = "any", count: int = 20, filter: str = "", target: str = "primary") -> str:
    """
    Safely captures network traffic using tcpdump with strict limits.
    """
    if not await check_tool_availability(manager, "tcpdump", target):
        return "Error: tcpdump is not installed."

    # Safety parameters:
    # timeout 10s: Hard system timeout prevents hanging
    # -c {count}: Exit after receiving count packets
    # -n: Don't convert addresses to names (avoids DNS lookups)
    
    cmd = f"timeout 10s sudo tcpdump -i {interface} -c {count} -n {filter}"
    
    output = await manager.execute(cmd, target=target)
    
    # Handle sudo issues gracefully
    if "sudo: a terminal is required" in output or "sudo: no tty" in output:
         return "Error: sudo requires a password or TTY. Ensure the user has passwordless sudo for tcpdump."
         
    return output

async def curl(manager: SSHManager, url: str, method: str = "GET", target: str = "primary") -> str:
    """Check connectivity to a URL."""
    if not await check_tool_availability(manager, "curl", target):
        return "Error: curl is not installed."
        
    # -I: Head only (if GET/HEAD) to be faster, unless we want body.
    # But usually 'curl' implies full check. We'll use -v for debug info.
    # -m 5: 5 second timeout
    cmd = f"curl -X {method} -m 5 -v {url}"
    return await manager.execute(cmd, target=target)
