// Package protocol contains the versioned wire types shared by the netprobe
// client and server.
package protocol

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	// Version is the current HTTP and UDP protocol version.
	Version = 1

	CreateSessionPath = "/api/v1/sessions"

	PacketTypeProbe       = "probe"
	PacketTypeObservation = "observation"

	ResponseKindDirect    = "direct"
	ResponseKindAlternate = "alternate"

	VerdictPass          = "pass"
	VerdictFail          = "fail"
	VerdictIndeterminate = "indeterminate"

	CheckPass = "pass"
	CheckFail = "fail"
	CheckWarn = "warn"
)

// CompleteSessionPath returns the completion endpoint for a session.
func CompleteSessionPath(sessionID string) string {
	return CreateSessionPath + "/" + sessionID + "/complete"
}

type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

type CreateSessionRequest struct {
	Version int        `json:"version"`
	Client  ClientInfo `json:"client"`
}

type UDPEndpoint struct {
	ID   string `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

type CreateSessionResponse struct {
	Version      int           `json:"version"`
	SessionID    string        `json:"session_id"`
	Token        string        `json:"token"`
	PublicIP     string        `json:"public_ip,omitempty"`
	ExpiresAt    time.Time     `json:"expires_at,omitempty"`
	UDPEndpoints []UDPEndpoint `json:"udp_endpoints"`
}

// ProbePacket is sent to each advertised UDP endpoint from the same local
// socket. The token authenticates the datagram but is never echoed back.
type ProbePacket struct {
	Version          int    `json:"version"`
	Type             string `json:"type"`
	SessionID        string `json:"session_id"`
	Token            string `json:"token"`
	EndpointID       string `json:"endpoint_id"`
	ProbeID          string `json:"probe_id"`
	Sequence         uint64 `json:"sequence"`
	SentAtUnixNano   int64  `json:"sent_at_unix_nano"`
	RequestAlternate bool   `json:"request_alternate"`
}

// ObservationPacket is returned once from the receiving endpoint and, when
// requested, once from another endpoint. SentAtUnixNano is copied verbatim
// from the probe so the client can reject mismatched responses.
type ObservationPacket struct {
	Version            int    `json:"version"`
	Type               string `json:"type"`
	SessionID          string `json:"session_id"`
	EndpointID         string `json:"endpoint_id"`
	ResponseEndpointID string `json:"response_endpoint_id"`
	ResponseKind       string `json:"response_kind"`
	ProbeID            string `json:"probe_id"`
	Sequence           uint64 `json:"sequence"`
	SentAtUnixNano     int64  `json:"sent_at_unix_nano"`
	ObservedIP         string `json:"observed_ip"`
	ObservedPort       int    `json:"observed_port"`
	Proof              string `json:"proof"`
}

type UDPAttempt struct {
	EndpointID     string `json:"endpoint_id"`
	ProbeID        string `json:"probe_id"`
	Sequence       uint64 `json:"sequence"`
	SentAtUnixNano int64  `json:"sent_at_unix_nano"`
	AlternateAsked bool   `json:"alternate_asked"`
}

type UDPObservation struct {
	EndpointID         string  `json:"endpoint_id"`
	ResponseEndpointID string  `json:"response_endpoint_id"`
	ResponseKind       string  `json:"response_kind"`
	ProbeID            string  `json:"probe_id"`
	Sequence           uint64  `json:"sequence"`
	ObservedIP         string  `json:"observed_ip"`
	ObservedPort       int     `json:"observed_port"`
	RTTMilliseconds    float64 `json:"rtt_ms"`
	Proof              string  `json:"proof"`
}

type UDPReport struct {
	Attempts     []UDPAttempt     `json:"attempts"`
	Observations []UDPObservation `json:"observations"`
	Errors       []string         `json:"errors,omitempty"`
	DurationMS   int64            `json:"duration_ms"`
}

type CompleteSessionRequest struct {
	Version int       `json:"version"`
	UDP     UDPReport `json:"udp"`
}

type CheckResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail,omitempty"`
}

// CompleteSessionResponse is the server's authoritative assessment. Modern
// mapping/filtering behavior names should be preferred over legacy NAT labels.
type CompleteSessionResponse struct {
	Version                int           `json:"version"`
	SessionID              string        `json:"session_id"`
	Verdict                string        `json:"verdict"`
	Summary                string        `json:"summary,omitempty"`
	PublicIP               string        `json:"public_ip,omitempty"`
	PublicPort             int           `json:"public_port,omitempty"`
	UDPReachable           bool          `json:"udp_reachable"`
	AlternatePortReachable bool          `json:"alternate_port_reachable"`
	MappingBehavior        string        `json:"mapping_behavior,omitempty"`
	FilteringBehavior      string        `json:"filtering_behavior,omitempty"`
	LegacyNAT              string        `json:"legacy_nat,omitempty"`
	Checks                 []CheckResult `json:"checks,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorResponse struct {
	Error APIError `json:"error"`
}

func (r CreateSessionResponse) Validate() error {
	if r.Version != Version {
		return fmt.Errorf("unsupported protocol version %d", r.Version)
	}
	if strings.TrimSpace(r.SessionID) == "" {
		return errors.New("session_id is empty")
	}
	if strings.TrimSpace(r.Token) == "" {
		return errors.New("token is empty")
	}
	if r.PublicIP != "" && net.ParseIP(strings.Trim(r.PublicIP, "[]")) == nil {
		return errors.New("public_ip is invalid")
	}
	if len(r.UDPEndpoints) != 2 {
		return errors.New("exactly two UDP endpoints are required")
	}
	ids := make(map[string]struct{}, len(r.UDPEndpoints))
	addresses := make(map[string]struct{}, len(r.UDPEndpoints))
	for _, endpoint := range r.UDPEndpoints {
		if strings.TrimSpace(endpoint.ID) == "" {
			return errors.New("UDP endpoint id is empty")
		}
		if strings.TrimSpace(endpoint.Host) == "" {
			return fmt.Errorf("UDP endpoint %q has an empty host", endpoint.ID)
		}
		if endpoint.Port < 1 || endpoint.Port > 65535 {
			return fmt.Errorf("UDP endpoint %q has an invalid port", endpoint.ID)
		}
		if _, ok := ids[endpoint.ID]; ok {
			return fmt.Errorf("duplicate UDP endpoint id %q", endpoint.ID)
		}
		ids[endpoint.ID] = struct{}{}
		address := net.JoinHostPort(strings.ToLower(endpoint.Host), fmt.Sprint(endpoint.Port))
		if _, ok := addresses[address]; ok {
			return fmt.Errorf("duplicate UDP endpoint address %s", address)
		}
		addresses[address] = struct{}{}
	}
	return nil
}

func (r CompleteSessionResponse) Validate(sessionID string) error {
	if r.Version != Version {
		return fmt.Errorf("unsupported protocol version %d", r.Version)
	}
	if r.SessionID == "" || r.SessionID != sessionID {
		return errors.New("completion response has a mismatched session_id")
	}
	switch r.Verdict {
	case VerdictPass, VerdictFail, VerdictIndeterminate:
	default:
		return fmt.Errorf("invalid verdict %q", r.Verdict)
	}
	for _, check := range r.Checks {
		if check.Name == "" {
			return errors.New("completion response contains an unnamed check")
		}
		switch check.Status {
		case CheckPass, CheckFail, CheckWarn:
		default:
			return fmt.Errorf("check %q has invalid status %q", check.Name, check.Status)
		}
	}
	return nil
}
