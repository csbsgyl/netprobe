package deploy

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultOptionsFollowReleaseVersion(t *testing.T) {
	release := DefaultOptions("v1.2.3")
	if release.Ref != "v1.2.3" || release.Image != "ghcr.io/csbsgyl/netprobe:1.2.3" {
		t.Fatalf("unexpected release defaults: %+v", release)
	}
	development := DefaultOptions("dev")
	if development.Ref != "main" || development.Image != "netprobe:local" {
		t.Fatalf("unexpected development defaults: %+v", development)
	}
}

func TestNormalizeDomain(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "Check.Example.COM", want: "check.example.com"},
		{input: "https://check.example.com/", want: "check.example.com"},
		{input: "a-b.example.com", want: "a-b.example.com"},
	}
	for _, test := range tests {
		got, err := NormalizeDomain(test.input)
		if err != nil {
			t.Errorf("NormalizeDomain(%q) returned error: %v", test.input, err)
		} else if got != test.want {
			t.Errorf("NormalizeDomain(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestNormalizeDomainRejectsUnsafeValues(t *testing.T) {
	for _, value := range []string{
		"",
		"localhost",
		"203.0.113.1",
		"example.com:443",
		"example.com/path",
		"-bad.example",
		"bad_.example",
	} {
		if _, err := NormalizeDomain(value); err == nil {
			t.Errorf("NormalizeDomain(%q) unexpectedly succeeded", value)
		}
	}
}

func TestOptionsValidatePortsAndImage(t *testing.T) {
	options := DefaultOptions("dev")
	options.Domain = "check.example.com"
	options.HealthTimeout = time.Second
	options.AlternateUDPPort = options.PrimaryUDPPort
	if err := options.validate(); err == nil || !strings.Contains(err.Error(), "must differ") {
		t.Fatalf("validate error = %v, want different-port error", err)
	}
	options.AlternateUDPPort = 3479
	options.Image = "bad image"
	if err := options.validate(); err == nil || !strings.Contains(err.Error(), "container image") {
		t.Fatalf("validate error = %v, want image error", err)
	}
}

func TestOptionsRejectsDangerousInstallDirectory(t *testing.T) {
	for _, directory := range []string{"relative/path", "/"} {
		options := DefaultOptions("dev")
		options.Domain = "check.example.com"
		options.InstallDir = directory
		if err := options.validate(); err == nil || !strings.Contains(err.Error(), "install directory") {
			t.Errorf("validate(%q) error = %v, want install-directory error", directory, err)
		}
	}
}
