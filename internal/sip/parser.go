// Package sip provides SIP message parsing from PCAP data.
// This is equivalent to Python's dpkt-based parsing for VoIP analysis.
// Uses pure Go (no CGO) for maximum portability.
package sip

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
)

// Message represents a parsed SIP message.
type Message struct {
	Timestamp   time.Time `json:"timestamp"`
	Time        string    `json:"time"`
	SrcIP       string    `json:"src_ip"`
	SrcPort     int       `json:"src_port"`
	DstIP       string    `json:"dst_ip"`
	DstPort     int       `json:"dst_port"`
	Transport   string    `json:"transport"`
	Type        string    `json:"type"` // "request" or "response"
	Method      string    `json:"method,omitempty"`
	StatusCode  int       `json:"status_code,omitempty"`
	Reason      string    `json:"reason,omitempty"`
	CallID      string    `json:"call_id"`
	FromUser    string    `json:"from_user,omitempty"`
	ToUser      string    `json:"to_user,omitempty"`
	FromURI     string    `json:"from_uri,omitempty"`
	ToURI       string    `json:"to_uri,omitempty"`
	CSeqNumber  int       `json:"cseq_number,omitempty"`
	CSeqMethod  string    `json:"cseq_method,omitempty"`
	Contact     string    `json:"contact,omitempty"`
	ContentType string    `json:"content_type,omitempty"`
	HasSDP      bool      `json:"has_sdp"`
	SDP         *SDP      `json:"sdp,omitempty"`
}

// SDP represents parsed SDP content.
type SDP struct {
	ConnectionAddr string       `json:"connection_addr,omitempty"`
	Media          []MediaEntry `json:"media,omitempty"`
}

// MediaEntry represents an SDP media line.
type MediaEntry struct {
	Type      string   `json:"type"`
	Port      int      `json:"port"`
	Proto     string   `json:"proto"`
	Codecs    []string `json:"codecs,omitempty"`
	Direction string   `json:"direction,omitempty"`
}

// Call represents a SIP call/dialog.
type Call struct {
	CallID       string    `json:"call_id"`
	FromUser     string    `json:"from_user"`
	ToUser       string    `json:"to_user"`
	FromURI      string    `json:"from_uri"`
	ToURI        string    `json:"to_uri"`
	StartTime    string    `json:"start_time"`
	EndTime      string    `json:"end_time"`
	MessageCount int       `json:"message_count"`
	HasSDP       bool      `json:"has_sdp"`
	FinalStatus  string    `json:"final_status"`
	ErrorCode    int       `json:"error_code,omitempty"`
	Messages     []Message `json:"messages,omitempty"`
}

// Registration represents a SIP REGISTER dialog.
type Registration struct {
	FromUser   string `json:"from_user"`
	Contact    string `json:"contact"`
	StatusCode int    `json:"status_code"`
	Status     string `json:"status"` // "success", "failed", "pending"
	Time       string `json:"time"`
}

// Stats represents aggregated SIP statistics.
type Stats struct {
	TotalPackets    int            `json:"total_packets"`
	SIPMessages     int            `json:"sip_messages"`
	UniqueCallIDs   int            `json:"unique_call_ids"`
	Methods         map[string]int `json:"methods"`
	ResponseCodes   map[int]int    `json:"response_codes"`
	SuccessfulRegs  int            `json:"successful_regs"`
	FailedRegs      int            `json:"failed_regs"`
	SuccessfulCalls int            `json:"successful_calls"`
	FailedCalls     int            `json:"failed_calls"`
}

// ParseResult contains all parsed data from a PCAP.
type ParseResult struct {
	Messages      []Message      `json:"messages"`
	Calls         []Call         `json:"calls"`
	Registrations []Registration `json:"registrations"`
	Stats         Stats          `json:"stats"`
	Error         string         `json:"error,omitempty"`
}

// SIP methods that identify a SIP request.
var sipMethods = []string{
	"INVITE", "REGISTER", "ACK", "BYE", "CANCEL", "OPTIONS",
	"PRACK", "UPDATE", "MESSAGE", "SUBSCRIBE", "NOTIFY",
	"REFER", "PUBLISH", "INFO",
}

// IsSIPPayload checks if payload looks like SIP.
func IsSIPPayload(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	if bytes.HasPrefix(data, []byte("SIP/2.0")) {
		return true
	}
	for _, method := range sipMethods {
		if bytes.HasPrefix(data, []byte(method+" ")) {
			return true
		}
	}
	return false
}

// ParsePCAPBase64 parses a base64-encoded PCAP file.
func ParsePCAPBase64(b64Data string) (*ParseResult, error) {
	// Remove whitespace from base64
	b64Data = strings.ReplaceAll(b64Data, "\n", "")
	b64Data = strings.ReplaceAll(b64Data, "\r", "")
	b64Data = strings.ReplaceAll(b64Data, " ", "")

	data, err := base64.StdEncoding.DecodeString(b64Data)
	if err != nil {
		return nil, fmt.Errorf("base64 decode failed: %w", err)
	}

	return ParsePCAPBytes(data)
}

// ParsePCAPBytes parses raw PCAP bytes using pure Go.
func ParsePCAPBytes(data []byte) (*ParseResult, error) {
	result := &ParseResult{
		Messages:      make([]Message, 0),
		Calls:         make([]Call, 0),
		Registrations: make([]Registration, 0),
		Stats: Stats{
			Methods:       make(map[string]int),
			ResponseCodes: make(map[int]int),
		},
	}

	// Use pcapgo (pure Go, no CGO required)
	reader, err := pcapgo.NewReader(bytes.NewReader(data))
	if err != nil {
		// Fallback to manual parsing if pcap parsing fails
		return parseFromStrings(data), nil
	}

	linkType := reader.LinkType()

	for {
		packetData, ci, err := reader.ReadPacketData()
		if err != nil {
			break // EOF or error
		}

		result.Stats.TotalPackets++

		packet := gopacket.NewPacket(packetData, linkType, gopacket.Default)
		msg := extractSIPFromPacket(packet, ci.Timestamp)
		if msg != nil {
			result.Messages = append(result.Messages, *msg)
		}
	}

	// Aggregate into calls and registrations
	result.aggregateCalls()
	result.aggregateRegistrations()
	result.computeStats()

	return result, nil
}

// parseFromStrings does basic string-based extraction as fallback.
func parseFromStrings(data []byte) *ParseResult {
	result := &ParseResult{
		Messages:      make([]Message, 0),
		Calls:         make([]Call, 0),
		Registrations: make([]Registration, 0),
		Stats: Stats{
			Methods:       make(map[string]int),
			ResponseCodes: make(map[int]int),
		},
	}

	content := string(data)
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		for _, method := range sipMethods {
			if strings.HasPrefix(line, method+" ") {
				result.Stats.Methods[method]++
				result.Stats.SIPMessages++
			}
		}
		if strings.HasPrefix(line, "SIP/2.0 ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				if code, err := strconv.Atoi(parts[1]); err == nil {
					result.Stats.ResponseCodes[code]++
					result.Stats.SIPMessages++
				}
			}
		}
	}

	return result
}

// extractSIPFromPacket extracts SIP message from a network packet.
func extractSIPFromPacket(packet gopacket.Packet, ts time.Time) *Message {
	// Get IP layer
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		ipLayer = packet.Layer(layers.LayerTypeIPv6)
	}
	if ipLayer == nil {
		return nil
	}

	var srcIP, dstIP string
	if ip4, ok := ipLayer.(*layers.IPv4); ok {
		srcIP = ip4.SrcIP.String()
		dstIP = ip4.DstIP.String()
	} else if ip6, ok := ipLayer.(*layers.IPv6); ok {
		srcIP = ip6.SrcIP.String()
		dstIP = ip6.DstIP.String()
	}

	var srcPort, dstPort int
	var transport string
	var payload []byte

	// Check UDP
	if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
		udp := udpLayer.(*layers.UDP)
		srcPort = int(udp.SrcPort)
		dstPort = int(udp.DstPort)
		transport = "udp"
		if appLayer := packet.ApplicationLayer(); appLayer != nil {
			payload = appLayer.Payload()
		}
	}

	// Check TCP
	if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		tcp := tcpLayer.(*layers.TCP)
		srcPort = int(tcp.SrcPort)
		dstPort = int(tcp.DstPort)
		transport = "tcp"
		if appLayer := packet.ApplicationLayer(); appLayer != nil {
			payload = appLayer.Payload()
		}
	}

	if payload == nil || !IsSIPPayload(payload) {
		return nil
	}

	return parseSIPMessage(payload, ts, srcIP, dstIP, srcPort, dstPort, transport)
}

// parseSIPMessage parses raw SIP message bytes.
func parseSIPMessage(data []byte, ts time.Time, srcIP, dstIP string, srcPort, dstPort int, transport string) *Message {
	text := string(data)
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return nil
	}

	startLine := strings.TrimSpace(lines[0])
	if startLine == "" {
		return nil
	}

	msg := &Message{
		Timestamp: ts,
		Time:      ts.UTC().Format(time.RFC3339),
		SrcIP:     srcIP,
		SrcPort:   srcPort,
		DstIP:     dstIP,
		DstPort:   dstPort,
		Transport: transport,
	}

	// Parse headers
	headers := parseHeaders(text)
	msg.CallID = getHeader(headers, "call-id")
	msg.Contact = getHeader(headers, "contact")
	msg.ContentType = getHeader(headers, "content-type")

	// Parse From/To
	fromHeader := getHeader(headers, "from")
	toHeader := getHeader(headers, "to")
	msg.FromURI = extractSIPURI(fromHeader)
	msg.ToURI = extractSIPURI(toHeader)
	msg.FromUser = extractUserFromURI(msg.FromURI)
	msg.ToUser = extractUserFromURI(msg.ToURI)

	// Parse CSeq
	cseq := getHeader(headers, "cseq")
	if cseq != "" {
		parts := strings.Fields(cseq)
		if len(parts) >= 1 {
			msg.CSeqNumber, _ = strconv.Atoi(parts[0])
		}
		if len(parts) >= 2 {
			msg.CSeqMethod = parts[1]
		}
	}

	// Determine if request or response
	if strings.HasPrefix(startLine, "SIP/2.0") {
		msg.Type = "response"
		parts := strings.Fields(startLine)
		if len(parts) >= 2 {
			msg.StatusCode, _ = strconv.Atoi(parts[1])
		}
		if len(parts) >= 3 {
			msg.Reason = strings.Join(parts[2:], " ")
		}
	} else {
		msg.Type = "request"
		parts := strings.Fields(startLine)
		if len(parts) >= 1 {
			msg.Method = parts[0]
		}
	}

	// Check for SDP
	if msg.ContentType != "" && strings.Contains(strings.ToLower(msg.ContentType), "application/sdp") {
		msg.HasSDP = true
		bodyStart := strings.Index(text, "\r\n\r\n")
		if bodyStart == -1 {
			bodyStart = strings.Index(text, "\n\n")
		}
		if bodyStart != -1 {
			msg.SDP = parseSDP(text[bodyStart:])
		}
	}

	return msg
}

// parseHeaders extracts headers from SIP message.
func parseHeaders(text string) map[string]string {
	headers := make(map[string]string)
	lines := strings.Split(text, "\n")

	for i := 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			break // End of headers
		}
		colonIdx := strings.Index(line, ":")
		if colonIdx == -1 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:colonIdx]))
		value := strings.TrimSpace(line[colonIdx+1:])
		headers[key] = value
	}

	return headers
}

// getHeader retrieves a header value.
func getHeader(headers map[string]string, name string) string {
	return headers[strings.ToLower(name)]
}

// extractSIPURI extracts SIP URI from header value.
func extractSIPURI(value string) string {
	if value == "" {
		return ""
	}
	re := regexp.MustCompile(`sips?:[^>;\s]+`)
	match := re.FindString(strings.ToLower(value))
	return match
}

// extractUserFromURI extracts user part from SIP URI.
func extractUserFromURI(uri string) string {
	if uri == "" {
		return ""
	}
	if strings.HasPrefix(uri, "sips:") {
		uri = uri[5:]
	} else if strings.HasPrefix(uri, "sip:") {
		uri = uri[4:]
	}
	if atIdx := strings.Index(uri, "@"); atIdx != -1 {
		return uri[:atIdx]
	}
	return ""
}

// parseSDP parses SDP content.
func parseSDP(body string) *SDP {
	sdp := &SDP{
		Media: make([]MediaEntry, 0),
	}

	lines := strings.Split(body, "\n")
	var currentMedia *MediaEntry
	rtpmap := make(map[string]string)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "c=") {
			parts := strings.Fields(line[2:])
			if len(parts) >= 3 {
				sdp.ConnectionAddr = parts[2]
			}
		} else if strings.HasPrefix(line, "m=") {
			parts := strings.Fields(line[2:])
			if len(parts) >= 3 {
				port, _ := strconv.Atoi(parts[1])
				currentMedia = &MediaEntry{
					Type:   parts[0],
					Port:   port,
					Proto:  parts[2],
					Codecs: make([]string, 0),
				}
				sdp.Media = append(sdp.Media, *currentMedia)
			}
		} else if strings.HasPrefix(line, "a=rtpmap:") {
			value := line[9:]
			if spaceIdx := strings.Index(value, " "); spaceIdx != -1 {
				payloadType := value[:spaceIdx]
				codec := value[spaceIdx+1:]
				rtpmap[payloadType] = codec
			}
		} else if line == "a=sendrecv" || line == "a=sendonly" || line == "a=recvonly" {
			if len(sdp.Media) > 0 {
				sdp.Media[len(sdp.Media)-1].Direction = strings.TrimPrefix(line, "a=")
			}
		}
	}

	// Associate codecs with media
	for i := range sdp.Media {
		for _, codec := range rtpmap {
			sdp.Media[i].Codecs = append(sdp.Media[i].Codecs, codec)
		}
	}

	return sdp
}

// aggregateCalls groups messages into calls by Call-ID.
func (r *ParseResult) aggregateCalls() {
	callMap := make(map[string][]Message)
	for _, msg := range r.Messages {
		if msg.CallID == "" {
			continue
		}
		callMap[msg.CallID] = append(callMap[msg.CallID], msg)
	}

	for callID, msgs := range callMap {
		// Skip REGISTER dialogs  
		isRegister := false
		for _, m := range msgs {
			if m.Method == "REGISTER" || m.CSeqMethod == "REGISTER" {
				isRegister = true
				break
			}
		}
		if isRegister {
			continue
		}

		call := Call{
			CallID:       callID,
			MessageCount: len(msgs),
			Messages:     msgs,
		}

		if len(msgs) > 0 {
			call.StartTime = msgs[0].Time
			call.EndTime = msgs[len(msgs)-1].Time
			for _, m := range msgs {
				if call.FromUser == "" && m.FromUser != "" {
					call.FromUser = m.FromUser
				}
				if call.ToUser == "" && m.ToUser != "" {
					call.ToUser = m.ToUser
				}
				if call.FromURI == "" && m.FromURI != "" {
					call.FromURI = m.FromURI
				}
				if call.ToURI == "" && m.ToURI != "" {
					call.ToURI = m.ToURI
				}
				if m.HasSDP {
					call.HasSDP = true
				}
			}
		}

		// Determine final status
		var finalCode int
		for _, m := range msgs {
			if m.Type == "response" && m.StatusCode >= 200 {
				finalCode = m.StatusCode
			}
		}
		if finalCode >= 200 && finalCode < 300 {
			call.FinalStatus = "success"
		} else if finalCode >= 300 {
			call.FinalStatus = "failed"
			call.ErrorCode = finalCode
		} else {
			call.FinalStatus = "unknown"
		}

		r.Calls = append(r.Calls, call)
	}
}

// aggregateRegistrations extracts REGISTER dialogs.
func (r *ParseResult) aggregateRegistrations() {
	regMap := make(map[string]*Registration)

	for _, msg := range r.Messages {
		if msg.Method == "REGISTER" {
			key := msg.FromUser + "@" + msg.CallID
			regMap[key] = &Registration{
				FromUser: msg.FromUser,
				Contact:  msg.Contact,
				Time:     msg.Time,
				Status:   "pending",
			}
		}
		if msg.CSeqMethod == "REGISTER" && msg.Type == "response" {
			key := msg.ToUser + "@" + msg.CallID
			if reg, ok := regMap[key]; ok {
				reg.StatusCode = msg.StatusCode
				if msg.StatusCode >= 200 && msg.StatusCode < 300 {
					reg.Status = "success"
				} else {
					reg.Status = "failed"
				}
			}
		}
	}

	for _, reg := range regMap {
		r.Registrations = append(r.Registrations, *reg)
	}
}

// computeStats computes aggregate statistics.
func (r *ParseResult) computeStats() {
	r.Stats.SIPMessages = len(r.Messages)

	callIDs := make(map[string]bool)
	for _, msg := range r.Messages {
		if msg.CallID != "" {
			callIDs[msg.CallID] = true
		}
		if msg.Method != "" {
			r.Stats.Methods[msg.Method]++
		}
		if msg.StatusCode > 0 {
			r.Stats.ResponseCodes[msg.StatusCode]++
		}
	}
	r.Stats.UniqueCallIDs = len(callIDs)

	for _, call := range r.Calls {
		if call.FinalStatus == "success" {
			r.Stats.SuccessfulCalls++
		} else if call.FinalStatus == "failed" {
			r.Stats.FailedCalls++
		}
	}

	for _, reg := range r.Registrations {
		if reg.Status == "success" {
			r.Stats.SuccessfulRegs++
		} else if reg.Status == "failed" {
			r.Stats.FailedRegs++
		}
	}
}
