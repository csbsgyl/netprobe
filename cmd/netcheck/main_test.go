package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/csbsgyl/netprobe/internal/protocol"
)

func TestParseOptions(t *testing.T) {
	var stderr bytes.Buffer
	opts, err := parseOptions([]string{"--server", "http://127.0.0.1", "--json", "--timeout", "3s"}, &stderr)
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if opts.server != "http://127.0.0.1" || !opts.json || opts.timeout != 3*time.Second {
		t.Fatalf("unexpected options: %+v", opts)
	}
}

func TestParseOptionsRejectsShortTimeout(t *testing.T) {
	var stderr bytes.Buffer
	if _, err := parseOptions([]string{"--timeout", "999ms"}, &stderr); err == nil {
		t.Fatal("expected short timeout error")
	}
}

func TestWriteHuman(t *testing.T) {
	result := protocol.CompleteSessionResponse{
		SessionID:              "report-1",
		Verdict:                protocol.VerdictPass,
		PublicIP:               "203.0.113.1",
		PublicPort:             12345,
		UDPReachable:           true,
		AlternatePortReachable: true,
		MappingBehavior:        "endpoint-independent",
		Checks: []protocol.CheckResult{
			{Name: "udp", Status: protocol.CheckPass, Detail: "reachable"},
		},
	}
	var output bytes.Buffer
	writeHuman(&output, result)
	for _, expected := range []string{"Status:", "PASS", "203.0.113.1:12345", "[PASS] udp: reachable", "report-1"} {
		if !strings.Contains(output.String(), expected) {
			t.Errorf("output does not contain %q:\n%s", expected, output.String())
		}
	}
}

func TestRunUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := run(context.Background(), []string{"--timeout", "0s"}, &stdout, &stderr)
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}
}
