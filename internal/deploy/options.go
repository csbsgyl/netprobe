package deploy

import (
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultRepository = "csbsgyl/netprobe"
	defaultInstallDir = "/opt/netprobe"
	defaultPrimaryUDP = 3478
	defaultAltUDP     = 3479
)

// Options controls a server deployment.
type Options struct {
	Domain            string
	InstallDir        string
	Repository        string
	Ref               string
	Image             string
	PrimaryUDPPort    int
	AlternateUDPPort  int
	HealthTimeout     time.Duration
	InstallDocker     bool
	ConfigureFirewall bool
	RequireRoot       bool
}

// DefaultOptions returns release-aware defaults. Release binaries use the
// matching source tag and container image; development builds track main.
func DefaultOptions(version string) Options {
	reference := "main"
	image := "netprobe:local"
	if normalized := releaseVersion(version); normalized != "" {
		reference = version
		image = "ghcr.io/" + defaultRepository + ":" + normalized
	}
	return Options{
		InstallDir:        defaultInstallDir,
		Repository:        defaultRepository,
		Ref:               reference,
		Image:             image,
		PrimaryUDPPort:    defaultPrimaryUDP,
		AlternateUDPPort:  defaultAltUDP,
		HealthTimeout:     5 * time.Minute,
		InstallDocker:     true,
		ConfigureFirewall: true,
		RequireRoot:       true,
	}
}

func releaseVersion(version string) string {
	version = strings.TrimSpace(version)
	if version == "" || version == "dev" {
		return ""
	}
	return strings.TrimPrefix(version, "v")
}

// NormalizeDomain accepts a hostname or a bare HTTP(S) URL and returns a
// lowercase DNS name. Paths, ports, IP literals, and malformed labels fail.
func NormalizeDomain(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimSuffix(value, "/")
	if value == "" {
		return "", errors.New("domain is required")
	}
	if strings.ContainsAny(value, "/:#?@[]") || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") {
		return "", errors.New("enter a plain domain without a path, port, or query")
	}
	if net.ParseIP(value) != nil || len(value) > 253 {
		return "", errors.New("domain must be a DNS hostname, not an IP address")
	}
	labels := strings.Split(value, ".")
	if len(labels) < 2 {
		return "", errors.New("domain must contain at least two labels")
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("invalid domain label %q", label)
		}
		for _, char := range label {
			if (char < 'a' || char > 'z') && (char < '0' || char > '9') && char != '-' {
				return "", fmt.Errorf("invalid character %q in domain", char)
			}
		}
	}
	if len(labels[len(labels)-1]) < 2 {
		return "", errors.New("top-level domain must contain at least two characters")
	}
	return value, nil
}

func (options *Options) validate() error {
	domain, err := NormalizeDomain(options.Domain)
	if err != nil {
		return err
	}
	options.Domain = domain
	options.InstallDir = filepath.Clean(strings.TrimSpace(options.InstallDir))
	if !filepath.IsAbs(options.InstallDir) || options.InstallDir == string(filepath.Separator) {
		return errors.New("install directory must be an absolute path other than the filesystem root")
	}
	parts := strings.Split(options.Repository, "/")
	if len(parts) != 2 || !safeIdentifier(parts[0]) || !safeIdentifier(parts[1]) {
		return errors.New("repository must have the form owner/name")
	}
	if !safeReference(options.Ref) {
		return errors.New("source ref is invalid")
	}
	if !safeImage(options.Image) {
		return errors.New("container image is invalid")
	}
	if err := validatePort("primary UDP port", options.PrimaryUDPPort); err != nil {
		return err
	}
	if err := validatePort("alternate UDP port", options.AlternateUDPPort); err != nil {
		return err
	}
	if options.PrimaryUDPPort == options.AlternateUDPPort {
		return errors.New("primary and alternate UDP ports must differ")
	}
	if options.HealthTimeout <= 0 {
		return errors.New("health timeout must be positive")
	}
	return nil
}

func safeReference(value string) bool {
	if value = strings.TrimSpace(value); value == "" {
		return false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
		for _, char := range segment {
			if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') &&
				(char < '0' || char > '9') && !strings.ContainsRune("._-+", char) {
				return false
			}
		}
	}
	return true
}

func safeImage(value string) bool {
	if value = strings.TrimSpace(value); value == "" {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') && !strings.ContainsRune("._/:@-", char) {
			return false
		}
	}
	return true
}

func safeIdentifier(value string) bool {
	if value == "" || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') && !strings.ContainsRune("._-", char) {
			return false
		}
	}
	return true
}

func validatePort(name string, port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("%s must be between 1 and 65535", name)
	}
	return nil
}

func envFile(options Options, secret string) []byte {
	return []byte(strings.Join([]string{
		"DOMAIN=" + options.Domain,
		"NETPROBE_SECRET=" + secret,
		"NETPROBE_IMAGE=" + options.Image,
		"UDP_PORT_PRIMARY=" + strconv.Itoa(options.PrimaryUDPPort),
		"UDP_PORT_ALTERNATE=" + strconv.Itoa(options.AlternateUDPPort),
		"",
	}, "\n"))
}
