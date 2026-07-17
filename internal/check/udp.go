package check

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/csbsgyl/netprobe/internal/protocol"
)

const (
	probeInterval   = 75 * time.Millisecond
	maxDatagramSize = 4096
)

type sentProbe struct {
	endpointID string
	sequence   uint64
	sentAt     time.Time
}

// ProbeUDP sends versioned datagrams to every endpoint through one local UDP
// socket. Failures are included in the report so the completion API can still
// return a diagnostic verdict.
func ProbeUDP(ctx context.Context, session protocol.CreateSessionResponse, rounds int) (report protocol.UDPReport) {
	started := time.Now()
	report = protocol.UDPReport{
		Attempts:     make([]protocol.UDPAttempt, 0),
		Observations: make([]protocol.UDPObservation, 0),
	}
	defer func() {
		report.DurationMS = time.Since(started).Milliseconds()
	}()

	if rounds < 1 {
		rounds = 1
	}
	if len(session.UDPEndpoints) != 2 {
		report.Errors = append(report.Errors, "exactly two UDP endpoints are required")
		return report
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultUDPTimeout)
		defer cancel()
	}
	resolved, network, err := resolveEndpoints(ctx, session.UDPEndpoints)
	if err != nil {
		report.Errors = append(report.Errors, err.Error())
		return report
	}
	bind := &net.UDPAddr{Port: 0}
	if network == "udp4" {
		bind.IP = net.IPv4zero
	} else {
		bind.IP = net.IPv6unspecified
	}
	conn, err := net.ListenUDP(network, bind)
	if err != nil {
		report.Errors = append(report.Errors, "open UDP socket: "+err.Error())
		return report
	}
	defer conn.Close()

	sent := make(map[string]sentProbe, len(resolved)*rounds)
	seen := make(map[string]struct{})
	direct := make(map[string]bool)
	alternate := make(map[string]bool)
	buffer := make([]byte, maxDatagramSize)
	alternates := map[string]string{
		session.UDPEndpoints[0].ID: session.UDPEndpoints[1].ID,
		session.UDPEndpoints[1].ID: session.UDPEndpoints[0].ID,
	}
	state := udpState{
		session:    session,
		resolved:   resolved,
		sent:       sent,
		seen:       seen,
		direct:     direct,
		alternate:  alternate,
		alternates: alternates,
		buffer:     buffer,
		report:     &report,
	}

	// The filtering phase contacts only the primary endpoint. An alternate
	// response received here therefore reflects unsolicited source-port
	// filtering rather than a mapping opened by a prior probe to that port.
	primary := session.UDPEndpoints[0]
	firstDeadline := splitDeadline(ctx)
	state.runPhase(ctx, conn, primary, rounds, firstDeadline, true, func() bool {
		return direct[primary.ID] && alternate[primary.ID]
	})

	// Once the alternate endpoint has been contacted, alternate responses are
	// no longer valid filtering evidence. Only direct responses are collected
	// in this mapping phase.
	secondary := session.UDPEndpoints[1]
	state.runPhase(ctx, conn, secondary, rounds, contextDeadline(ctx), false, func() bool {
		return direct[secondary.ID]
	})
	report.Errors = appendContextError(report.Errors, ctx.Err())
	return report
}

type udpState struct {
	session    protocol.CreateSessionResponse
	resolved   map[string]*net.UDPAddr
	sent       map[string]sentProbe
	seen       map[string]struct{}
	direct     map[string]bool
	alternate  map[string]bool
	alternates map[string]string
	buffer     []byte
	report     *protocol.UDPReport
}

func (s *udpState) runPhase(ctx context.Context, conn *net.UDPConn, endpoint protocol.UDPEndpoint, rounds int, deadline time.Time, allowAlternate bool, complete func() bool) {
	nextSend := time.Now()
	sends := 0
	for ctx.Err() == nil && (sends < rounds || !complete()) && time.Now().Before(deadline) {
		now := time.Now()
		if sends < rounds && !now.Before(nextSend) {
			s.sendProbe(conn, endpoint)
			sends++
			nextSend = now.Add(probeInterval)
			continue
		}

		readDeadline := deadline
		if sends < rounds && nextSend.Before(readDeadline) {
			readDeadline = nextSend
		}
		if pollDeadline := time.Now().Add(200 * time.Millisecond); pollDeadline.Before(readDeadline) {
			readDeadline = pollDeadline
		}
		if err := conn.SetReadDeadline(readDeadline); err != nil {
			s.report.Errors = append(s.report.Errors, "set UDP deadline: "+err.Error())
			return
		}
		n, source, err := conn.ReadFromUDP(s.buffer)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			s.report.Errors = append(s.report.Errors, "read UDP response: "+err.Error())
			return
		}
		s.collect(s.buffer[:n], source, allowAlternate)
	}
}

func (s *udpState) sendProbe(conn *net.UDPConn, endpoint protocol.UDPEndpoint) {
	probeID, err := randomProbeID()
	if err != nil {
		s.report.Errors = append(s.report.Errors, "generate probe id: "+err.Error())
		return
	}
	now := time.Now()
	sequence := uint64(len(s.report.Attempts) + 1)
	attempt := protocol.UDPAttempt{
		EndpointID:     endpoint.ID,
		ProbeID:        probeID,
		Sequence:       sequence,
		SentAtUnixNano: now.UnixNano(),
		AlternateAsked: true,
	}
	s.report.Attempts = append(s.report.Attempts, attempt)
	packet := protocol.ProbePacket{
		Version:          protocol.Version,
		Type:             protocol.PacketTypeProbe,
		SessionID:        s.session.SessionID,
		Token:            s.session.Token,
		EndpointID:       endpoint.ID,
		ProbeID:          probeID,
		Sequence:         sequence,
		SentAtUnixNano:   attempt.SentAtUnixNano,
		RequestAlternate: true,
	}
	payload, err := json.Marshal(packet)
	if err != nil {
		s.report.Errors = append(s.report.Errors, fmt.Sprintf("encode probe for %s: %v", endpoint.ID, err))
		return
	}
	if _, err := conn.WriteToUDP(payload, s.resolved[endpoint.ID]); err != nil {
		s.report.Errors = append(s.report.Errors, fmt.Sprintf("send probe to %s: %v", endpoint.ID, err))
		return
	}
	s.sent[probeID] = sentProbe{endpointID: endpoint.ID, sequence: sequence, sentAt: now}
}

func (s *udpState) collect(payload []byte, source *net.UDPAddr, allowAlternate bool) {
	var observation protocol.ObservationPacket
	if err := json.Unmarshal(payload, &observation); err != nil {
		return
	}
	probe, ok := s.sent[observation.ProbeID]
	if !ok || !validObservation(s.session.SessionID, observation, probe, s.alternates) {
		return
	}
	if observation.ResponseKind == protocol.ResponseKindAlternate && !allowAlternate {
		return
	}
	expectedSource, ok := s.resolved[observation.ResponseEndpointID]
	if !ok || source.Port != expectedSource.Port || !source.IP.Equal(expectedSource.IP) {
		return
	}
	key := observation.ProbeID + "\x00" + observation.ResponseEndpointID + "\x00" + observation.ResponseKind
	if _, ok := s.seen[key]; ok {
		return
	}
	s.seen[key] = struct{}{}
	rtt := time.Since(probe.sentAt).Seconds() * 1000
	if rtt < 0 {
		rtt = 0
	}
	s.report.Observations = append(s.report.Observations, protocol.UDPObservation{
		EndpointID:         observation.EndpointID,
		ResponseEndpointID: observation.ResponseEndpointID,
		ResponseKind:       observation.ResponseKind,
		ProbeID:            observation.ProbeID,
		Sequence:           observation.Sequence,
		ObservedIP:         observation.ObservedIP,
		ObservedPort:       observation.ObservedPort,
		RTTMilliseconds:    rtt,
		Proof:              observation.Proof,
	})
	if observation.ResponseKind == protocol.ResponseKindDirect {
		s.direct[observation.EndpointID] = true
	} else {
		s.alternate[observation.EndpointID] = true
	}
}

func splitDeadline(ctx context.Context) time.Time {
	deadline := contextDeadline(ctx)
	return time.Now().Add(time.Until(deadline) / 2)
}

func contextDeadline(ctx context.Context) time.Time {
	deadline, _ := ctx.Deadline()
	return deadline
}

func resolveEndpoints(ctx context.Context, endpoints []protocol.UDPEndpoint) (map[string]*net.UDPAddr, string, error) {
	if len(endpoints) == 0 {
		return nil, "", fmt.Errorf("no UDP endpoints were provided")
	}
	allIPs := make(map[string][]net.IP, len(endpoints))
	hasIPv4 := true
	for _, endpoint := range endpoints {
		ips, err := lookupIPs(ctx, endpoint.Host)
		if err != nil {
			return nil, "", fmt.Errorf("resolve UDP endpoint %s: %w", endpoint.ID, err)
		}
		allIPs[endpoint.ID] = ips
		if firstIPv4(ips) == nil {
			hasIPv4 = false
		}
	}

	network := "udp6"
	if hasIPv4 {
		network = "udp4"
	}
	resolved := make(map[string]*net.UDPAddr, len(endpoints))
	for _, endpoint := range endpoints {
		var ip net.IP
		if network == "udp4" {
			ip = firstIPv4(allIPs[endpoint.ID])
		} else {
			ip = firstIPv6(allIPs[endpoint.ID])
		}
		if ip == nil {
			return nil, "", fmt.Errorf("UDP endpoints do not share an IP address family")
		}
		resolved[endpoint.ID] = &net.UDPAddr{IP: ip, Port: endpoint.Port}
	}
	return resolved, network, nil
}

func lookupIPs(ctx context.Context, host string) ([]net.IP, error) {
	if parsed := net.ParseIP(strings.Trim(host, "[]")); parsed != nil {
		return []net.IP{parsed}, nil
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	ips := make([]net.IP, 0, len(addresses))
	for _, address := range addresses {
		ips = append(ips, address.IP)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("host has no IP addresses")
	}
	return ips, nil
}

func firstIPv4(ips []net.IP) net.IP {
	for _, ip := range ips {
		if v4 := ip.To4(); v4 != nil {
			return v4
		}
	}
	return nil
}

func firstIPv6(ips []net.IP) net.IP {
	for _, ip := range ips {
		if ip.To4() == nil && ip.To16() != nil {
			return ip
		}
	}
	return nil
}

func validObservation(sessionID string, observation protocol.ObservationPacket, probe sentProbe, alternates map[string]string) bool {
	if observation.Version != protocol.Version || observation.Type != protocol.PacketTypeObservation {
		return false
	}
	if observation.SessionID != sessionID || observation.EndpointID != probe.endpointID {
		return false
	}
	if observation.Sequence != probe.sequence || observation.SentAtUnixNano != probe.sentAt.UnixNano() {
		return false
	}
	if net.ParseIP(observation.ObservedIP) == nil || observation.ObservedPort < 1 || observation.ObservedPort > 65535 || observation.Proof == "" {
		return false
	}
	switch observation.ResponseKind {
	case protocol.ResponseKindDirect:
		return observation.ResponseEndpointID == observation.EndpointID
	case protocol.ResponseKindAlternate:
		return observation.ResponseEndpointID == alternates[observation.EndpointID]
	default:
		return false
	}
}

func randomProbeID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func appendContextError(values []string, err error) []string {
	if err == nil || err == context.DeadlineExceeded {
		return values
	}
	return append(values, "UDP probe canceled: "+err.Error())
}
