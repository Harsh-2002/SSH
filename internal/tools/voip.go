package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ssh-mcp/internal/ssh"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// VoIP constants
const (
	SIPUDPPort    = 5060
	SIPTCPPort    = 5060
	SIPTLSPort    = 5061
	RTPPortRange  = "50000-60000"
	DefaultPCAPLimit = 5 * 1024 * 1024
)

// registerVoIPTools registers VoIP troubleshooting tools.
func registerVoIPTools(s *server.MCPServer, pool *ssh.Pool) {
	// voip_discover_containers
	s.AddTool(
		mcp.NewTool("voip_discover_containers",
			mcp.WithDescription("Find VoIP-related containers by name/image keywords"),
			mcp.WithArray("keywords", mcp.Description("Keywords to match (default: gw, media, fs, sbc, sw)")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createVoIPDiscoverHandler(pool),
	)

	// voip_sip_capture
	s.AddTool(
		mcp.NewTool("voip_sip_capture",
			mcp.WithDescription("Capture SIP signaling to PCAP using sngrep inside container"),
			mcp.WithString("container", mcp.Required(), mcp.Description("Container name")),
			mcp.WithNumber("duration", mcp.Description("Capture duration in seconds (default: 30)")),
			mcp.WithNumber("port", mcp.Description("SIP port to filter (default: 5060)")),
			mcp.WithString("protocol", mcp.Description("Protocol: udp, tcp, tls (default: all)")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createSIPCaptureHandler(pool),
	)

	// voip_call_flow
	s.AddTool(
		mcp.NewTool("voip_call_flow",
			mcp.WithDescription("Parse SIP call flow from a PCAP file"),
			mcp.WithString("container", mcp.Required(), mcp.Description("Container name")),
			mcp.WithString("pcap_file", mcp.Required(), mcp.Description("Path to PCAP file in container")),
			mcp.WithString("call_id", mcp.Description("Filter by Call-ID")),
			mcp.WithString("phone_number", mcp.Description("Filter by phone number")),
			mcp.WithBoolean("summary_only", mcp.Description("Return summary only, no message details")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createCallFlowHandler(pool),
	)

	// voip_registrations
	s.AddTool(
		mcp.NewTool("voip_registrations",
			mcp.WithDescription("Extract REGISTER dialogs and outcomes from SIP PCAP"),
			mcp.WithString("container", mcp.Required(), mcp.Description("Container name")),
			mcp.WithString("pcap_file", mcp.Required(), mcp.Description("Path to PCAP file")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createRegistrationsHandler(pool),
	)

	// voip_call_stats
	s.AddTool(
		mcp.NewTool("voip_call_stats",
			mcp.WithDescription("Aggregate SIP call statistics from PCAP"),
			mcp.WithString("container", mcp.Required(), mcp.Description("Container name")),
			mcp.WithString("pcap_file", mcp.Required(), mcp.Description("Path to PCAP file")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createCallStatsHandler(pool),
	)

	// voip_extract_sdp
	s.AddTool(
		mcp.NewTool("voip_extract_sdp",
			mcp.WithDescription("Extract SDP (codecs, RTP ports) from SIP messages"),
			mcp.WithString("container", mcp.Required(), mcp.Description("Container name")),
			mcp.WithString("pcap_file", mcp.Required(), mcp.Description("Path to PCAP file")),
			mcp.WithString("call_id", mcp.Description("Filter by specific Call-ID")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createExtractSDPHandler(pool),
	)

	// voip_packet_check
	s.AddTool(
		mcp.NewTool("voip_packet_check",
			mcp.WithDescription("Quick SIP packet presence check on standard ports"),
			mcp.WithString("container", mcp.Required(), mcp.Description("Container name")),
			mcp.WithNumber("duration", mcp.Description("Check duration in seconds (default: 5)")),
			mcp.WithString("interface", mcp.Description("Network interface (default: any)")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createPacketCheckHandler(pool),
	)

	// voip_network_capture
	s.AddTool(
		mcp.NewTool("voip_network_capture",
			mcp.WithDescription("Capture SIP packets with tcpdump for analysis"),
			mcp.WithString("container", mcp.Required(), mcp.Description("Container name")),
			mcp.WithNumber("duration", mcp.Description("Capture duration in seconds (default: 30)")),
			mcp.WithString("interface", mcp.Description("Network interface (default: any)")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createNetworkCaptureHandler(pool),
	)

	// voip_rtp_capture
	s.AddTool(
		mcp.NewTool("voip_rtp_capture",
			mcp.WithDescription("Capture RTP packets to verify media flow"),
			mcp.WithString("container", mcp.Required(), mcp.Description("Container name")),
			mcp.WithNumber("duration", mcp.Description("Capture duration in seconds (default: 10)")),
			mcp.WithString("port_range", mcp.Description("RTP port range (default: 50000-60000)")),
			mcp.WithString("interface", mcp.Description("Network interface (default: any)")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createRTPCaptureHandler(pool),
	)

	// voip_network_diagnostics
	s.AddTool(
		mcp.NewTool("voip_network_diagnostics",
			mcp.WithDescription("Run network diagnostics: ping, traceroute, TCP port checks"),
			mcp.WithString("host", mcp.Required(), mcp.Description("Target host for diagnostics")),
			mcp.WithArray("ports", mcp.Description("TCP ports to check (default: 5060, 5061)")),
			mcp.WithNumber("ping_count", mcp.Description("Number of pings (default: 3)")),
			mcp.WithBoolean("traceroute", mcp.Description("Include traceroute (default: true)")),
			mcp.WithNumber("timeout", mcp.Description("Timeout in seconds (default: 15)")),
			mcp.WithString("target", mcp.Description("Connection alias (default: primary)")),
		),
		createNetworkDiagnosticsHandler(pool),
	)
}

func createVoIPDiscoverHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Default VoIP keywords
		defaultKeywords := []string{"gw", "media", "fs", "sbc", "sw", "freeswitch", "asterisk", "kamailio", "opensips", "rtpengine"}

		// Build grep pattern
		pattern := strings.Join(defaultKeywords, "\\|")
		cmd := fmt.Sprintf(`docker ps --format '{{.Names}}|{{.Image}}' | grep -iE '%s' 2>/dev/null || echo ''`, pattern)

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if trimOutput(output) == "" {
			return mcp.NewToolResultText("No VoIP containers found"), nil
		}

		// Parse output into structured format
		var containers []map[string]string
		for _, line := range strings.Split(output, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "|", 2)
			if len(parts) == 2 {
				containers = append(containers, map[string]string{
					"name":  parts[0],
					"image": parts[1],
				})
			}
		}

		jsonBytes, _ := json.MarshalIndent(containers, "", "  ")
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

func createSIPCaptureHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		container, _ := req.RequireString("container")
		duration := req.GetInt("duration", 30)
		port := req.GetInt("port", 0)
		protocol := req.GetString("protocol", "")
		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Check if sngrep is available
		checkCmd := fmt.Sprintf("docker exec %s command -v sngrep >/dev/null 2>&1 && echo 'ok' || echo 'missing'", shellQuote(container))
		checkOutput, err := mgr.Execute(ctx, checkCmd, target)
		if err != nil || !containsString(checkOutput, "ok") {
			return mcp.NewToolResultError("sngrep not available in container. Install with: apt-get install sngrep"), nil
		}

		// Build BPF filter
		bpfFilter := buildSIPFilter(port, protocol)

		// Generate unique PCAP path
		pcapPath := fmt.Sprintf("/tmp/voip_sip_%d.pcap", time.Now().Unix())

		// Run sngrep capture
		cmd := fmt.Sprintf("docker exec %s timeout %ds sngrep -N -q -d any -O %s '%s' 2>&1 || true",
			shellQuote(container), duration, pcapPath, bpfFilter)

		mgr.Execute(ctx, cmd, target) // Ignore timeout error

		result := map[string]interface{}{
			"container":  container,
			"pcap_file":  pcapPath,
			"duration":   duration,
			"filter":     bpfFilter,
			"message":    fmt.Sprintf("SIP capture completed. Use voip_call_flow to analyze %s", pcapPath),
		}

		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

func createCallFlowHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		container, _ := req.RequireString("container")
		pcapFile, _ := req.RequireString("pcap_file")
		callID := req.GetString("call_id", "")
		phoneNumber := req.GetString("phone_number", "")
		summaryOnly := req.GetBool("summary_only", false)
		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Use tshark for PCAP analysis (most reliable)
		var filter string
		if callID != "" {
			filter = fmt.Sprintf("-Y 'sip.Call-ID == \"%s\"'", callID)
		} else if phoneNumber != "" {
			filter = fmt.Sprintf("-Y 'sip contains \"%s\"'", phoneNumber)
		}

		var cmd string
		if summaryOnly {
			// Get summary: method, response codes, call-ids
			cmd = fmt.Sprintf(`docker exec %s sh -c 'if command -v tshark >/dev/null 2>&1; then tshark -r %s -T fields -e frame.time -e ip.src -e ip.dst -e sip.Method -e sip.Status-Code -e sip.Call-ID %s 2>/dev/null | head -100; else sngrep -I %s -q 2>/dev/null | head -50 || echo "No analysis tool available"; fi'`,
				shellQuote(container), pcapFile, filter, pcapFile)
		} else {
			// Get detailed call flow
			cmd = fmt.Sprintf(`docker exec %s sh -c 'if command -v tshark >/dev/null 2>&1; then tshark -r %s -V -Y sip %s 2>/dev/null | head -500; else cat %s 2>/dev/null | strings | grep -E "^(INVITE|REGISTER|BYE|ACK|CANCEL|SIP/2.0)" | head -100 || echo "No analysis tool available"; fi'`,
				shellQuote(container), pcapFile, filter, pcapFile)
		}

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createRegistrationsHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		container, _ := req.RequireString("container")
		pcapFile, _ := req.RequireString("pcap_file")
		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Extract REGISTER dialogs using tshark or strings
		cmd := fmt.Sprintf(`docker exec %s sh -c 'if command -v tshark >/dev/null 2>&1; then tshark -r %s -Y "sip.Method == REGISTER or (sip.CSeq.method == REGISTER and sip.Status-Code)" -T fields -e frame.time -e sip.from.user -e sip.to.user -e sip.contact.uri -e sip.Status-Code -E header=y 2>/dev/null; else cat %s 2>/dev/null | strings | grep -E "(REGISTER|200 OK|401|403)" | head -50; fi'`,
			shellQuote(container), pcapFile, pcapFile)

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createCallStatsHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		container, _ := req.RequireString("container")
		pcapFile, _ := req.RequireString("pcap_file")
		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Aggregate stats using tshark
		cmd := fmt.Sprintf(`docker exec %s sh -c '
if command -v tshark >/dev/null 2>&1; then
  echo "=== SIP STATISTICS ==="
  echo ""
  echo "--- Request Methods ---"
  tshark -r %s -Y sip.Method -T fields -e sip.Method 2>/dev/null | sort | uniq -c | sort -rn
  echo ""
  echo "--- Response Codes ---"
  tshark -r %s -Y sip.Status-Code -T fields -e sip.Status-Code 2>/dev/null | sort | uniq -c | sort -rn
  echo ""
  echo "--- Unique Call-IDs ---"
  tshark -r %s -Y sip -T fields -e sip.Call-ID 2>/dev/null | sort -u | wc -l | xargs echo "Total calls:"
else
  cat %s 2>/dev/null | strings | grep -oE "^(INVITE|REGISTER|BYE|ACK|CANCEL|OPTIONS|SIP/2.0 [0-9]+)" | sort | uniq -c | sort -rn
fi'`, shellQuote(container), pcapFile, pcapFile, pcapFile, pcapFile)

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createExtractSDPHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		container, _ := req.RequireString("container")
		pcapFile, _ := req.RequireString("pcap_file")
		callID := req.GetString("call_id", "")
		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		var filter string
		if callID != "" {
			filter = fmt.Sprintf("-Y 'sip.Call-ID == \"%s\" and sdp'", callID)
		} else {
			filter = "-Y 'sdp'"
		}

		// Extract SDP with tshark
		cmd := fmt.Sprintf(`docker exec %s sh -c 'if command -v tshark >/dev/null 2>&1; then tshark -r %s %s -T fields -e sdp.connection_info -e sdp.media -e sdp.media.port -e sdp.media.format -E header=y 2>/dev/null | head -50; else cat %s 2>/dev/null | strings | grep -E "^(c=|m=|a=rtpmap)" | head -50; fi'`,
			shellQuote(container), pcapFile, filter, pcapFile)

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		return mcp.NewToolResultText(output), nil
	}
}

func createPacketCheckHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		container, _ := req.RequireString("container")
		duration := req.GetInt("duration", 5)
		iface := req.GetString("interface", "any")
		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Quick packet check using tcpdump
		cmd := fmt.Sprintf(`docker exec %s sh -c 'if command -v tcpdump >/dev/null 2>&1; then timeout %ds tcpdump -i %s -c 20 port 5060 or port 5061 2>&1 | tail -25; else echo "tcpdump not available"; fi'`,
			shellQuote(container), duration, iface)

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Summarize
		hasPackets := containsString(output, "UDP") || containsString(output, "TCP") || containsString(output, "SIP")
		summary := "SIP packets detected: NO"
		if hasPackets {
			summary = "SIP packets detected: YES"
		}

		return mcp.NewToolResultText(fmt.Sprintf("%s\n\n%s", summary, output)), nil
	}
}

func createNetworkCaptureHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		container, _ := req.RequireString("container")
		duration := req.GetInt("duration", 30)
		iface := req.GetString("interface", "any")
		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		pcapPath := fmt.Sprintf("/tmp/voip_net_%d.pcap", time.Now().Unix())

		// Capture with tcpdump
		cmd := fmt.Sprintf(`docker exec %s sh -c 'if command -v tcpdump >/dev/null 2>&1; then timeout %ds tcpdump -i %s -w %s port 5060 or port 5061 2>&1 || true; else echo "tcpdump not available"; fi'`,
			shellQuote(container), duration, iface, pcapPath)

		mgr.Execute(ctx, cmd, target) // Ignore timeout

		result := map[string]interface{}{
			"container":  container,
			"pcap_file":  pcapPath,
			"duration":   duration,
			"interface":  iface,
			"message":    fmt.Sprintf("Network capture complete. Analyze with voip_call_flow or copy with docker_cp_from"),
		}

		jsonBytes, _ := json.MarshalIndent(result, "", "  ")
		return mcp.NewToolResultText(string(jsonBytes)), nil
	}
}

func createRTPCaptureHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		container, _ := req.RequireString("container")
		duration := req.GetInt("duration", 10)
		portRange := req.GetString("port_range", RTPPortRange)
		iface := req.GetString("interface", "any")
		target := req.GetString("target", "primary")

		if err := checkDockerAvailable(ctx, mgr, target); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Parse port range
		ports := strings.Split(portRange, "-")
		startPort := "50000"
		endPort := "60000"
		if len(ports) == 2 {
			startPort = ports[0]
			endPort = ports[1]
		}

		// Capture RTP packets and count
		cmd := fmt.Sprintf(`docker exec %s sh -c 'if command -v tcpdump >/dev/null 2>&1; then timeout %ds tcpdump -i %s -c 100 "udp portrange %s-%s" 2>&1 | tail -20; else echo "tcpdump not available"; fi'`,
			shellQuote(container), duration, iface, startPort, endPort)

		output, err := mgr.Execute(ctx, cmd, target)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		hasRTP := containsString(output, "UDP") && containsString(output, "packets")
		summary := "RTP packets detected: NO"
		if hasRTP {
			summary = "RTP packets detected: YES"
		}

		return mcp.NewToolResultText(fmt.Sprintf("%s\nPort range: %s\n\n%s", summary, portRange, output)), nil
	}
}

func createNetworkDiagnosticsHandler(pool *ssh.Pool) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mgr := getManager(ctx, pool)
		if mgr == nil {
			return mcp.NewToolResultError("No active session"), nil
		}

		host, _ := req.RequireString("host")
		pingCount := req.GetInt("ping_count", 3)
		doTraceroute := req.GetBool("traceroute", true)
		timeout := req.GetInt("timeout", 15)
		target := req.GetString("target", "primary")

		// Default ports
		ports := []int{5060, 5061}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("=== NETWORK DIAGNOSTICS: %s ===\n\n", host))

		// Ping
		sb.WriteString("--- PING ---\n")
		pingCmd := fmt.Sprintf("ping -c %d -W 3 %s 2>&1 || echo 'Ping failed'", pingCount, shellQuote(host))
		pingOutput, _ := mgr.Execute(ctx, pingCmd, target)
		sb.WriteString(pingOutput)
		sb.WriteString("\n\n")

		// Traceroute
		if doTraceroute {
			sb.WriteString("--- TRACEROUTE ---\n")
			traceCmd := fmt.Sprintf("timeout %ds traceroute -m 15 %s 2>&1 || tracepath %s 2>&1 || echo 'Traceroute not available'",
				timeout, shellQuote(host), shellQuote(host))
			traceOutput, _ := mgr.Execute(ctx, traceCmd, target)
			sb.WriteString(traceOutput)
			sb.WriteString("\n\n")
		}

		// TCP Port checks
		sb.WriteString("--- TCP PORT CHECKS ---\n")
		for _, port := range ports {
			checkCmd := fmt.Sprintf("timeout 3 bash -c 'echo >/dev/tcp/%s/%d' 2>&1 && echo 'Port %d: OPEN' || echo 'Port %d: CLOSED/FILTERED'",
				shellQuote(host), port, port, port)
			checkOutput, _ := mgr.Execute(ctx, checkCmd, target)
			sb.WriteString(fmt.Sprintf("%s\n", trimOutput(checkOutput)))
		}

		return mcp.NewToolResultText(sb.String()), nil
	}
}

// buildSIPFilter creates BPF filter for SIP traffic.
func buildSIPFilter(port int, protocol string) string {
	if protocol != "" {
		proto := strings.ToLower(protocol)
		switch proto {
		case "tls":
			if port == 0 {
				port = SIPTLSPort
			}
			return fmt.Sprintf("tcp port %d", port)
		case "tcp":
			if port == 0 {
				port = SIPTCPPort
			}
			return fmt.Sprintf("tcp port %d", port)
		case "udp":
			if port == 0 {
				port = SIPUDPPort
			}
			return fmt.Sprintf("udp port %d", port)
		}
	}

	if port != 0 {
		return fmt.Sprintf("udp port %d or tcp port %d", port, port)
	}

	return "udp port 5060 or tcp port 5060 or tcp port 5061"
}
