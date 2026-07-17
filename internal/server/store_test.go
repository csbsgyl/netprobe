package server

import (
	"net"
	"testing"
	"time"

	"github.com/csbsgyl/netprobe/internal/protocol"
)

func TestProbeAuthorizationBindsIPAndLimitsRequests(t *testing.T) {
	store := NewSessionStore("secret", time.Minute)
	session, err := store.Create("203.0.113.9", protocol.ClientInfo{Name: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := store.AuthorizeProbe(session.ID, session.Token, net.ParseIP("203.0.113.10")); ok {
		t.Fatal("probe from wrong IP was authorized")
	}
	for i := 0; i < 32; i++ {
		if _, ok := store.AuthorizeProbe(session.ID, session.Token, net.ParseIP("203.0.113.9")); !ok {
			t.Fatalf("probe %d was rejected", i)
		}
	}
	if _, ok := store.AuthorizeProbe(session.ID, session.Token, net.ParseIP("203.0.113.9")); ok {
		t.Fatal("probe limit was not enforced")
	}
}
