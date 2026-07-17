package server

import "testing"

func TestParsePorts(t *testing.T) {
	ports, err := parsePorts("3478,3479")
	if err != nil || len(ports) != 2 || ports[0] != 3478 || ports[1] != 3479 {
		t.Fatalf("parsePorts returned %v, %v", ports, err)
	}
	for _, invalid := range []string{"3478", "3478,3478", "0,3479", "x,3479"} {
		if _, err := parsePorts(invalid); err == nil {
			t.Fatalf("parsePorts(%q) unexpectedly succeeded", invalid)
		}
	}
}
