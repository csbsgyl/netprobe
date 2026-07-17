package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/csbsgyl/netprobe/internal/protocol"
)

type Server struct {
	config Config
	store  *SessionStore
	logger *slog.Logger
	http   *http.Server
	udp    []*net.UDPConn
	mu     sync.RWMutex
}

func New(config Config, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	server := &Server{config: config, store: NewSessionStore(config.SessionSecret, config.SessionTTL), logger: logger}
	server.http = &http.Server{
		Addr: config.HTTPAddr, Handler: server.routes(), ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	return server
}

func (s *Server) Run(ctx context.Context) error {
	if err := s.startUDP(ctx); err != nil {
		return err
	}
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("HTTP server listening", "address", s.config.HTTPAddr)
		if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()
	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	for _, conn := range s.udp {
		_ = conn.Close()
	}
	s.udp = nil
	s.mu.Unlock()
	return s.http.Shutdown(ctx)
}

func (s *Server) Handler() http.Handler { return s.http.Handler }

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.health)
	mux.HandleFunc("/api/v1/browser-check", s.browserCheck)
	// Protocol v1 did not authenticate UDP observations with response proofs.
	// Keep its retired paths out of the SPA fallback so old clients fail with a
	// clear 404 instead of receiving an unrelated HTML or method response.
	mux.Handle("/api/v1/sessions", http.NotFoundHandler())
	mux.Handle("/api/v1/sessions/", http.NotFoundHandler())
	mux.HandleFunc(protocol.CreateSessionPath, s.sessions)
	mux.HandleFunc(protocol.CreateSessionPath+"/", s.sessionByID)
	mux.HandleFunc("/install.sh", s.installShell)
	mux.HandleFunc("/install.ps1", s.installPowerShell)
	mux.Handle("/downloads/", http.StripPrefix("/downloads/", http.FileServer(http.Dir(s.config.DownloadDir))))
	mux.Handle("/", s.rootOrAssets(spaFileServer(s.config.WebDir)))
	return s.logging(mux)
}

func spaFileServer(root string) http.Handler {
	if strings.TrimSpace(root) == "" {
		return http.NotFoundHandler()
	}
	files := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		cleanPath := filepath.Clean("/" + request.URL.Path)
		name := filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(cleanPath, "/")))
		if info, err := os.Stat(name); err == nil && (!info.IsDir() || cleanPath == "/") {
			if cleanPath == "/" || filepath.Base(cleanPath) == "index.html" {
				w.Header().Set("Cache-Control", "no-cache")
			} else if strings.HasPrefix(cleanPath, "/assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			files.ServeHTTP(w, request)
			return
		}

		if !strings.Contains(request.Header.Get("Accept"), "text/html") {
			http.NotFound(w, request)
			return
		}
		index := filepath.Join(root, "index.html")
		if _, err := os.Stat(index); err != nil {
			http.NotFound(w, request)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFile(w, request, index)
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "udp_ports": s.config.UDPPorts})
}

func (s *Server) browserCheck(w http.ResponseWriter, request *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"public_ip": clientIP(request), "https": request.TLS != nil || forwardedProto(request) == "https",
		"method": "browser-https-stun", "verdict": "quick-check-ok", "tested_at": time.Now().UTC(),
		"stun_url": "stun:" + net.JoinHostPort(s.config.PublicHost, strconv.Itoa(s.config.UDPPorts[0])),
	})
}

func (s *Server) sessions(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var input protocol.CreateSessionRequest
	if err := decodeJSON(request, &input); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	if input.Version != protocol.Version {
		writeProtocolVersionError(w, input.Version)
		return
	}
	session, err := s.store.Create(clientIP(request), input.Client)
	if err != nil {
		http.Error(w, "could not create session", http.StatusInternalServerError)
		return
	}
	endpoints := make([]protocol.UDPEndpoint, 0, len(s.config.UDPPorts))
	for index, port := range s.config.UDPPorts {
		endpoints = append(endpoints, protocol.UDPEndpoint{ID: endpointID(index), Host: s.config.PublicHost, Port: port})
	}
	writeJSON(w, http.StatusCreated, protocol.CreateSessionResponse{
		Version: protocol.Version, SessionID: session.ID, Token: session.Token,
		PublicIP: session.PublicIP, ExpiresAt: session.ExpiresAt, UDPEndpoints: endpoints,
	})
}

func (s *Server) sessionByID(w http.ResponseWriter, request *http.Request) {
	path := strings.TrimPrefix(request.URL.Path, protocol.CreateSessionPath+"/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 || parts[1] != "complete" || request.Method != http.MethodPost {
		http.NotFound(w, request)
		return
	}
	session, ok := s.store.Valid(parts[0], bearerToken(request))
	if !ok {
		http.Error(w, "invalid or expired session", http.StatusUnauthorized)
		return
	}
	var input protocol.CompleteSessionRequest
	if err := decodeJSON(request, &input); err != nil {
		http.Error(w, "invalid result", http.StatusBadRequest)
		return
	}
	if input.Version != protocol.Version {
		writeProtocolVersionError(w, input.Version)
		return
	}
	if latest, ok := s.store.Snapshot(parts[0], bearerToken(request)); ok {
		session = latest
	}
	writeJSON(w, http.StatusOK, Evaluate(session, input))
}

func (s *Server) startUDP(ctx context.Context) error {
	for index, port := range s.config.UDPPorts {
		address := net.JoinHostPort(s.config.UDPBindHost, strconv.Itoa(port))
		resolved, err := net.ResolveUDPAddr("udp", address)
		if err != nil {
			return err
		}
		conn, err := net.ListenUDP("udp", resolved)
		if err != nil {
			return fmt.Errorf("listen UDP %s: %w", address, err)
		}
		s.mu.Lock()
		s.udp = append(s.udp, conn)
		s.mu.Unlock()
		go s.serveUDP(ctx, index, conn)
		s.logger.Info("UDP probe listening", "address", address, "id", endpointID(index))
	}
	return nil
}

func (s *Server) serveUDP(ctx context.Context, index int, conn *net.UDPConn) {
	buffer := make([]byte, 1400)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		length, remote, err := conn.ReadFromUDP(buffer)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			continue
		}
		if response, ok := stunResponse(buffer[:length], remote); ok {
			_, _ = conn.WriteToUDP(response, remote)
			continue
		}
		var probe protocol.ProbePacket
		if err := json.Unmarshal(buffer[:length], &probe); err != nil || probe.Version != protocol.Version || probe.Type != protocol.PacketTypeProbe || probe.EndpointID != endpointID(index) {
			continue
		}
		if _, ok := s.store.AuthorizeProbe(probe.SessionID, probe.Token); !ok {
			continue
		}
		s.replyUDP(index, index, protocol.ResponseKindDirect, probe, remote)
		if probe.RequestAlternate && len(s.config.UDPPorts) > 1 {
			s.replyUDP(index, (index+1)%len(s.config.UDPPorts), protocol.ResponseKindAlternate, probe, remote)
		}
	}
}

func (s *Server) rootOrAssets(assets http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		userAgent := strings.ToLower(request.UserAgent())
		if request.URL.Path == "/" && (strings.HasPrefix(userAgent, "curl/") || strings.HasPrefix(userAgent, "wget/") || strings.Contains(request.Header.Get("Accept"), "text/plain")) {
			base := publicBaseURL(request)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_, _ = fmt.Fprintf(w, "NetProbe Quick Check\n\nPublic IP : %s\nHTTPS     : %t\n\nDeep test\nLinux     : curl -fsSL %s/install.sh | sh\nWindows   : irm %s/install.ps1 | iex\n", clientIP(request), request.TLS != nil || forwardedProto(request) == "https", base, base)
			return
		}
		assets.ServeHTTP(w, request)
	})
}

func (s *Server) replyUDP(receivedAt, respondFrom int, kind string, probe protocol.ProbePacket, remote *net.UDPAddr) {
	proof, err := randomID(18)
	if err != nil {
		return
	}
	reply := protocol.ObservationPacket{
		Version: protocol.Version, Type: protocol.PacketTypeObservation, SessionID: probe.SessionID,
		EndpointID: endpointID(receivedAt), ResponseEndpointID: endpointID(respondFrom), ResponseKind: kind,
		ProbeID: probe.ProbeID, Sequence: probe.Sequence, SentAtUnixNano: probe.SentAtUnixNano,
		ObservedIP: remote.IP.String(), ObservedPort: remote.Port,
		Proof: proof,
	}
	payload, _ := json.Marshal(reply)
	s.mu.RLock()
	responseConn := s.udp[respondFrom]
	_, _ = responseConn.WriteToUDP(payload, remote)
	s.mu.RUnlock()
	s.store.Record(probe.SessionID, protocol.UDPObservation{
		EndpointID: reply.EndpointID, ResponseEndpointID: reply.ResponseEndpointID, ResponseKind: reply.ResponseKind,
		ProbeID: reply.ProbeID, Sequence: reply.Sequence, ObservedIP: reply.ObservedIP, ObservedPort: reply.ObservedPort,
		Proof: reply.Proof,
	})
}

func endpointID(index int) string {
	if index == 0 {
		return "primary"
	}
	return "alternate"
}

func decodeJSON(request *http.Request, target any) error {
	request.Body = http.MaxBytesReader(nil, request.Body, 64<<10)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeProtocolVersionError(w http.ResponseWriter, received int) {
	writeJSON(w, http.StatusBadRequest, protocol.ErrorResponse{Error: protocol.APIError{
		Code:    "unsupported_protocol_version",
		Message: fmt.Sprintf("protocol version %d is unsupported; expected %d", received, protocol.Version),
	}})
}

func bearerToken(request *http.Request) string {
	return strings.TrimSpace(strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer "))
}

func forwardedProto(request *http.Request) string {
	return strings.ToLower(strings.TrimSpace(strings.Split(request.Header.Get("X-Forwarded-Proto"), ",")[0]))
}

func clientIP(request *http.Request) string {
	if forwarded := strings.TrimSpace(request.Header.Get("X-Forwarded-For")); forwarded != "" {
		return strings.TrimSpace(strings.Split(forwarded, ",")[0])
	}
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil {
		return host
	}
	return request.RemoteAddr
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, request)
		s.logger.Info("HTTP request", "method", request.Method, "path", request.URL.Path, "duration", time.Since(started))
	})
}
