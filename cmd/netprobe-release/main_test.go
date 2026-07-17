package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildEnvironmentReplacesTargetVariables(t *testing.T) {
	environment := buildEnvironment([]string{
		"PATH=/bin",
		"CGO_ENABLED=1",
		"GOOS=darwin",
		"GOARCH=arm64",
	}, "linux", "amd64")
	joined := strings.Join(environment, "\n")
	for _, expected := range []string{"PATH=/bin", "CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64"} {
		if !strings.Contains(joined, expected) {
			t.Errorf("environment does not contain %q: %v", expected, environment)
		}
	}
	if strings.Contains(joined, "GOOS=darwin") || strings.Contains(joined, "CGO_ENABLED=1") {
		t.Fatalf("old target variables remain: %v", environment)
	}
}

func TestWriteChecksumUsesAssetBaseName(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "netprobe-deploy-linux-amd64")
	if err := os.WriteFile(filename, []byte("netprobe"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeChecksum(filename); err != nil {
		t.Fatalf("writeChecksum returned error: %v", err)
	}
	contents, err := os.ReadFile(filename + ".sha256")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(contents), "  netprobe-deploy-linux-amd64\n") {
		t.Fatalf("unexpected checksum file: %q", contents)
	}
}

func TestParseOptionsUsesGitHubRef(t *testing.T) {
	t.Setenv("VERSION", "")
	t.Setenv("GITHUB_REF_NAME", "v1.0.0")
	t.Setenv("OUT", "")
	t.Setenv("GO", "")
	options, err := parseOptions(nil, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseOptions returned error: %v", err)
	}
	if options.version != "v1.0.0" || options.output != "dist" || options.goBin != "go" {
		t.Fatalf("unexpected options: %+v", options)
	}
}
