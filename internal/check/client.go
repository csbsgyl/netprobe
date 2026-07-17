// Package check implements the HTTP session flow and UDP network probe.
package check

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/csbsgyl/netprobe/internal/protocol"
)

const (
	maxResponseBytes  = 1 << 20
	defaultUDPTimeout = 4 * time.Second
)

// Client runs a complete probe against one netprobe server.
type Client struct {
	baseURL     *url.URL
	httpClient  *http.Client
	clientName  string
	clientVer   string
	udpTimeout  time.Duration
	probeRounds int
}

// Option customizes a Client.
type Option func(*Client)

// WithHTTPClient supplies the HTTP transport. It is primarily useful for
// embedding and tests. Request cancellation is still controlled by context.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		if client != nil {
			c.httpClient = client
		}
	}
}

// WithClientInfo sets the client name and build version reported to the API.
func WithClientInfo(name, version string) Option {
	return func(c *Client) {
		if name != "" {
			c.clientName = name
		}
		if version != "" {
			c.clientVer = version
		}
	}
}

// WithUDPTimeout caps the UDP collection phase, leaving the caller's overall
// context available for the completion request.
func WithUDPTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		if timeout > 0 {
			c.udpTimeout = timeout
		}
	}
}

// WithProbeRounds sets how many datagrams are sent to each endpoint.
func WithProbeRounds(rounds int) Option {
	return func(c *Client) {
		if rounds > 0 {
			c.probeRounds = rounds
		}
	}
}

func NewClient(server string, options ...Option) (*Client, error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return nil, errors.New("server address is empty")
	}
	if !strings.Contains(server, "://") {
		server = "https://" + server
	}
	parsed, err := url.Parse(server)
	if err != nil {
		return nil, fmt.Errorf("parse server address: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("unsupported server scheme %q", parsed.Scheme)
	}
	if parsed.Host == "" {
		return nil, errors.New("server address has no host")
	}
	if parsed.User != nil {
		return nil, errors.New("server address must not contain credentials")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, errors.New("server address must not contain a query or fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")

	client := &Client{
		baseURL:     parsed,
		httpClient:  http.DefaultClient,
		clientName:  "netcheck",
		clientVer:   "dev",
		udpTimeout:  defaultUDPTimeout,
		probeRounds: 3,
	}
	for _, option := range options {
		if option != nil {
			option(client)
		}
	}
	return client, nil
}

// Run creates a session, performs UDP probes, and asks the server for the
// authoritative verdict.
func (c *Client) Run(ctx context.Context) (protocol.CompleteSessionResponse, error) {
	session, err := c.CreateSession(ctx)
	if err != nil {
		return protocol.CompleteSessionResponse{}, err
	}

	udpTimeout := c.udpTimeoutFor(ctx)
	udpCtx, cancel := context.WithTimeout(ctx, udpTimeout)
	report := ProbeUDP(udpCtx, session, c.probeRounds)
	cancel()

	result, err := c.CompleteSession(ctx, session, report)
	if err != nil {
		return protocol.CompleteSessionResponse{}, err
	}
	return result, nil
}

func (c *Client) CreateSession(ctx context.Context) (protocol.CreateSessionResponse, error) {
	request := protocol.CreateSessionRequest{
		Version: protocol.Version,
		Client: protocol.ClientInfo{
			Name:    c.clientName,
			Version: c.clientVer,
			OS:      runtime.GOOS,
			Arch:    runtime.GOARCH,
		},
	}
	var response protocol.CreateSessionResponse
	if err := c.postJSON(ctx, protocol.CreateSessionPath, "", request, &response); err != nil {
		return response, fmt.Errorf("create session: %w", err)
	}
	if err := response.Validate(); err != nil {
		return response, fmt.Errorf("create session: invalid response: %w", err)
	}
	return response, nil
}

func (c *Client) CompleteSession(ctx context.Context, session protocol.CreateSessionResponse, report protocol.UDPReport) (protocol.CompleteSessionResponse, error) {
	request := protocol.CompleteSessionRequest{Version: protocol.Version, UDP: report}
	var response protocol.CompleteSessionResponse
	endpoint := protocol.CompleteSessionPath(session.SessionID)
	if err := c.postJSON(ctx, endpoint, session.Token, request, &response); err != nil {
		return response, fmt.Errorf("complete session: %w", err)
	}
	if err := response.Validate(session.SessionID); err != nil {
		return response, fmt.Errorf("complete session: invalid response: %w", err)
	}
	return response, nil
}

func (c *Client) postJSON(ctx context.Context, endpoint, token string, input, output any) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	requestURL := *c.baseURL
	requestURL.Path = strings.TrimRight(requestURL.Path, "/") + endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, requestURL.String(), bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.clientName+"/"+c.clientVer)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if len(body) > maxResponseBytes {
		return errors.New("response exceeds 1 MiB")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiError protocol.ErrorResponse
		if json.Unmarshal(body, &apiError) == nil && apiError.Error.Message != "" {
			if apiError.Error.Code != "" {
				return fmt.Errorf("server returned %s (%s): %s", resp.Status, apiError.Error.Code, apiError.Error.Message)
			}
			return fmt.Errorf("server returned %s: %s", resp.Status, apiError.Error.Message)
		}
		return fmt.Errorf("server returned %s", resp.Status)
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return errors.New("server returned an empty response")
	}
	if err := json.Unmarshal(body, output); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}

func (c *Client) udpTimeoutFor(ctx context.Context) time.Duration {
	timeout := c.udpTimeout
	deadline, ok := ctx.Deadline()
	if !ok {
		return timeout
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		return time.Nanosecond
	}
	// Keep at least 40 percent of the remaining time for the final HTTP call.
	share := remaining * 3 / 5
	if share < timeout {
		timeout = share
	}
	if timeout <= 0 {
		return time.Nanosecond
	}
	return timeout
}
