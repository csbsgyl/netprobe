package server

import (
	"encoding/binary"
	"net"
	"testing"
)

func TestSTUNBindingIPv4(t *testing.T) {
	request := make([]byte, stunHeaderLength)
	binary.BigEndian.PutUint16(request[0:2], stunBindingRequest)
	binary.BigEndian.PutUint32(request[4:8], stunMagicCookie)
	copy(request[8:], []byte("transaction!"))
	response, ok := stunResponse(request, &net.UDPAddr{IP: net.ParseIP("203.0.113.9"), Port: 45678})
	if !ok || len(response) != 32 || binary.BigEndian.Uint16(response[0:2]) != stunBindingSuccess {
		t.Fatalf("invalid response: ok=%v bytes=%x", ok, response)
	}
	port := binary.BigEndian.Uint16(response[26:28]) ^ uint16(stunMagicCookie>>16)
	if port != 45678 {
		t.Fatalf("decoded port = %d", port)
	}
}

func TestSTUNRejectsInvalidRequest(t *testing.T) {
	if _, ok := stunResponse([]byte("not stun"), &net.UDPAddr{}); ok {
		t.Fatal("invalid request accepted")
	}
}
