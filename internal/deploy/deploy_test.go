package deploy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeCommands struct {
	ready bool
	runs  []command
}

func (commands *fakeCommands) Run(_ context.Context, spec command, _, _ io.Writer) error {
	commands.runs = append(commands.runs, spec)
	return nil
}

func (commands *fakeCommands) Output(_ context.Context, spec command) (string, error) {
	if spec.Name == "docker" && len(spec.Args) >= 2 && spec.Args[0] == "compose" && spec.Args[1] == "version" {
		if commands.ready {
			return "Docker Compose version v2.30.0", nil
		}
		return "", errors.New("docker not found")
	}
	if spec.Name == "docker" && len(spec.Args) == 1 && spec.Args[0] == "info" {
		if commands.ready {
			return "ready", nil
		}
		return "", errors.New("daemon unavailable")
	}
	return "", errors.New("unexpected command")
}

func (commands *fakeCommands) LookPath(string) (string, error) {
	return "", errors.New("not found")
}

type staticResolver []netip.Addr

func (resolver staticResolver) Lookup(context.Context, string) ([]netip.Addr, error) {
	return resolver, nil
}

type staticPublicIPs []netip.Addr

func (addresses staticPublicIPs) Discover(context.Context) ([]netip.Addr, error) {
	return addresses, nil
}

type fakeSource struct {
	repository string
	reference  string
}

func (source *fakeSource) Install(_ context.Context, repository, reference, destination string) error {
	source.repository = repository
	source.reference = reference
	return os.MkdirAll(filepath.Join(destination, "deploy"), 0o755)
}

type fakeDocker struct {
	commands *fakeCommands
	called   bool
}

func (docker *fakeDocker) Install(context.Context, io.Writer, io.Writer) error {
	docker.called = true
	docker.commands.ready = true
	return nil
}

type fakeHealth struct {
	endpoint string
	timeout  time.Duration
}

func (health *fakeHealth) Wait(_ context.Context, endpoint string, timeout time.Duration) error {
	health.endpoint = endpoint
	health.timeout = timeout
	return nil
}

func TestInstallerRunOrchestratesDeployment(t *testing.T) {
	commands := &fakeCommands{ready: true}
	source := &fakeSource{}
	health := &fakeHealth{}
	var stdout, stderr bytes.Buffer
	installer := &Installer{
		stdout:    &stdout,
		stderr:    &stderr,
		commands:  commands,
		resolver:  staticResolver{netip.MustParseAddr("203.0.113.8")},
		publicIPs: staticPublicIPs{netip.MustParseAddr("203.0.113.8")},
		sources:   source,
		docker:    &fakeDocker{commands: commands},
		health:    health,
		random:    bytes.NewReader(make([]byte, 32)),
		euid:      func() int { return 0 },
	}
	options := DefaultOptions("dev")
	options.Domain = "CHECK.EXAMPLE.COM"
	options.InstallDir = t.TempDir()
	options.ConfigureFirewall = false
	options.HealthTimeout = 12 * time.Second

	if err := installer.Run(context.Background(), options); err != nil {
		t.Fatalf("Run returned error: %v\nstderr: %s", err, stderr.String())
	}
	if source.repository != "csbsgyl/netprobe" || source.reference != "main" {
		t.Fatalf("unexpected source: %s@%s", source.repository, source.reference)
	}
	environment, err := os.ReadFile(filepath.Join(options.InstallDir, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	for _, expected := range []string{
		"DOMAIN=check.example.com",
		"NETPROBE_IMAGE=netprobe:local",
		"NETPROBE_SECRET=" + strings.Repeat("0", 64),
	} {
		if !strings.Contains(string(environment), expected) {
			t.Errorf(".env does not contain %q:\n%s", expected, environment)
		}
	}
	if health.endpoint != "https://check.example.com/healthz" || health.timeout != 12*time.Second {
		t.Fatalf("unexpected health check: %s, %s", health.endpoint, health.timeout)
	}
	if !containsCommand(commands.runs, "up", "-d", "--build") {
		t.Fatalf("Compose up --build was not run: %+v", commands.runs)
	}
	if !strings.Contains(stdout.String(), "Deployment complete") {
		t.Fatalf("missing completion output: %s", stdout.String())
	}
}

func TestEnsureDockerUsesOfficialInstallerWhenMissing(t *testing.T) {
	commands := &fakeCommands{}
	docker := &fakeDocker{commands: commands}
	installer := &Installer{
		stdout:   io.Discard,
		stderr:   io.Discard,
		commands: commands,
		docker:   docker,
	}
	if err := installer.ensureDocker(context.Background(), true); err != nil {
		t.Fatalf("ensureDocker returned error: %v", err)
	}
	if !docker.called {
		t.Fatal("Docker installer was not called")
	}
}

func TestExistingOrNewSecretPreservesValidSecret(t *testing.T) {
	filename := filepath.Join(t.TempDir(), ".env")
	existing := strings.Repeat("a", 64)
	if err := os.WriteFile(filename, []byte("DOMAIN=old.example.com\nNETPROBE_SECRET="+existing+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	secret, err := existingOrNewSecret(filename, bytes.NewReader(make([]byte, 32)))
	if err != nil {
		t.Fatalf("existingOrNewSecret returned error: %v", err)
	}
	if secret != existing {
		t.Fatalf("secret = %q, want existing secret", secret)
	}
}

func TestWriteEnvironmentIsPrivate(t *testing.T) {
	filename := filepath.Join(t.TempDir(), ".env")
	if err := writeEnvironment(filename, []byte("KEY=value\n")); err != nil {
		t.Fatalf("writeEnvironment returned error: %v", err)
	}
	info, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
}

func containsCommand(commands []command, tail ...string) bool {
	for _, command := range commands {
		if len(command.Args) < len(tail) {
			continue
		}
		start := len(command.Args) - len(tail)
		if strings.Join(command.Args[start:], "\x00") == strings.Join(tail, "\x00") {
			return true
		}
	}
	return false
}
