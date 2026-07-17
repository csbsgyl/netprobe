package check

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/csbsgyl/netprobe/internal/protocol"
)

func TestClientRunEndToEnd(t *testing.T) {
	udpServers := newUDPTestPair(t)
	defer udpServers.close()

	var mu sync.Mutex
	created := false
	completed := false
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		switch request.URL.Path {
		case protocol.CreateSessionPath:
			if request.Method != http.MethodPost {
				t.Errorf("create method = %s", request.Method)
			}
			var input protocol.CreateSessionRequest
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				t.Errorf("decode create request: %v", err)
			}
			if input.Version != protocol.Version || input.Client.Name != "test-client" {
				t.Errorf("unexpected create request: %+v", input)
			}
			mu.Lock()
			created = true
			mu.Unlock()
			_ = json.NewEncoder(writer).Encode(protocol.CreateSessionResponse{
				Version:      protocol.Version,
				SessionID:    "session-1",
				Token:        "token-1",
				UDPEndpoints: udpServers.endpoints(),
			})
		case protocol.CompleteSessionPath("session-1"):
			if request.Header.Get("Authorization") != "Bearer token-1" {
				t.Errorf("authorization = %q", request.Header.Get("Authorization"))
			}
			var input protocol.CompleteSessionRequest
			if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
				t.Errorf("decode completion request: %v", err)
			}
			if input.Version != protocol.Version || len(input.UDP.Observations) < 3 {
				t.Errorf("unexpected completion request: %+v", input)
			}
			mu.Lock()
			completed = true
			mu.Unlock()
			_ = json.NewEncoder(writer).Encode(protocol.CompleteSessionResponse{
				Version:                protocol.Version,
				SessionID:              "session-1",
				Verdict:                protocol.VerdictPass,
				Summary:                "UDP is reachable",
				PublicIP:               "127.0.0.1",
				UDPReachable:           true,
				AlternatePortReachable: true,
				MappingBehavior:        "endpoint-independent",
				FilteringBehavior:      "endpoint-independent",
				Checks: []protocol.CheckResult{
					{Name: "udp", Status: protocol.CheckPass},
				},
			})
		default:
			http.NotFound(writer, request)
		}
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	client, err := NewClient(server.URL,
		WithClientInfo("test-client", "test-version"),
		WithUDPTimeout(time.Second),
		WithProbeRounds(1),
	)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result, err := client.Run(ctx)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Verdict != protocol.VerdictPass {
		t.Fatalf("verdict = %q, want pass", result.Verdict)
	}
	mu.Lock()
	defer mu.Unlock()
	if !created || !completed {
		t.Fatalf("request state: created=%t completed=%t", created, completed)
	}
}

func TestNewClientValidation(t *testing.T) {
	for _, server := range []string{"", "ftp://example.com", "https://user@example.com", "https://example.com?a=b"} {
		if _, err := NewClient(server); err == nil {
			t.Errorf("NewClient(%q) succeeded, want error", server)
		}
	}
	if _, err := NewClient("probe.example.com"); err != nil {
		t.Fatalf("hostname-only server rejected: %v", err)
	}
}
