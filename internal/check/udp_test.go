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
	if len(report.Observations) < 4 {
		t.Fatalf("observation count = %d, want at least 4; errors: %v", len(report.Observations), report.Errors)
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
		if !direct[endpoint.ID] || !alternate[endpoint.ID] {
			t.Errorf("endpoint %q responses: direct=%t alternate=%t", endpoint.ID, direct[endpoint.ID], alternate[endpoint.ID])
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
