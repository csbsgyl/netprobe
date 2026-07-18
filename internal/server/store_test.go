package server

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/csbsgyl/netprobe/internal/protocol"
)

func TestProbeAuthorizationChecksTokenAndLimitsRequests(t *testing.T) {
	store := NewSessionStore("secret", time.Minute)
	session, err := store.Create("203.0.113.9", protocol.ClientInfo{Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.AuthorizeProbe(session.ID, "wrong-token"); ok {
		t.Fatal("probe with wrong token was authorized")
	}
	for i := 0; i < 32; i++ {
		if _, ok := store.AuthorizeProbe(session.ID, session.Token); !ok {
			t.Fatalf("probe %d was rejected", i)
		}
	}
	if _, ok := store.AuthorizeProbe(session.ID, session.Token); ok {
		t.Fatal("probe limit was not enforced")
	}
}

func TestSessionStorePrunesExpiredSessionsAtCapacity(t *testing.T) {
	store := NewSessionStore("secret", time.Minute)
	now := time.Now().UTC()
	for index := 0; index < maxSessions; index++ {
		id := fmt.Sprintf("session-%d", index)
		store.sessions[id] = &Session{ID: id, ExpiresAt: now.Add(time.Minute)}
	}
	if _, err := store.Create("203.0.113.9", protocol.ClientInfo{Name: "test"}); err == nil {
		t.Fatal("Create succeeded while the active session limit was full")
	}

	store.sessions["session-0"].ExpiresAt = now.Add(-time.Second)
	created, err := store.Create("203.0.113.9", protocol.ClientInfo{Name: "test"})
	if err != nil {
		t.Fatalf("Create after expiry returned error: %v", err)
	}
	if created == nil || len(store.sessions) != maxSessions {
		t.Fatalf("session count = %d, created = %+v", len(store.sessions), created)
	}
	if _, ok := store.sessions["session-0"]; ok {
		t.Fatal("expired session was not removed")
	}
}

func TestSessionStoreCapsRecordedObservations(t *testing.T) {
	store := NewSessionStore("secret", time.Minute)
	session, err := store.Create("203.0.113.9", protocol.ClientInfo{Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < maxSessionObservations+10; index++ {
		store.Record(session.ID, protocol.UDPObservation{Sequence: uint64(index)})
	}
	stored := store.sessions[session.ID]
	if len(stored.ServerProbes) != maxSessionObservations {
		t.Fatalf("recorded observations = %d, want %d", len(stored.ServerProbes), maxSessionObservations)
	}
}

func TestSessionStoreValidCanSnapshotWhileRecording(t *testing.T) {
	store := NewSessionStore("secret", time.Minute)
	session, err := store.Create("203.0.113.9", protocol.ClientInfo{Name: "test"})
	if err != nil {
		t.Fatal(err)
	}

	var group sync.WaitGroup
	group.Add(2)
	go func() {
		defer group.Done()
		for index := 0; index < 500; index++ {
			store.Record(session.ID, protocol.UDPObservation{Sequence: uint64(index)})
		}
	}()
	go func() {
		defer group.Done()
		for index := 0; index < 500; index++ {
			if _, ok := store.Valid(session.ID, session.Token); !ok {
				t.Errorf("snapshot %d was rejected", index)
				return
			}
		}
	}()
	group.Wait()
}
