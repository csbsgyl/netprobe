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
	readInterval    = 200 * time.Millisecond
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
	for round := 0; round < rounds; round++ {
		for _, endpoint := range session.UDPEndpoints {
			select {
			case <-ctx.Done():
				report.Errors = appendContextError(report.Errors, ctx.Err())
				return report
			default:
			}

			probeID, err := randomProbeID()
			if err != nil {
				report.Errors = append(report.Errors, "generate probe id: "+err.Error())
				return report
			}
			now := time.Now()
			sequence := uint64(len(report.Attempts) + 1)
			attempt := protocol.UDPAttempt{
				EndpointID:     endpoint.ID,
				ProbeID:        probeID,
				Sequence:       sequence,
				SentAtUnixNano: now.UnixNano(),
				AlternateAsked: true,
			}
			report.Attempts = append(report.Attempts, attempt)
			packet := protocol.ProbePacket{
				Version:          protocol.Version,
				Type:             protocol.PacketTypeProbe,
				SessionID:        session.SessionID,
				Token:            session.Token,
				EndpointID:       endpoint.ID,
				ProbeID:          probeID,
				Sequence:         sequence,
				SentAtUnixNano:   attempt.SentAtUnixNano,
				RequestAlternate: true,
			}
			payload, err := json.Marshal(packet)
			if err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("encode probe for %s: %v", endpoint.ID, err))
				continue
			}
			if _, err := conn.WriteToUDP(payload, resolved[endpoint.ID]); err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("send probe to %s: %v", endpoint.ID, err))
				continue
			}
			sent[probeID] = sentProbe{endpointID: endpoint.ID, sequence: sequence, sentAt: now}
		}
		if round+1 < rounds {
			timer := time.NewTimer(probeInterval)
			select {
			case <-ctx.Done():
				if !timer.Stop() {
					<-timer.C
				}
				report.Errors = appendContextError(report.Errors, ctx.Err())
				return report
			case <-timer.C:
			}
		}
	}

	seen := make(map[string]struct{})
	direct := make(map[string]bool)
	alternate := make(map[string]bool)
	buffer := make([]byte, maxDatagramSize)
	for {
		if allResponseKindsSeen(session.UDPEndpoints, direct, alternate) {
			return report
		}
		deadline := time.Now().Add(readInterval)
		if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
			deadline = contextDeadline
		}
		if err := conn.SetReadDeadline(deadline); err != nil {
			report.Errors = append(report.Errors, "set UDP deadline: "+err.Error())
			return report
		}
		n, source, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if ctx.Err() != nil {
				return report
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			report.Errors = append(report.Errors, "read UDP response: "+err.Error())
			return report
		}
		var observation protocol.ObservationPacket
		if err := json.Unmarshal(buffer[:n], &observation); err != nil {
			continue
		}
		probe, ok := sent[observation.ProbeID]
		if !ok || !validObservation(session.SessionID, observation, probe) {
			continue
		}
		expectedSource, ok := resolved[observation.ResponseEndpointID]
		if !ok || source.Port != expectedSource.Port || !source.IP.Equal(expectedSource.IP) {
			continue
		}
		key := observation.ProbeID + "\x00" + observation.ResponseEndpointID + "\x00" + observation.ResponseKind
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		rtt := time.Since(probe.sentAt).Seconds() * 1000
		if rtt < 0 {
			rtt = 0
		}
		report.Observations = append(report.Observations, protocol.UDPObservation{
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
		switch observation.ResponseKind {
		case protocol.ResponseKindDirect:
			direct[observation.EndpointID] = true
		case protocol.ResponseKindAlternate:
			alternate[observation.EndpointID] = true
		}
	}
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

func validObservation(sessionID string, observation protocol.ObservationPacket, probe sentProbe) bool {
	if observation.Version != protocol.Version || observation.Type != protocol.PacketTypeObservation {
		return false
	}
	if observation.SessionID != sessionID || observation.EndpointID != probe.endpointID {
		return false
	}
	if observation.Sequence != probe.sequence || observation.SentAtUnixNano != probe.sentAt.UnixNano() {
		return false
	}
	if observation.ResponseKind != protocol.ResponseKindDirect && observation.ResponseKind != protocol.ResponseKindAlternate {
		return false
	}
	if net.ParseIP(observation.ObservedIP) == nil || observation.ObservedPort < 1 || observation.ObservedPort > 65535 || observation.Proof == "" {
		return false
	}
	return observation.ResponseEndpointID != ""
}

func allResponseKindsSeen(endpoints []protocol.UDPEndpoint, direct, alternate map[string]bool) bool {
	for _, endpoint := range endpoints {
		if !direct[endpoint.ID] || !alternate[endpoint.ID] {
			return false
		}
	}
	return len(endpoints) > 0
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
