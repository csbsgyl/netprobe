package server

import (
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
