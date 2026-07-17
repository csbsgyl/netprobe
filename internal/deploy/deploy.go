package deploy

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Installer performs a complete NetProbe server deployment. Its dependencies
// are replaceable inside this package's tests; callers should use NewInstaller.
type Installer struct {
	stdout    io.Writer
	stderr    io.Writer
	commands  commandRunner
	resolver  addressResolver
	publicIPs publicIPDiscoverer
	sources   sourceInstaller
	docker    dockerInstaller
	health    healthChecker
	random    io.Reader
	euid      func() int
}

// NewInstaller creates an installer backed by the operating system and public
// NetProbe/Docker endpoints.
func NewInstaller(stdout, stderr io.Writer) *Installer {
	commands := osCommandRunner{}
	return &Installer{
		stdout:   stdout,
		stderr:   stderr,
		commands: commands,
		resolver: dnsResolver{},
		publicIPs: httpPublicIPDiscoverer{
			IPv4URL: "https://api4.ipify.org",
			IPv6URL: "https://api6.ipify.org",
			Timeout: 8 * time.Second,
		},
		sources: githubSourceInstaller{},
		docker: officialDockerInstaller{
			Client:   &http.Client{Timeout: time.Minute},
			Commands: commands,
		},
		health: httpHealthChecker{},
		random: rand.Reader,
		euid:   os.Geteuid,
	}
}

// Run validates the host and DNS, installs prerequisites when allowed, writes
// the deployment configuration, starts Compose, and waits for HTTPS.
func (installer *Installer) Run(ctx context.Context, options Options) error {
	if installer.stdout == nil {
		installer.stdout = io.Discard
	}
	if installer.stderr == nil {
		installer.stderr = io.Discard
	}
	if options.RequireRoot && installer.euid() != 0 {
		return errors.New("run netprobe-deploy as root")
	}
	if err := options.validate(); err != nil {
		return fmt.Errorf("invalid deployment configuration: %w", err)
	}

	installer.info("Checking public IP and DNS for %s...", options.Domain)
	serverAddresses, err := installer.publicIPs.Discover(ctx)
	if err != nil {
		return err
	}
	domainAddresses, err := installer.resolver.Lookup(ctx, options.Domain)
	if err != nil {
		return err
	}
	if err := verifyDNS(options.Domain, serverAddresses, domainAddresses); err != nil {
		return fmt.Errorf("%w. Create an A or AAAA record pointing directly to this server, wait for propagation, then rerun", err)
	}
	installer.info("DNS matches this server (%s).", matchingAddress(serverAddresses, domainAddresses))

	if err := installer.ensureDocker(ctx, options.InstallDocker); err != nil {
		return err
	}
	if options.ConfigureFirewall {
		if err := installer.configureUFW(ctx, options); err != nil {
			return err
		}
	}

	installer.info("Downloading NetProbe source %s@%s...", options.Repository, options.Ref)
	if err := installer.sources.Install(ctx, options.Repository, options.Ref, options.InstallDir); err != nil {
		return err
	}
	secret, err := existingOrNewSecret(filepath.Join(options.InstallDir, ".env"), installer.random)
	if err != nil {
		return err
	}
	if err := writeEnvironment(filepath.Join(options.InstallDir, ".env"), envFile(options, secret)); err != nil {
		return err
	}

	build, err := installer.prepareImages(ctx, options)
	if err != nil {
		return err
	}
	installer.info("Starting NetProbe with Docker Compose...")
	upArgs := []string{"up", "-d"}
	if build {
		upArgs = append(upArgs, "--build")
	} else {
		upArgs = append(upArgs, "--no-build")
	}
	if err := installer.runCompose(ctx, options, upArgs...); err != nil {
		return fmt.Errorf("start NetProbe: %w", err)
	}

	endpoint := "https://" + options.Domain + "/healthz"
	installer.info("Waiting for Caddy to obtain a certificate and for HTTPS to become healthy...")
	if err := installer.health.Wait(ctx, endpoint, options.HealthTimeout); err != nil {
		_, _ = fmt.Fprintln(installer.stderr, "\nRecent container logs:")
		_ = installer.runCompose(ctx, options, "logs", "--tail=80", "caddy", "netprobe")
		return fmt.Errorf("%w; verify that the cloud firewall allows 80/tcp, 443/tcp, %d/udp, and %d/udp", err, options.PrimaryUDPPort, options.AlternateUDPPort)
	}

	installer.info("Deployment complete: https://%s", options.Domain)
	installer.info("Linux deep test: curl -fsSL https://%s/install.sh | sh", options.Domain)
	installer.info("Windows deep test: irm https://%s/install.ps1 | iex", options.Domain)
	return nil
}

func (installer *Installer) ensureDocker(ctx context.Context, allowInstall bool) error {
	initialErr := dockerReady(ctx, installer.commands)
	if initialErr == nil {
		installer.info("Docker Engine and Compose v2 are ready.")
		return nil
	}
	if _, err := installer.commands.LookPath("docker"); err == nil {
		if _, err := installer.commands.LookPath("systemctl"); err == nil {
			_ = installer.commands.Run(ctx, command{
				Name: "systemctl",
				Args: []string{"enable", "--now", "docker"},
			}, installer.stdout, installer.stderr)
			if err := dockerReady(ctx, installer.commands); err == nil {
				installer.info("Docker Engine and Compose v2 are ready.")
				return nil
			}
		}
	}
	if !allowInstall {
		return fmt.Errorf("%w; install Docker Engine with the Compose v2 plugin, then rerun", initialErr)
	}
	installer.info("Installing Docker Engine with Docker's official installer...")
	if err := installer.docker.Install(ctx, installer.stdout, installer.stderr); err != nil {
		return err
	}
	if _, err := installer.commands.LookPath("systemctl"); err == nil {
		if err := installer.commands.Run(ctx, command{
			Name: "systemctl",
			Args: []string{"enable", "--now", "docker"},
		}, installer.stdout, installer.stderr); err != nil {
			installer.warn("Could not enable Docker with systemd: %v", err)
		}
	}
	if err := dockerReady(ctx, installer.commands); err != nil {
		return fmt.Errorf("Docker installation finished but validation failed: %w", err)
	}
	return nil
}

func (installer *Installer) configureUFW(ctx context.Context, options Options) error {
	if _, err := installer.commands.LookPath("ufw"); err != nil {
		return nil
	}
	output, err := installer.commands.Output(ctx, command{Name: "ufw", Args: []string{"status"}})
	if err != nil || !strings.Contains(output, "Status: active") {
		return nil
	}
	installer.info("Opening required ports in UFW...")
	rules := []string{
		"80/tcp",
		"443/tcp",
		"443/udp",
		fmt.Sprintf("%d/udp", options.PrimaryUDPPort),
		fmt.Sprintf("%d/udp", options.AlternateUDPPort),
	}
	for _, rule := range rules {
		if err := installer.commands.Run(ctx, command{
			Name: "ufw",
			Args: []string{"allow", rule},
		}, installer.stdout, installer.stderr); err != nil {
			return fmt.Errorf("open UFW port %s: %w", rule, err)
		}
	}
	return nil
}

func (installer *Installer) prepareImages(ctx context.Context, options Options) (bool, error) {
	if err := installer.runCompose(ctx, options, "pull", "caddy"); err != nil {
		installer.warn("Could not pre-pull Caddy; Compose will retry: %v", err)
	}
	if options.Image == "netprobe:local" {
		return true, nil
	}
	installer.info("Pulling NetProbe image %s...", options.Image)
	if err := installer.runCompose(ctx, options, "pull", "netprobe"); err != nil {
		installer.warn("Could not pull the release image; building the matching source locally instead: %v", err)
		return true, nil
	}
	return false, nil
}

func (installer *Installer) runCompose(ctx context.Context, options Options, args ...string) error {
	composeArgs := []string{
		"compose",
		"--project-directory", options.InstallDir,
		"--env-file", filepath.Join(options.InstallDir, ".env"),
		"-f", filepath.Join(options.InstallDir, "deploy", "compose.yaml"),
	}
	composeArgs = append(composeArgs, args...)
	return installer.commands.Run(ctx, command{
		Name: "docker",
		Args: composeArgs,
		Dir:  options.InstallDir,
	}, installer.stdout, installer.stderr)
}

func existingOrNewSecret(filename string, random io.Reader) (string, error) {
	if file, err := os.Open(filename); err == nil {
		scanner := bufio.NewScanner(io.LimitReader(file, 64<<10))
		for scanner.Scan() {
			key, value, ok := strings.Cut(scanner.Text(), "=")
			if ok && key == "NETPROBE_SECRET" {
				value = strings.TrimSpace(value)
				decoded, decodeErr := hex.DecodeString(value)
				if decodeErr == nil && len(decoded) == 32 {
					_ = file.Close()
					return value, nil
				}
			}
		}
		scanErr := scanner.Err()
		closeErr := file.Close()
		if err := errors.Join(scanErr, closeErr); err != nil {
			return "", fmt.Errorf("read existing environment: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read existing environment: %w", err)
	}
	bytes := make([]byte, 32)
	if _, err := io.ReadFull(random, bytes); err != nil {
		return "", fmt.Errorf("generate session secret: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func writeEnvironment(filename string, contents []byte) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
		return fmt.Errorf("create install directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(filename), ".env-*")
	if err != nil {
		return fmt.Errorf("create environment file: %w", err)
	}
	tempName := file.Name()
	defer os.Remove(tempName)
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return fmt.Errorf("secure environment file: %w", err)
	}
	if _, err := file.Write(contents); err != nil {
		_ = file.Close()
		return fmt.Errorf("write environment file: %w", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return fmt.Errorf("sync environment file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close environment file: %w", err)
	}
	if err := os.Rename(tempName, filename); err != nil {
		return fmt.Errorf("install environment file: %w", err)
	}
	return nil
}

func matchingAddress(serverAddresses, domainAddresses []netip.Addr) string {
	serverSet := make(map[netip.Addr]struct{}, len(serverAddresses))
	for _, address := range serverAddresses {
		serverSet[address.Unmap()] = struct{}{}
	}
	for _, address := range domainAddresses {
		if _, ok := serverSet[address.Unmap()]; ok {
			return address.Unmap().String()
		}
	}
	return "unknown"
}

func (installer *Installer) info(format string, values ...any) {
	_, _ = fmt.Fprintf(installer.stdout, "[NetProbe] "+format+"\n", values...)
}

func (installer *Installer) warn(format string, values ...any) {
	_, _ = fmt.Fprintf(installer.stderr, "[NetProbe] Warning: "+format+"\n", values...)
}
