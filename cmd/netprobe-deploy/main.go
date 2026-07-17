package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/csbsgyl/netprobe/internal/deploy"
)

var version = "dev"

const (
	exitSuccess = 0
	exitFailure = 1
	exitUsage   = 2
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	input := io.Reader(os.Stdin)
	if terminal, err := os.Open("/dev/tty"); err == nil {
		defer terminal.Close()
		input = terminal
	}
	os.Exit(run(ctx, os.Args[1:], input, os.Stdout, os.Stderr))
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	options, showVersion, err := parseOptions(args, stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitSuccess
		}
		fmt.Fprintf(stderr, "netprobe-deploy: %v\n", err)
		return exitUsage
	}
	if showVersion {
		fmt.Fprintln(stdout, version)
		return exitSuccess
	}
	if strings.TrimSpace(options.Domain) == "" {
		fmt.Fprint(stdout, "Domain for NetProbe (example: check.example.com): ")
		line, readErr := bufio.NewReader(stdin).ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			fmt.Fprintf(stderr, "netprobe-deploy: read domain: %v\n", readErr)
			return exitUsage
		}
		options.Domain = strings.TrimSpace(line)
	}
	if err := deploy.NewInstaller(stdout, stderr).Run(ctx, options); err != nil {
		fmt.Fprintf(stderr, "netprobe-deploy: %v\n", err)
		return exitFailure
	}
	return exitSuccess
}

func parseOptions(args []string, stderr io.Writer) (deploy.Options, bool, error) {
	defaults := deploy.DefaultOptions(version)
	options := defaults
	if err := applyEnvironment(&options); err != nil {
		return options, false, err
	}
	var showVersion bool
	flags := flag.NewFlagSet("netprobe-deploy", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&options.Domain, "domain", options.Domain, "public domain for NetProbe")
	flags.StringVar(&options.InstallDir, "install-dir", options.InstallDir, "installation directory")
	flags.StringVar(&options.Repository, "repo", options.Repository, "GitHub repository (owner/name)")
	flags.StringVar(&options.Ref, "ref", options.Ref, "Git source tag or branch")
	flags.StringVar(&options.Image, "image", options.Image, "NetProbe container image")
	flags.IntVar(&options.PrimaryUDPPort, "udp-primary", options.PrimaryUDPPort, "primary UDP probe port")
	flags.IntVar(&options.AlternateUDPPort, "udp-alternate", options.AlternateUDPPort, "alternate UDP probe port")
	flags.DurationVar(&options.HealthTimeout, "health-timeout", options.HealthTimeout, "maximum HTTPS startup wait")
	flags.BoolVar(&options.InstallDocker, "install-docker", options.InstallDocker, "install Docker when missing")
	flags.BoolVar(&options.ConfigureFirewall, "configure-firewall", options.ConfigureFirewall, "open ports in active UFW")
	flags.BoolVar(&showVersion, "version", false, "print version and exit")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: netprobe-deploy [options]")
		fmt.Fprintln(stderr)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return options, false, err
	}
	if flags.NArg() != 0 {
		flags.Usage()
		return options, false, fmt.Errorf("unexpected positional argument %q", flags.Arg(0))
	}
	if options.Repository != defaults.Repository && options.Image == defaults.Image {
		if release := strings.TrimPrefix(strings.TrimSpace(version), "v"); release != "" && release != "dev" {
			options.Image = "ghcr.io/" + options.Repository + ":" + release
		}
	}
	return options, showVersion, nil
}

func applyEnvironment(options *deploy.Options) error {
	setString(&options.Domain, "DOMAIN")
	setString(&options.InstallDir, "NETPROBE_INSTALL_DIR")
	setString(&options.Repository, "NETPROBE_REPO")
	setString(&options.Ref, "NETPROBE_REF")
	setString(&options.Image, "NETPROBE_IMAGE")
	if err := setInt(&options.PrimaryUDPPort, "UDP_PORT_PRIMARY"); err != nil {
		return err
	}
	if err := setInt(&options.AlternateUDPPort, "UDP_PORT_ALTERNATE"); err != nil {
		return err
	}
	if err := setDuration(&options.HealthTimeout, "NETPROBE_HEALTH_TIMEOUT"); err != nil {
		return err
	}
	if err := setBool(&options.InstallDocker, "NETPROBE_INSTALL_DOCKER"); err != nil {
		return err
	}
	if err := setBool(&options.ConfigureFirewall, "NETPROBE_CONFIGURE_FIREWALL"); err != nil {
		return err
	}
	return nil
}

func setString(destination *string, key string) {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		*destination = value
	}
}

func setInt(destination *int, key string) error {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fmt.Errorf("%s must be an integer: %w", key, err)
	}
	*destination = parsed
	return nil
}

func setDuration(destination *time.Duration, key string) error {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("%s must be a duration: %w", key, err)
	}
	*destination = parsed
	return nil
}

func setBool(destination *bool, key string) error {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fmt.Errorf("%s must be true or false: %w", key, err)
	}
	*destination = parsed
	return nil
}
