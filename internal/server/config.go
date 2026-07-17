package server

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr      string
	PublicHost    string
	DownloadDir   string
	UDPBindHost   string
	UDPPorts      []int
	SessionTTL    time.Duration
	SessionSecret string
}

func ConfigFromEnv() (Config, error) {
	ports, err := parsePorts(env("NETPROBE_UDP_PORTS", "3478,3479"))
	if err != nil {
		return Config{}, err
	}
	secret := strings.TrimSpace(os.Getenv("NETPROBE_SECRET"))
	if secret == "" {
		return Config{}, fmt.Errorf("NETPROBE_SECRET must be set")
	}
	return Config{
		HTTPAddr:      env("NETPROBE_HTTP_ADDR", ":8080"),
		PublicHost:    env("NETPROBE_PUBLIC_HOST", "127.0.0.1"),
		DownloadDir:   env("NETPROBE_DOWNLOAD_DIR", "/srv/downloads"),
		UDPBindHost:   env("NETPROBE_UDP_BIND_HOST", "0.0.0.0"),
		UDPPorts:      ports,
		SessionTTL:    durationEnv("NETPROBE_SESSION_TTL", 5*time.Minute),
		SessionSecret: secret,
	}, nil
}

func parsePorts(value string) ([]int, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return nil, fmt.Errorf("NETPROBE_UDP_PORTS must contain exactly two ports")
	}
	ports := make([]int, 0, 2)
	for _, part := range parts {
		port, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid UDP port %q", part)
		}
		ports = append(ports, port)
	}
	if ports[0] == ports[1] {
		return nil, fmt.Errorf("UDP probe ports must differ")
	}
	return ports, nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	if value, err := time.ParseDuration(strings.TrimSpace(os.Getenv(key))); err == nil && value > 0 {
		return value
	}
	return fallback
}
