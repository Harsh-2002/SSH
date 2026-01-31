from __future__ import annotations

import base64
import io
import logging
import socket
import time
import uuid
from datetime import datetime
from typing import Any

import dpkt

from .base import docker_available
from ..ssh_manager import SSHManager

logger = logging.getLogger("ssh-mcp")

SIP_UDP_PORT = 5060
SIP_TCP_PORT = 5060
SIP_TLS_PORT = 5061
RTP_PORT_RANGE = "50000-60000"
DEFAULT_PCAP_LIMIT_BYTES = 5 * 1024 * 1024


def _utc_iso(ts: float) -> str:
    return datetime.utcfromtimestamp(ts).isoformat() + "Z"


def _build_sip_bpf_filter(port: int | None, protocol: str | None) -> str:
    if protocol:
        proto = protocol.lower()
        if proto == "tls":
            return f"tcp port {SIP_TLS_PORT if port is None else port}"
        if proto == "tcp":
            return f"tcp port {SIP_TCP_PORT if port is None else port}"
        if proto == "udp":
            return f"udp port {SIP_UDP_PORT if port is None else port}"

    if port is not None:
        return f"udp port {port} or tcp port {port}"

    return "udp port 5060 or tcp port 5060 or tcp port 5061"


async def _check_tool_in_container(manager: SSHManager, container: str, tool: str, target: str | None) -> bool:
    cmd = f"docker exec {container} command -v {tool} >/dev/null 2>&1 && echo 'present' || echo 'missing'"
    output = await manager.run(cmd, target=target)
    return "present" in output


async def _container_exists(manager: SSHManager, container: str, target: str | None) -> bool:
    cmd = f"docker inspect -f '{{{{.Id}}}}' {container} 2>/dev/null"
    output = await manager.run(cmd, target=target)
    return bool(output.strip())


async def _check_tool_on_host(manager: SSHManager, tool: str, target: str | None) -> bool:
    cmd = f"command -v {tool} >/dev/null 2>&1 && echo 'present' || echo 'missing'"
    output = await manager.run(cmd, target=target)
    return "present" in output


def _limit_output(text: str, max_lines: int = 50, max_chars: int = 4000) -> tuple[str, bool]:
    lines = text.splitlines()
    truncated = False
    if len(lines) > max_lines:
        lines = lines[:max_lines]
        truncated = True
    output = "\n".join(lines)
    if len(output) > max_chars:
        output = output[:max_chars]
        truncated = True
    return output, truncated


def _parse_ping_summary(output: str) -> dict[str, Any]:
    summary: dict[str, Any] = {
        "transmitted": None,
        "received": None,
        "packet_loss": None,
    }
    for line in output.splitlines():
        if "packets transmitted" in line and "received" in line:
            tokens = [tok.strip() for tok in line.replace(",", "").split()]
            for idx, token in enumerate(tokens):
                if token == "transmitted" and idx > 0:
                    prev = tokens[idx - 1]
                    if prev.isdigit():
                        summary["transmitted"] = int(prev)
                if token == "received" and idx > 0:
                    prev = tokens[idx - 1]
                    if prev.isdigit():
                        summary["received"] = int(prev)
                if token.endswith("%"):
                    if token[:-1].isdigit():
                        summary["packet_loss"] = int(token[:-1])
            break
    return summary


async def _copy_from_container(
    manager: SSHManager,
    container: str,
    container_path: str,
    host_path: str,
    target: str | None,
) -> str:
    cmd = f"docker cp {container}:{container_path} {host_path} 2>&1"
    return await manager.run(cmd, target=target)


async def _get_file_size(manager: SSHManager, path: str, target: str | None) -> int | None:
    output = await manager.run(f"stat -c %s {path} 2>/dev/null", target=target)
    output = output.strip()
    if output.isdigit():
        return int(output)
    return None


async def _read_pcap_from_container(
    manager: SSHManager,
    container: str,
    pcap_path: str,
    target: str | None,
    max_bytes: int | None = None,
) -> tuple[bytes | None, dict[str, Any]]:
    result: dict[str, Any] = {
        "error": None,
        "pcap_size": None,
        "pcap_path": pcap_path,
    }

    if max_bytes is None or max_bytes <= 0:
        max_bytes = DEFAULT_PCAP_LIMIT_BYTES

    if not await docker_available(manager, target or "primary"):
        result["error"] = "docker_not_available"
        return None, result

    if not await _container_exists(manager, container, target):
        result["error"] = "container_not_found"
        return None, result

    host_path = f"/tmp/voip_pcap_{uuid.uuid4().hex}.pcap"
    copy_out = await _copy_from_container(manager, container, pcap_path, host_path, target)
    if "No such" in copy_out or "error" in copy_out.lower():
        result["error"] = f"pcap_copy_failed: {copy_out.strip()}"
        return None, result

    size = await _get_file_size(manager, host_path, target)
    result["pcap_size"] = size
    if size is None:
        await manager.run(f"rm -f {host_path}", target=target)
        result["error"] = "pcap_size_unavailable"
        return None, result

    if size > max_bytes:
        await manager.run(f"rm -f {host_path}", target=target)
        result["error"] = "pcap_too_large"
        result["max_bytes"] = max_bytes
        return None, result

    b64 = await manager.run(f"base64 {host_path} 2>/dev/null", target=target)
    await manager.run(f"rm -f {host_path}", target=target)
    if not b64.strip():
        result["error"] = "pcap_read_failed"
        return None, result

    compact = "".join(b64.split())
    try:
        data = base64.b64decode(compact, validate=False)
    except Exception as exc:
        result["error"] = f"pcap_decode_failed: {exc}"
        return None, result

    return data, result


def _looks_like_sip(payload: bytes) -> bool:
    if not payload:
        return False
    methods = (
        b"INVITE ",
        b"REGISTER ",
        b"ACK ",
        b"BYE ",
        b"CANCEL ",
        b"OPTIONS ",
        b"PRACK ",
        b"UPDATE ",
        b"MESSAGE ",
        b"SUBSCRIBE ",
        b"NOTIFY ",
        b"REFER ",
        b"PUBLISH ",
        b"INFO ",
    )
    if payload.startswith(b"SIP/2.0"):
        return True
    return any(payload.startswith(m) for m in methods)


def _parse_headers_body(text: str) -> tuple[dict[str, Any], str]:
    lines = text.splitlines()
    if not lines:
        return {}, ""

    header_lines: list[str] = []
    body_lines: list[str] = []
    in_body = False
    for line in lines[1:]:
        if not in_body:
            if line == "":
                in_body = True
                continue
            header_lines.append(line)
        else:
            body_lines.append(line)

    merged: list[str] = []
    for line in header_lines:
        if line.startswith((" ", "\t")) and merged:
            merged[-1] += " " + line.strip()
        else:
            merged.append(line.strip())

    headers: dict[str, Any] = {}
    for line in merged:
        name, sep, value = line.partition(":")
        if not sep:
            continue
        key = name.strip().lower()
        val = value.strip()
        if key in headers:
            existing = headers[key]
            if isinstance(existing, list):
                existing.append(val)
            else:
                headers[key] = [existing, val]
        else:
            headers[key] = val

    return headers, "\n".join(body_lines)


def _get_header(headers: dict[str, Any], name: str) -> str | None:
    value = headers.get(name)
    if isinstance(value, list):
        return value[0]
    return value


def _extract_sip_uri(value: str | None) -> str | None:
    if not value:
        return None
    lower = value.lower()
    idx = lower.find("sip:")
    if idx == -1:
        idx = lower.find("sips:")
        if idx == -1:
            return None
    end = len(value)
    for delim in (">", ";", " ", "\t", "\r", "\n"):
        pos = value.find(delim, idx)
        if pos != -1 and pos < end:
            end = pos
    return value[idx:end]


def _extract_user_from_uri(uri: str | None) -> str | None:
    if not uri:
        return None
    if uri.startswith("sips:"):
        rest = uri[5:]
    elif uri.startswith("sip:"):
        rest = uri[4:]
    else:
        rest = uri
    if "@" in rest:
        return rest.split("@", 1)[0]
    return None


def _parse_cseq(value: str | None) -> tuple[int | None, str | None]:
    if not value:
        return None, None
    parts = value.split()
    if not parts:
        return None, None
    number = int(parts[0]) if parts[0].isdigit() else None
    method = parts[1] if len(parts) > 1 else None
    return number, method


def _parse_sip_message(
    payload: bytes,
    ts: float,
    src_ip: str,
    dst_ip: str,
    src_port: int,
    dst_port: int,
    transport: str,
) -> dict[str, Any] | None:
    text = payload.decode("utf-8", errors="replace")
    lines = text.splitlines()
    if not lines:
        return None
    start_line = lines[0].strip()
    if not start_line:
        return None

    headers, body = _parse_headers_body(text)
    call_id = _get_header(headers, "call-id")
    from_header = _get_header(headers, "from")
    to_header = _get_header(headers, "to")
    cseq_header = _get_header(headers, "cseq")
    content_type = _get_header(headers, "content-type")
    content_length = _get_header(headers, "content-length")
    contact = _get_header(headers, "contact")

    from_uri = _extract_sip_uri(from_header)
    to_uri = _extract_sip_uri(to_header)
    from_user = _extract_user_from_uri(from_uri)
    to_user = _extract_user_from_uri(to_uri)
    cseq_number, cseq_method = _parse_cseq(cseq_header)

    message: dict[str, Any] = {
        "timestamp": ts,
        "time": _utc_iso(ts),
        "src": {"ip": src_ip, "port": src_port},
        "dst": {"ip": dst_ip, "port": dst_port},
        "transport": transport,
        "call_id": call_id,
        "from_user": from_user,
        "to_user": to_user,
        "from_uri": from_uri,
        "to_uri": to_uri,
        "cseq_number": cseq_number,
        "cseq_method": cseq_method,
        "content_type": content_type,
        "content_length": int(content_length) if content_length and content_length.isdigit() else None,
        "contact": contact,
        "_body": body,
    }

    if start_line.startswith("SIP/2.0"):
        parts = start_line.split()
        code = int(parts[1]) if len(parts) > 1 and parts[1].isdigit() else None
        reason = " ".join(parts[2:]) if len(parts) > 2 else ""
        message.update({
            "type": "response",
            "status_code": code,
            "reason": reason,
        })
    else:
        parts = start_line.split()
        if not parts:
            return None
        method = parts[0]
        uri = parts[1] if len(parts) > 1 else None
        message.update({
            "type": "request",
            "method": method,
            "uri": uri,
        })

    return message


def _find_header_end(buffer: bytearray) -> int | None:
    idx = buffer.find(b"\r\n\r\n")
    if idx != -1:
        return idx + 4
    idx = buffer.find(b"\n\n")
    if idx != -1:
        return idx + 2
    return None


def _parse_headers_for_length(header_text: str) -> int:
    lines = header_text.splitlines()
    if not lines:
        return 0
    merged: list[str] = []
    for line in lines[1:]:
        if line.startswith((" ", "\t")) and merged:
            merged[-1] += " " + line.strip()
        else:
            merged.append(line.strip())
    for line in merged:
        name, sep, value = line.partition(":")
        if not sep:
            continue
        if name.strip().lower() == "content-length":
            val = value.strip()
            if val.isdigit():
                return int(val)
    return 0


def _extract_sip_messages_from_stream(
    buffer: bytearray,
    ts: float,
    src_ip: str,
    dst_ip: str,
    src_port: int,
    dst_port: int,
) -> tuple[list[dict[str, Any]], bytearray]:
    messages: list[dict[str, Any]] = []
    while True:
        header_end = _find_header_end(buffer)
        if header_end is None:
            break
        header_text = buffer[:header_end].decode("utf-8", errors="replace")
        body_len = _parse_headers_for_length(header_text)
        total_len = header_end + body_len
        if len(buffer) < total_len:
            break
        message_bytes = bytes(buffer[:total_len])
        del buffer[:total_len]
        if _looks_like_sip(message_bytes):
            msg = _parse_sip_message(message_bytes, ts, src_ip, dst_ip, src_port, dst_port, "tcp")
            if msg:
                messages.append(msg)
    return messages, buffer


def _parse_pcap_bytes(pcap_bytes: bytes) -> dict[str, Any]:
    messages: list[dict[str, Any]] = []
    packet_count = 0
    tls_packets = 0
    tcp_streams: dict[tuple[str, str, int, int], bytearray] = {}

    reader = dpkt.pcap.Reader(io.BytesIO(pcap_bytes))
    for ts, buf in reader:
        packet_count += 1
        try:
            eth = dpkt.ethernet.Ethernet(buf)
        except Exception:
            continue
        data = eth.data
        if eth.type == dpkt.ethernet.ETH_TYPE_8021Q:
            data = eth.data.data

        if isinstance(data, dpkt.ip.IP):
            ip = data
            src_ip = socket.inet_ntoa(ip.src)
            dst_ip = socket.inet_ntoa(ip.dst)

            if isinstance(ip.data, dpkt.udp.UDP):
                udp = ip.data
                payload = bytes(udp.data)
                if _looks_like_sip(payload):
                    msg = _parse_sip_message(payload, ts, src_ip, dst_ip, udp.sport, udp.dport, "udp")
                    if msg:
                        messages.append(msg)
                continue

            if isinstance(ip.data, dpkt.tcp.TCP):
                tcp = ip.data
                payload = bytes(tcp.data)
                if not payload:
                    continue
                if tcp.sport == SIP_TLS_PORT or tcp.dport == SIP_TLS_PORT:
                    if not _looks_like_sip(payload):
                        tls_packets += 1
                        continue

                key = (src_ip, dst_ip, tcp.sport, tcp.dport)
                stream = tcp_streams.get(key)
                if stream is None:
                    stream = bytearray()
                    tcp_streams[key] = stream
                stream.extend(payload)
                msgs, remaining = _extract_sip_messages_from_stream(stream, ts, src_ip, dst_ip, tcp.sport, tcp.dport)
                tcp_streams[key] = remaining
                messages.extend(msgs)

    return {
        "messages": messages,
        "packet_count": packet_count,
        "tls_packets": tls_packets,
    }


def _group_by_call_id(messages: list[dict[str, Any]]) -> dict[str, list[dict[str, Any]]]:
    grouped: dict[str, list[dict[str, Any]]] = {}
    for msg in messages:
        call_id = msg.get("call_id")
        if not call_id:
            continue
        grouped.setdefault(call_id, []).append(msg)
    for call_id, items in grouped.items():
        items.sort(key=lambda m: m.get("timestamp", 0))
    return grouped


def _final_response(messages: list[dict[str, Any]]) -> dict[str, Any] | None:
    final = None
    for msg in messages:
        if msg.get("type") == "response" and msg.get("status_code") is not None:
            code = msg.get("status_code")
            if isinstance(code, int) and code >= 200:
                final = msg
    return final


def _call_matches_number(messages: list[dict[str, Any]], number: str) -> bool:
    candidate = number.strip().lower()
    if not candidate:
        return False
    for msg in messages:
        for field in ("from_user", "to_user", "from_uri", "to_uri"):
            value = msg.get(field)
            if not value:
                continue
            if candidate in str(value).lower():
                return True
    return False


def _call_status(final_msg: dict[str, Any] | None) -> tuple[str, int | None]:
    if not final_msg:
        return "unknown", None
    code = final_msg.get("status_code")
    if isinstance(code, int) and 200 <= code < 300:
        return "success", code
    if isinstance(code, int) and code >= 300:
        return "failed", code
    return "unknown", code if isinstance(code, int) else None


def _parse_sdp(body: str) -> dict[str, Any]:
    lines = [line.strip() for line in body.splitlines() if line.strip()]
    connection_address = None
    media: list[dict[str, Any]] = []
    payload_codecs: dict[str, str] = {}
    current_media: dict[str, Any] | None = None

    for line in lines:
        if line.startswith("c="):
            parts = line[2:].split()
            if len(parts) >= 3:
                connection_address = parts[2]
        elif line.startswith("m="):
            parts = line[2:].split()
            if len(parts) >= 3:
                m_type = parts[0]
                port = int(parts[1]) if parts[1].isdigit() else None
                proto = parts[2]
                payloads = []
                for item in parts[3:]:
                    if item.isdigit():
                        payloads.append(int(item))
                current_media = {
                    "type": m_type,
                    "port": port,
                    "proto": proto,
                    "payloads": payloads,
                    "codecs": [],
                    "direction": None,
                }
                media.append(current_media)
        elif line.startswith("a=rtpmap:"):
            value = line[len("a=rtpmap:"):].strip()
            if " " in value:
                payload_id, codec = value.split(" ", 1)
                payload_codecs[payload_id] = codec.strip()
        elif line == "a=sendrecv":
            if current_media is not None:
                current_media["direction"] = "sendrecv"
        elif line == "a=sendonly":
            if current_media is not None:
                current_media["direction"] = "sendonly"
        elif line == "a=recvonly":
            if current_media is not None:
                current_media["direction"] = "recvonly"

    for entry in media:
        codecs = []
        for payload in entry.get("payloads", []):
            codec = payload_codecs.get(str(payload))
            if codec:
                codecs.append(codec)
        entry["codecs"] = codecs

    return {
        "connection_address": connection_address,
        "media": media,
    }


def _parse_port_range(port_range: str) -> tuple[int, int] | None:
    if not port_range:
        return None
    if "-" not in port_range:
        if port_range.isdigit():
            val = int(port_range)
            return val, val
        return None
    start, _, end = port_range.partition("-")
    if start.isdigit() and end.isdigit():
        return int(start), int(end)
    return None


def _count_udp_packets_in_range(pcap_bytes: bytes, port_range: str) -> dict[str, Any]:
    bounds = _parse_port_range(port_range)
    if not bounds:
        return {"error": "invalid_port_range"}
    start, end = bounds
    packet_count = 0
    sources: set[str] = set()
    destinations: set[str] = set()

    reader = dpkt.pcap.Reader(io.BytesIO(pcap_bytes))
    for _, buf in reader:
        try:
            eth = dpkt.ethernet.Ethernet(buf)
        except Exception:
            continue
        data = eth.data
        if eth.type == dpkt.ethernet.ETH_TYPE_8021Q:
            data = eth.data.data
        if not isinstance(data, dpkt.ip.IP):
            continue
        ip = data
        if not isinstance(ip.data, dpkt.udp.UDP):
            continue
        udp = ip.data
        if start <= udp.sport <= end or start <= udp.dport <= end:
            packet_count += 1
            sources.add(socket.inet_ntoa(ip.src))
            destinations.add(socket.inet_ntoa(ip.dst))

    return {
        "packet_count": packet_count,
        "sources": sorted(sources),
        "destinations": sorted(destinations),
    }


async def voip_sip_capture(
    manager: SSHManager,
    container: str,
    duration: int = 30,
    port: int | None = None,
    protocol: str | None = None,
    target: str | None = None,
) -> dict[str, Any]:
    """Capture SIP signaling using sngrep and save to PCAP inside the container."""
    result = {
        "container": container,
        "duration": duration,
        "pcap_file": None,
        "filter": None,
        "tool": "sngrep",
        "error": None,
    }

    if not await docker_available(manager, target or "primary"):
        result["error"] = "docker_not_available"
        return result

    if not await _container_exists(manager, container, target):
        result["error"] = "container_not_found"
        return result

    if not await _check_tool_in_container(manager, container, "sngrep", target):
        result["error"] = "sngrep_not_available"
        return result

    pcap_path = f"/tmp/voip_sip_{int(time.time())}_{uuid.uuid4().hex}.pcap"
    bpf_filter = _build_sip_bpf_filter(port, protocol)
    result["pcap_file"] = pcap_path
    result["filter"] = bpf_filter

    cmd = f"docker exec {container} timeout {duration}s sngrep -N -q -d any -O {pcap_path} '{bpf_filter}'"
    try:
        await manager.run(cmd, target=target, timeout=duration + 10)
    except Exception as exc:
        result["error"] = f"capture_failed: {exc}"
    return result


async def voip_call_flow(
    manager: SSHManager,
    container: str,
    pcap_file: str,
    call_id: str | None = None,
    phone_number: str | None = None,
    max_bytes: int | None = None,
    summary_only: bool = False,
    target: str | None = None,
) -> dict[str, Any]:
    """Parse SIP call flow from a PCAP captured by sngrep, optionally filtered by number."""
    result = {
        "pcap_file": pcap_file,
        "call_id": call_id,
        "phone_number": phone_number,
        "summary_only": summary_only,
        "calls": [],
        "call_count": 0,
        "tls_packets": 0,
        "error": None,
    }

    pcap_bytes, meta = await _read_pcap_from_container(manager, container, pcap_file, target, max_bytes)
    if not pcap_bytes:
        result.update(meta)
        return result

    parsed = _parse_pcap_bytes(pcap_bytes)
    messages = parsed["messages"]
    grouped = _group_by_call_id(messages)
    tls_packets = parsed.get("tls_packets", 0)

    calls_out: list[dict[str, Any]] = []
    for cid, msgs in grouped.items():
        if call_id and cid != call_id:
            continue
        if phone_number and not _call_matches_number(msgs, phone_number):
            continue
        final = _final_response(msgs)
        status, code = _call_status(final)
        from_user = None
        to_user = None
        from_uri = None
        to_uri = None
        for msg in msgs:
            if not from_user and msg.get("from_user"):
                from_user = msg.get("from_user")
            if not to_user and msg.get("to_user"):
                to_user = msg.get("to_user")
            if not from_uri and msg.get("from_uri"):
                from_uri = msg.get("from_uri")
            if not to_uri and msg.get("to_uri"):
                to_uri = msg.get("to_uri")

        call_messages = []
        if not summary_only:
            for msg in msgs:
                call_messages.append({
                    "time": msg.get("time"),
                    "type": msg.get("type"),
                    "method": msg.get("method"),
                    "status_code": msg.get("status_code"),
                    "reason": msg.get("reason"),
                    "src": msg.get("src"),
                    "dst": msg.get("dst"),
                    "transport": msg.get("transport"),
                })

        start_time = msgs[0].get("time") if msgs else None
        end_time = msgs[-1].get("time") if msgs else None
        has_sdp = any(
            msg.get("content_type") and "application/sdp" in msg.get("content_type")
            for msg in msgs
        )

        calls_out.append({
            "call_id": cid,
            "from_user": from_user,
            "to_user": to_user,
            "from_uri": from_uri,
            "to_uri": to_uri,
            "start_time": start_time,
            "end_time": end_time,
            "message_count": len(msgs),
            "has_sdp": has_sdp,
            "messages": call_messages,
            "final_status": status,
            "error_code": code if status == "failed" else None,
        })

    result["calls"] = calls_out
    result["call_count"] = len(calls_out)
    result["tls_packets"] = tls_packets
    return result


async def voip_registrations(
    manager: SSHManager,
    container: str,
    pcap_file: str,
    max_bytes: int | None = None,
    target: str | None = None,
) -> dict[str, Any]:
    """Extract REGISTER dialogs and their outcomes from a SIP PCAP."""
    result = {
        "pcap_file": pcap_file,
        "registrations": [],
        "total": 0,
        "success": 0,
        "failed": 0,
        "error": None,
    }

    pcap_bytes, meta = await _read_pcap_from_container(manager, container, pcap_file, target, max_bytes)
    if not pcap_bytes:
        result.update(meta)
        return result

    parsed = _parse_pcap_bytes(pcap_bytes)
    messages = parsed["messages"]
    grouped = _group_by_call_id(messages)

    registrations: list[dict[str, Any]] = []
    for cid, msgs in grouped.items():
        requests = [m for m in msgs if m.get("type") == "request" and m.get("method") == "REGISTER"]
        if not requests:
            continue
        from_user = requests[0].get("from_user")
        contact = requests[0].get("contact")
        final = _final_response(msgs)
        status, code = _call_status(final)
        registrations.append({
            "call_id": cid,
            "user": from_user,
            "contact": contact,
            "status": status,
            "response_code": code,
            "response_reason": final.get("reason") if final else None,
        })

    result["registrations"] = registrations
    result["total"] = len(registrations)
    result["success"] = sum(1 for r in registrations if r.get("status") == "success")
    result["failed"] = sum(1 for r in registrations if r.get("status") == "failed")
    return result


async def voip_call_stats(
    manager: SSHManager,
    container: str,
    pcap_file: str,
    max_bytes: int | None = None,
    target: str | None = None,
) -> dict[str, Any]:
    """Aggregate SIP call statistics from a PCAP."""
    result = {
        "pcap_file": pcap_file,
        "total_calls": 0,
        "successful_calls": 0,
        "failed_calls": 0,
        "response_codes": {},
        "avg_setup_time_ms": None,
        "error": None,
    }

    pcap_bytes, meta = await _read_pcap_from_container(manager, container, pcap_file, target, max_bytes)
    if not pcap_bytes:
        result.update(meta)
        return result

    parsed = _parse_pcap_bytes(pcap_bytes)
    messages = parsed["messages"]
    grouped = _group_by_call_id(messages)

    setup_times: list[int] = []
    response_codes: dict[str, int] = {}
    success = 0
    failed = 0

    for _, msgs in grouped.items():
        final = _final_response(msgs)
        status, code = _call_status(final)
        if status == "success":
            success += 1
        elif status == "failed":
            failed += 1
        if code is not None:
            key = str(code)
            response_codes[key] = response_codes.get(key, 0) + 1

        invite_time = None
        final_time = None
        for msg in msgs:
            if invite_time is None and msg.get("type") == "request" and msg.get("method") == "INVITE":
                invite_time = msg.get("timestamp")
            if msg.get("type") == "response" and msg.get("status_code") and msg.get("status_code") >= 200:
                final_time = msg.get("timestamp")
        if invite_time is not None and final_time is not None:
            setup_times.append(int((final_time - invite_time) * 1000))

    result["total_calls"] = len(grouped)
    result["successful_calls"] = success
    result["failed_calls"] = failed
    result["response_codes"] = response_codes
    if setup_times:
        result["avg_setup_time_ms"] = sum(setup_times) // len(setup_times)
    return result


async def voip_extract_sdp(
    manager: SSHManager,
    container: str,
    pcap_file: str,
    call_id: str | None = None,
    max_bytes: int | None = None,
    target: str | None = None,
) -> dict[str, Any]:
    """Extract SDP from SIP messages in a PCAP."""
    result = {
        "pcap_file": pcap_file,
        "call_id": call_id,
        "sessions": [],
        "error": None,
    }

    pcap_bytes, meta = await _read_pcap_from_container(manager, container, pcap_file, target, max_bytes)
    if not pcap_bytes:
        result.update(meta)
        return result

    parsed = _parse_pcap_bytes(pcap_bytes)
    messages = parsed["messages"]

    sessions: list[dict[str, Any]] = []
    for msg in messages:
        if call_id and msg.get("call_id") != call_id:
            continue
        content_type = msg.get("content_type") or ""
        if "application/sdp" not in content_type:
            continue
        body = msg.get("_body") or ""
        if not body:
            continue
        sdp = _parse_sdp(body)
        sessions.append({
            "call_id": msg.get("call_id"),
            "type": msg.get("type"),
            "method": msg.get("method"),
            "status_code": msg.get("status_code"),
            "from_user": msg.get("from_user"),
            "to_user": msg.get("to_user"),
            "sdp": sdp,
        })

    result["sessions"] = sessions
    return result


async def voip_packet_check(
    manager: SSHManager,
    container: str,
    duration: int = 5,
    interface: str = "any",
    target: str | None = None,
) -> dict[str, Any]:
    """Quick network check for SIP packets on standard ports."""
    result = {
        "container": container,
        "duration": duration,
        "packet_count": 0,
        "sip_5060_udp": False,
        "sip_5060_tcp": False,
        "sip_5061_tls": False,
        "sources": [],
        "error": None,
    }

    if not await docker_available(manager, target or "primary"):
        result["error"] = "docker_not_available"
        return result

    if not await _container_exists(manager, container, target):
        result["error"] = "container_not_found"
        return result

    if not await _check_tool_in_container(manager, container, "tcpdump", target):
        result["error"] = "tcpdump_not_available"
        return result

    bpf_filter = "udp port 5060 or tcp port 5060 or tcp port 5061"
    cmd = (
        f"docker exec {container} timeout {duration}s tcpdump -i {interface} -n -c 20"
        f" '{bpf_filter}' 2>/dev/null"
    )
    output = await manager.run(cmd, target=target, timeout=duration + 5)
    lines = [line for line in output.splitlines() if line.strip()]
    sources: set[str] = set()

    for line in lines:
        tokens = line.split()
        if len(tokens) < 3:
            continue
        if tokens[1] != "IP":
            continue
        src_token = tokens[2].rstrip(":")
        dest_token = tokens[4].rstrip(":") if len(tokens) > 4 else ""

        def split_ip_port(token: str) -> tuple[str | None, int | None]:
            token = token.strip()
            if token.endswith(":"):
                token = token[:-1]
            if "." not in token:
                return token, None
            parts = token.split(".")
            if parts and parts[-1].isdigit():
                port = int(parts[-1])
                ip = ".".join(parts[:-1])
                return ip, port
            return token, None

        src_ip, src_port = split_ip_port(src_token)
        _, dst_port = split_ip_port(dest_token)
        if src_ip:
            sources.add(src_ip)

        if src_port == 5060 or dst_port == 5060:
            if "UDP" in line:
                result["sip_5060_udp"] = True
            else:
                result["sip_5060_tcp"] = True
        if src_port == 5061 or dst_port == 5061:
            result["sip_5061_tls"] = True

    result["packet_count"] = len(lines)
    result["sources"] = sorted(sources)
    return result


async def voip_network_capture(
    manager: SSHManager,
    container: str,
    duration: int = 30,
    interface: str = "any",
    target: str | None = None,
) -> dict[str, Any]:
    """Capture SIP packets using tcpdump for network-level analysis."""
    result = {
        "container": container,
        "duration": duration,
        "pcap_file": None,
        "filter": "udp port 5060 or tcp port 5060 or tcp port 5061",
        "packet_count": 0,
        "error": None,
    }

    if not await docker_available(manager, target or "primary"):
        result["error"] = "docker_not_available"
        return result

    if not await _container_exists(manager, container, target):
        result["error"] = "container_not_found"
        return result

    if not await _check_tool_in_container(manager, container, "tcpdump", target):
        result["error"] = "tcpdump_not_available"
        return result

    pcap_path = f"/tmp/voip_net_{int(time.time())}_{uuid.uuid4().hex}.pcap"
    result["pcap_file"] = pcap_path
    cmd = (
        f"docker exec {container} timeout {duration}s tcpdump -i {interface} -n -s 0"
        f" -w {pcap_path} '{result['filter']}' 2>/dev/null"
    )
    try:
        await manager.run(cmd, target=target, timeout=duration + 10)
    except Exception as exc:
        result["error"] = f"capture_failed: {exc}"
        return result

    pcap_bytes, meta = await _read_pcap_from_container(manager, container, pcap_path, target)
    if not pcap_bytes:
        result.update(meta)
        return result
    parsed = _parse_pcap_bytes(pcap_bytes)
    result["packet_count"] = parsed.get("packet_count", 0)
    return result


async def voip_rtp_capture(
    manager: SSHManager,
    container: str,
    duration: int = 10,
    interface: str = "any",
    port_range: str = RTP_PORT_RANGE,
    target: str | None = None,
) -> dict[str, Any]:
    """Capture RTP packets on the configured RTP port range."""
    result = {
        "container": container,
        "duration": duration,
        "port_range": port_range,
        "pcap_file": None,
        "packet_count": 0,
        "sources": [],
        "destinations": [],
        "error": None,
    }

    if not await docker_available(manager, target or "primary"):
        result["error"] = "docker_not_available"
        return result

    if not await _container_exists(manager, container, target):
        result["error"] = "container_not_found"
        return result

    if not await _check_tool_in_container(manager, container, "tcpdump", target):
        result["error"] = "tcpdump_not_available"
        return result

    pcap_path = f"/tmp/voip_rtp_{int(time.time())}_{uuid.uuid4().hex}.pcap"
    result["pcap_file"] = pcap_path
    cmd = (
        f"docker exec {container} timeout {duration}s tcpdump -i {interface} -n -s 0"
        f" -w {pcap_path} 'udp portrange {port_range}' 2>/dev/null"
    )
    try:
        await manager.run(cmd, target=target, timeout=duration + 10)
    except Exception as exc:
        result["error"] = f"capture_failed: {exc}"
        return result

    pcap_bytes, meta = await _read_pcap_from_container(manager, container, pcap_path, target)
    if not pcap_bytes:
        result.update(meta)
        return result
    count_info = _count_udp_packets_in_range(pcap_bytes, port_range)
    if "error" in count_info:
        result["error"] = count_info["error"]
        return result
    result["packet_count"] = count_info["packet_count"]
    result["sources"] = count_info["sources"]
    result["destinations"] = count_info["destinations"]
    return result


async def voip_discover_containers(
    manager: SSHManager,
    keywords: list[str] | None = None,
    target: str | None = None,
) -> dict[str, Any]:
    """Discover VoIP-related containers by name/image keyword matching."""
    result = {
        "keywords": [],
        "containers": [],
        "error": None,
    }

    if not await docker_available(manager, target or "primary"):
        result["error"] = "docker_not_available"
        return result

    default_keywords = ["gw", "media", "fs", "sbc", "sw"]
    use_keywords = [k.strip().lower() for k in (keywords or default_keywords) if k.strip()]
    result["keywords"] = use_keywords

    output = await manager.run("docker ps --format '{{.Names}}|{{.Image}}'", target=target)
    for line in output.splitlines():
        if "|" not in line:
            continue
        name, image = line.split("|", 1)
        name_lower = name.lower()
        image_lower = image.lower()
        matches = [k for k in use_keywords if k in name_lower or k in image_lower]
        if not matches:
            continue
        result["containers"].append({
            "name": name,
            "image": image,
            "matches": matches,
        })

    return result


async def voip_network_diagnostics(
    manager: SSHManager,
    host: str,
    ports: list[int] | None = None,
    ping_count: int = 3,
    traceroute: bool = True,
    timeout_seconds: int = 15,
    target: str | None = None,
) -> dict[str, Any]:
    """Run basic network diagnostics: ping, traceroute/tracepath, TCP port checks."""
    result = {
        "host": host,
        "ping": {"attempted": False, "available": False, "reachable": None, "summary": None, "output": None},
        "traceroute": {"attempted": False, "tool": None, "output": None, "truncated": False},
        "tcp_checks": [],
        "notes": [],
        "error": None,
    }

    ping_available = await _check_tool_on_host(manager, "ping", target)
    result["ping"]["available"] = ping_available
    if ping_available:
        result["ping"]["attempted"] = True
        cmd = f"ping -n -c {ping_count} -W 2 {host}"
        ping_res = await manager.run_result(cmd, target=target, timeout=timeout_seconds)
        summary = _parse_ping_summary(ping_res["stdout"])
        result["ping"]["summary"] = summary
        result["ping"]["reachable"] = summary.get("received") not in (None, 0)
        output, _ = _limit_output(ping_res["stdout"])
        result["ping"]["output"] = output
        if summary.get("received") == 0:
            result["notes"].append("ICMP may be blocked; use traceroute/tcp checks")
    else:
        result["notes"].append("ping not available on target")

    if traceroute:
        trace_cmd = None
        trace_tool = None
        if await _check_tool_on_host(manager, "traceroute", target):
            trace_tool = "traceroute"
            trace_cmd = f"traceroute -n -w 2 -q 1 -m 20 {host}"
        elif await _check_tool_on_host(manager, "tracepath", target):
            trace_tool = "tracepath"
            trace_cmd = f"tracepath -n -m 20 {host}"

        if trace_cmd:
            result["traceroute"]["attempted"] = True
            result["traceroute"]["tool"] = trace_tool
            trace_res = await manager.run_result(trace_cmd, target=target, timeout=timeout_seconds)
            output, truncated = _limit_output(trace_res["stdout"])
            result["traceroute"]["output"] = output
            result["traceroute"]["truncated"] = truncated
        else:
            result["notes"].append("traceroute/tracepath not available on target")

    tcp_ports = ports or []
    if tcp_ports:
        has_nc = await _check_tool_on_host(manager, "nc", target)
        has_telnet = await _check_tool_on_host(manager, "telnet", target)
        has_timeout = await _check_tool_on_host(manager, "timeout", target)
        for port in tcp_ports:
            entry = {
                "port": port,
                "reachable": None,
                "tool": None,
                "output": None,
            }
            if has_nc:
                entry["tool"] = "nc"
                cmd = f"nc -zvw {min(5, timeout_seconds)} {host} {port}"
                res = await manager.run_result(cmd, target=target, timeout=timeout_seconds)
                entry["reachable"] = res["exit_code"] == 0
                entry["output"], _ = _limit_output(res["stdout"] + "\n" + res["stderr"])
            elif has_telnet and has_timeout:
                entry["tool"] = "telnet"
                cmd = f"timeout {timeout_seconds}s sh -c 'printf "" | telnet {host} {port}'"
                res = await manager.run_result(cmd, target=target, timeout=timeout_seconds + 2)
                entry["reachable"] = "Connected" in res["stdout"] or "Escape character" in res["stdout"]
                entry["output"], _ = _limit_output(res["stdout"] + "\n" + res["stderr"])
            else:
                entry["tool"] = None
                entry["reachable"] = None
                entry["output"] = "nc or telnet not available"
                result["notes"].append("TCP port checks require nc or telnet")
            result["tcp_checks"].append(entry)
    else:
        result["notes"].append("no TCP ports provided for reachability checks")

    result["notes"].append("UDP port reachability is not confirmed via telnet; use packet captures for UDP")
    return result
