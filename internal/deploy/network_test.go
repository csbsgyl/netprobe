package deploy

import (
	"net/netip"
	"strings"
	"testing"
)

func TestVerifyDNSAcceptsEitherAddressFamily(t *testing.T) {
	server := []netip.Addr{
		netip.MustParseAddr("203.0.113.10"),
		netip.MustParseAddr("2001:db8::10"),
	}
	domain := []netip.Addr{
		netip.MustParseAddr("198.51.100.20"),
		netip.MustParseAddr("2001:db8::10"),
	}
	if err := verifyDNS("check.example.com", server, domain); err != nil {
		t.Fatalf("verifyDNS returned error: %v", err)
	}
}

func TestVerifyDNSReportsBothSidesOnMismatch(t *testing.T) {
	err := verifyDNS(
		"check.example.com",
		[]netip.Addr{netip.MustParseAddr("203.0.113.10")},
		[]netip.Addr{netip.MustParseAddr("198.51.100.20")},
	)
	if err == nil {
		t.Fatal("verifyDNS unexpectedly succeeded")
	}
	for _, expected := range []string{"check.example.com", "203.0.113.10", "198.51.100.20"} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("error %q does not contain %q", err, expected)
		}
	}
}

func TestUniqueAddressesUnmapsAndSorts(t *testing.T) {
	addresses := uniqueAddresses([]netip.Addr{
		netip.MustParseAddr("2001:db8::2"),
		netip.MustParseAddr("::ffff:192.0.2.1"),
		netip.MustParseAddr("192.0.2.1"),
		netip.Addr{},
	})
	if len(addresses) != 2 || addresses[0].String() != "192.0.2.1" || addresses[1].String() != "2001:db8::2" {
		t.Fatalf("unexpected addresses: %v", addresses)
	}
}
