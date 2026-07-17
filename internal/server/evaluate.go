package server

import (
	"fmt"
	"sort"

	"github.com/csbsgyl/netprobe/internal/protocol"
)

func Evaluate(session *Session, request protocol.CompleteSessionRequest) protocol.CompleteSessionResponse {
	report := protocol.CompleteSessionResponse{
		Version: protocol.Version, SessionID: session.ID, PublicIP: session.PublicIP,
		Verdict: protocol.VerdictIndeterminate, MappingBehavior: "unknown", FilteringBehavior: "unknown",
	}
	observations := request.UDP.Observations
	report.UDPReachable = len(observations) > 0
	portsByTarget := make(map[string]map[int]struct{})
	alternate := false
	for _, observation := range observations {
		if portsByTarget[observation.EndpointID] == nil {
			portsByTarget[observation.EndpointID] = make(map[int]struct{})
		}
		portsByTarget[observation.EndpointID][observation.ObservedPort] = struct{}{}
		if report.PublicPort == 0 {
			report.PublicPort = observation.ObservedPort
		}
		if observation.ResponseKind == protocol.ResponseKindAlternate && observation.ResponseEndpointID != observation.EndpointID {
			alternate = true
		}
	}
	report.AlternatePortReachable = alternate
	if len(portsByTarget) >= 2 {
		sets := make([]map[int]struct{}, 0, len(portsByTarget))
		for _, set := range portsByTarget {
			sets = append(sets, set)
		}
		if intersect(sets[0], sets[1]) {
			report.MappingBehavior = "endpoint-independent-likely"
		} else {
			report.MappingBehavior = "destination-port-dependent"
			report.LegacyNAT = "symmetric-nat-likely"
		}
	}
	if alternate {
		report.FilteringBehavior = "alternate-source-port-accepted"
	}
	if !report.UDPReachable {
		report.Verdict = protocol.VerdictFail
		report.Summary = "UDP probe did not receive a response"
		report.Checks = []protocol.CheckResult{{Name: "udp", Status: protocol.CheckFail, Detail: "No authenticated UDP response was received"}}
		return report
	}
	report.Checks = append(report.Checks, protocol.CheckResult{Name: "udp", Status: protocol.CheckPass, Detail: fmt.Sprintf("Received %d responses", len(observations))})
	if len(portsByTarget) < 2 {
		report.Verdict = protocol.VerdictIndeterminate
		report.Summary = "UDP works, but both probe ports were not observed"
		report.Checks = append(report.Checks, protocol.CheckResult{Name: "mapping", Status: protocol.CheckWarn, Detail: "Insufficient observations"})
	} else if report.MappingBehavior == "destination-port-dependent" {
		report.Verdict = protocol.VerdictIndeterminate
		report.Summary = "UDP works; destination-port-dependent mapping was observed"
		report.Checks = append(report.Checks, protocol.CheckResult{Name: "mapping", Status: protocol.CheckWarn, Detail: report.MappingBehavior})
	} else {
		report.Verdict = protocol.VerdictPass
		report.Summary = "UDP is reachable and the mapping stayed stable across server ports"
		report.Checks = append(report.Checks, protocol.CheckResult{Name: "mapping", Status: protocol.CheckPass, Detail: report.MappingBehavior})
	}
	if alternate {
		report.Checks = append(report.Checks, protocol.CheckResult{Name: "alternate-port", Status: protocol.CheckPass, Detail: report.FilteringBehavior})
	} else {
		report.Checks = append(report.Checks, protocol.CheckResult{Name: "alternate-port", Status: protocol.CheckWarn, Detail: "Alternate source-port reply was not observed"})
	}
	sort.Slice(report.Checks, func(i, j int) bool { return report.Checks[i].Name < report.Checks[j].Name })
	return report
}

func intersect(left, right map[int]struct{}) bool {
	for port := range left {
		if _, ok := right[port]; ok {
			return true
		}
	}
	return false
}
