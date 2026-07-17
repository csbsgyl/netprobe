package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestParseOptionsUsesReleaseAwareDefaults(t *testing.T) {
	clearDeploymentEnvironment(t)
	previousVersion := version
	version = "v2.3.4"
	t.Cleanup(func() { version = previousVersion })

	options, showVersion, err := parseOptions([]string{
		"--domain", "check.example.com",
		"--repo", "acme/netprobe",
	}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if showVersion {
		t.Fatal("showVersion is true")
	}
	if options.Ref != "v2.3.4" || options.Image != "ghcr.io/acme/netprobe:2.3.4" {
		t.Fatalf("unexpected options: %+v", options)
	}
}

func TestParseOptionsReadsEnvironment(t *testing.T) {
	clearDeploymentEnvironment(t)
	t.Setenv("DOMAIN", "env.example.com")
	t.Setenv("UDP_PORT_PRIMARY", "4000")
	t.Setenv("NETPROBE_INSTALL_DOCKER", "false")

	options, _, err := parseOptions(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if options.Domain != "env.example.com" || options.PrimaryUDPPort != 4000 || options.InstallDocker {
		t.Fatalf("unexpected options: %+v", options)
	}
}

func TestRunVersionDoesNotStartDeployment(t *testing.T) {
	previousVersion := version
	version = "v9.9.9"
	t.Cleanup(func() { version = previousVersion })
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"--version"}, strings.NewReader(""), &stdout, &stderr)
	if code != exitSuccess || strings.TrimSpace(stdout.String()) != "v9.9.9" || stderr.Len() != 0 {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestParseOptionsRejectsInvalidBooleanEnvironment(t *testing.T) {
	clearDeploymentEnvironment(t)
	t.Setenv("NETPROBE_INSTALL_DOCKER", "sometimes")
	_, _, err := parseOptions(nil, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "NETPROBE_INSTALL_DOCKER") {
		t.Fatalf("parseOptions error = %v", err)
	}
}

func clearDeploymentEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DOMAIN",
		"NETPROBE_INSTALL_DIR",
		"NETPROBE_REPO",
		"NETPROBE_REF",
		"NETPROBE_IMAGE",
		"UDP_PORT_PRIMARY",
		"UDP_PORT_ALTERNATE",
		"NETPROBE_HEALTH_TIMEOUT",
		"NETPROBE_INSTALL_DOCKER",
		"NETPROBE_CONFIGURE_FIREWALL",
	} {
		t.Setenv(key, "")
	}
}
