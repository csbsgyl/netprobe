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
	if !strings.Contains(stderr.String(), "--timeout must be at least 1s") {
		t.Fatalf("stderr does not contain validation error: %q", stderr.String())
	}
}

func TestRunUsageErrorAsJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := run(context.Background(), []string{"--json", "--timeout", "0s"}, &stdout, &stderr)
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}
	if !strings.Contains(stderr.String(), `"code":"invalid_configuration"`) ||
		!strings.Contains(stderr.String(), `"message":"--timeout must be at least 1s"`) {
		t.Fatalf("stderr is not a structured validation error: %q", stderr.String())
	}
}

func TestRunFlagErrorAsJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	exitCode := run(context.Background(), []string{"--unknown", "--json"}, &stdout, &stderr)
	if exitCode != exitUsage {
		t.Fatalf("exit code = %d, want %d", exitCode, exitUsage)
	}
	if !strings.HasPrefix(stderr.String(), `{"error":`) ||
		!strings.Contains(stderr.String(), `"code":"invalid_configuration"`) ||
		strings.Contains(stderr.String(), "Usage:") {
		t.Fatalf("stderr is not a strict JSON flag error: %q", stderr.String())
	}
}

func TestRunFlagErrorUsesOneRequestedFormat(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantJSON bool
	}{
		{name: "single dash json", args: []string{"--unknown", "-json"}, wantJSON: true},
		{name: "later json false", args: []string{"--json", "--unknown", "--json=false"}, wantJSON: false},
		{name: "invalid duration as json", args: []string{"--timeout", "invalid", "--json"}, wantJSON: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(context.Background(), test.args, &stdout, &stderr); code != exitUsage {
				t.Fatalf("exit code = %d, want %d", code, exitUsage)
			}
			isJSON := strings.HasPrefix(stderr.String(), `{"error":`)
			if isJSON != test.wantJSON {
				t.Fatalf("stderr format JSON = %t, want %t: %q", isJSON, test.wantJSON, stderr.String())
			}
			if test.wantJSON && strings.Contains(stderr.String(), "Usage:") {
				t.Fatalf("JSON error contains text usage: %q", stderr.String())
			}
		})
	}
}
