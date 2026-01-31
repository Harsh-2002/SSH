# Future Inventory System - Implementation Plan

**Status:** Planning Phase (Not Implemented Yet)  
**Created:** January 31,2026  
**Last Updated:** January 31,2026  

---

## Executive Summary

This plan describes a design and implementation approach for an inventory management system that integrates with existing SSH-MCP tools. The inventory system will provide structured host information (IP, hostname, services, groups) and enable hostname-based references in existing tools.

**Key Design Principle:** Simple, CSV-based inventory that serves as source of truth for existing tools, with optional intelligent cross-referencing and composite workflow helpers.

---

## 1. Current Context & Motivation

### 1.1 Problem Statement

Current workflow limitations:
- SSH-MCP tools use `target: str = "primary"` parameter
- No centralized source of truth for host information
- No way to reference hosts by human-readable name
- No context about what services run on each host
- Example: "On sg41 this custom gw is running and he's facing audio black issue"
  - User must manually know sg41 is a SIP gateway container
  - LLM has to guess which tools to run
  - No easy way to run diagnostics on all VoIP-related hosts

### 1.2 Desired Capabilities

Primary use cases identified:
1. **Hostname-based references**
   - Use human-readable names instead of IPs: `voip_sip_capture(container="sg41", ...)`
   - Example: LLM query: "What hosts are in the 'voip' group?"
   - Quick context: "Show me all hosts running postgres"

2. **Service awareness**
   - Know what services run on each host without querying every time
   - Example: sg41 runs postgres, freeswitch, and custom gw
   - Quick context: "Which hosts have freeswitch?"

3. **Group-based operations**
   - Run tools on multiple related hosts at once
   - Example: Capture SIP on all 'voip-signaling' hosts
   - Bulk operations: health check on all 'production' hosts

4. **Quick host lookup**
   - Get full host details (IP, services, groups) by hostname
   - Example: "What's the IP of sg41?" → returns 203.153.54.41, services, groups

---

## 2. Inventory Format Specification

### 2.1 CSV Structure (Agreed Format)

**File Location:** `/root/SSH-MCP/inventory.csv`

**Columns:**
```
IP,Hostname,Services,Groups
```

**Example Entry:**
```
203.153.54.41,sg41,scylladb,postgres,gw,media,voip-signaling
```

**Data Types:**
- `IP` - IPv4 address (string format)
- `Hostname` - Human-readable name (alphanumeric, hyphens, underscores)
- `Services` - Comma-separated list (e.g., "postgres,freeswitch,gw")
- `Groups` - Comma-separated list (e.g., "voip-signaling,media,production")

**File Headers:** Required, always first line

---

## 3. File Management Strategy

### 3.1 Source of Truth
- `/root/SSH-MCP/inventory.csv` is the authoritative source
- All tools read from this file
- CSV is manually editable by users
- No automatic updates by default (prevents unexpected changes)

### 3.2 File Operations
- Manual editing: Text editor, Excel, etc.
- Optional tool auto-update: Can be offered after discovery operations
- File permissions: `0644` (readable/writable by owner)
- Path validation: Must be within `allowed_root` (/root/SSH-MCP)

---

## 4. Core Inventory Tools

### 4.1 Tool: `inventory_list(filter=None, group=None)`

**Purpose:** Return all hosts or filter by group from inventory CSV.

**Parameters:**
- `filter: str | None` - Filter by hostname (partial match, case-insensitive)
- `group: str | None` - Filter by group name (exact match, case-insensitive)

**Returns:**
```json
{
  "hosts": [
    {
      "ip": "203.153.54.41",
      "hostname": "sg41",
      "services": ["scylladb", "postgres", "gw", "media"],
      "groups": ["voip-signaling", "production"]
    }
    // ... more hosts
  ],
  "total": 5,
  "file": "/root/SSH-MCP/inventory.csv"
}
```

**Implementation Notes:**
- Parse CSV using Python `csv` module
- Case-insensitive hostname matching for user convenience
- Group filtering uses exact string match
- Empty filters return all hosts
- File not found error returns empty list with error message

---

### 4.2 Tool: `inventory_add(ip, hostname, services=None, groups=None)`

**Purpose:** Add new host to inventory CSV.

**Parameters:**
- `ip: str` - Required, must be valid IPv4 address
- `hostname: str` - Required, must be unique, alphanumeric + hyphens/underscores
- `services: str | None` - Optional comma-separated list
- `groups: str | None` - Optional comma-separated list

**Returns:**
```json
{
  "success": true,
  "hostname": "sg41",
  "ip": "203.153.54.41",
  "action": "added",
  "file": "/root/SSH-MCP/inventory.csv"
}
```

**Implementation Notes:**
- Validate hostname uniqueness before adding (case-insensitive check)
- Validate IPv4 format (basic: 4 dot-separated octets)
- Append line to CSV (no full file rewrite for performance)
- Handle special characters in CSV fields
- Duplicate detection: Warn if hostname exists, still allow add

---

### 4.3 Tool: `inventory_remove(hostname)`

**Purpose:** Remove host from inventory CSV by hostname.

**Parameters:**
- `hostname: str` - Required, case-insensitive match

**Returns:**
```json
{
  "success": true,
  "hostname": "sg41",
  "ip": "203.153.54.41",
  "action": "removed",
  "remaining_hosts": 4,
  "file": "/root/SSH-MCP/inventory.csv"
}
```

**Implementation Notes:**
- Remove first matching line (handles duplicates gracefully)
- Case-insensitive hostname lookup
- Return count of remaining hosts
- If hostname not found, return error

---

### 4.4 Tool: `inventory_status(hostname=None, ip=None)`

**Purpose:** Get detailed information for one or all hosts.

**Parameters:**
- `hostname: str | None` - Filter by hostname (partial match, case-insensitive)
- `ip: str | None` - Filter by IP (exact match)

**Returns (single host):**
```json
{
  "ip": "203.153.54.41",
  "hostname": "sg41",
  "services": ["scylladb", "postgres", "gw", "media"],
  "groups": ["voip-signaling", "production"],
  "file": "/root/SSH-MCP/inventory.csv"
}
```

**Returns (all hosts - no params):**
```json
{
  "hosts": [
    // ... all hosts
  ],
  "total": 5,
  "file": "/root/SSH-MCP/inventory.csv"
}
```

**Implementation Notes:**
- If both params provided, return best match (hostname takes precedence)
- Include connection status if available (call `connect()` internally)
- If no params, return all hosts with no connection status

---

### 4.5 Tool: `inventory_load(path="/root/SSH-MCP/inventory.csv")`

**Purpose:** Load inventory from custom path (for multiple environments).

**Parameters:**
- `path: str` - File path (must be within allowed_root)

**Returns:**
```json
{
  "success": true,
  "path": "/root/SSH-MCP/inventory.csv",
  "hosts": [ /* loaded hosts */ ],
  "total": 5
}
```

**Implementation Notes:**
- Validate file exists and is readable
- Validate CSV format
- Store loaded inventory in SSHManager memory (cache)
- Reload only on explicit call (no auto-refresh)
- File not found error with suggestions

---

## 5. Hostname Resolution Strategy

### 5.1 Design Decision: CSV as Resolution Table

**Approach:** Inventory CSV serves as hostname → IP mapping table.

**Why this approach:**
- Simple, no external DNS dependency
- Human-readable and manually editable
- Works with current architecture (SSHManager, no new state management)
- Fast lookup: O(1) for hostname resolution with in-memory caching

### 5.2 Implementation in SSHManager

**New Method:** `_resolve_target(target: str | None, inventory: dict | None) -> str`

**Logic:**
```python
def _resolve_target(self, target: str | None, inventory: dict | None) -> str:
    """Resolve target to IP address using inventory CSV.
    
    Returns:
    - If target is IP or alias: return as-is
    - If target is hostname: lookup in inventory, return IP
    - If target is None or "primary": return default alias or "primary"
    """
    # Check if target looks like an IP address (basic validation)
    if not target or target == "primary":
        return self.primary_alias or "primary"
    
    if self._is_ip_address(target):
        return target
    
    # Otherwise treat as hostname, look up in inventory
    ip = self._hostname_to_ip(target, inventory) if inventory else None
    
    if ip:
        logger.info(f"Resolved hostname '{target}' to IP '{ip}'")
        return ip
    
    # Not found in inventory
    logger.warning(f"Hostname '{target}' not found in inventory")
    return target
```

**Hostname to IP Lookup:**
```python
def _hostname_to_ip(self, hostname: str, inventory: dict | None) -> str | None:
    """Lookup hostname in inventory cache and return corresponding IP."""
    
    if not inventory:
        return None
    
    # Case-insensitive lookup in hostname column
    for ip, host_data in inventory.items():
        if host_data.get("hostname", "").lower() == hostname.lower():
            return ip
    
    return None
```

### 5.3 Inventory Caching

**Caching Strategy:**
- Load CSV once per SSHManager instance into dict: `{hostname: {"ip", "services", "groups"}}`
- Lookup is O(1) with in-memory cache
- No file I/O on repeated resolutions
- Cache persists for SSHManager lifetime (session or global)

**Cache Format:**
```python
{
    "sg41": {"ip": "203.153.54.41", "services": [...], "groups": [...]},
    "media-gw": {"ip": "203.153.54.42", "services": [...], "groups": [...]},
    "media-sw1": {"ip": "203.153.54.43", "services": [...], "groups": [...]},
    "prod-db": {"ip": "203.153.54.44", "services": [...], "groups": [...]}
}
```

### 5.4 Fallback Behavior

**Hostname not found in inventory:**
- Log warning: `hostname 'unknown-host' not found in inventory`
- Use hostname as-is (attempt SSH connection with hostname)
- No hard failure - graceful degradation
- Tool can optionally call DNS resolution for better experience

**CSV file missing:**
- Log error: `Inventory CSV not found, specify custom path`
- Fallback to IP/alias mode for all operations
- Tools continue to work with `target` parameter without resolution

---

## 6. Integration with Existing Tools

### 6.1 VoIP Tools Enhancement

**Current State:**
- `voip_sip_capture(container="...", ...)` - Uses container name
- `voip_call_flow(container="...", ...)` - Uses container name
- `voip_network_diagnostics(host="...", ...)` - Uses host (IP or alias)

**Enhanced Behavior:**
```text
User: "Capture SIP on sg41 for 2 minutes"
LLM Process:
1. inventory_status(hostname="sg41")
   → Returns: ip="203.153.54.41", services=[gw, media, freeswitch]
2. voip_sip_capture(container="sg41", duration=120)
   → Internally resolves "sg41" to IP: 203.153.54.41
   → Connects to 203.153.54.41
   → Runs: docker exec -i 203.153.54.41 sngrep ...
```

**Benefits:**
- One hostname lookup instead of remembering IP
- Service awareness before running tools
- Better context for LLM: "sg41 runs: gw, freeswitch, media"

### 6.2 Docker Tools Enhancement

**Future Enhancement:**
- Option to list containers across all inventory hosts
- New tool: `inventory_docker_list(group=None)`
- Example: `inventory_docker_list(group="voip-signaling")`
- For each host in group: run `docker_ps(all=False)`

**Implementation:**
```python
async def inventory_docker_list(self, group: str | None = None, target: str | None = None) -> dict[str, Any]:
    """List Docker containers for all hosts in a group."""
    
    if not self._inventory:
        return {"error": "inventory_not_loaded", "group": group}
    
    # Get hostnames in group
    hostnames = [
        data["hostname"] 
        for data in self._inventory.values() 
        if group and group.lower() in [g.lower() for g in data.get("groups", [])]
    ]
    
    if not hostnames:
        return {"group": group, "hosts": [], "count": 0}
    
    # Query containers for each host
    results = []
    for hostname in hostnames:
        # Resolve hostname to IP
        ip = self._hostname_to_ip(hostname, self._inventory)
        
        # Get connection for this host
        conn = self._get_connection_for_target(hostname)
        if not conn:
            continue
        
        # Run docker_ps on this host
        output = await docker.docker_ps(conn, all=False, target=None)
        
        # Add inventory metadata
        results.append({
            "hostname": hostname,
            "ip": ip,
            "containers": output.get("containers", [])
        })
    
    return {"group": group, "results": results, "count": len(results)}
```

### 6.3 Database Tools Enhancement

**Future Enhancement:**
- Query specific databases across all hosts with a service tag
- New tool: `inventory_db_query(query, service="postgres", group="voip-signaling")`
- For each host in group with postgres service: run db_query()

**Example:**
```text
User: "Show active connections from postgres on all voip hosts"
LLM Process:
1. inventory_list(group="voip-signaling")
2. For each host with postgres service:
   inventory_db_query(query="SELECT count(*) FROM pg_stat_activity", service="postgres", group="voip-signaling")
3. Return aggregated results
```

### 6.4 Network Tools Enhancement

**Future Enhancement:**
- Ping multiple hosts at once
- New tool: `inventory_ping(group="production", ping_count=3)`
- For each host in production group: run voip_network_diagnostics (ping only)

**Benefits:**
- Bulk health checks on production hosts
- Faster than running one by one
- Group-based operational visibility

---

## 7. Advanced Composite Tools

### 7.1 Tool: `inventory_quick_diagnostics(hostname)`

**Purpose:** One-shot comprehensive diagnostics for a host.

**Workflow:**
```text
1. inventory_status(hostname) → Get host info (IP, services, groups)
2. voip_packet_check(container="sg41", duration=5) → Check SIP packets
3. voip_network_diagnostics(host="sg41", ping_count=3, ports=[5060,5061]) → Network check
4. usage(target="sg41") → System resources
```

**Returns:**
```json
{
  "hostname": "sg41",
  "ip": "203.153.54.41",
  "inventory": {
    "services": ["scylladb", "postgres", "gw", "media", "freeswitch"],
    "groups": ["voip-signaling", "production"]
  },
  "sip_packets": true,
  "ping": {
    "reachable": true,
    "packet_loss": "0%",
    "summary": "3 packets transmitted, 3 received"
  },
  "network": {
    "tcp_5060": true,
    "tcp_5061": true,
    "traceroute": { /* hops */ }
  },
  "resources": {
    "cpu_percent": 15,
    "memory_percent": 62,
    "disk_percent": 45
  },
  "summary": "All systems nominal"
}
```

**Implementation Notes:**
- Run inventory operations first (get host info)
- Run VoIP tools with resolved IP or container name
- Return unified diagnostic picture
- Handle partial failures gracefully (e.g., ping fails but SIP works)

### 7.2 Tool: `inventory_voip_troubleshoot(hostname, phone_number=None, duration=30)`

**Purpose:** Complete VoIP troubleshooting workflow for a specific host.

**Workflow:**
```text
1. inventory_status(hostname) → Get host info
2. voip_sip_capture(container="sg41", duration=duration)
3. voip_call_flow(container="sg41", pcap_file="...", phone_number=phone_number)
4. voip_extract_sdp(container="sg41", pcap_file="...", call_id="...")
5. voip_rtp_capture(container="sg41", duration=10)
6. usage(target="sg41")
```

**Returns:**
```json
{
  "hostname": "sg41",
  "ip": "203.153.54.41",
  "capture_file": "/tmp/voip_sip_123456.pcap",
  "call_analysis": {
    "call_id": "abc@123",
    "from_user": "1001",
    "to_user": "1002",
    "final_status": "failed",
    "error_code": 404,
    "diagnosis": "User 1002 does not exist"
  },
  "sdp_analysis": {
    "codec": "PCMU/8000",
    "local_rtp": {"ip": "203.153.54.41", "port": 10000},
    "remote_rtp": {"ip": "203.153.54.42", "port": 20000}
  },
  "rtp_status": {
    "packets_detected": 0,
    "duration": 10,
    "diagnosis": "No RTP packets - possible firewall/NAT issue"
  },
  "resources": {
    "cpu_percent": 12,
    "memory_percent": 58,
    "disk_percent": 40
  },
  "recommendations": [
    "Check firewall rules for RTP ports 50000-60000",
    "Verify NAT traversal configuration",
    "Check if 203.153.54.42 (remote endpoint) is accessible"
  ]
}
```

**Implementation Notes:**
- Sequential execution of VoIP tools
- Use hostname parameter to resolve to IP internally
- Aggregate results into unified response
- Provide actionable recommendations based on analysis
- Handle errors gracefully (e.g., capture fails)

---

## 8. Tool Naming & Scope

### 8.1 Inventory-Specific Tools

Proposed naming convention:
```python
inventory_list(filter=None, group=None)       # List hosts/groups
inventory_add(ip, hostname, ...)          # Add host
inventory_remove(hostname)                 # Remove host
inventory_status(hostname=None, ip=None)  # Get host details
inventory_load(path=...)                    # Load inventory
inventory_docker_list(group=None)          # List containers by group
inventory_db_query(query, service, ...)   # Query DBs by group
inventory_ping(group, ...)                 # Ping hosts by group
inventory_quick_diagnostics(hostname)        # One-shot diagnostics
inventory_voip_troubleshoot(...)         # Complete VoIP workflow
```

### 8.2 Integration Pattern

**Modified Existing Tools:**
All tools currently accepting `target: str = "primary"` will be enhanced:

1. Accept `target: str | None` 
2. Call `_resolve_target(target, self._inventory)` first
3. Pass resolved IP or target as-is if no inventory match

**Example changes:**
```python
# In voip.py
async def voip_sip_capture(
    manager: SSHManager,
    container: str | None,  # Can be hostname now
    duration: int = 30,
    target: str | None = None,  # Can be hostname now
) -> dict[str, Any]:
    # Resolve hostname to IP internally
    resolved_target = manager._resolve_target(target, self._inventory)
    return await _execute_capture(manager, container, duration, resolved_target)
```

---

## 9. AI/LLM Integration Strategy

### 9.1 Approach A: Tool-Based (Recommended for V1)

**Description:** LLM explicitly calls `inventory_list()` when needed.

**Workflow:**
```text
User: "What hosts are in the 'voip' group?"
LLM: inventory_list(group="voip-signaling")
→ Returns list with hostname, IP, services
User: "Capture SIP on sg41"
LLM: voip_sip_capture(container="sg41", ...)
→ Internally resolves "sg41" to IP, connects to container
```

**Pros:**
- Explicit, predictable behavior
- No automatic context injection
- Simpler state management
- User controls when inventory is consulted

### 9.2 Approach B: Context Injection (Future Enhancement)

**Description:** Inventory is pre-loaded into MCP context on session start.

**Workflow:**
```text
Session Start:
1. Load inventory CSV into memory
2. Parse hosts into structured context
3. Inject into LLM context: "Available hosts: [sg41, media-gw, prod-db]"

User: "What services are on sg41?"
LLM has context: sg41 runs [gw, media, freeswitch]
→ No need to call inventory_status
```

**Pros:**
- Faster interactions (no inventory tool calls)
- Better context for LLM
- Host list always available in prompt

**Cons:**
- Complex session management
- Context invalidation challenges
- Larger initial response times

**Recommendation:** Start with Approach A, add Approach B as future enhancement.

---

## 10. Error Handling & Validation

### 10.1 CSV Validation

**Hostname Validation:**
```python
def _validate_hostname(hostname: str) -> tuple[bool, str | None]:
    """Validate hostname format."""
    if not hostname or len(hostname) == 0:
        return False, "Hostname cannot be empty"
    
    if len(hostname) > 63:
        return False, "Hostname must be 63 characters or less"
    
    allowed = set("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_")
    if not all(c.isalnum() or c == '-' or c == '_' for c in hostname):
        return False, "Hostname must contain only alphanumeric characters, hyphens, and underscores"
    
    return True, None
```

**IP Validation:**
```python
def _validate_ip(ip: str) -> tuple[bool, str | None]:
    """Validate IPv4 address format."""
    try:
        parts = ip.split('.')
        if len(parts) != 4:
            return False, "IP must have 4 octets"
        
        for part in parts:
            if not part or int(part) > 255 or int(part) < 0:
                return False, f"Invalid octet: {part}"
        
        # Check for leading zeros (discouraged but valid)
        if len(parts[0]) > 1 and parts[0].startswith('0'):
            return True, "IP has leading zeros (discouraged but valid)"
        
        return True, None
    except ValueError:
        return False, "Invalid IP address format"
```

### 10.2 Service List Validation

```python
def _validate_services(services: str) -> tuple[bool, str | None]:
    """Validate comma-separated service list."""
    if not services:
        return True, None
    
    parts = [s.strip() for s in services.split(',')]
    
    for part in parts:
        if not part:
            return False, f"Empty service name in list: {services}"
        
        if not part.replace('-', '').replace('_', '').isalnum():
            return False, f"Service names must be alphanumeric (hyphens/underscores allowed): {part}"
    
    return True, None
```

### 10.3 Group List Validation

```python
def _validate_groups(groups: str) -> tuple[bool, str | None]:
    """Validate comma-separated group list."""
    if not groups:
        return True, None
    
    parts = [g.strip() for g in groups.split(',')]
    
    for part in parts:
        if not part:
            return False, f"Empty group name in list: {groups}"
        
        if not part.replace('-', '').replace('_', '').isalnum():
            return False, f"Group names must be alphanumeric (hyphens/underscores allowed): {part}"
    
    return True, None
```

### 10.4 Error Messages

**File-level Errors:**
```json
{
  "error": "inventory_file_not_found",
  "message": "Inventory CSV does not exist or is not readable",
  "suggestion": "Create /root/SSH-MCP/inventory.csv or specify custom path"
}
```

**Validation Errors:**
```json
{
  "error": "invalid_hostname",
  "message": "Hostname contains invalid characters",
  "hostname": "sg@41",  // Shows invalid input
  "suggestion": "Hostnames must be alphanumeric with hyphens/underscores only"
}
```

**Duplicate Errors:**
```json
{
  "error": "hostname_already_exists",
  "message": "Hostname 'sg41' already exists in inventory",
  "existing_entry": {
    "ip": "203.153.54.41",
    "services": ["gw", "media"]
  }
}
```

---

## 11. Implementation Phases

### Phase 1: Core Infrastructure (Priority: High)

**Files to Create/Modify:**
- `src/ssh/inventory.py` - New inventory module
- `src/ssh/ssh_manager.py` - Add inventory caching and hostname resolution
- `src/ssh/__init__.py` - Add `inventory` to exports
- `src/ssh/mcp_server.py` - Add inventory tool wrappers
- `/root/SSH-MCP/inventory.csv` - Create example inventory file

**Tasks:**
1. Implement CSV parsing and validation
2. Implement core inventory tools (list, add, remove, status, load)
3. Implement hostname resolution in SSHManager
4. Add inventory caching
5. Create example inventory with multiple hosts
6. Register all inventory MCP tools

**Estimated Effort:** 2-3 hours

---

### Phase 2: Cross-Referencing (Priority: Medium)

**Tasks:**
1. Update `voip_sip_capture()` to support hostname target
2. Update `voip_packet_check()` to support hostname target
3. Update `voip_network_diagnostics()` to support hostname target
4. Update `voip_rtp_capture()` to support hostname target
5. Update all other VoIP tools with hostname support
6. Test hostname resolution with example inventory

**Estimated Effort:** 1-2 hours

---

### Phase 3: Composite Tools (Priority: Low)

**Tasks:**
1. Implement `inventory_quick_diagnostics(hostname)`
2. Implement `inventory_voip_troubleshoot(hostname, ...)`
3. Implement `inventory_group_operation(group, operation, target=None)`
4. Test composite workflows with example inventory
5. Comprehensive testing with various scenarios

**Estimated Effort:** 2-3 hours

---

### Phase 4: Documentation & Testing (Priority: Low)

**Tasks:**
1. Update `README.md` with inventory tool reference
2. Add inventory examples to documentation
3. Create test inventory files for different scenarios
4. Test CSV validation edge cases
5. Test hostname resolution with cache
6. Test concurrent inventory access
7. Document common workflows and patterns

**Estimated Effort:** 1-2 hours

---

## 12. Testing Strategy

### 12.1 Unit Tests (Manual)

**Test Cases:**
- CSV parsing: valid file, invalid format, empty file, duplicate entries
- Hostname validation: valid names, invalid characters, empty, too long
- IP validation: valid IPv4, invalid format, leading zeros
- Service list validation: valid, empty, invalid characters
- Group list validation: valid, empty, invalid characters
- Resolution logic: IP lookup, hostname lookup, cache hits/misses

**Expected Coverage:** 80-90% of core logic paths

### 12.2 Integration Tests (Manual)

**Test Scenario 1: Basic Operations**
```bash
# Add host
inventory_add(ip="10.0.0.5", hostname="test-db", services="postgres")

# List hosts
inventory_list()

# Get status
inventory_status(hostname="test-db")

# Remove host
inventory_remove(hostname="test-db")
```

**Test Scenario 2: Hostname Resolution**
```bash
# Use hostname in VoIP tools
voip_sip_capture(container="test-gw", duration=10)

# Use hostname in network diagnostics
voip_network_diagnostics(host="test-gw", ping_count=3)
```

**Test Scenario 3: Example Inventory**
```csv
IP,Hostname,Services,Groups
10.0.0.1,db-primary,postgres,database
10.0.0.2,db-replica,postgres,database
10.0.0.3,cache,redis,database,cache
10.0.0.4,web,nginx,webserver
```

**Expected Behavior:**
- `inventory_status(hostname="db-primary")` returns: 10.0.0.1, services=[postgres]
- `voip_sip_capture(container="test-gw", ...)` resolves "test-gw" to 10.0.0.1
- `inventory_list(group="database")` returns all 3 hosts

---

## 13. Decision Summary & Rationale

### 13.1 Design Decisions Made

**Decision 1: CSV Format Over JSON**
- **Rationale:** Human-readable, easy to edit, maps to spreadsheet
- **Tradeoff:** Requires parsing logic (vs native JSON), but more accessible
- **Verdict:** Simpler implementation outweighs parsing complexity

**Decision 2: Manual CSV Management Over Auto-Discovery**
- **Rationale:** Source of truth should be user-controlled
- **Tradeoff:** No automatic updates, but no unexpected changes
- **Verdict:** Prevents accidental data corruption, worth tradeoff

**Decision 3: Hostname Resolution via CSV Lookup**
- **Rationale:** Simple, no external DNS dependency
- **Tradeoff:** Doesn't handle dynamic DNS, requires manual updates
- **Verdict:** Good enough for static infrastructure, add DNS later if needed

**Decision 4: Single Inventory File Over Multiple Files**
- **Rationale:** Simpler implementation, easier user understanding
- **Tradeoff:** All environments share same file
- **Verdict:** Use environment profiles instead (dev.csv, prod.csv) if needed

**Decision 5: No Built-in UI Over LLM Interface**
- **Rationale:** LLM provides optimal interface
- **Tradeoff:** No visual dashboard
- **Verdict:** Avoids over-engineering, reduces maintenance burden

**Decision 6: Tool-Based LLM Integration Over Context Injection**
- **Rationale:** Explicit tool calls, cleaner separation of concerns
- **Tradeoff:** Slightly more verbose initial prompts
- **Verdict:** Start with this, add context injection later if needed

**Decision 7: Inventory as Source of Truth Over Dynamic Service Discovery**
- **Rationale:** CSV is authoritative, services are static metadata
- **Tradeoff:** Requires manual updates when services change
- **Verdict:** Better for user control, add discovery tools later if needed

### 13.2 Key Non-Goals (What We're NOT Building)

- **NOT** building a web dashboard
- **NOT** building a TUI (terminal UI)
- **NOT** building auto-discovery agents
- **NOT** building connection pooling (Phase 1)
- **NOT** building inventory versioning (Phase 1)
- **NOT** pre-loading inventory into LLM context (Phase 1)

### 13.3 What We ARE Building (V1 Scope)

Core inventory functionality that provides structured host information and hostname-based references across existing tools.

**Features to Implement:**
- [x] CSV parsing and validation
- [x] Core inventory tools (list, add, remove, status, load)
- [x] Hostname → IP resolution via CSV lookup
- [x] Inventory caching in SSHManager
- [x] Integration with VoIP tools (hostname support)
- [x] Error handling and validation
- [x] MCP tool wrappers for all inventory functions
- [x] Documentation updates
- [x] Example inventory file creation

**Estimated Implementation Time:** 6-10 hours for full V1

---

## 14. Conclusion

This plan provides a comprehensive design for an inventory management system that integrates seamlessly with existing SSH-MCP and VoIP tools. The design prioritizes simplicity, human readability, and seamless tool integration while leaving room for future enhancements.

**Next Steps:**
1. User review of this plan
2. Approval to proceed with implementation
3. Begin Phase 1 implementation upon approval
4. Phased approach: Core → Cross-reference → Composite → Documentation

**Status:** Ready for implementation when approved
