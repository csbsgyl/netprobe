package server

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/csbsgyl/netprobe/internal/protocol"
)

type Session struct {
	ID           string
	Token        string
	PublicIP     string
	Client       protocol.ClientInfo
	CreatedAt    time.Time
	ExpiresAt    time.Time
	ServerProbes []protocol.UDPObservation
	ProbeCount   int
}

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	secret   []byte
	ttl      time.Duration
}

func NewSessionStore(secret string, ttl time.Duration) *SessionStore {
	return &SessionStore{sessions: make(map[string]*Session), secret: []byte(secret), ttl: ttl}
}

func (s *SessionStore) Create(publicIP string, client protocol.ClientInfo) (*Session, error) {
	id, err := randomID(18)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	session := &Session{ID: id, PublicIP: publicIP, Client: client, CreatedAt: now, ExpiresAt: now.Add(s.ttl)}
	session.Token = s.sign(session.ID, session.ExpiresAt)
	s.mu.Lock()
	s.sessions[id] = session
	s.mu.Unlock()
	return cloneSession(session), nil
}

func (s *SessionStore) Valid(id, token string) (*Session, bool) {
	s.mu.RLock()
	session, ok := s.sessions[id]
	s.mu.RUnlock()
	if !ok || time.Now().After(session.ExpiresAt) || !hmac.Equal([]byte(token), []byte(s.sign(id, session.ExpiresAt))) {
		return nil, false
	}
	return cloneSession(session), true
}

func (s *SessionStore) AuthorizeProbe(id, token string, remoteIP net.IP) (*Session, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[id]
	if !ok || time.Now().After(session.ExpiresAt) || session.ProbeCount >= 32 || !hmac.Equal([]byte(token), []byte(s.sign(id, session.ExpiresAt))) {
		return nil, false
	}
	expected := net.ParseIP(session.PublicIP)
	if expected == nil || !expected.Equal(remoteIP) {
		return nil, false
	}
	session.ProbeCount++
	return cloneSession(session), true
}

func (s *SessionStore) Record(id string, observation protocol.UDPObservation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session, ok := s.sessions[id]; ok && time.Now().Before(session.ExpiresAt) {
		session.ServerProbes = append(session.ServerProbes, observation)
	}
}

func (s *SessionStore) sign(id string, expires time.Time) string {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = fmt.Fprintf(mac, "%s:%d", id, expires.Unix())
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func randomID(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func cloneSession(session *Session) *Session {
	copy := *session
	copy.ServerProbes = append([]protocol.UDPObservation(nil), session.ServerProbes...)
	return &copy
}
