package server

import (
	"testing"
	"time"

	"github.com/csbsgyl/netprobe/internal/protocol"
)

func TestEvaluateStableMappingPasses(t *testing.T) {
	session := &Session{ID: "test", PublicIP: "203.0.113.8", ExpiresAt: time.Now().Add(time.Minute)}
	request := protocol.CompleteSessionRequest{Version: protocol.Version, UDP: protocol.UDPReport{
		Observations: []protocol.UDPObservation{
			{EndpointID: "primary", ResponseEndpointID: "primary", ResponseKind: protocol.ResponseKindDirect, ObservedPort: 42000, Proof: "one"},
			{EndpointID: "primary", ResponseEndpointID: "alternate", ResponseKind: protocol.ResponseKindAlternate, ObservedPort: 42000, Proof: "two"},
			{EndpointID: "alternate", ResponseEndpointID: "alternate", ResponseKind: protocol.ResponseKindDirect, ObservedPort: 42000, Proof: "three"},
		},
	}}
	session.ServerProbes = append(session.ServerProbes, request.UDP.Observations...)
	report := Evaluate(session, request)
	if report.Verdict != protocol.VerdictPass {
		t.Fatalf("verdict = %q, want pass", report.Verdict)
	}
	if report.MappingBehavior != "endpoint-independent-likely" || !report.AlternatePortReachable {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestEvaluateNoUDPFail(t *testing.T) {
	report := Evaluate(&Session{ID: "test"}, protocol.CompleteSessionRequest{Version: protocol.Version})
	if report.Verdict != protocol.VerdictFail || report.UDPReachable {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestEvaluateRejectsMissingOrMismatchedProof(t *testing.T) {
	recorded := protocol.UDPObservation{
		EndpointID: "primary", ResponseEndpointID: "primary", ResponseKind: protocol.ResponseKindDirect,
		ProbeID: "probe-1", Sequence: 1, ObservedIP: "203.0.113.8", ObservedPort: 42000, Proof: "server-proof",
	}
	session := &Session{ID: "test", PublicIP: "203.0.113.8", ServerProbes: []protocol.UDPObservation{recorded}}
	for _, proof := range []string{"", "different-proof"} {
		candidate := recorded
		candidate.Proof = proof
		report := Evaluate(session, protocol.CompleteSessionRequest{Version: protocol.Version, UDP: protocol.UDPReport{
			Observations: []protocol.UDPObservation{candidate},
		}})
		if report.Verdict != protocol.VerdictFail || report.UDPReachable {
			t.Fatalf("proof %q produced an authenticated UDP result: %+v", proof, report)
		}
	}
}
