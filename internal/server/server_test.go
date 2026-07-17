package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/csbsgyl/netprobe/internal/protocol"
)

func testServer() *Server {
	return New(Config{
		HTTPAddr: ":0", PublicHost: "probe.example.com", DownloadDir: "/tmp/none",
		UDPBindHost: "127.0.0.1", UDPPorts: []int{3478, 3479},
		SessionTTL: time.Minute, SessionSecret: "test-secret",
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestCreateAndCompleteSession(t *testing.T) {
	server := testServer()
	payload, _ := json.Marshal(protocol.CreateSessionRequest{
		Version: protocol.Version, Client: protocol.ClientInfo{Name: "test", Version: "1", OS: "linux", Arch: "amd64"},
	})
	create := httptest.NewRequest(http.MethodPost, protocol.CreateSessionPath, bytes.NewReader(payload))
	create.RemoteAddr = "203.0.113.4:50000"
	created := httptest.NewRecorder()
	server.Handler().ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.Code, created.Body.String())
	}
	var session protocol.CreateSessionResponse
	if err := json.NewDecoder(created.Body).Decode(&session); err != nil || session.Validate() != nil {
		t.Fatalf("invalid session response: %+v, %v", session, err)
	}

	completePayload, _ := json.Marshal(protocol.CompleteSessionRequest{Version: protocol.Version})
	complete := httptest.NewRequest(http.MethodPost, protocol.CompleteSessionPath(session.SessionID), bytes.NewReader(completePayload))
	complete.Header.Set("Authorization", "Bearer "+session.Token)
	done := httptest.NewRecorder()
	server.Handler().ServeHTTP(done, complete)
	if done.Code != http.StatusOK {
		t.Fatalf("complete status = %d, body = %s", done.Code, done.Body.String())
	}
	var report protocol.CompleteSessionResponse
	_ = json.NewDecoder(done.Body).Decode(&report)
	if report.Verdict != protocol.VerdictFail {
		t.Fatalf("verdict = %q", report.Verdict)
	}
}

func TestCurlRootGetsPlainText(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://probe.example.com/", nil)
	request.Header.Set("User-Agent", "curl/8.0")
	response := httptest.NewRecorder()
	testServer().Handler().ServeHTTP(response, request)
	if contentType := response.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/plain") {
		t.Fatalf("content type = %q", contentType)
	}
	if !strings.Contains(response.Body.String(), "Deep test") {
		t.Fatalf("unexpected body: %s", response.Body.String())
	}
}

func TestInstallerUsesForwardedHTTPSHost(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "http://netprobe:8080/install.sh", nil)
	request.Header.Set("X-Forwarded-Proto", "https")
	request.Header.Set("X-Forwarded-Host", "check.example.com")
	response := httptest.NewRecorder()
	testServer().Handler().ServeHTTP(response, request)
	if !strings.Contains(response.Body.String(), "BASE_URL='https://check.example.com'") {
		t.Fatalf("unexpected script: %s", response.Body.String())
	}
}
