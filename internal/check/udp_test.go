package check

import (
	"context"
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/csbsgyl/netprobe/internal/protocol"
)

func TestProbeUDPUsesOneSocketAndCollectsAlternateResponses(t *testing.T) {
	servers := newUDPTestPair(t)
	defer servers.close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	report := ProbeUDP(ctx, protocol.CreateSessionResponse{
		Version:      protocol.Version,
		SessionID:    "session-1",
		Token:        "token-1",
		UDPEndpoints: servers.endpoints(),
	}, 1)

	if len(report.Attempts) != 2 {
		t.Fatalf("attempt count = %d, want 2; errors: %v", len(report.Attempts), report.Errors)
	}
	if !report.Attempts[0].AlternateAsked || report.Attempts[1].AlternateAsked {
		t.Fatalf("alternate callback requests = [%t, %t], want [true, false]",
			report.Attempts[0].AlternateAsked, report.Attempts[1].AlternateAsked)
	}
	if len(report.Observations) < 3 {
		t.Fatalf("observation count = %d, want at least 3; errors: %v", len(report.Observations), report.Errors)
	}
	ports := servers.sourcePorts()
	if len(ports) != 1 {
		t.Fatalf("observed %d client source ports, want 1: %v", len(ports), ports)
	}
	direct := make(map[string]bool)
	alternate := make(map[string]bool)
	for _, observation := range report.Observations {
		if observation.RTTMilliseconds < 0 {
			t.Fatalf("negative RTT: %f", observation.RTTMilliseconds)
		}
		switch observation.ResponseKind {
		case protocol.ResponseKindDirect:
			direct[observation.EndpointID] = true
		case protocol.ResponseKindAlternate:
			alternate[observation.EndpointID] = true
		}
	}
	for _, endpoint := range servers.endpoints() {
		if !direct[endpoint.ID] {
			t.Errorf("endpoint %q has no direct response", endpoint.ID)
		}
	}
	if !alternate[servers.ids[0]] {
		t.Error("primary endpoint has no pre-contact alternate response")
	}
	if alternate[servers.ids[1]] {
		t.Error("secondary alternate response must not count after contacting the secondary endpoint")
	}
}

func TestProbeUDPMultipleRoundsDoNotInflateRTT(t *testing.T) {
	servers := newUDPTestPair(t)
	defer servers.close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	report := ProbeUDP(ctx, protocol.CreateSessionResponse{
		Version:      protocol.Version,
		SessionID:    "session-1",
		Token:        "token-1",
		UDPEndpoints: servers.endpoints(),
	}, 3)
	if len(report.Attempts) != 6 {
		t.Fatalf("attempt count = %d, want 6; errors: %v", len(report.Attempts), report.Errors)
	}
	if len(report.Observations) < 3 {
		t.Fatalf("observation count = %d, want at least 3", len(report.Observations))
	}
	if report.Observations[0].RTTMilliseconds >= 100 {
		t.Fatalf("first response RTT = %.2fms, want less than 100ms", report.Observations[0].RTTMilliseconds)
	}
}

func TestValidObservationRequiresResponseEndpointSemantics(t *testing.T) {
	probeTime := time.Now()
	probe := sentProbe{endpointID: "primary", sequence: 1, sentAt: probeTime}
	observation := protocol.ObservationPacket{
		Version:            protocol.Version,
		Type:               protocol.PacketTypeObservation,
		SessionID:          "session-1",
		EndpointID:         "primary",
		ResponseEndpointID: "primary",
		ResponseKind:       protocol.ResponseKindDirect,
		ProbeID:            "probe-1",
		Sequence:           1,
		SentAtUnixNano:     probeTime.UnixNano(),
		ObservedIP:         "203.0.113.1",
		ObservedPort:       12345,
		Proof:              "proof-1",
	}
	alternates := map[string]string{"primary": "alternate", "alternate": "primary"}
	if !validObservation("session-1", observation, probe, alternates) {
		t.Fatal("valid direct observation rejected")
	}
	observation.ResponseEndpointID = "alternate"
	if validObservation("session-1", observation, probe, alternates) {
		t.Fatal("direct observation from alternate endpoint accepted")
	}
	observation.ResponseKind = protocol.ResponseKindAlternate
	if !validObservation("session-1", observation, probe, alternates) {
		t.Fatal("valid alternate observation rejected")
	}
	observation.ResponseEndpointID = "primary"
	if validObservation("session-1", observation, probe, alternates) {
		t.Fatal("alternate observation from receiving endpoint accepted")
	}
}

func TestValidObservationRejectsIncompatibleVersionAndMissingProof(t *testing.T) {
	probeTime := time.Now()
	probe := sentProbe{endpointID: "primary", sequence: 1, sentAt: probeTime}
	observation := protocol.ObservationPacket{
		Version:            protocol.Version,
		Type:               protocol.PacketTypeObservation,
		SessionID:          "session-1",
		EndpointID:         "primary",
		ResponseEndpointID: "primary",
		ResponseKind:       protocol.ResponseKindDirect,
		ProbeID:            "probe-1",
		Sequence:           1,
		SentAtUnixNano:     probeTime.UnixNano(),
		ObservedIP:         "203.0.113.1",
		ObservedPort:       12345,
	}
	if validObservation("session-1", observation, probe, map[string]string{"primary": "alternate"}) {
		t.Fatal("proof-less protocol v2 observation was accepted")
	}
	observation.Version = protocol.Version - 1
	observation.Proof = "legacy-proof"
	if validObservation("session-1", observation, probe, map[string]string{"primary": "alternate"}) {
		t.Fatal("protocol v1 observation was accepted by the v2 client")
	}
}

func TestResolveEndpointsPrefersSessionFamilyAndCachesHost(t *testing.T) {
	endpoints := []protocol.UDPEndpoint{
		{ID: "primary", Host: "probe.example.com", Port: 3478},
		{ID: "alternate", Host: "PROBE.EXAMPLE.COM", Port: 3479},
	}
	lookups := 0
	lookup := func(context.Context, string) ([]net.IP, error) {
		lookups++
		return []net.IP{net.ParseIP("192.0.2.10"), net.ParseIP("2001:db8::10")}, nil
	}
	resolved, network, err := resolveEndpointsWithLookup(context.Background(), endpoints, "2001:db8::20", lookup)
	if err != nil {
		t.Fatalf("resolveEndpointsWithLookup returned error: %v", err)
	}
	if lookups != 1 {
		t.Fatalf("DNS lookup count = %d, want 1", lookups)
	}
	if network != "udp6" {
		t.Fatalf("network = %q, want udp6", network)
	}
	for _, endpoint := range endpoints {
		if got := resolved[endpoint.ID].IP.String(); got != "2001:db8::10" {
			t.Fatalf("endpoint %s resolved to %s", endpoint.ID, got)
		}
	}
}

type udpTestPair struct {
	t       *testing.T
	ctx     context.Context
	cancel  context.CancelFunc
	sockets []*net.UDPConn
	ids     []string
	wg      sync.WaitGroup
	mu      sync.Mutex
	ports   map[int]struct{}
}

func newUDPTestPair(t *testing.T) *udpTestPair {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	pair := &udpTestPair{
		t:      t,
		ctx:    ctx,
		cancel: cancel,
		ids:    []string{"primary", "alternate"},
		ports:  make(map[int]struct{}),
	}
	for range pair.ids {
		socket, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
		if err != nil {
			pair.close()
			t.Fatalf("listen UDP: %v", err)
		}
		pair.sockets = append(pair.sockets, socket)
	}
	for index := range pair.sockets {
		pair.wg.Add(1)
		go pair.serve(index)
	}
	return pair
}

func (p *udpTestPair) endpoints() []protocol.UDPEndpoint {
	endpoints := make([]protocol.UDPEndpoint, len(p.sockets))
	for index, socket := range p.sockets {
		endpoints[index] = protocol.UDPEndpoint{
			ID:   p.ids[index],
			Host: "127.0.0.1",
			Port: socket.LocalAddr().(*net.UDPAddr).Port,
		}
	}
	return endpoints
}

func (p *udpTestPair) sourcePorts() map[int]struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make(map[int]struct{}, len(p.ports))
	for port := range p.ports {
		result[port] = struct{}{}
	}
	return result
}

func (p *udpTestPair) serve(index int) {
	defer p.wg.Done()
	socket := p.sockets[index]
	buffer := make([]byte, maxDatagramSize)
	for {
		if err := socket.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			return
		}
		n, clientAddress, err := socket.ReadFromUDP(buffer)
		if err != nil {
			if p.ctx.Err() != nil {
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}
		var probe protocol.ProbePacket
		if err := json.Unmarshal(buffer[:n], &probe); err != nil {
			continue
		}
		p.mu.Lock()
		p.ports[clientAddress.Port] = struct{}{}
		p.mu.Unlock()
		p.respond(index, index, protocol.ResponseKindDirect, probe, clientAddress)
		if probe.RequestAlternate {
			p.respond(index, 1-index, protocol.ResponseKindAlternate, probe, clientAddress)
		}
	}
}

func (p *udpTestPair) respond(receivedAt, sentFrom int, kind string, probe protocol.ProbePacket, client *net.UDPAddr) {
	packet := protocol.ObservationPacket{
		Version:            protocol.Version,
		Type:               protocol.PacketTypeObservation,
		SessionID:          probe.SessionID,
		EndpointID:         p.ids[receivedAt],
		ResponseEndpointID: p.ids[sentFrom],
		ResponseKind:       kind,
		ProbeID:            probe.ProbeID,
		Sequence:           probe.Sequence,
		SentAtUnixNano:     probe.SentAtUnixNano,
		ObservedIP:         client.IP.String(),
		ObservedPort:       client.Port,
		Proof:              probe.ProbeID + "-proof-" + kind,
	}
	payload, err := json.Marshal(packet)
	if err != nil {
		return
	}
	_, _ = p.sockets[sentFrom].WriteToUDP(payload, client)
}

func (p *udpTestPair) close() {
	if p.cancel != nil {
		p.cancel()
	}
	for _, socket := range p.sockets {
		_ = socket.Close()
	}
	p.wg.Wait()
}
