package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
	if session.PublicIP != "203.0.113.4" {
		t.Fatalf("session public IP = %q", session.PublicIP)
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

func TestSessionProtocolV2RejectsLegacyTrafficExplicitly(t *testing.T) {
	server := testServer()

	legacyPath := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", strings.NewReader(`{"version":1}`))
	legacyResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(legacyResponse, legacyPath)
	if legacyResponse.Code != http.StatusNotFound {
		t.Fatalf("legacy session path status = %d, want 404", legacyResponse.Code)
	}

	legacyVersion := httptest.NewRequest(http.MethodPost, protocol.CreateSessionPath, strings.NewReader(`{"version":1,"client":{"name":"v0.1"}}`))
	versionResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(versionResponse, legacyVersion)
	if versionResponse.Code != http.StatusBadRequest {
		t.Fatalf("legacy protocol status = %d, want 400", versionResponse.Code)
	}
	var apiError protocol.ErrorResponse
	if err := json.NewDecoder(versionResponse.Body).Decode(&apiError); err != nil {
		t.Fatalf("decode version error: %v", err)
	}
	if apiError.Error.Code != "unsupported_protocol_version" || !strings.Contains(apiError.Error.Message, "expected 2") {
		t.Fatalf("unexpected version error: %+v", apiError)
	}

	browser := httptest.NewRequest(http.MethodGet, "/api/v1/browser-check", nil)
	browserResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(browserResponse, browser)
	if browserResponse.Code != http.StatusOK {
		t.Fatalf("v1 browser-check status = %d, want 200", browserResponse.Code)
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

func TestSPAFileServer(t *testing.T) {
	webDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<main>netprobe app</main>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(webDir, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webDir, "assets", "app.js"), []byte("export {}"), 0o600); err != nil {
		t.Fatal(err)
	}

	server := testServer()
	server.config.WebDir = webDir
	server.http.Handler = server.routes()

	rootRequest := httptest.NewRequest(http.MethodGet, "http://probe.example.com/", nil)
	rootRequest.Header.Set("Accept", "text/html")
	rootResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(rootResponse, rootRequest)
	if rootResponse.Code != http.StatusOK || rootResponse.Header().Get("Cache-Control") != "no-cache" {
		t.Fatalf("root response = %d, cache = %q", rootResponse.Code, rootResponse.Header().Get("Cache-Control"))
	}

	pageRequest := httptest.NewRequest(http.MethodGet, "http://probe.example.com/results/latest", nil)
	pageRequest.Header.Set("Accept", "text/html")
	pageResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(pageResponse, pageRequest)
	if pageResponse.Code != http.StatusOK || !strings.Contains(pageResponse.Body.String(), "netprobe app") {
		t.Fatalf("SPA response = %d, %q", pageResponse.Code, pageResponse.Body.String())
	}

	assetRequest := httptest.NewRequest(http.MethodGet, "http://probe.example.com/assets/app.js", nil)
	assetResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(assetResponse, assetRequest)
	if assetResponse.Code != http.StatusOK || !strings.Contains(assetResponse.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("asset response = %d, cache = %q", assetResponse.Code, assetResponse.Header().Get("Cache-Control"))
	}

	missingRequest := httptest.NewRequest(http.MethodGet, "http://probe.example.com/assets/missing.js", nil)
	missingResponse := httptest.NewRecorder()
	server.Handler().ServeHTTP(missingResponse, missingRequest)
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("missing asset status = %d", missingResponse.Code)
	}
}
